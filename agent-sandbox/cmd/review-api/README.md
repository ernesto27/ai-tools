# Review API

A small in-memory task API that listens on `:8080`. Task records have an integer
`id`, a string `title`, and a boolean `completed` field. New tasks receive a
generated ID and start with `completed` set to `false`.

## Run

From the `agent-sandbox` module directory:

```sh
go run ./cmd/review-api
```

## Contract

- `GET /health` returns `200` and `{"status":"ok"}`.
- `POST /tasks` creates a task and returns `201` with its JSON representation.
- `GET /tasks` returns `200` with a JSON array, including `[]` when empty.
- `GET /tasks/{id}` returns the requested task or `404` if it does not exist.
- `DELETE /tasks/{id}` removes the requested task and returns `204` with an empty body, or `404` if it does not exist.
- Missing or whitespace-only titles, malformed JSON, and invalid IDs return `400` with a JSON error.
- Unsupported HTTP methods return `405`.
- JSON responses use `Content-Type: application/json`.

## Examples

Create a task:

```sh
curl -i -X POST http://localhost:8080/tasks \
  -H 'Content-Type: application/json' \
  -d '{"title":"Review the API"}'
```

List tasks:

```sh
curl -i http://localhost:8080/tasks
```

Fetch a task (replace `1` with its ID):

```sh
curl -i http://localhost:8080/tasks/1
```

Delete a task (replace `1` with its ID):

```sh
curl -i -X DELETE http://localhost:8080/tasks/1
```
