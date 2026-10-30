use tauri::Manager;
mod factory_config;
mod factory_process;
mod factory_run;
mod stage_log;

use factory_config::FactoryConfigFile;
use factory_run::FactoryRun;

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

#[derive(serde::Deserialize)]
#[serde(rename_all = "camelCase")]
enum FactoryRunMode {
    Run,
    Continue,
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
    let output = std::process::Command::new("agent-sandbox")
        .arg("-h")
        .output()
        .map_err(|e| format!("could not run agent-sandbox: {e}"))?;

    if !output.status.success() {
        return Err(String::from_utf8_lossy(&output.stderr).into_owned());
    }
    Ok(String::from_utf8_lossy(&output.stdout).into_owned())
}

#[derive(serde::Serialize, serde::Deserialize)]
struct Requirement {
    name: String,
    installed: bool,
}

#[tauri::command]
async fn check_requirements() -> Result<Vec<Requirement>, String> {
    let output = std::process::Command::new("agent-sandbox")
        .args(["doctor", "--json"])
        .output()
        .map_err(|e| format!("could not run agent-sandbox: {e}"))?;

    if !output.status.success() {
        return Err(String::from_utf8_lossy(&output.stderr).into_owned());
    }
    serde_json::from_slice(&output.stdout).map_err(|e| e.to_string())
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

#[derive(serde::Deserialize)]
struct Worktree {
    branch: String,
}

#[tauri::command]
async fn list_branches(path: String) -> Result<Vec<String>, String> {
    let output = std::process::Command::new("agent-sandbox")
        .args(["worktree-list", "--json", "--repo"])
        .arg(&path)
        .output()
        .map_err(|e| format!("could not run agent-sandbox: {e}"))?;

    if !output.status.success() {
        return Err(String::from_utf8_lossy(&output.stderr).into_owned());
    }

    let worktrees: Option<Vec<Worktree>> =
        serde_json::from_slice(&output.stdout).map_err(|e| e.to_string())?;
    Ok(worktrees
        .unwrap_or_default()
        .into_iter()
        .map(|w| w.branch)
        .collect())
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

#[tauri::command]
async fn start_factory_run(
    path: String,
    jira_url: Option<String>,
    mode: FactoryRunMode,
) -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(move || {
        let workflow = FactoryRun::new(path);

        match mode {
            FactoryRunMode::Run => workflow.run(jira_url.as_deref()),
            FactoryRunMode::Continue => workflow.continue_run(jira_url.as_deref()),
        }
    })
    .await
    .map_err(|error| format!("Could not wait for software-factory: {error}"))?
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
async fn resume_factory_run(
    app: tauri::AppHandle,
    path: String,
    branch: String,
    run_id: String,
) -> Result<(), String> {
    let home = app
        .path()
        .home_dir()
        .map_err(|error| format!("Could not resolve home directory: {error}"))?;

    tauri::async_runtime::spawn_blocking(move || {
        let logs = stage_log::StageLog::new(&home, std::path::Path::new(&path))?;
        let directory = logs.run_directory(&branch, &run_id)?;

        FactoryRun::new(path).resume(&directory)
    })
    .await
    .map_err(|error| format!("Could not wait for resume: {error}"))?
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_opener::init())
        .invoke_handler(tauri::generate_handler![
            greet,
            inspect_folder,
            save_repo,
            sandbox_help,
            check_requirements,
            list_repos,
            remove_repo,
            list_branches,
            list_branch_runs,
            load_run_detail,
            create_factory_config,
            load_factory_config,
            update_factory_config,
            start_factory_run,
            resume_factory_run
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
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
