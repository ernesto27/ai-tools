# Tasks API

A Go HTTP API with an in-memory store. Tasks are cleared when the server restarts.

From `tasks-api/`, run `go run .` with Go 1.22 or later. The server listens on port 8080.
Run the smoke tests with `go test ./...` from the same directory.

## API contract

Tasks are JSON objects with a server-assigned integer `id` and a string `title`.

| Request | Response |
| --- | --- |
| `POST /tasks` with `{"title":"example"}` | `201 Created` with the created task. Malformed JSON or a blank title returns `400 Bad Request`. |
| `GET /tasks/{id}` | `200 OK` with the task, or `404 Not Found` if it does not exist. |
| `GET /tasks?limit=N` | `200 OK` with a JSON array of at most `N` tasks, ordered by ascending ID. The default is 20. Values outside the integer range 1–100 return `400 Bad Request`. |
| `DELETE /tasks/{id}` | `204 No Content` with an empty body, including when the same ID is deleted repeatedly. |
