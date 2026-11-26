use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};

use crate::factory_process::{RunOutcome, RunProcesses};

#[derive(serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum FactoryRunMode {
    Run,
    Continue,
}

pub struct FactoryRun {
    repository: PathBuf,
}

#[derive(Debug, serde::Deserialize, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ScheduledTask {
    name: String,
    file_prompt: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    cron: Option<String>,
}

impl FactoryRun {
    pub fn new(repository: impl Into<PathBuf>) -> Self {
        Self {
            repository: repository.into(),
        }
    }

    pub fn start_tracked(
        &self,
        processes: &RunProcesses,
        run_id: String,
        mode: FactoryRunMode,
        jira_url: Option<&str>,
        image_paths: &[String],
        config_path: Option<&Path>,
    ) -> Result<RunOutcome, String> {
        processes.run(
            run_id,
            self.tracked_command(mode, jira_url, image_paths, config_path),
        )
    }

    fn tracked_command(
        &self,
        mode: FactoryRunMode,
        jira_url: Option<&str>,
        image_paths: &[String],
        config_path: Option<&Path>,
    ) -> Command {
        let mut command = self.command();
        if matches!(mode, FactoryRunMode::Continue) {
            command.arg("continue");
        }
        Self::add_config_path(&mut command, config_path);
        Self::add_jira(&mut command, jira_url);
        for path in image_paths {
            command.arg("--image").arg(path);
        }
        command
    }

    pub fn resume(
        &self,
        processes: &RunProcesses,
        execution_id: String,
        directory: &Path,
        config_path: Option<&Path>,
    ) -> Result<RunOutcome, String> {
        processes.run(execution_id, self.resume_command(directory, config_path))
    }

    fn resume_command(&self, directory: &Path, config_path: Option<&Path>) -> Command {
        let mut command = self.command();
        command.arg("resume").arg(directory);
        Self::add_config_path(&mut command, config_path);
        command
    }

    fn add_config_path(command: &mut Command, config_path: Option<&Path>) {
        if let Some(path) = config_path {
            command.arg("--config").arg(path);
        }
    }

    pub fn audit(
        &self,
        processes: &RunProcesses,
        execution_id: String,
        log_path: String,
    ) -> Result<RunOutcome, String> {
        let mut command = self.command();
        command.arg("audit");
        command.arg("--log").arg(log_path);

        processes.run(execution_id, command)
    }

    pub fn check_bin() -> bool {
        Self::bin_command()
            .arg("-h")
            .output()
            .map(|output| output.status.success())
            .unwrap_or(false)
    }

    pub fn check_and_update() {
        let Ok(output) = Self::bin_command()
            .arg("check-version")
            .stdin(Stdio::null())
            .stdout(Stdio::piped())
            .stderr(Stdio::null())
            .output()
        else {
            return;
        };

        if !output.status.success() {
            return;
        }

        let newer = serde_json::from_slice::<serde_json::Value>(&output.stdout)
            .ok()
            .and_then(|value| value.get("newer").and_then(|newer| newer.as_bool()));
        if newer != Some(true) {
            return;
        }

        let _ = Self::bin_command()
            .arg("update")
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .status();
    }

    pub fn scheduled_task_create(&self, name: &str, file: &str, cron: &str) -> Result<(), String> {
        self.run_scheduled_task("create", name, Some(file), Some(cron))
    }

    pub fn scheduled_task_edit(
        &self,
        name: &str,
        file: &str,
        cron: Option<&str>,
    ) -> Result<(), String> {
        self.run_scheduled_task("edit", name, Some(file), cron)
    }

    pub fn scheduled_task_delete(&self, name: &str) -> Result<(), String> {
        self.run_scheduled_task("delete", name, None, None)
    }

    pub fn scheduled_task_list(&self) -> Result<Vec<ScheduledTask>, String> {
        let mut command = Self::bin_command();
        command
            .args(["scheduled-task", "list", "--path"])
            .arg(&self.repository);
        let stdout = Self::scheduled_task_output(&mut command, "list schedules")?;

        serde_json::from_slice(&stdout)
            .map_err(|error| format!("Could not parse scheduled tasks: {error}"))
    }

