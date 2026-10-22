use std::io::{self, BufRead};

use crate::cli::InputFormat;
use crate::filtering::Filter;
use crate::parsing::parse_line;
use crate::reporting::{Reporter, Summary};

/// Includes the line ending, if present. Oversized lines are drained without allocation.
pub const MAX_LINE_BYTES: usize = 1024 * 1024;

fn read_bounded_line(reader: &mut impl BufRead, line: &mut Vec<u8>) -> io::Result<bool> {
    line.clear();
    let mut found_input = false;
    loop {
        let available = reader.fill_buf()?;
        if available.is_empty() {
            return Ok(found_input);
        }
        found_input = true;
        let newline = available.iter().position(|&byte| byte == b'\n');
        let consumed = newline.map_or(available.len(), |index| index + 1);
        // One extra byte is sufficient to mark an oversized line.
        let retained = consumed.min((MAX_LINE_BYTES + 1).saturating_sub(line.len()));
        line.extend_from_slice(&available[..retained]);
        reader.consume(consumed);
        if newline.is_some() {
            return Ok(true);
        }
    }
}

pub fn analyze(
    mut reader: impl BufRead,
    format: InputFormat,
    filter: &Filter,
    strict: bool,
    top: usize,
) -> Result<Summary, String> {
    let mut reporter = Reporter::default();
    let mut line = Vec::new();
    while read_bounded_line(&mut reader, &mut line)
        .map_err(|err| format!("reading line {}: {err}", reporter.total_lines + 1))?
    {
        reporter.total_lines += 1;
        let entry = if line.len() > MAX_LINE_BYTES {
            Err(format!("line exceeds the {MAX_LINE_BYTES}-byte limit"))
        } else {
            std::str::from_utf8(&line)
                .map_err(|err| format!("invalid UTF-8: {err}"))
                .and_then(|line| parse_line(line, format))
        };
        match entry {
            Ok(entry) if filter.matches(&entry) => reporter.record(entry),
            Ok(_) => {}
            Err(error) if strict => {
                return Err(format!("line {}: {error}", reporter.total_lines));
            }
            Err(_) => reporter.malformed_lines += 1,
        }
    }
    Ok(reporter.finish(top))
}

#[cfg(test)]
mod tests {
    use std::io::{BufReader, Cursor, Read};

    use super::*;

    #[test]
    fn drains_oversized_lines_and_recovers_at_next_line() {
        let input = Cursor::new(vec![b'x'; MAX_LINE_BYTES + 100])
            .chain(Cursor::new(b"\nERROR recovered\r\nINFO no final newline"));
        let result = analyze(
            BufReader::with_capacity(17, input),
            InputFormat::Auto,
            &Filter::default(),
            false,
            10,
        )
        .unwrap();
        assert_eq!(result.total_lines, 3);
        assert_eq!(result.malformed_lines, 1);
        assert_eq!(result.matched_lines, 2);
    }

    #[test]
    fn invalid_utf8_is_malformed() {
        let result = analyze(
            Cursor::new(b"INFO valid\nERROR \xff\n"),
            InputFormat::Auto,
            &Filter::default(),
            false,
            10,
        )
        .unwrap();
        assert_eq!(result.malformed_lines, 1);
        assert_eq!(result.matched_lines, 1);
    }

    #[test]
    fn read_errors_have_line_context() {
        struct FailingReader;
        impl Read for FailingReader {
            fn read(&mut self, _: &mut [u8]) -> io::Result<usize> {
                Err(io::Error::other("simulated read failure"))
            }
        }
        let error = analyze(
            BufReader::new(FailingReader),
            InputFormat::Auto,
            &Filter::default(),
            false,
            10,
        )
        .unwrap_err();
        assert!(error.contains("reading line 1"));
        assert!(error.contains("simulated read failure"));
    }
}
