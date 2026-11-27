use tauri::Manager;
mod agent_sandbox_run;
mod desktop_updates;
mod docker;
mod factory_config;
mod factory_process;
mod factory_profiles;
mod factory_run;
mod git;
mod stage_log;

use agent_sandbox_run::{AgentSandboxRun, Requirement, Worktree, WorktreeDeleteError};
use desktop_updates::{DesktopUpdates, check_desktop_update, install_desktop_update};
use docker::{Docker, Image};
use factory_config::FactoryConfigFile;
use factory_process::{RunOutcome, RunProcesses};
use factory_profiles::{ProfileLaunch, ProfileStore, ProfileSummary};
use factory_run::{FactoryRun, FactoryRunMode, ScheduledTask};
use std::fs;
use std::path::Path;
// Learn more about Tauri commands at https://tauri.app/develop/calling-rust/
#[tauri::command]
fn greet(name: &str) -> String {
    format!("Hello, {}! You've been greeted from Rust!", name)
}

#[derive(serde::Serialize)]
#[serde(rename_all = "camelCase")]
struct FolderInfo {
    is_git: bool,
    has_config: bool,
}

#[derive(serde::Serialize, serde::Deserialize)]
struct SavedRepo {
    name: String,
    path: String,
}

#[tauri::command]
fn inspect_folder(path: String) -> FolderInfo {
    let root = std::path::Path::new(&path)
        .ancestors()
        .find(|dir| dir.join(".git").exists());
    FolderInfo {
        is_git: root.is_some(),
        has_config: std::path::Path::new(&path)
            .join("software-factory.json")
            .is_file(),
    }
}

#[tauri::command]
fn save_repo(app: tauri::AppHandle, path: String) -> Result<(), String> {
    let file = repos_file(&app)?;
    let mut repos = read_repos(&file)?;

    if repos.iter().any(|r| r.path == path) {
        return Err("This repository is already added.".to_string());
    }

    let name = std::path::Path::new(&path)
        .file_name()
        .map(|n| n.to_string_lossy().into_owned())
        .unwrap_or_else(|| path.clone());
    repos.push(SavedRepo { name, path });

    let text = serde_json::to_string_pretty(&repos).map_err(|e| e.to_string())?;
    std::fs::write(&file, text).map_err(|e| e.to_string())
}

#[tauri::command]
async fn sandbox_help() -> Result<String, String> {
    AgentSandboxRun::help()
}

#[tauri::command]
async fn check_requirements() -> Result<Vec<Requirement>, String> {
    AgentSandboxRun::check_requirements()
}

fn repos_file(app: &tauri::AppHandle) -> Result<std::path::PathBuf, String> {
    let dir = app.path().app_data_dir().map_err(|e| e.to_string())?;
    std::fs::create_dir_all(&dir).map_err(|e| e.to_string())?;
    Ok(dir.join("repos.json"))
}

fn read_repos(file: &std::path::Path) -> Result<Vec<SavedRepo>, String> {
    match std::fs::read_to_string(file) {
        Ok(text) => serde_json::from_str(&text).map_err(|e| e.to_string()),
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(Vec::new()),
        Err(e) => Err(e.to_string()),
    }
}

#[tauri::command]
fn list_repos(app: tauri::AppHandle) -> Result<Vec<SavedRepo>, String> {
    read_repos(&repos_file(&app)?)
}

fn remove_saved_repo(repos: &mut Vec<SavedRepo>, path: &str) -> bool {
    let previous_len = repos.len();
    repos.retain(|repo| repo.path != path);
    repos.len() != previous_len
}

fn remove_repo_from_file(file: &std::path::Path, path: &str) -> Result<(), String> {
    let mut repos = read_repos(file)?;
    if !remove_saved_repo(&mut repos, path) {
        return Ok(());
    }
    let text = serde_json::to_string_pretty(&repos).map_err(|e| e.to_string())?;
    std::fs::write(file, text).map_err(|e| e.to_string())
}

#[tauri::command]
fn remove_repo(app: tauri::AppHandle, path: String) -> Result<(), String> {
    remove_repo_from_file(&repos_file(&app)?, &path)
}

#[tauri::command]
async fn list_branches(path: String) -> Result<Vec<Worktree>, String> {
    AgentSandboxRun::new(path).list_branches()
}

