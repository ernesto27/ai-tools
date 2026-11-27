use std::{path::Path, process::Command};

const MAX_PATCH_BYTES: usize = 512 * 1024;
const MAX_PATCH_LINES: usize = 5_000;
const PATCH_HEADER: &str = "diff --git ";

#[derive(serde::Serialize)]
pub struct ChangedFile {
    path: String,
    patch: Option<String>,
}

pub fn diff(repo: &Path, branch: &str) -> Result<Vec<ChangedFile>, String> {
    let comparison = format!("HEAD...refs/heads/{branch}");

    let options = [
        "--numstat",
        "--patch",
        "-z",
        "--no-color",
        "--no-renames",
        "--no-ext-diff",
        "--no-textconv",
        "--no-relative",
        "--submodule=short",
    ];
    let output = Command::new("git")
        .current_dir(repo)
        .arg("diff")
        .args(options)
        .arg(&comparison)
        .arg("--")
        .output()
        .map_err(|error| error.to_string())?;
    if !output.status.success() {
        return Err(String::from_utf8_lossy(&output.stderr).trim().to_string());
    }
    if output.stdout.is_empty() {
        return Ok(Vec::new());
    }

    // NUL-delimited numstat records supply the paths in patch order, including
    // paths containing spaces, tabs, or newlines.
    let output = String::from_utf8_lossy(&output.stdout);
    let (stats, patches) = output
        .split_once("\0\0")
        .ok_or("Invalid Git diff output.")?;
    let mut patches = patches
        .strip_prefix(PATCH_HEADER)
        .ok_or("Missing Git patches.")?
        .split("\ndiff --git ");
    let files = stats
        .split('\0')
        .map(|stat| {
            let path = stat
                .splitn(3, '\t')
                .nth(2)
                .ok_or("Missing Git diff path.")?;
            let patch = patches.next().ok_or("Missing Git file patch.")?;
            let patch = patch.trim_end_matches('\n');
            let fits = PATCH_HEADER.len() + patch.len() + 1 <= MAX_PATCH_BYTES
                && patch.lines().take(MAX_PATCH_LINES + 1).count() <= MAX_PATCH_LINES;
            Ok(ChangedFile {
                path: path.to_string(),
                patch: fits.then(|| format!("{PATCH_HEADER}{patch}\n")),
            })
        })
        .collect::<Result<Vec<_>, &str>>()?;
    if patches.next().is_some() {
        return Err("Unexpected Git file patch.".to_string());
    }
    Ok(files)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn compares_commits_in_a_linked_worktree() {
        let fixture = tempfile::tempdir().unwrap();
        let repo = fixture.path().join("repo");
        let worktree = fixture.path().join("worktree");
        std::fs::create_dir(&repo).unwrap();
        let git = |directory: &Path, args: &[&str]| {
            let output = Command::new("git")
                .current_dir(directory)
                .args([
                    "-c",
                    "user.name=Test",
                    "-c",
                    "user.email=test@example.invalid",
                    "-c",
                    "commit.gpgsign=false",
                    "-c",
                    "core.hooksPath=/dev/null",
                ])
                .args(args)
                .output()
                .unwrap();
            assert!(
                output.status.success(),
                "{}",
                String::from_utf8_lossy(&output.stderr)
            );
        };
        git(&repo, &["init", "--initial-branch=main"]);
        std::fs::write(repo.join("root.txt"), "initial\n").unwrap();
        git(&repo, &["add", "."]);
        git(&repo, &["commit", "-m", "Initial"]);
        assert!(diff(&repo, "main").unwrap().is_empty());

        git(
            &repo,
            &[
                "worktree",
                "add",
                "-b",
                "feature",
                worktree.to_str().unwrap(),
            ],
        );
        let name = "file with\ta tab.txt";
        std::fs::write(worktree.join(name), "committed\n").unwrap();
        std::fs::write(worktree.join("second.txt"), "second\n").unwrap();
        git(&worktree, &["add", "."]);
        git(&worktree, &["commit", "-m", "Changes"]);
        std::fs::write(worktree.join(name), "uncommitted\n").unwrap();

        let files = diff(&repo, "feature").unwrap();
        assert_eq!(files.len(), 2);
        assert_eq!(files[0].path, name);
        let patch = files[0].patch.as_deref().unwrap();
        assert!(patch.contains("+committed"));
        assert!(!patch.contains("uncommitted"));
        assert!(files[1].patch.as_deref().unwrap().contains("+second"));

        std::fs::write(worktree.join("wide.txt"), "x".repeat(MAX_PATCH_BYTES)).unwrap();
        std::fs::write(worktree.join("long.txt"), "x\n".repeat(MAX_PATCH_LINES)).unwrap();
        std::fs::write(worktree.join("small.txt"), "small\n").unwrap();
        git(&worktree, &["add", "wide.txt", "long.txt", "small.txt"]);
        git(&worktree, &["commit", "-m", "Mixed patch sizes"]);
        let files = diff(&repo, "feature").unwrap();
        assert_eq!(files.len(), 5);
        assert_eq!(files[0].path, name);
        assert_eq!(files[1].path, "long.txt");
        assert!(files[1].patch.is_none());
        assert_eq!(files[2].path, "second.txt");
        assert_eq!(files[3].path, "small.txt");
        assert!(files[3].patch.as_deref().unwrap().contains("+small"));
        assert_eq!(files[4].path, "wide.txt");
        assert!(files[4].patch.is_none());

        git(
            &worktree,
            &["commit", "--allow-empty", "-m", "Empty commit"],
        );
        assert_eq!(diff(&repo, "feature").unwrap().len(), 5);

        git(&repo, &["merge", "--ff-only", "feature"]);
        assert!(diff(&repo, "feature").unwrap().is_empty());
        assert!(diff(&repo, "missing").is_err());
    }
}
