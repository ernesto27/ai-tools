# Taskline

A local-first command-line todo app built with Rust and SQLite. Each command prints its result and exits. It works in scripts and pipes without an interactive terminal.

## Setup

Install a current stable Rust toolchain and a C compiler (for bundled SQLite):

```sh
cd todo
cargo build --release
./target/release/taskline --help
```

The database is created automatically at `taskline.sqlite3` in the working directory when a task command runs. Set `DATABASE_PATH=/path/to/tasks.sqlite3` to choose another file. Existing Taskline databases retain all tasks and metadata without migration. Help does not open a database; no sample tasks are inserted.

## Commands

```sh
# Alternatively, use cargo run -- <command> [arguments].
./target/release/taskline add "Write documentation" --category Work --priority High --notes "Include examples"
./target/release/taskline list
./target/release/taskline list active
./target/release/taskline show 1
./target/release/taskline edit 1 --title "Review documentation" --notes ""
./target/release/taskline complete 1
./target/release/taskline list completed
./target/release/taskline reopen 1
./target/release/taskline delete 1
```

`list` accepts `all` (the default), `active`, or `completed` and prints tab-separated ID, status, category, priority, and title. `show` prints all fields, including notes and creation time. Control characters in stored text are displayed as spaces to keep terminal output safe and each record on one line.

`add` accepts a quoted title and optional `--notes`, `--category`, and `--priority`. Categories are `Personal` (default), `Work`, and `Learning`; priorities are `Low`, `Medium` (default), and `High`. `edit` accepts the same options plus `--title`, requires at least one option, and preserves omitted fields and completion state. Pass `--notes ""` to clear notes.

Titles are trimmed and must contain 1–200 characters. Notes allow up to 2,000 characters. IDs must be positive integers. Invalid arguments and database errors print to stderr and exit with status 1; successful commands exit with status 0. Deletion is immediate, with no confirmation prompt. No arguments, `help`, `-h`, and `--help` display usage. There is no TUI or web server.

## Verify

```sh
cargo fmt --check
cargo test
cargo clippy --all-targets -- -D warnings
cargo build --release
```

Integration tests invoke the CLI without a terminal and check persistence, task lifecycle, filters, partial edits, argument validation, and error exit codes.
