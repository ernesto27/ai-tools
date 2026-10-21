use std::process::{Command, Output};

struct Database(std::path::PathBuf);
impl Database {
    fn new() -> Self {
        Self(std::env::temp_dir().join(format!(
            "taskline-cli-{}-{}.sqlite3",
            std::process::id(),
            std::time::SystemTime::now().duration_since(std::time::UNIX_EPOCH).unwrap().as_nanos()
        )))
    }
    fn run(&self, args: &[&str]) -> Output {
        Command::new(env!("CARGO_BIN_EXE_taskline"))
            .env("DATABASE_PATH", &self.0)
            .args(args)
            .output()
            .unwrap()
    }
    fn ok(&self, args: &[&str]) -> String {
        let output = self.run(args);
        assert!(
            output.status.success(),
            "{:?}: {}",
            args,
            String::from_utf8_lossy(&output.stderr)
        );
        String::from_utf8(output.stdout).unwrap()
    }
}
impl Drop for Database {
    fn drop(&mut self) {
        for suffix in ["", "-wal", "-shm"] {
            let _ = std::fs::remove_file(format!("{}{suffix}", self.0.display()));
        }
    }
}

#[test]
fn lifecycle_persists_and_partial_edits_preserve_fields() {
    let db = Database::new();
    assert_eq!(db.ok(&["list"]), "No tasks.\n");
    let added = db.ok(&[
        "add",
        "  Plan café  ",
        "--category",
        "Work",
        "--priority",
        "High",
        "--notes",
        "Details",
    ]);
    assert!(added.contains("Title: Plan café\n"));
    assert!(added.contains("Created: "));
    assert!(db
        .ok(&["list", "active"])
        .contains("1\tactive\tWork\tHigh\tPlan café"));
    db.ok(&["complete", "1"]);
    assert_eq!(db.ok(&["list", "active"]), "No tasks.\n");
    let edited = db.ok(&["edit", "1", "--title", "Revised"]);
    for field in [
        "Title: Revised",
        "Status: completed",
        "Category: Work",
        "Priority: High",
        "Notes: Details",
    ] {
        assert!(edited.contains(field), "{edited}");
    }
    assert!(db.ok(&["list", "completed"]).contains("Revised"));
    db.ok(&["edit", "1", "--notes", ""]);
    assert!(db.ok(&["show", "1"]).contains("Notes: \n"));
    assert!(db.ok(&["reopen", "1"]).contains("Status: active"));
    db.ok(&["delete", "1"]);
    assert_eq!(db.ok(&["list"]), "No tasks.\n");
    for command in ["show", "edit", "complete", "reopen", "delete"] {
        let args = if command == "edit" {
            vec![command, "1", "--title", "Missing"]
        } else {
            vec![command, "1"]
        };
        let result = db.run(&args);
        assert_eq!(result.status.code(), Some(1));
        assert!(String::from_utf8_lossy(&result.stderr).contains("task 1 not found"));
    }
}

#[test]
fn help_and_invalid_arguments_do_not_create_database() {
    let db = Database::new();
    for args in [vec![], vec!["help"], vec!["--help"], vec!["-h"]] {
        assert!(db.ok(&args).contains("Usage: taskline"));
    }
    let long_title = "界".repeat(201);
    let long_notes = "n".repeat(2001);
    for args in [
        vec!["unknown"],
        vec!["list", "invalid"],
        vec!["list", "all", "extra"],
        vec!["show", "0"],
        vec!["delete", "-1"],
        vec!["complete", "abc"],
        vec!["reopen", "1", "extra"],
        vec!["add"],
        vec!["add", " "],
        vec!["add", &long_title],
        vec!["add", "Title", "--notes", &long_notes],
        vec!["add", "Title", "--category", "invalid"],
        vec!["add", "Title", "--priority", "invalid"],
        vec!["add", "Title", "--notes"],
        vec!["add", "Title", "--unknown", "x"],
        vec!["edit", "1"],
        vec!["edit", "1", "--title", " "],
        vec!["add", "Title", "--notes", "one", "--notes", "two"],
    ] {
        let output = db.run(&args);
        assert_eq!(output.status.code(), Some(1), "{args:?}");
        assert!(output.stdout.is_empty());
        assert!(!output.stderr.is_empty());
    }
    assert!(!db.0.exists());
}

#[test]
fn database_errors_are_reported_and_control_characters_are_sanitized() {
    let db = Database::new();
    db.ok(&["add", "Safe\u{1b}[31m\ttitle", "--notes", "line\nnext"]);
    let shown = db.ok(&["show", "1"]);
    assert!(!shown.contains('\u{1b}'));
    assert!(shown.contains("Notes: line next"));
    let output = Command::new(env!("CARGO_BIN_EXE_taskline"))
        .env("DATABASE_PATH", db.0.join("missing.sqlite3"))
        .arg("list")
        .output()
        .unwrap();
    assert_eq!(output.status.code(), Some(1));
    assert!(String::from_utf8_lossy(&output.stderr).contains("error:"));
}
