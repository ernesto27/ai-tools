# Taskline

A local-first todo TUI built with Rust, Ratatui, Crossterm, and SQLite. All interaction happens in your terminal; no web server or browser is needed.

## Setup

Install a current stable Rust toolchain and a C compiler (for bundled SQLite). Run in an interactive terminal:

```sh
cd todo
cargo run --release
```

The database is created automatically at `taskline.sqlite3` in the working directory. Existing Taskline SQLite databases work without migration and retain titles, notes, categories, priorities, completion state, and timestamps. No sample tasks are inserted.

Choose a different database with:

```sh
DATABASE_PATH=/path/to/tasks.sqlite3 cargo run --release
```

The old `--tui` switch is unnecessary. Server configuration and JSON API endpoints have been removed.

## Keyboard controls

| Key | Action |
| --- | --- |
| ↑/↓ or k/j | Select and scroll tasks |
| n | Create a task |
| e | Edit the selected task |
| Space | Complete or reopen the selected task |
| d | Request deletion; y confirms, n or Esc cancels |
| f | Cycle All, Active, Completed filters |
| q or Esc | Quit from the task list |
| Ctrl-C | Quit from any screen |

The editor starts with existing values when editing. Type to append text and use Backspace to remove characters, including Unicode. Tab and Shift-Tab move between title, notes, category, and priority. Left/Right cycle categories (Personal, Work, Learning) and priorities (Low, Medium, High). Enter saves; Esc discards edits. Completion is preserved when editing.

Titles are trimmed and must contain 1–200 characters; notes allow up to 2,000 characters. Validation and database errors appear in the help area, keeping edits available for retry. Existing multiline notes are preserved; the editor accepts single-line typing. The list scrolls with selection, shows task details, and keeps keyboard help visible. Use a terminal of at least 80 columns and 24 rows for comfortable editing. Terminal mode is restored on normal exit, errors, and panics.

## Verify

```sh
cargo fmt --check
cargo test
cargo clippy --all-targets -- -D warnings
cargo build --release
```

Tests exercise keyboard-driven creation, editing, validation, cancellation, completion/reopening, deletion, filters, empty lists, rendering/help, small terminals, and SQLite persistence across reopening.
