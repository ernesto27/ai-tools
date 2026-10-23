use std::collections::BTreeMap;
use std::io::{self, Write};

use serde::Serialize;

use crate::cli::OutputFormat;
use crate::parsing::{Entry, Level};

pub const MAX_ERROR_MESSAGES: usize = 1000;
pub const MAX_ERROR_MESSAGE_BYTES: usize = 4096;

#[derive(Debug, Serialize)]
pub struct ErrorCount {
    pub message: String,
    pub count: u64,
}

#[derive(Debug, Serialize)]
pub struct ErrorAggregation {
    pub max_distinct_messages: usize,
    pub max_message_bytes: usize,
    pub tracked_messages: usize,
    pub untracked_occurrences: u64,
    pub complete: bool,
}

#[derive(Debug, Serialize)]
pub struct Summary {
    pub total_lines: u64,
    pub parsed_lines: u64,
    pub malformed_lines: u64,
    pub matched_lines: u64,
    pub counts_by_level: BTreeMap<Level, u64>,
    pub top_errors: Vec<ErrorCount>,
    pub error_aggregation: ErrorAggregation,
}

pub struct Reporter {
    pub total_lines: u64,
    pub malformed_lines: u64,
    matched_lines: u64,
    counts: BTreeMap<Level, u64>,
    errors: BTreeMap<String, u64>,
    untracked_errors: u64,
}

impl Default for Reporter {
    fn default() -> Self {
        Self {
            total_lines: 0,
            malformed_lines: 0,
            matched_lines: 0,
            counts: [
                Level::Trace,
                Level::Debug,
                Level::Info,
                Level::Warn,
                Level::Error,
                Level::Fatal,
            ]
            .into_iter()
            .map(|level| (level, 0))
            .collect(),
            errors: BTreeMap::new(),
            untracked_errors: 0,
        }
    }
}

impl Reporter {
    pub fn record(&mut self, entry: Entry) {
        self.matched_lines += 1;
        *self.counts.entry(entry.level).or_default() += 1;
        if !matches!(entry.level, Level::Error | Level::Fatal) {
            return;
        }
        if let Some(count) = self.errors.get_mut(&entry.message) {
            *count += 1;
        } else if self.errors.len() < MAX_ERROR_MESSAGES
            && entry.message.len() <= MAX_ERROR_MESSAGE_BYTES
        {
            self.errors.insert(entry.message, 1);
        } else {
            self.untracked_errors += 1;
        }
    }

    pub fn finish(self, top: usize) -> Summary {
        let tracked_messages = self.errors.len();
        let mut top_errors: Vec<_> = self
            .errors
            .into_iter()
            .map(|(message, count)| ErrorCount { message, count })
            .collect();
        top_errors.sort_unstable_by(|a, b| {
            b.count
                .cmp(&a.count)
                .then_with(|| a.message.cmp(&b.message))
        });
        top_errors.truncate(top);
        Summary {
            total_lines: self.total_lines,
            parsed_lines: self.total_lines - self.malformed_lines,
            malformed_lines: self.malformed_lines,
            matched_lines: self.matched_lines,
            counts_by_level: self.counts,
            top_errors,
            error_aggregation: ErrorAggregation {
                max_distinct_messages: MAX_ERROR_MESSAGES,
                max_message_bytes: MAX_ERROR_MESSAGE_BYTES,
                tracked_messages,
                untracked_occurrences: self.untracked_errors,
                complete: self.untracked_errors == 0,
            },
        }
    }
}

pub fn write_report(
    mut writer: impl Write,
    summary: &Summary,
    format: OutputFormat,
) -> io::Result<()> {
    match format {
        OutputFormat::Json => {
            serde_json::to_writer_pretty(&mut writer, summary).map_err(io::Error::other)?;
            writeln!(writer)
        }
        OutputFormat::Human => {
            writeln!(writer, "Lines read: {}", summary.total_lines)?;
            writeln!(writer, "Parsed: {}", summary.parsed_lines)?;
            writeln!(writer, "Malformed (skipped): {}", summary.malformed_lines)?;
            writeln!(writer, "Matched: {}", summary.matched_lines)?;
            writeln!(writer, "\nCounts by level (matched entries):")?;
            for (level, count) in &summary.counts_by_level {
                writeln!(writer, "  {level:<5} {count}")?;
            }
            writeln!(writer, "\nMost frequent error/fatal messages:")?;
            if summary.top_errors.is_empty() {
                writeln!(writer, "  (none)")?;
            }
            for error in &summary.top_errors {
                // Debug quoting keeps multiline/control characters on one output line.
                writeln!(writer, "  {}  {:?}", error.count, error.message)?;
            }
            if !summary.error_aggregation.complete {
                writeln!(
                    writer,
                    "Warning: error aggregation limit reached; {} error occurrences were not tracked. Top errors may be incomplete.",
                    summary.error_aggregation.untracked_occurrences
                )?;
            }
            Ok(())
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn error(message: String) -> Entry {
        Entry {
            timestamp: None,
            level: Level::Error,
            message,
        }
    }

    #[test]
    fn bounded_aggregation_keeps_counting_existing_messages() {
        let mut reporter = Reporter::default();
        for index in 0..MAX_ERROR_MESSAGES + 10 {
            reporter.record(error(format!("error {index:04}")));
        }
        reporter.record(error("error 0000".into()));
        reporter.record(error("x".repeat(MAX_ERROR_MESSAGE_BYTES + 1)));
        let summary = reporter.finish(2);
        assert_eq!(
            summary.error_aggregation.tracked_messages,
            MAX_ERROR_MESSAGES
        );
        assert_eq!(summary.error_aggregation.untracked_occurrences, 11);
        assert!(!summary.error_aggregation.complete);
        assert_eq!(summary.top_errors[0].count, 2);
        assert_eq!(summary.top_errors[0].message, "error 0000");
        assert_eq!(summary.top_errors[1].message, "error 0001");
        assert_eq!(summary.counts_by_level[&Level::Error], 1012);
    }

    #[test]
    fn message_limit_uses_bytes_and_never_truncates_keys() {
        let mut reporter = Reporter::default();
        reporter.record(error("é".repeat(MAX_ERROR_MESSAGE_BYTES / 2)));
        reporter.record(error("é".repeat(MAX_ERROR_MESSAGE_BYTES / 2 + 1)));
        let summary = reporter.finish(10);
        assert_eq!(summary.error_aggregation.tracked_messages, 1);
        assert_eq!(summary.error_aggregation.untracked_occurrences, 1);
    }
}
