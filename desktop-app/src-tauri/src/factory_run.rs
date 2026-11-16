use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};

use crate::factory_process::{RunOutcome, RunProcesses};

pub struct FactoryRun {
    repository: PathBuf,
}

impl FactoryRun {
    pub fn new(repository: impl Into<PathBuf>) -> Self {
        Self {
            repository: repository.into(),
        }
    }

    pub fn run_tracked(
        &self,
        processes: &RunProcesses,
        run_id: String,
        jira_url: Option<&str>,
    ) -> Result<RunOutcome, String> {
        let mut command = self.command();
        Self::add_jira(&mut command, jira_url);

        processes.run(run_id, command)
    }

    pub fn continue_tracked(
        &self,
        processes: &RunProcesses,
        run_id: String,
        jira_url: Option<&str>,
    ) -> Result<RunOutcome, String> {
        let mut command = self.command();
        command.arg("continue");
        Self::add_jira(&mut command, jira_url);

        processes.run(run_id, command)
    }

    pub fn resume(
        &self,
        processes: &RunProcesses,
        execution_id: String,
        directory: &Path,
    ) -> Result<RunOutcome, String> {
        let mut command = self.command();
        command.arg("resume").arg(directory);

        processes.run(execution_id, command)
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