#[tauri::command]
async fn delete_worktree(
    path: String,
    branch: String,
    force: Option<bool>,
) -> Result<(), WorktreeDeleteError> {
    tauri::async_runtime::spawn_blocking(move || {
        AgentSandboxRun::new(path).delete_worktree(&branch, force.unwrap_or(false))
    })
    .await
    .map_err(|error| {
        WorktreeDeleteError::Other(format!("Could not wait for worktree deletion: {error}"))
    })?
}

#[tauri::command]
async fn create_factory_config(path: String, config: serde_json::Value) -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(move || FactoryConfigFile::new(&path).create(&config))
        .await
        .map_err(|error| format!("Could not save configuration: {error}"))?
}

#[tauri::command]
fn load_factory_config(path: String) -> Result<Option<serde_json::Value>, String> {
    FactoryConfigFile::new(&path).load()
}

#[tauri::command]
async fn update_factory_config(path: String, config: serde_json::Value) -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(move || FactoryConfigFile::new(&path).update(&config))
        .await
        .map_err(|error| format!("Could not save configuration: {error}"))?
}

fn repo_profiles(app: &tauri::AppHandle, path: &str) -> Result<ProfileStore, String> {
    let repos = read_repos(&repos_file(app)?)?;
    if !Path::new(path).is_absolute() || !repos.iter().any(|repo| repo.path == path) {
        return Err("Repository is not registered.".into());
    }
    let app_data = app
        .path()
        .app_data_dir()
        .map_err(|error| error.to_string())?;
    Ok(ProfileStore::new(&app_data, path))
}

#[derive(serde::Deserialize)]
struct FactoryLaunchProfile {
    id: String,
    config: serde_json::Value,
}

fn prepare_factory_profile(
    app: &tauri::AppHandle,
    path: &str,
    profile: Option<FactoryLaunchProfile>,
) -> Result<Option<ProfileLaunch>, String> {
    profile
        .map(|profile| repo_profiles(app, path)?.prepare_launch(&profile.id, &profile.config))
        .transpose()
}

/// Profile failures happen before the CLI starts, so they are reported apart
/// from run failures and the desktop never adopts another run's log directory.
#[derive(Debug, serde::Serialize)]
#[serde(untagged)]
enum FactoryLaunchError {
    Profile {
        #[serde(rename = "profileError")]
        profile_error: String,
    },
    Run(String),
}

/// A resent IPC request reuses its run ID; it must report the run as already
/// tracked before the profile is saved or found in use by the original request.
fn launch_with_profile(
    processes: &RunProcesses,
    run_id: &str,
    prepare: impl FnOnce() -> Result<Option<ProfileLaunch>, String>,
    launch: impl FnOnce(Option<&Path>) -> Result<RunOutcome, String>,
) -> Result<RunOutcome, FactoryLaunchError> {
    let _reservation = processes.reserve(run_id).map_err(FactoryLaunchError::Run)?;
    let profile =
        prepare().map_err(|profile_error| FactoryLaunchError::Profile { profile_error })?;
    launch(profile.as_ref().map(ProfileLaunch::path)).map_err(FactoryLaunchError::Run)
}

#[tauri::command]
async fn list_factory_profiles(
    app: tauri::AppHandle,
    path: String,
) -> Result<Vec<ProfileSummary>, String> {
    tauri::async_runtime::spawn_blocking(move || repo_profiles(&app, &path)?.list())
        .await
        .map_err(|error| error.to_string())?
}

#[tauri::command]
async fn load_factory_profile(
    app: tauri::AppHandle,
    path: String,
    id: String,
) -> Result<serde_json::Value, String> {
    tauri::async_runtime::spawn_blocking(move || repo_profiles(&app, &path)?.load(&id))
        .await
        .map_err(|error| error.to_string())?
}

#[tauri::command]
async fn create_factory_profile(
    app: tauri::AppHandle,
    path: String,
    name: String,
) -> Result<ProfileSummary, String> {
    tauri::async_runtime::spawn_blocking(move || repo_profiles(&app, &path)?.create(&name))
        .await
        .map_err(|error| error.to_string())?
}

#[tauri::command]
async fn update_factory_profile(
    app: tauri::AppHandle,
    path: String,
    id: String,
    config: serde_json::Value,
) -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(move || repo_profiles(&app, &path)?.update(&id, &config))
        .await
        .map_err(|error| error.to_string())?
}

#[tauri::command]
async fn rename_factory_profile(
    app: tauri::AppHandle,
    path: String,
    id: String,
    name: String,
) -> Result<ProfileSummary, String> {
    tauri::async_runtime::spawn_blocking(move || repo_profiles(&app, &path)?.rename(&id, &name))
        .await
        .map_err(|error| error.to_string())?
}