    fn run_scheduled_task(
        &self,
        action: &str,
        name: &str,
        file: Option<&str>,
        cron: Option<&str>,
    ) -> Result<(), String> {
        let mut command = self.scheduled_task_command(action, name, file, cron);
        Self::scheduled_task_output(&mut command, &format!("{action} schedule")).map(|_| ())
    }

    fn scheduled_task_output(command: &mut Command, operation: &str) -> Result<Vec<u8>, String> {
        let output = command
            .output()
            .map_err(|error| format!("Could not start software-factory: {error}"))?;

        if output.status.success() {
            return Ok(output.stdout);
        }

        let stderr = String::from_utf8_lossy(&output.stderr);
        Err(format!(
            "Could not {operation} ({}): {}",
            output.status,
            stderr.trim()
        ))
    }

    fn scheduled_task_command(
        &self,
        action: &str,
        name: &str,
        file: Option<&str>,
        cron: Option<&str>,
    ) -> Command {
        let mut command = self.command();
        // Go uses PWD when it resolves to the working directory. Keep the saved
        // repository path as the registry key even when it contains a symlink.
        command
            .env("PWD", &self.repository)
            .args(["scheduled-task", action, "--name", name]);
        if let Some(file) = file {
            command.args(["--file", file]);
        }
        if let Some(cron) = cron {
            command.args(["--cron", cron]);
        }
        command
    }

    pub fn install_bin() -> Result<(), String> {
        let output = Command::new("bash")
            .args([
                "-o",
                "pipefail",
                "-c",
                "curl -fsSL https://raw.githubusercontent.com/ernesto27/ai-tools/master/soft-factory/install.sh | bash",
            ])
            .output()
            .map_err(|error| format!("Could not start installer: {error}"))?;

        if !output.status.success() {
            return Err(format!(
                "software-factory installation failed: {}",
                String::from_utf8_lossy(&output.stderr)
            ));
        }

        Ok(())
    }

    fn bin_command() -> Command {
        Command::new("software-factory")
    }

    fn command(&self) -> Command {
        let mut command = Self::bin_command();
        command.current_dir(&self.repository);
        command
    }

