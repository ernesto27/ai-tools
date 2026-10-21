# Taskline

A local-first task app with a terminal-inspired web interface, an Axum JSON API, SQLite persistence, and an interactive Rust terminal interface. Create, edit, complete, reopen, delete, and filter tasks in either interface. The web app also includes notes, lists, priorities, search, sorting, and keyboard shortcuts.

## Run

Requires a Rust toolchain and a C compiler (for bundled SQLite). From this directory:

```sh
cargo run
```

Open http://127.0.0.1:3000. No Node build or database setup is required. The database is automatically created at `taskline.sqlite3` in the working directory. It starts empty; no sample tasks are inserted. Fonts use Google Fonts with system fallbacks.

```sh
cargo run -- --tui
```

The TUI shares the same database when started from the same directory. Use arrows to select, `n` to create, `e` to rename, Space to complete/reopen, `d` to delete with confirmation, `f` to cycle status filters, and `q` to quit. Web notes, lists, and priorities are preserved when editing in the TUI.

Override `DATABASE_PATH` to select a database file and `BIND_ADDR` to select the server address. Default binding is loopback; this single-user app has no authentication. Keep it local or protect it with an authenticated reverse proxy.

Web shortcuts: `n` creates a task, `/` focuses search, `?` opens help, Escape closes dialogs. Tasks are only shown as saved after the server confirms the database write. Errors leave edits available to retry.

## JSON API

- `GET /api/tasks?status=all|active|completed`
- `POST /api/tasks` creates a task (201).
- `PUT /api/tasks/{id}` replaces editable fields (200).
- `DELETE /api/tasks/{id}` deletes a task (204).

Create/update body:

```json
{"title":"Read a chapter","notes":"Start small","category":"Learning","priority":"Low","completed":false}
```

Only `title` is required. Defaults: empty notes, Personal list, Medium priority, active status. Lists: Work, Personal, Learning. Priorities: Low, Medium, High. Titles are trimmed and limited to 200 characters; notes to 2,000. Invalid fields/status return 400, missing tasks 404. Malformed JSON is rejected by Axum. Query parameters are validated and SQL values are bound parameters.

## Verify

```sh
cargo fmt --check
cargo test
cargo build --release
```

Tests exercise the API lifecycle, filtering, input rejection, missing tasks, and persistence across database reopening. Static frontend assets are embedded in the binary; rebuild after changing them.