#[tauri::command]
async fn delete_factory_profile(
    app: tauri::AppHandle,
    path: String,
    id: String,
) -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(move || repo_profiles(&app, &path)?.delete(&id))
        .await
        .map_err(|error| error.to_string())?
}

#[tauri::command]
async fn agent_sandbox_check_bin() -> bool {
    AgentSandboxRun::check_bin()
}

#[tauri::command]
async fn agent_sandbox_check_and_update() {
    let _ = tauri::async_runtime::spawn_blocking(AgentSandboxRun::check_and_update).await;
}

#[tauri::command]
async fn agent_sandbox_install_bin() -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(AgentSandboxRun::install_bin)
        .await
        .map_err(|error| format!("Could not wait for agent-sandbox installation: {error}"))?
}

#[tauri::command]
async fn software_factory_check_bin() -> bool {
    FactoryRun::check_bin()
}

#[tauri::command]
async fn software_factory_check_and_update() {
    let _ = tauri::async_runtime::spawn_blocking(FactoryRun::check_and_update).await;
}

#[tauri::command]
async fn software_factory_install_bin() -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(FactoryRun::install_bin)
        .await
        .map_err(|error| format!("Could not wait for software-factory installation: {error}"))?
}

#[tauri::command]
async fn start_tracked_factory_run(
    app: tauri::AppHandle,
    run_id: String,
    path: String,
    jira_url: Option<String>,
    mode: FactoryRunMode,
    image_paths: Option<Vec<String>>,
    profile: Option<FactoryLaunchProfile>,
) -> Result<RunOutcome, FactoryLaunchError> {
    tauri::async_runtime::spawn_blocking(move || {
        let processes = app.state::<RunProcesses>();
        let workflow = FactoryRun::new(path.clone());

        launch_with_profile(
            &processes,
            &run_id.clone(),
            || prepare_factory_profile(&app, &path, profile),
            |config_path| {
                workflow.start_tracked(
                    &processes,
                    run_id,
                    mode,
                    jira_url.as_deref(),
                    image_paths.as_deref().unwrap_or_default(),
                    config_path,
                )
            },
        )
    })
    .await
    .map_err(|error| {
        FactoryLaunchError::Run(format!("Could not wait for software-factory: {error}"))
    })?
}

#[tauri::command]
fn supports_factory_stop() -> bool {
    cfg!(unix)
}

#[tauri::command]
fn stop_factory_run(app: tauri::AppHandle, run_id: String) -> Result<(), String> {
    #[cfg(unix)]
    {
        app.state::<RunProcesses>().request_stop(&run_id)
    }

    #[cfg(not(unix))]
    {
        let _ = (app, run_id);
        Err("Stopping runs is supported only on Unix platforms.".to_string())
    }
}

#[tauri::command]
async fn list_branch_runs(
    app: tauri::AppHandle,
    path: String,
    branch: String,
) -> Result<Vec<stage_log::RunEntry>, String> {
    let home = app
        .path()
        .home_dir()
        .map_err(|error| format!("Could not resolve home directory: {error}"))?;

    tauri::async_runtime::spawn_blocking(move || {
        let logs = stage_log::StageLog::new(&home, std::path::Path::new(&path))?;
        logs.list_run_entries(&branch)
    })
    .await
    .map_err(|error| format!("Could not load runs: {error}"))?
}

#[tauri::command]
async fn load_run_detail(
    app: tauri::AppHandle,
    path: String,
    branch: String,
    run_id: String,
) -> Result<stage_log::RunDetail, String> {
    let home = app
        .path()
        .home_dir()
        .map_err(|error| format!("Could not resolve home directory: {error}"))?;

    tauri::async_runtime::spawn_blocking(move || {
        let logs = stage_log::StageLog::new(&home, std::path::Path::new(&path))?;
        logs.load_run_detail(&branch, &run_id)
    })
    .await
    .map_err(|error| format!("Could not load run detail: {error}"))?
}

#[tauri::command]
async fn load_git_diff(path: String, branch: String) -> Result<Vec<git::ChangedFile>, String> {
    tauri::async_runtime::spawn_blocking(move || git::diff(Path::new(&path), &branch))
        .await
        .map_err(|error| format!("Could not load Git changes: {error}"))?
}

