use std::io::Write;
use std::path::PathBuf;
use std::process::{Command, Output, Stdio};

use serde_json::Value;

fn sample(name: &str) -> String {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("samples")
        .join(name)
        .to_string_lossy()
        .into_owned()
}

fn run(args: &[&str], input: &[u8]) -> Output {
    let mut child = Command::new(env!("CARGO_BIN_EXE_loglens"))
        .args(args)
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .unwrap();
    child.stdin.take().unwrap().write_all(input).unwrap();
    child.wait_with_output().unwrap()
}

fn json(output: &Output) -> Value {
    assert_eq!(
        output.status.code(),
        Some(0),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    serde_json::from_slice(&output.stdout).unwrap()
}

#[test]
fn reads_text_file_and_displays_human_report() {
    let output = run(&[&sample("app.log")], b"");
    assert_eq!(output.status.code(), Some(0));
    let report = String::from_utf8(output.stdout).unwrap();
    assert!(report.contains("Lines read: 7"));
    assert!(report.contains("Malformed (skipped): 0"));
    assert!(report.contains("Matched: 7"));
    assert!(report.contains("2  \"database connection timeout\""));
    assert!(report.contains("1  \"worker pool exhausted\""));
    assert!(output.stderr.is_empty());
}

#[test]
fn json_file_report_has_exact_counts_and_frequencies() {
    let report = json(&run(
        &[&sample("app.jsonl"), "--format", "json", "--output", "json"],
        b"",
    ));
    assert_eq!(report["total_lines"], 7);
    assert_eq!(report["matched_lines"], 7);
    assert_eq!(report["counts_by_level"]["info"], 2);
    assert_eq!(report["counts_by_level"]["error"], 2);
    assert_eq!(report["counts_by_level"]["fatal"], 1);
    assert_eq!(report["counts_by_level"]["trace"], 0);
    assert_eq!(
        report["top_errors"][0]["message"],
        "database connection timeout"
    );
    assert_eq!(report["top_errors"][0]["count"], 2);
    assert_eq!(report["error_aggregation"]["complete"], true);
}

#[test]
fn reads_stdin_implicitly_and_with_dash_and_combines_filters() {
    let input = b"2026-10-22T10:00:00Z ERROR database timeout\n\
        2026-10-22T10:00:01Z WARN database timeout\n\
        2026-10-22T10:00:02Z ERROR database timeout\n\
        2026-10-22T10:00:01Z ERROR request rejected\n\
        ERROR database timeout\n";
    for prefix in [vec![], vec!["-"]] {
        let mut args = prefix;
        args.extend([
            "--level",
            "ERR,WARNING",
            "--text",
            "timeout",
            "--since",
            "2026-10-22T10:00:00Z",
            "--until",
            "2026-10-22T10:00:01Z",
            "--output",
            "json",
        ]);
        let report = json(&run(&args, input));
        assert_eq!(report["total_lines"], 5);
        assert_eq!(report["matched_lines"], 2);
        assert_eq!(report["counts_by_level"]["error"], 1);
        assert_eq!(report["counts_by_level"]["warn"], 1);
    }
}

#[test]
fn auto_detects_mixed_formats_and_repeated_levels() {
    let report = json(&run(
        &["--output", "json", "--level", "INFO", "--level", "error"],
        b"INFO ready\n{\"level\":\"error\",\"message\":\"timeout\"}\nWARN ignored\n",
    ));
    assert_eq!(report["matched_lines"], 2);
    assert_eq!(report["malformed_lines"], 0);
}

#[test]
fn counts_malformed_lines_by_default_and_strict_fails_with_context() {
    let file = sample("malformed.jsonl");
    let report = json(&run(&[&file, "--output", "json"], b""));
    assert_eq!(report["total_lines"], 4);
    assert_eq!(report["parsed_lines"], 2);
    assert_eq!(report["malformed_lines"], 2);
    assert_eq!(report["matched_lines"], 2);

    let strict = run(&[&file, "--strict", "--output", "json"], b"");
    assert_eq!(strict.status.code(), Some(1));
    assert!(strict.stdout.is_empty());
    let diagnostic = String::from_utf8(strict.stderr).unwrap();
    assert!(diagnostic.contains(&file));
    assert!(diagnostic.contains("line 2"));
    assert!(diagnostic.contains("invalid JSON"));
}

#[test]
fn strict_checks_malformed_lines_even_if_filters_would_exclude_them() {
    let output = run(&["--strict", "--level", "fatal"], b"INFO valid\ninvalid\n");
    assert_eq!(output.status.code(), Some(1));
    assert!(String::from_utf8_lossy(&output.stderr).contains("standard input: line 2"));
}

#[test]
fn malformed_utf8_blank_and_oversized_lines_are_skipped() {
    let mut input = b"\nERROR \xff\n".to_vec();
    input.extend(std::iter::repeat_n(b'x', 1024 * 1024 + 10));
    input.extend_from_slice(b"\nINFO recovered");
    let report = json(&run(&["--output", "json"], &input));
    assert_eq!(report["total_lines"], 4);
    assert_eq!(report["malformed_lines"], 3);
    assert_eq!(report["matched_lines"], 1);
}

#[test]
fn reports_aggregation_limit_without_losing_level_counts() {
    let mut input = String::new();
    for index in 0..1002 {
        input.push_str(&format!("ERROR unique {index}\n"));
    }
    input.push_str("ERROR unique 0\n");
    let report = json(&run(&["--output", "json", "--top", "1"], input.as_bytes()));
    assert_eq!(report["counts_by_level"]["error"], 1003);
    assert_eq!(report["error_aggregation"]["tracked_messages"], 1000);
    assert_eq!(report["error_aggregation"]["untracked_occurrences"], 2);
    assert_eq!(report["error_aggregation"]["complete"], false);
    assert_eq!(report["top_errors"].as_array().unwrap().len(), 1);
    assert_eq!(report["top_errors"][0]["count"], 2);
    let human = run(&[], input.as_bytes());
    assert!(String::from_utf8_lossy(&human.stdout).contains("Top errors may be incomplete"));
}

#[test]
fn empty_input_and_no_matches_succeed() {
    for input in [b"".as_slice(), b"INFO no errors\n".as_slice()] {
        let report = json(&run(&["--output", "json", "--level", "error"], input));
        assert_eq!(report["matched_lines"], 0);
        assert!(report["top_errors"].as_array().unwrap().is_empty());
    }
}

#[test]
fn argument_errors_exit_two_with_helpful_diagnostics() {
    for args in [
        vec!["--level", "nonsense"],
        vec!["--since", "yesterday"],
        vec!["--until", "2026-99-99T00:00:00Z"],
        vec![
            "--since",
            "2026-10-23T00:00:00Z",
            "--until",
            "2026-10-22T00:00:00Z",
        ],
        vec!["--top", "0"],
        vec!["--top", "1001"],
        vec!["--top", "-1"],
        vec!["--format", "xml"],
        vec!["--output", "xml"],
        vec!["--unknown"],
    ] {
        let output = run(&args, b"");
        assert_eq!(output.status.code(), Some(2), "{args:?}");
        assert!(String::from_utf8_lossy(&output.stderr).contains("error:"));
        assert!(output.stdout.is_empty());
    }
}

#[test]
fn missing_file_exits_one_with_path_context() {
    let file = sample("does-not-exist.log");
    let output = run(&[&file], b"");
    assert_eq!(output.status.code(), Some(1));
    let diagnostic = String::from_utf8(output.stderr).unwrap();
    assert!(diagnostic.contains("cannot open"));
    assert!(diagnostic.contains(&file));
}

#[test]
fn help_and_version_succeed() {
    let help = run(&["--help"], b"");
    assert_eq!(help.status.code(), Some(0));
    let help = String::from_utf8(help.stdout).unwrap();
    for term in [
        "--strict",
        "--since",
        "--until",
        "--output",
        "Exit codes:",
        "1000",
    ] {
        assert!(help.contains(term), "missing {term}");
    }
    let version = run(&["--version"], b"");
    assert_eq!(version.status.code(), Some(0));
    assert!(String::from_utf8_lossy(&version.stdout).starts_with("loglens "));
}

#[cfg(target_os = "linux")]
#[test]
fn output_write_failure_exits_one() {
    let output = Command::new(env!("CARGO_BIN_EXE_loglens"))
        .stdin(Stdio::null())
        .stdout(
            std::fs::File::options()
                .write(true)
                .open("/dev/full")
                .unwrap(),
        )
        .stderr(Stdio::piped())
        .output()
        .unwrap();
    assert_eq!(output.status.code(), Some(1));
    assert!(String::from_utf8_lossy(&output.stderr).contains("writing report to standard output"));
}

#[test]
fn forced_formats_reject_other_records_and_json_arrays() {
    for (format, input) in [
        ("json", b"INFO text record\n".as_slice()),
        (
            "text",
            b"{\"level\":\"info\",\"message\":\"json record\"}\n".as_slice(),
        ),
        ("json", b"[null,\"error\",\"array record\"]\n".as_slice()),
        ("auto", b"[null,\"error\",\"array record\"]\n".as_slice()),
    ] {
        let report = json(&run(&["--format", format, "--output", "json"], input));
        assert_eq!(report["malformed_lines"], 1);
        assert_eq!(report["matched_lines"], 0);
    }
}
