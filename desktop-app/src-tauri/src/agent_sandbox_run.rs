use std::path::PathBuf;
use std::process::Command;

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

    pub fn check_bin() -> bool {
        Self::command()
            .arg("-h")
            .output()
            .map(|output| output.status.success())
            .unwrap_or(false)
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
    use super::parse_worktrees;

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
