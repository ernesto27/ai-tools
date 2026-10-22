# loglens

A small Rust CLI that reads local logs incrementally and summarizes matching entries. No database or service is required.

## Build and run

Requires stable Rust with Cargo. From this directory:

```sh
cargo build --release
./target/release/loglens samples/app.log
./target/release/loglens samples/app.jsonl --output json
cat samples/app.jsonl | ./target/release/loglens --level error,warn --text timeout
./target/release/loglens samples/app.log --since 2026-10-22T09:00:03Z --until 2026-10-22T09:00:05Z
./target/release/loglens --help
```

Omit the file argument or pass `-` for standard input. `--format auto` (default) detects text or JSON for each line, so mixed files are supported. Use `--format text` or `--format json` to require one format.

## Log formats

Text records contain an optional RFC 3339 timestamp, a level, and a nonempty message:

```text
2026-10-22T09:00:03Z ERROR database connection timeout
[2026-10-22T11:00:03+02:00] [ERROR] database connection timeout
WARN: retrying request
```

Timestamp and level tokens can be bracketed. Whitespace separates tokens; a colon after the level is accepted. Other unstructured text, date-only timestamps, and multiline stack-trace continuation lines are malformed. Each physical line is one record; blank lines are malformed too. LF and CRLF line endings and a final line without a newline are supported. Text messages lose surrounding whitespace.

JSON Lines records are objects with string `level` and `message` fields and an optional RFC 3339 `timestamp`:

```json
{"timestamp":"2026-10-22T09:00:03Z","level":"ERROR","message":"database connection timeout","request_id":"req-101"}
```

`time`/`ts`, `severity`, and `msg` are accepted aliases for `timestamp`, `level`, and `message`. Extra fields are ignored. A missing or null timestamp means the time is unknown; malformed supplied timestamps, unknown levels, non-string messages, and non-object records are malformed. JSON messages retain their content, including escaped newlines.

Levels are case insensitive in both formats and CLI arguments. Canonical levels and aliases:

| Canonical | Other accepted names |
| --- | --- |
| trace | — |
| debug | dbg |
| info | information, notice |
| warn | warning |
| error | err |
| fatal | critical, crit, panic, emerg, emergency, alert |

## Filters and reports

- `--level error,warn` selects exact levels, with aliases normalized. Repeat `--level` to add more levels. Fatal entries require `--level fatal` if a level filter is active.
- `--text timeout` searches the message for a literal, case-sensitive substring.
- `--since` and `--until` take RFC 3339 timestamps and include their endpoints. Offsets are compared as instants. Either bound can be used alone. Entries without timestamps are excluded when a time bound is active. A reversed range is an argument error.
- Filters combine with AND; multiple levels combine with OR. All input is parsed, including records that would not match filters.
- `--top N` displays up to N frequent error/fatal messages, default 10, allowed range 1–1000. Identical messages across error and fatal entries are combined. Ties sort lexicographically by message.

Human output is the default; `--output json` produces one JSON summary on stdout. Diagnostics go to stderr. `total_lines`, `parsed_lines`, and `malformed_lines` describe all input; `matched_lines`, `counts_by_level`, and `top_errors` describe matching entries only. All six canonical levels are always included. For example, `samples/app.jsonl` produces 7 parsed entries, 2 errors, 1 fatal entry, and 2 occurrences of `database connection timeout`.

Malformed lines are counted and skipped by default. `--strict` fails on the first malformed line with source and line-number context and emits no summary. `samples/malformed.jsonl` demonstrates recovery after damaged records.

## Memory limits and frequency accuracy

Input is consumed one line at a time. A physical line is limited to **1 MiB (1,048,576 bytes), including its line ending**. Larger lines are drained without retaining the excess and count as malformed; invalid UTF-8 is also malformed. Strict mode fails on either condition.

Error aggregation keeps the **first 1,000 distinct matching error/fatal messages**, each at most **4,096 UTF-8 bytes**, for at most 4,096,000 bytes of retained message content plus map/counter overhead. Longer messages and new distinct messages beyond capacity do not enter the frequency table; existing tracked messages keep accumulating exact counts. No key is truncated or merged. `--top` changes the displayed count, not the tracking capacity. Input size and the number of unique messages cannot grow this table beyond these limits.

Level and line counts always cover all applicable records. If any matching errors cannot be tracked, human output warns and JSON sets `error_aggregation.complete` to `false` and reports `untracked_occurrences`. The top-errors list then ranks only tracked messages and can omit a globally frequent message. `tracked_messages`, `max_distinct_messages`, and `max_message_bytes` expose the table limits in JSON. Only aggregate data and a bounded current line are retained; input is never loaded as a whole.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Completed, including empty input, no matches, or skipped malformed lines; also help/version |
| 1 | File/read/write failure or malformed input with `--strict` |
| 2 | Invalid CLI arguments, including invalid timestamps or reversed time range |

## Development

The modules separate CLI arguments (`cli`), log records (`parsing`), predicates (`filtering`), bounded input processing (`processing`), and aggregation/output (`reporting`).

```sh
cargo fmt --check
cargo clippy --all-targets -- -D warnings
cargo test
```

Unit tests cover log parsing, filters, memory bounds, and read failures. Integration tests exercise the binary with files and stdin, human/JSON reports, malformed and oversized lines, argument validation, aggregation limits, and exit codes.
