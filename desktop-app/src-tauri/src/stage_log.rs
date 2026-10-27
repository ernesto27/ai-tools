use sha2::{Digest, Sha256};
use std::{
    fs,
    path::{Component, Path, PathBuf},
};

#[derive(Debug, serde::Serialize)]
pub struct StageStatus {
    pub stage: String,
    pub status: String,
}

pub struct StageLog {
    directory: PathBuf,
}

#[derive(Debug, serde::Serialize)]
pub struct RunEntry {
    pub name: String, 
    pub directory: PathBuf,
    pub stages: Vec<StageStatus>,
    pub error: Option<String>,
}

impl StageLog {
    pub fn new(home: &Path, repository: &Path) -> Result<Self, String> {
        Ok(Self {
            directory: Self::project_log_directory(home, repository)?,
        })
    }

    pub fn directory(&self) -> &Path {
        &self.directory
    }

    pub fn branch_directory(&self, branch: &str) -> Result<PathBuf, String> {
        let name: String = branch
            .chars()
            .map(|character| {
                if character.is_ascii_alphanumeric() || matches!(character, '.' | '_' | '-') {
                    character
                } else {
                    '-'
                }
            })
            .collect();

        if name.is_empty() || name == "." || name == ".." {
            return Err("Invalid branch name.".to_string());
        }

        Ok(self.directory.join(name))
    }

    fn read_stage_status(run_directory: &Path) -> Result<Vec<StageStatus>, String> {
        let path = run_directory.join("stage_status.txt");

        let text = match fs::read_to_string(&path) {
            Ok(text) => text,
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
                return Ok(Vec::new());
            }
            Err(error) => {
                return Err(format!("Could not read {}: {error}", path.display()));
            }
        };