    fn add_jira(command: &mut Command, jira_url: Option<&str>) {
        if let Some(url) = jira_url.map(str::trim).filter(|url| !url.is_empty()) {
            command.arg("--jira").arg(url);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::ffi::OsStr;

    #[test]
    fn runs_cli_from_selected_repository() {
        let repository = tempfile::tempdir().unwrap();
        let command = FactoryRun::new(repository.path()).command();

        assert_eq!(command.get_program(), OsStr::new("software-factory"));
        assert_eq!(command.get_current_dir(), Some(repository.path()));
        assert_eq!(command.get_args().count(), 0);
    }

    #[test]
    fn builds_run_and_continue_args_with_optional_images() {
        let repository = tempfile::tempdir().unwrap();
        let factory = FactoryRun::new(repository.path());
        let images = [
            String::from("/tmp/design review.png"),
            String::from("/tmp/a,b.jpg"),
        ];
        let jira_url = "https://jira.example/ABC-1";

        let run = factory.tracked_command(FactoryRunMode::Run, None, &images, None);
        assert_eq!(run.get_current_dir(), Some(repository.path()));
        assert_eq!(
            run.get_args().collect::<Vec<_>>(),
            [
                "--image",
                "/tmp/design review.png",
                "--image",
                "/tmp/a,b.jpg"
            ]
        );

        let continuing =
            factory.tracked_command(FactoryRunMode::Continue, Some(jira_url), &images, None);
        assert_eq!(
            continuing.get_args().collect::<Vec<_>>(),
            [
                "continue",
                "--jira",
                jira_url,
                "--image",
                "/tmp/design review.png",
                "--image",
                "/tmp/a,b.jpg"
            ]
        );

        let run_without_images = factory.tracked_command(FactoryRunMode::Run, None, &[], None);
        assert_eq!(run_without_images.get_args().count(), 0);

        let continue_without_images =
            factory.tracked_command(FactoryRunMode::Continue, None, &[], None);
        assert_eq!(
            continue_without_images.get_args().collect::<Vec<_>>(),
            ["continue"]
        );
    }

    #[test]
    fn profile_commands_use_the_same_config_argument_in_each_mode() {
        let repo = tempfile::tempdir().unwrap();
        let factory = FactoryRun::new(repo.path());
        let config = repo.path().join("profile config.json");
        let run_directory = repo.path().join("logs/run");
        let run = factory.tracked_command(FactoryRunMode::Run, None, &[], Some(&config));
        let continuation =
            factory.tracked_command(FactoryRunMode::Continue, None, &[], Some(&config));
        let resume = factory.resume_command(&run_directory, Some(&config));

        assert_eq!(
            run.get_args().collect::<Vec<_>>(),
            vec![OsStr::new("--config"), config.as_os_str()]
        );
        assert_eq!(
            continuation.get_args().collect::<Vec<_>>(),
            vec![
                OsStr::new("continue"),
                OsStr::new("--config"),
                config.as_os_str()
            ]
        );
        assert_eq!(
            resume.get_args().collect::<Vec<_>>(),
            vec![
                OsStr::new("resume"),
                run_directory.as_os_str(),
                OsStr::new("--config"),
                config.as_os_str()
            ]
        );
        for command in [run, continuation, resume] {
            assert_eq!(command.get_current_dir(), Some(repo.path()));
        }
    }

    #[test]
    fn builds_scheduled_task_arguments() {
        let factory = FactoryRun::new(".");
        let cases = [
            (
                "create",
                "weekday",
                Some("task.txt"),
                Some("30 8 * * 1-5"),
                &[
                    "scheduled-task",
                    "create",
                    "--name",
                    "weekday",
                    "--file",
                    "task.txt",
                    "--cron",
                    "30 8 * * 1-5",
                ][..],
            ),
            (
                "edit",
                "-weekday morning",
                Some("-task prompt.txt"),
                None,
                &[
                    "scheduled-task",
                    "edit",
                    "--name",
                    "-weekday morning",
                    "--file",
                    "-task prompt.txt",
                ][..],
            ),
            (
                "delete",
                "weekday",
                None,
                None,
                &["scheduled-task", "delete", "--name", "weekday"][..],
            ),
        ];

        for (action, name, file, cron, expected) in cases {
            let command = factory.scheduled_task_command(action, name, file, cron);
            let actual = command.get_args().collect::<Vec<_>>();
            assert_eq!(actual.as_slice(), expected, "{action}");
        }
    }

    #[test]
    fn scheduled_tasks_preserve_cli_json_fields() {
        let json = serde_json::json!([
            {"name": "weekday", "filePrompt": "task.txt", "cron": "30 8 * * 1-5"},
            {"name": "legacy", "filePrompt": "old.txt"}
        ]);
        let tasks: Vec<ScheduledTask> = serde_json::from_value(json.clone()).unwrap();

        assert_eq!(tasks[0].cron.as_deref(), Some("30 8 * * 1-5"));
        assert_eq!(tasks[1].cron, None);
        assert_eq!(serde_json::to_value(tasks).unwrap(), json);
    }

    #[test]
    fn scheduled_task_writes_use_the_saved_repository_path() {
        let repository = tempfile::tempdir().unwrap();
        let command = FactoryRun::new(repository.path())
            .scheduled_task_command("delete", "weekday", None, None);

        assert_eq!(command.get_current_dir(), Some(repository.path()));
        assert_eq!(
            command
                .get_envs()
                .find(|(key, _)| *key == OsStr::new("PWD")),
            Some((OsStr::new("PWD"), Some(repository.path().as_os_str())))
        );
    }

    #[test]
    fn trims_jira_url_and_preserves_it_as_one_argument() {
        let mut command = Command::new("software-factory");
        command.arg("continue");
        FactoryRun::add_jira(&mut command, Some("  https://jira.example/ABC-1?q=a b  "));

        assert_eq!(
            command.get_args().collect::<Vec<_>>(),
            ["continue", "--jira", "https://jira.example/ABC-1?q=a b"]
        );
    }

    #[test]
    fn omits_missing_or_blank_jira_url() {
        for url in [None, Some(""), Some(" \t\n ")] {
            let mut command = Command::new("software-factory");
            FactoryRun::add_jira(&mut command, url);

            assert_eq!(command.get_args().count(), 0);
        }
    }
}
