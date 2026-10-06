# Rust Todo Service

A small, dependency-free Rust HTTP service for managing todos. It listens on port `8080` and keeps todos in memory while the process runs.

## Run

Install Rust and Cargo, then start the service from this directory:

```sh
cargo run
```

## Health check

```sh
curl -i http://localhost:8080/health
```

The endpoint returns HTTP 200 with `Content-Type: application/json` and `{"status":"ok"}`.

## Todo API

- `GET /todos` lists todos.
- `POST /todos` creates a todo from `{"title":"Buy milk"}`.
- `GET /todos/{id}` returns one todo.
- `PATCH /todos/{id}` updates a supplied `title` and/or `completed` value.
- `PUT /todos/{id}` also updates supplied fields.
- `DELETE /todos/{id}` removes a todo.

For example:

```sh
curl -i -X POST http://localhost:8080/todos \
  -H 'Content-Type: application/json' \
  -d '{"title":"Buy milk"}'
curl -i http://localhost:8080/todos
```