        Self::parse_stage_status(&text)
            .map_err(|error| format!("Invalid {}: {error}", path.display()))
    }

    fn parse_stage_status(text: &str) -> Result<Vec<StageStatus>, String> {
        let mut stages = Vec::new();

        for line in text.lines() {
            let line = line.trim();

            if line.is_empty() {
                continue;
            }

            let (stage, status) = line
                .split_once(",")
                .ok_or_else(|| format!("Invalid stage status line: {line}"))?;

            if stage.trim().is_empty() || status.trim().is_empty() {
                return Err(format!("Incomplete stage status line: {line}"));
            }

            stages.push(StageStatus {
                stage: stage.trim().to_string(),
                status: status.trim().to_string(),
            });
        }

        Ok(stages)
    }

    fn project_log_directory(home: &Path, repository: &Path) -> Result<PathBuf, String> {
        if !home.is_absolute() || !repository.is_absolute() {
            return Err("Home and repository paths must be absolute.".to_string());
        }

        let mut clean = PathBuf::new();

        for component in repository.components() {
            match component {
                Component::CurDir => {}
                Component::ParentDir => {
                    clean.pop();
                }
                other => clean.push(other.as_os_str()),
            }
        }

        let repository_text = clean
            .to_str()
            .ok_or_else(|| "Repository path must be valid UTF-8.".to_string())?;

        let name = clean
            .file_name()
            .and_then(|name| name.to_str())
            .unwrap_or("project");

        let digest = Sha256::digest(repository_text.as_bytes());

        let fingerprint: String = digest[..6]
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect();

        Ok(home
            .join(".config")
            .join("software-factory")
            .join("logs")
            .join(format!("{name}-{fingerprint}")))
    }

    pub fn list_runs(&self, branch: &str) -> Result<Vec<PathBuf>, String> {
        let directory = self.branch_directory(branch)?;

        let entries = match fs::read_dir(&directory) {
            Ok(entries) => entries,
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
                return Ok(Vec::new());
            }
            Err(error) => {
                return Err(format!("Could not list {}: {error}", directory.display()));
            }
        };

        let mut runs = Vec::new();

        for entry in entries {
            let entry =
                entry.map_err(|error| format!("Could not read directory entry: {error}"))?;

            let file_type = entry.file_type().map_err(|error| {
                format!("Could not inspect {}: {error}", entry.path().display())
            })?;

            if !file_type.is_dir() {
                continue;
            }

            let name = entry.file_name();
            let Some(name) = name.to_str() else {
                continue;
            };

            if name.starts_with("run-") || name.starts_with("continue-") {
                runs.push(entry.path());
            }
        }

        runs.sort();
        Ok(runs)
    }

    pub fn list_run_entries(&self, branch: &str) -> Result<Vec<RunEntry>, String> {
        let mut entries = Vec::new();

        for directory in self.list_runs(branch)? {
            let name = directory
                .file_name() 
                .and_then(|name| name.to_str())
                .ok_or_else(|| {
                     format!("Invalid run directory: {}", directory.display())
                })?
                .to_string();
            
            let (stages, error) = match Self::read_stage_status(&directory) {
                Ok(stages) => (stages, None), 
                Err(error) => (Vec::new(), Some(error)),
            };

            entries.push(RunEntry{
                name,
                directory,
                stages,
                error
            });
        }

        Ok(entries)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn reads_stage_status_file() {
        let run = tempfile::tempdir().unwrap();

        fs::write(
            run.path().join("stage_status.txt"),
            "implementation,SUCCEEDED\ncodeReview,FAILED\n",
        )
        .unwrap();

        let stages = StageLog::read_stage_status(run.path()).unwrap();

        assert_eq!(stages.len(), 2);
        assert_eq!(stages[0].stage, "implementation");
        assert_eq!(stages[1].status, "FAILED");
    }

    #[test]
    fn missing_status_file_returns_no_stages() {
        let run = tempfile::tempdir().unwrap();

        let stages = StageLog::read_stage_status(run.path()).unwrap();

        assert!(stages.is_empty());
    }

    #[test]
    fn reports_invalid_status_file() {
        let run = tempfile::tempdir().unwrap();

        fs::write(run.path().join("stage_status.txt"), "broken line").unwrap();

        let error = StageLog::read_stage_status(run.path()).unwrap_err();

        assert!(error.contains("stage_status.txt"));
        assert!(error.contains("Invalid stage status line"));
    }

    #[test]
    fn calculates_branch_directory() {
        let absolute = std::env::current_dir().unwrap();
        let logs = StageLog::new(&absolute, &absolute).unwrap();

        let directory = logs.branch_directory("feature/add-logs").unwrap();

        assert_eq!(directory, logs.directory().join("feature-add-logs"));
    }

    #[test]
    fn rejects_invalid_branch_directories() {
        let absolute = std::env::current_dir().unwrap();
        let logs = StageLog::new(&absolute, &absolute).unwrap();

        for branch in ["", ".", ".."] {
            assert!(logs.branch_directory(branch).is_err());
        }
    }

    #[test]
    fn parses_stage_results() {
        let stages =
            StageLog::parse_stage_status("implementation,SUCCEEDED\ncodeReview,FAILED\n").unwrap();

        assert_eq!(stages.len(), 2);
        assert_eq!(stages[0].stage, "implementation");
        assert_eq!(stages[0].status, "SUCCEEDED");
        assert_eq!(stages[1].status, "FAILED");
    }

    #[test]
    fn rejects_malformed_lines() {
        assert!(StageLog::parse_stage_status("broken line").is_err());
        assert!(StageLog::parse_stage_status("implementation,").is_err());
    }

    #[cfg(unix)]
    #[test]
    fn calculates_project_log_directory() {
        let logs = StageLog::new(
            Path::new("/home/test"),
            Path::new("/work/temporary/../agent-sandbox"),
        )
        .unwrap();

        assert_eq!(
            logs.directory(),
            Path::new("/home/test/.config/software-factory/logs/agent-sandbox-f7c2b9c472e1",),
        );
    }

    #[test]
    fn rejects_relative_paths() {
        let absolute = std::env::current_dir().unwrap();
        assert!(StageLog::new(Path::new("relative-home"), &absolute).is_err());
        assert!(StageLog::new(&absolute, Path::new("relative-repository")).is_err());
    }

    #[test]
    fn lists_only_run_directories() {
        let temporary = tempfile::tempdir().unwrap();
        let logs = StageLog::new(temporary.path(), temporary.path()).unwrap();
        let branch = logs.branch_directory("feature/logs").unwrap();

        let first = branch.join("run-2026-10-27_13-29-59");
        let second = branch.join("continue-2026-10-28_09-00-00");

        fs::create_dir_all(&first).unwrap();
        fs::create_dir_all(&second).unwrap();
        fs::create_dir_all(branch.join("other")).unwrap();
        fs::write(branch.join("run-not-a-directory"), "ignore").unwrap();

        let mut expected = vec![first, second];
        expected.sort();

        assert_eq!(logs.list_runs("feature/logs").unwrap(), expected);
    }

    #[test]
    fn missing_branch_has_no_runs() {
        let temporary = tempfile::tempdir().unwrap();
        let logs = StageLog::new(temporary.path(), temporary.path()).unwrap();

        assert!(logs.list_runs("missing").unwrap().is_empty());
    }

    #[test]
    fn lists_run_entries_without_hiding_invalid_runs() {
        let temporary = tempfile::tempdir().unwrap();
        let logs = StageLog::new(temporary.path(), temporary.path()).unwrap();
        let branch = logs.branch_directory("feature/logs").unwrap();

        let good = branch.join("run-2026-10-27_13-29-59");
        let bad = branch.join("run-2026-10-27_14-00-00");

        fs::create_dir_all(&good).unwrap();
        fs::create_dir_all(&bad).unwrap();

        fs::write(
            good.join("stage_status.txt"),
            "implementation,SUCCEEDED\n",
        )
        .unwrap();

        fs::write(bad.join("stage_status.txt"), "broken line").unwrap();

        let entries = logs.list_run_entries("feature/logs").unwrap();

        assert_eq!(entries.len(), 2);

        assert_eq!(entries[0].name, "run-2026-10-27_13-29-59");
        assert_eq!(entries[0].directory, good);
        assert_eq!(entries[0].stages[0].status, "SUCCEEDED");
        assert!(entries[0].error.is_none());

        assert!(entries[1].stages.is_empty());
        assert!(entries[1].error.is_some());
    }

}
