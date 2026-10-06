# Rust Todo Service

A small in-memory todo HTTP API written in Rust. It listens on port `8080`.

## Run

Install Rust and Cargo, then start the service from this directory:

```sh
cargo run
```

Check the health endpoint:

```sh
curl -i http://localhost:8080/health
```

It returns `200 OK` with `Content-Type: application/json` and `{"status":"ok"}`.

## Todo API

- `GET /todos` lists todos.
- `POST /todos` creates a todo. Send `{"title":"Buy milk"}` as JSON.
- `PUT /todos/{id}` updates a todo. Send either or both `title` and `completed` fields.
- `DELETE /todos/{id}` deletes a todo.

Todos are held in memory and reset whenever the service restarts. Todo titles cannot be empty. Unknown IDs return `404 Not Found`.
