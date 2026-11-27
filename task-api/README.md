# Task API

This Go HTTP API uses `net/http` and stores tasks in memory. Restarting the server clears all tasks.

From this directory, run `go run .` to serve the API at `http://localhost:8080`. Run `go test ./...` to run the tests. Go 1.22 or later is required.

## API contract

A task is a JSON object with a positive integer `id` and a string `title`, for example `{"id":1,"title":"example"}`. The server assigns IDs in increasing order.

| Request | Response |
| --- | --- |
| `POST /tasks` with `{"title":"example"}` | `201 Created` with the created task as JSON. Malformed JSON or a blank title returns `400 Bad Request`. |
| `GET /tasks/{id}` | `200 OK` with the task as JSON, or `404 Not Found` if the task does not exist. |
| `GET /tasks?limit=N` | `200 OK` with a JSON array of at most `N` tasks in ascending ID order. The default limit is 20. A limit outside 1–100 or an invalid integer returns `400 Bad Request`. |
| `DELETE /tasks/{id}` | `204 No Content` with an empty body, including when the same ID is deleted again. |
