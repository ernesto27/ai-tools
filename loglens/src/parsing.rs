use std::fmt;
use std::str::FromStr;

use chrono::{DateTime, FixedOffset};
use serde::{Deserialize, Serialize};

use crate::cli::InputFormat;

#[derive(Clone, Copy, Debug, Eq, Ord, PartialEq, PartialOrd, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum Level {
    Trace,
    Debug,
    Info,
    Warn,
    Error,
    Fatal,
}

impl FromStr for Level {
    type Err = String;

    fn from_str(value: &str) -> Result<Self, Self::Err> {
        match value.to_ascii_lowercase().as_str() {
            "trace" => Ok(Self::Trace),
            "debug" | "dbg" => Ok(Self::Debug),
            "info" | "information" | "notice" => Ok(Self::Info),
            "warn" | "warning" => Ok(Self::Warn),
            "error" | "err" => Ok(Self::Error),
            "fatal" | "critical" | "crit" | "panic" | "emerg" | "emergency" | "alert" => {
                Ok(Self::Fatal)
            }
            _ => Err(format!(
                "unknown log level {value:?}; expected trace, debug, info, warn, error, or fatal"
            )),
        }
    }
}

impl fmt::Display for Level {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(match self {
            Self::Trace => "trace",
            Self::Debug => "debug",
            Self::Info => "info",
            Self::Warn => "warn",
            Self::Error => "error",
            Self::Fatal => "fatal",
        })
    }
}

#[derive(Debug)]
pub struct Entry {
    pub timestamp: Option<DateTime<FixedOffset>>,
    pub level: Level,
    pub message: String,
}

#[derive(Deserialize)]
struct JsonEntry {
    #[serde(default, alias = "time", alias = "ts")]
    timestamp: Option<DateTime<FixedOffset>>,
    #[serde(alias = "severity")]
    level: String,
    #[serde(alias = "msg")]
    message: String,
}

pub fn parse_line(line: &str, format: InputFormat) -> Result<Entry, String> {
    let line = line.trim();
    match format {
        InputFormat::Json => parse_json(line),
        InputFormat::Text => parse_text(line),
        InputFormat::Auto if line.starts_with(['{', '[']) => {
            // A bracketed text timestamp/level is also a supported text record.
            if line.starts_with('[') {
                parse_text(line).or_else(|_| parse_json(line))
            } else {
                parse_json(line)
            }
        }
        InputFormat::Auto => parse_text(line),
    }
}

fn parse_json(line: &str) -> Result<Entry, String> {
    if !line.starts_with('{') {
        return Err("invalid JSON log: expected an object".into());
    }
    let record: JsonEntry =
        serde_json::from_str(line).map_err(|err| format!("invalid JSON log: {err}"))?;
    Ok(Entry {
        timestamp: record.timestamp,
        level: record.level.parse()?,
        message: record.message,
    })
}

fn token(line: &str) -> (&str, &str) {
    let (first, rest) = line.split_once(char::is_whitespace).unwrap_or((line, ""));
    (first, rest.trim_start())
}

fn unwrap_token(value: &str) -> &str {
    value.trim_end_matches(':').trim_matches(['[', ']'])
}

fn parse_text(line: &str) -> Result<Entry, String> {
    let (first, rest) = token(line);
    let first = unwrap_token(first);
    let (timestamp, level, message) = if let Ok(level) = first.parse::<Level>() {
        (None, level, rest)
    } else {
        let timestamp = DateTime::parse_from_rfc3339(first)
            .map_err(|_| "expected LEVEL message or RFC3339-timestamp LEVEL message".to_string())?;
        let (level, message) = token(rest);
        (Some(timestamp), unwrap_token(level).parse()?, message)
    };
    if message.is_empty() {
        return Err("text log is missing a message".into());
    }
    Ok(Entry {
        timestamp,
        level,
        message: message.to_string(),
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn recognizes_case_insensitive_aliases() {
        for (name, expected) in [
            ("TrAcE", Level::Trace),
            ("DBG", Level::Debug),
            ("Information", Level::Info),
            ("NOTICE", Level::Info),
            ("WARNING", Level::Warn),
            ("eRr", Level::Error),
            ("CRITICAL", Level::Fatal),
            ("panic", Level::Fatal),
        ] {
            assert_eq!(name.parse::<Level>().unwrap(), expected);
        }
        assert!("verbose".parse::<Level>().is_err());
    }

    #[test]
    fn parses_plain_text_with_optional_timestamp() {
        let entry = parse_line(
            "[2026-10-22T12:00:00+02:00] [ERROR] database unavailable",
            InputFormat::Auto,
        )
        .unwrap();
        assert_eq!(entry.level, Level::Error);
        assert_eq!(entry.timestamp.unwrap().offset().local_minus_utc(), 7200);
        assert_eq!(entry.message, "database unavailable");
        let entry = parse_line("WARN: retrying request", InputFormat::Text).unwrap();
        assert!(entry.timestamp.is_none());
        assert_eq!(entry.level, Level::Warn);
    }

    #[test]
    fn parses_json_and_aliases() {
        let entry = parse_line(
            r#"{"time":"2026-10-22T10:00:00.123Z","severity":"ERR","msg":"timeout","request_id":42}"#,
            InputFormat::Auto,
        )
        .unwrap();
        assert_eq!(entry.level, Level::Error);
        assert!(entry.timestamp.is_some());
        assert_eq!(entry.message, "timeout");
        assert!(
            parse_line(r#"{"level":"info","message":"ready"}"#, InputFormat::Json)
                .unwrap()
                .timestamp
                .is_none()
        );
    }

    #[test]
    fn rejects_malformed_records() {
        for line in [
            "",
            "unstructured text",
            "INFO",
            "2026-99-99 INFO bad date",
            "{}",
            "[1,2]",
            r#"[null,"error","array record"]"#,
            r#"{"level":"error","message":42}"#,
            r#"{"level":"error","message":"bad","timestamp":"yesterday"}"#,
            r#"{"level":"unknown","message":"bad"}"#,
            r#"{"level":"info","message":"ok"} trailing"#,
        ] {
            assert!(parse_line(line, InputFormat::Auto).is_err(), "{line}");
        }
    }
}