#[tauri::command]
async fn resume_factory_run(
    app: tauri::AppHandle,
    path: String,
    branch: String,
    run_id: String,
    execution_id: String,
    profile: Option<FactoryLaunchProfile>,
) -> Result<RunOutcome, FactoryLaunchError> {
    let home = app.path().home_dir().map_err(|error| {
        FactoryLaunchError::Run(format!("Could not resolve home directory: {error}"))
    })?;

    tauri::async_runtime::spawn_blocking(move || {
        let logs = stage_log::StageLog::new(&home, std::path::Path::new(&path))
            .map_err(FactoryLaunchError::Run)?;
        let directory = logs
            .run_directory(&branch, &run_id)
            .map_err(FactoryLaunchError::Run)?;
        let processes = app.state::<RunProcesses>();
        let workflow = FactoryRun::new(path.clone());

        launch_with_profile(
            &processes,
            &execution_id.clone(),
            || prepare_factory_profile(&app, &path, profile),
            |config_path| workflow.resume(&processes, execution_id, &directory, config_path),
        )
    })
    .await
    .map_err(|error| FactoryLaunchError::Run(format!("Could not wait for resume: {error}")))?
}

#[tauri::command]
async fn audit_factory_run(
    app: tauri::AppHandle,
    path: String,
    execution_id: String,
    log_path: String,
) -> Result<RunOutcome, String> {
    tauri::async_runtime::spawn_blocking(move || {
        let process = app.state::<RunProcesses>();

        FactoryRun::new(path).audit(&process, execution_id, log_path)
    })
    .await
    .map_err(|error| format!("Could not wait for audit: {error}"))?
}

#[derive(Debug, serde::Serialize, serde::Deserialize)]
struct Skill {
    name: String,
}

#[tauri::command]
async fn list_skills(path: String) -> Result<Vec<Skill>, String> {
    tauri::async_runtime::spawn_blocking(move || {
        let root = Path::new(&path);
        let mut skills: Vec<Skill> = Vec::new();
        let mut seen = std::collections::HashSet::new();

        for relative in [".agents/skills", ".claude/skills"] {
            let folder = root.join(relative);

            if !folder.is_dir() {
                continue;
            }

            for entry in fs::read_dir(&folder).map_err(|e| e.to_string())? {
                let entry = entry.map_err(|e| e.to_string())?;
                let skill_path = entry.path();

                if skill_path.is_dir() {
                    let name = entry.file_name().to_string_lossy().into_owned();
                    if seen.insert(name.clone()) {
                        skills.push(Skill { name });
                    }
                }
            }
        }

        Ok(skills)
    })
    .await
    .map_err(|error| format!("Could not load skills: {error}"))?
}

#[tauri::command]
async fn scheduled_create_factory_run(
    path: Option<String>,
    name: String,
    file: String,
    cron: String,
) -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(move || {
        let project_dir = path.as_deref().unwrap_or(".");
        FactoryRun::new(project_dir).scheduled_task_create(&name, &file, &cron)
    })
    .await
    .map_err(|error| format!("Could not wait for schedule creation: {error}"))?
}

#[tauri::command]
async fn scheduled_edit_factory_run(
    path: Option<String>,
    name: String,
    file: String,
    cron: Option<String>,
) -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(move || {
        let project_dir = path.as_deref().unwrap_or(".");
        FactoryRun::new(project_dir).scheduled_task_edit(&name, &file, cron.as_deref())
    })
    .await
    .map_err(|error| format!("Could not wait for schedule edit: {error}"))?
}

#[tauri::command]
async fn scheduled_delete_factory_run(path: Option<String>, name: String) -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(move || {
        let project_dir = path.as_deref().unwrap_or(".");
        FactoryRun::new(project_dir).scheduled_task_delete(&name)
    })
    .await
    .map_err(|error| format!("Could not wait for schedule deletion: {error}"))?
}

#[tauri::command]
async fn scheduled_list_factory_runs(path: String) -> Result<Vec<ScheduledTask>, String> {
    tauri::async_runtime::spawn_blocking(move || FactoryRun::new(path).scheduled_task_list())
        .await
        .map_err(|error| format!("Could not wait for schedule listing: {error}"))?
}

