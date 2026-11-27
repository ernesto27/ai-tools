use std::path::PathBuf;
use std::process::{Command, Stdio};

#[derive(serde::Serialize, serde::Deserialize)]
pub struct Requirement {
    pub name: String,
    pub installed: bool,
}

#[derive(Debug, serde::Serialize, serde::Deserialize)]
pub struct Worktree {
    pub branch: String,
    pub path: String,
}

#[derive(Debug, serde::Serialize)]
#[serde(untagged)]
pub enum WorktreeDeleteError {
    Dirty {
        #[serde(rename = "dirtyWorktreeError")]
        dirty_worktree_error: String,
    },
    Other(String),
}

fn classify_worktree_delete_error(message: String, force: bool) -> WorktreeDeleteError {
    const DIRTY_WORKTREE_DIAGNOSTIC: &str =
        " has uncommitted changes; pass --force to delete it anyway";

    if !force
        && message
            .lines()
            .any(|line| line.trim_end().ends_with(DIRTY_WORKTREE_DIAGNOSTIC))
    {
        WorktreeDeleteError::Dirty {
            dirty_worktree_error: message,
        }
    } else {
        WorktreeDeleteError::Other(message)
    }
}

pub struct AgentSandboxRun {
    repository: PathBuf,
}

impl AgentSandboxRun {
    pub fn new(repository: impl Into<PathBuf>) -> Self {
        Self {
            repository: repository.into(),
        }
    }

    pub fn help() -> Result<String, String> {
        let mut command = Self::command();
        command.arg("-h");

        let stdout = Self::output(command)?;
        Ok(String::from_utf8_lossy(&stdout).into_owned())
    }

    pub fn check_requirements() -> Result<Vec<Requirement>, String> {
        let mut command = Self::command();
        command.args(["doctor", "--json"]);

        let stdout = Self::output(command)?;
        serde_json::from_slice(&stdout).map_err(|error| error.to_string())
    }

    pub fn list_branches(&self) -> Result<Vec<Worktree>, String> {
        let mut command = Self::command();
        command.current_dir(&self.repository);
        command.args(["worktree-list", "--json", "--repo"]);
        command.arg(&self.repository);

        parse_worktrees(&Self::output(command)?)
    }

    pub fn delete_worktree(&self, branch: &str, force: bool) -> Result<(), WorktreeDeleteError> {
        Self::output(self.worktree_delete_command(branch, force))
            .map(|_| ())
            .map_err(|message| classify_worktree_delete_error(message, force))
    }

    fn worktree_delete_command(&self, branch: &str, force: bool) -> Command {
        let mut command = Self::command();
        command.current_dir(&self.repository);
        command.args(["worktree-delete", "-b", branch]);
        if force {
            command.arg("--force");
        }
        command
    }

    pub fn check_bin() -> bool {
        Self::command()
            .arg("-h")
            .output()
            .map(|output| output.status.success())
            .unwrap_or(false)
    }

    pub fn check_and_update() {
        let Ok(output) = Self::command()
            .arg("version-check")
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

        let _ = Self::command()
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
                "curl -fsSL https://raw.githubusercontent.com/ernesto27/ai-tools/master/agent-sandbox/install.sh | bash",
            ])
            .output()
            .map_err(|error| format!("Could not start installer: {error}"))?;

        if !output.status.success() {
            return Err(format!(
                "agent-sandbox installation failed: {}",
                String::from_utf8_lossy(&output.stderr)
            ));
        }

        Ok(())
    }

    fn command() -> Command {
        Command::new("agent-sandbox")
    }

    fn output(mut command: Command) -> Result<Vec<u8>, String> {
        let output = command
            .output()
            .map_err(|error| format!("could not run agent-sandbox: {error}"))?;

        if !output.status.success() {
            return Err(String::from_utf8_lossy(&output.stderr).into_owned());
        }

        Ok(output.stdout)
    }
}

fn parse_worktrees(bytes: &[u8]) -> Result<Vec<Worktree>, String> {
    let worktrees: Option<Vec<Worktree>> =
        serde_json::from_slice(bytes).map_err(|error| error.to_string())?;
    Ok(worktrees.unwrap_or_default())
}

#[cfg(test)]
mod worktree_tests {
    use super::{AgentSandboxRun, classify_worktree_delete_error, parse_worktrees};

    #[test]
    fn adds_force_only_for_the_explicit_retry() {
        use std::ffi::OsStr;
        use std::path::Path;

        let runner = AgentSandboxRun::new("repo with spaces");
        let branch = "feature/árbol";
        for force in [false, true] {
            let command = runner.worktree_delete_command(branch, force);
            let mut expected = vec!["worktree-delete", "-b", branch];
            if force {
                expected.push("--force");
            }
            assert_eq!(command.get_program(), OsStr::new("agent-sandbox"));
            assert_eq!(
                command.get_current_dir(),
                Some(Path::new("repo with spaces"))
            );
            assert!(command.get_args().eq(expected.into_iter().map(OsStr::new)));
        }
    }

    #[test]
    fn only_tags_the_dirty_worktree_refusal_from_a_normal_delete() {
        let dirty = concat!(
            "Error: The operation failed.\r\n\r\nCause:\r\n",
            "/tmp/árbol with spaces has uncommitted changes; pass --force to delete it anyway \t\r\n",
        );

        let ipc_error = |message: &str, force| {
            serde_json::to_value(classify_worktree_delete_error(message.to_owned(), force)).unwrap()
        };

        assert_eq!(
            ipc_error(dirty, false),
            serde_json::json!({ "dirtyWorktreeError": dirty })
        );
        assert_eq!(ipc_error(dirty, true), serde_json::json!(dirty));

        for message in [
            "git worktree remove: worktree is locked",
            "could not run agent-sandbox: executable missing",
            "uncommitted changes",
            "try --force",
        ] {
            assert_eq!(ipc_error(message, false), serde_json::json!(message));
        }
    }

    #[test]
    fn preserves_exact_branches_and_paths() {
        let entries = parse_worktrees(
            r#"[{"branch":"feature/a","path":"/tmp/project space/árbol"},{"branch":"other","path":"/tmp/other"}]"#.as_bytes(),
        )
        .unwrap();
        assert_eq!(entries.len(), 2);
        assert_eq!(entries[0].branch, "feature/a");
        assert_eq!(entries[0].path, "/tmp/project space/árbol");
        let value = serde_json::to_value(&entries).unwrap();
        assert_eq!(value[1]["branch"], "other");
        assert_eq!(value[1]["path"], "/tmp/other");
    }

    #[test]
    fn supports_empty_output() {
        assert!(parse_worktrees(b"null").unwrap().is_empty());
        assert!(parse_worktrees(b"[]").unwrap().is_empty());
    }

    #[test]
    fn rejects_malformed_or_wrongly_typed_output() {
        for bytes in [
            &b"invalid"[..],
            &br#"{}"#[..],
            &br#"[{"path":"/tmp/no-branch"}]"#[..],
            &br#"[{"branch":"missing-path"}]"#[..],
            &br#"[{"branch":"null-path","path":null}]"#[..],
            &br#"[{"branch":"a","path":42}]"#[..],
        ] {
            assert!(parse_worktrees(bytes).is_err());
        }
    }
}
