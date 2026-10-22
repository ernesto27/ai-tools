use chrono::{DateTime, FixedOffset};

use crate::parsing::{Entry, Level};

#[derive(Debug, Default)]
pub struct Filter {
    pub levels: Vec<Level>,
    pub text: Option<String>,
    pub since: Option<DateTime<FixedOffset>>,
    pub until: Option<DateTime<FixedOffset>>,
}

impl Filter {
    pub fn matches(&self, entry: &Entry) -> bool {
        if !self.levels.is_empty() && !self.levels.contains(&entry.level) {
            return false;
        }
        if let Some(text) = &self.text
            && !entry.message.contains(text)
        {
            return false;
        }
        if self.since.is_some() || self.until.is_some() {
            let Some(timestamp) = entry.timestamp else {
                return false;
            };
            if self.since.is_some_and(|since| timestamp < since)
                || self.until.is_some_and(|until| timestamp > until)
            {
                return false;
            }
        }
        true
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn entry(timestamp: Option<&str>, level: Level, message: &str) -> Entry {
        Entry {
            timestamp: timestamp.map(|time| time.parse().unwrap()),
            level,
            message: message.into(),
        }
    }

    #[test]
    fn combines_levels_and_literal_case_sensitive_text() {
        let filter = Filter {
            levels: vec![Level::Error, Level::Fatal],
            text: Some("timeout".into()),
            ..Filter::default()
        };
        assert!(filter.matches(&entry(None, Level::Error, "request timeout")));
        assert!(filter.matches(&entry(None, Level::Fatal, "timeout")));
        assert!(!filter.matches(&entry(None, Level::Info, "timeout")));
        assert!(!filter.matches(&entry(None, Level::Error, "Timeout")));
        assert!(!filter.matches(&entry(None, Level::Error, "ready")));
    }

    #[test]
    fn time_range_is_inclusive_and_compares_instants() {
        let filter = Filter {
            since: Some("2026-10-22T10:00:00Z".parse().unwrap()),
            until: Some("2026-10-22T11:00:00Z".parse().unwrap()),
            ..Filter::default()
        };
        for timestamp in ["2026-10-22T12:00:00+02:00", "2026-10-22T11:00:00Z"] {
            assert!(filter.matches(&entry(Some(timestamp), Level::Info, "ok")));
        }
        for timestamp in [
            None,
            Some("2026-10-22T09:59:59Z"),
            Some("2026-10-22T11:00:01Z"),
        ] {
            assert!(!filter.matches(&entry(timestamp, Level::Info, "ok")));
        }
    }

    #[test]
    fn supports_open_time_ranges_and_no_filters() {
        let log = entry(Some("2026-10-22T10:00:00Z"), Level::Debug, "ok");
        assert!(Filter::default().matches(&log));
        assert!(
            Filter {
                since: log.timestamp,
                ..Filter::default()
            }
            .matches(&log)
        );
        assert!(
            !Filter {
                until: Some("2026-10-22T09:00:00Z".parse().unwrap()),
                ..Filter::default()
            }
            .matches(&log)
        );
    }
}