#[tauri::command]
async fn list_docker_images() -> Result<Vec<Image>, String> {
    tauri::async_runtime::spawn_blocking(Docker::get_images)
        .await
        .map_err(|err| format!("Could not complete Docker image lookup: {err}"))?
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_updater::Builder::new().build())
        .manage(DesktopUpdates::default())
        .plugin(tauri_plugin_notification::init())
        .manage(RunProcesses::default())
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_clipboard_manager::init())
        .invoke_handler(tauri::generate_handler![
            check_desktop_update,
            install_desktop_update,
            greet,
            inspect_folder,
            save_repo,
            sandbox_help,
            check_requirements,
            list_repos,
            remove_repo,
            list_branches,
            delete_worktree,
            list_branch_runs,
            load_run_detail,
            load_git_diff,
            create_factory_config,
            load_factory_config,
            update_factory_config,
            list_factory_profiles,
            load_factory_profile,
            create_factory_profile,
            update_factory_profile,
            rename_factory_profile,
            delete_factory_profile,
            start_tracked_factory_run,
            supports_factory_stop,
            resume_factory_run,
            stop_factory_run,
            audit_factory_run,
            scheduled_create_factory_run,
            scheduled_edit_factory_run,
            scheduled_delete_factory_run,
            scheduled_list_factory_runs,
            list_skills,
            agent_sandbox_check_bin,
            software_factory_check_bin,
            agent_sandbox_check_and_update,
            software_factory_check_and_update,
            software_factory_install_bin,
            agent_sandbox_install_bin,
            list_docker_images,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

#[cfg(test)]
mod skill_tests {
    use super::*;

    #[cfg(unix)]
    #[test]
    fn lists_skills_without_symlink_duplicates() {
        let temp = tempfile::tempdir().unwrap();
        let root = temp.path();

        fs::create_dir_all(root.join(".agents/skills/shared")).unwrap();
        fs::create_dir_all(root.join(".claude/skills/claude-only")).unwrap();

        std::os::unix::fs::symlink(
            "../../.agents/skills/shared",
            root.join(".claude/skills/shared"),
        )
        .unwrap();

        let mut skills =
            tauri::async_runtime::block_on(list_skills(root.to_str().unwrap().to_owned())).unwrap();

        skills.sort_by(|a, b| a.name.cmp(&b.name));

        assert_eq!(
            serde_json::to_value(&skills).unwrap(),
            serde_json::json!([
                {"name": "claude-only"},
                {"name": "shared"}
            ])
        );
    }
}

#[cfg(test)]
mod repo_removal_tests {
    use super::*;

    fn repo(name: &str, path: &str) -> SavedRepo {
        SavedRepo {
            name: name.into(),
            path: path.into(),
        }
    }

    #[test]
    fn removes_only_the_exact_path_even_when_names_match() {
        let mut repos = vec![repo("app", "/one/app"), repo("app", "/two/app")];
        assert!(remove_saved_repo(&mut repos, "/one/app"));
        assert_eq!(repos.len(), 1);
        assert_eq!(repos[0].path, "/two/app");
        assert!(!remove_saved_repo(&mut repos, "/one/app"));
        assert!(remove_saved_repo(&mut repos, "/two/app"));
        assert!(repos.is_empty());
    }

    #[test]
    fn persists_removal_and_preserves_project_files() {
        let temp = tempfile::tempdir().unwrap();
        let project = temp.path().join("project");
        std::fs::create_dir(&project).unwrap();
        let marker = project.join("software-factory.json");
        std::fs::write(&marker, "project config sentinel").unwrap();
        let path = project.to_str().unwrap();
        let file = temp.path().join("repos.json");
        let initial = vec![repo("project", path), repo("other", "/other")];
        std::fs::write(&file, serde_json::to_string(&initial).unwrap()).unwrap();
        remove_repo_from_file(&file, path).unwrap();
        let remaining = read_repos(&file).unwrap();
        assert_eq!(remaining.len(), 1);
        assert_eq!(remaining[0].name, "other");
        assert_eq!(remaining[0].path, "/other");
        assert_eq!(
            std::fs::read_to_string(&marker).unwrap(),
            "project config sentinel"
        );
        remove_repo_from_file(&file, "/other").unwrap();
        assert!(read_repos(&file).unwrap().is_empty());
    }

    #[test]
    fn absent_target_does_not_rewrite_or_create_registry() {
        let temp = tempfile::tempdir().unwrap();
        let file = temp.path().join("repos.json");
        remove_repo_from_file(&file, "/missing").unwrap();
        assert!(!file.exists());
        let original = "[ {\"name\":\"other\",\"path\":\"/other\"} ]";
        std::fs::write(&file, original).unwrap();
        remove_repo_from_file(&file, "/missing").unwrap();
        assert_eq!(std::fs::read_to_string(&file).unwrap(), original);
    }

    #[test]
    fn invalid_or_unreadable_registry_returns_error_without_replacement() {
        let temp = tempfile::tempdir().unwrap();
        let file = temp.path().join("repos.json");
        std::fs::write(&file, "invalid JSON").unwrap();
        assert!(remove_repo_from_file(&file, "/project").is_err());
        assert_eq!(std::fs::read_to_string(&file).unwrap(), "invalid JSON");
        assert!(remove_repo_from_file(temp.path(), "/project").is_err());
    }

    #[cfg(unix)]
    #[test]
    fn failed_write_returns_error() {
        use std::os::unix::fs::PermissionsExt;
        if unsafe { libc::geteuid() } == 0 {
            return;
        }
        let temp = tempfile::tempdir().unwrap();
        let file = temp.path().join("repos.json");
        let original = r#"[{"name":"project","path":"/project"}]"#;
        std::fs::write(&file, original).unwrap();
        std::fs::set_permissions(&file, std::fs::Permissions::from_mode(0o444)).unwrap();
        let result = remove_repo_from_file(&file, "/project");
        std::fs::set_permissions(&file, std::fs::Permissions::from_mode(0o600)).unwrap();
        assert!(result.is_err());
        assert_eq!(std::fs::read_to_string(&file).unwrap(), original);
    }
}

#[cfg(test)]
mod launch_tests {
    use super::*;
    use serde_json::json;
    use std::cell::Cell;

    #[test]
    fn profile_failures_are_reported_before_the_cli_is_started() {
        let root = tempfile::tempdir().unwrap();
        let repo = root.path().join("repo");
        fs::create_dir(&repo).unwrap();
        FactoryConfigFile::new(&repo)
            .create(&json!({"agentSandbox": {"agent": "codex"}}))
            .unwrap();
        let store = ProfileStore::new(root.path(), repo.to_str().unwrap());
        store.create("Quick review").unwrap();
        let edited = json!({"agentSandbox": {"agent": "claude"}});
        let running = store.prepare_launch("Quick review", &edited).unwrap();
        let started = Cell::new(false);

        let processes = RunProcesses::default();
        let error = launch_with_profile(
            &processes,
            "desktop:second",
            || store.prepare_launch("Quick review", &edited).map(Some),
            |_| {
                started.set(true);
                Ok(RunOutcome::Succeeded)
            },
        )
        .unwrap_err();
        assert!(!started.get());
        assert_eq!(
            serde_json::to_value(error).unwrap(),
            json!({"profileError": "This profile is in use by a running workflow."})
        );

        let profile_path = running.path().to_path_buf();
        drop(running);
        let error = launch_with_profile(
            &processes,
            "desktop:third",
            || store.prepare_launch("Quick review", &edited).map(Some),
            |config| {
                assert_eq!(config, Some(profile_path.as_path()));
                assert!(store.update("Quick review", &edited).is_err());
                Err("Could not start software-factory.".into())
            },
        )
        .unwrap_err();
        assert_eq!(
            serde_json::to_value(error).unwrap(),
            json!("Could not start software-factory.")
        );
        store.update("Quick review", &edited).unwrap();
    }

    #[test]
    fn repeated_run_ids_are_already_tracked_without_touching_the_profile() {
        let root = tempfile::tempdir().unwrap();
        let repo = root.path().join("repo");
        fs::create_dir(&repo).unwrap();
        let saved = json!({"agentSandbox": {"agent": "codex"}});
        FactoryConfigFile::new(&repo).create(&saved).unwrap();
        let store = ProfileStore::new(root.path(), repo.to_str().unwrap());
        store.create("Quick review").unwrap();
        let processes = RunProcesses::default();
        let original = processes.reserve("desktop:original").unwrap();
        let running = store.prepare_launch("Quick review", &saved).unwrap();
        let prepared = Cell::new(false);
        let started = Cell::new(false);

        let error = launch_with_profile(
            &processes,
            "desktop:original",
            || {
                prepared.set(true);
                store
                    .prepare_launch("Quick review", &json!({"edited": true}))
                    .map(Some)
            },
            |_| {
                started.set(true);
                Ok(RunOutcome::Succeeded)
            },
        )
        .unwrap_err();
        assert!(!prepared.get());
        assert!(!started.get());
        assert_eq!(
            serde_json::to_value(error).unwrap(),
            json!("This run is already tracked.")
        );
        drop(running);
        assert_eq!(store.load("Quick review").unwrap(), saved);
        drop(original);
    }
}
