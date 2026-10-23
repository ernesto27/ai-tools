use std::path::PathBuf;

use chrono::{DateTime, FixedOffset};
use clap::{Parser, ValueEnum};

use crate::parsing::Level;
use crate::reporting::MAX_ERROR_MESSAGES;

#[derive(Clone, Copy, Debug, ValueEnum)]
pub enum InputFormat {
    Auto,
    Text,
    Json,
}

#[derive(Clone, Copy, Debug, ValueEnum)]
pub enum OutputFormat {
    Human,
    Json,
}

#[derive(Debug, Parser)]
#[command(
    version,
    about = "Analyze local logs with bounded memory",
    after_help = "Examples:\n  loglens app.log --level error,warn --text timeout\n  cat app.jsonl | loglens --since 2026-10-22T09:00:00Z --output json\n\nExit codes: 0 = completed (including no matches or skipped malformed lines),\n1 = input/output failure or malformed input in strict mode, 2 = invalid arguments.\n\nText: [RFC3339 timestamp] LEVEL message, or LEVEL message (brackets optional).\nJSON: object with level, message, and optional RFC3339 timestamp.\nLimits: 1 MiB per line; first 1000 distinct error/fatal messages up to 4096 bytes\neach are tracked. Reports disclose untracked errors; top counts may be incomplete."
)]
pub struct Cli {
    /// Input file; omit or use '-' to read standard input
    pub file: Option<PathBuf>,

    /// Input format; auto detects JSON objects or text per line
    #[arg(long, value_enum, default_value_t = InputFormat::Auto)]
    pub format: InputFormat,

    /// Include these levels (comma separated or repeated; case insensitive)
    #[arg(long, value_delimiter = ',', value_parser = parse_level)]
    pub level: Vec<Level>,

    /// Include messages containing this literal, case-sensitive text
    #[arg(long)]
    pub text: Option<String>,

    /// Inclusive start time (RFC 3339); excludes entries without a timestamp
    #[arg(long, value_name = "TIMESTAMP")]
    pub since: Option<DateTime<FixedOffset>>,

    /// Inclusive end time (RFC 3339); excludes entries without a timestamp
    #[arg(long, value_name = "TIMESTAMP")]
    pub until: Option<DateTime<FixedOffset>>,

    /// Fail on the first malformed line instead of counting and skipping it
    #[arg(long)]
    pub strict: bool,

    /// Report format
    #[arg(long, value_enum, default_value_t = OutputFormat::Human)]
    pub output: OutputFormat,

    /// Number of frequent error/fatal messages to display (1-1000)
    #[arg(long, default_value = "10", value_parser = parse_top)]
    pub top: usize,
}

fn parse_level(value: &str) -> Result<Level, String> {
    value.parse()
}

fn parse_top(value: &str) -> Result<usize, String> {
    let top = value
        .parse::<usize>()
        .map_err(|_| "expected a positive integer".to_string())?;
    if !(1..=MAX_ERROR_MESSAGES).contains(&top) {
        return Err(format!("must be between 1 and {MAX_ERROR_MESSAGES}"));
    }
    Ok(top)
}

impl Cli {
    pub fn validate(&self) -> Result<(), String> {
        if let (Some(since), Some(until)) = (self.since, self.until)
            && since > until
        {
            return Err("--since must be earlier than or equal to --until".into());
        }
        Ok(())
    }
}
