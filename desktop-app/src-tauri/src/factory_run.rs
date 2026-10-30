use std::path::{Path, PathBuf};
use std::process::Command;

use crate::factory_process;

pub struct FactoryRun {
    repository: PathBuf,
}

impl FactoryRun {
    pub fn new(repository: impl Into<PathBuf>) -> Self {
        Self {
            repository: repository.into(),
        }
    }

    pub fn run(&self, jira_url: Option<&str>) -> Result<(), String> {
        let mut command = self.command();
        Self::add_jira(&mut command, jira_url);

        factory_process::run(command)
    }

    pub fn continue_run(&self, jira_url: Option<&str>) -> Result<(), String> {
        let mut command = self.command();
        command.arg("continue");
        Self::add_jira(&mut command, jira_url);

        factory_process::run(command)
    }

    pub fn resume(&self, directory: &Path) -> Result<(), String> {
        let mut command = self.command();
        command.arg("resume").arg(directory);

        factory_process::run(command)
    }

    fn command(&self) -> Command {
        let mut command = Command::new("software-factory");
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
