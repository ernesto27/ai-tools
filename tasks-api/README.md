# Tasks API

Run `go run .` from this directory with Go 1.22 or later. The server listens
on port 8080 and stores tasks in memory until it exits. Each task has a
positive integer `id` and a string `title`, for example
`{"id":1,"title":"example"}`.

| Request | Response |
| --- | --- |
| `POST /tasks` with `{"title":"example"}` | `201 Created` with the created task as JSON. Malformed JSON, a blank title, or a title longer than 200 characters returns `400 Bad Request`. A request body larger than 1 MiB returns `413 Request Entity Too Large`. |
| `GET /tasks/{id}` | `200 OK` with the task as JSON, or `404 Not Found` if it does not exist. |
| `GET /tasks?limit=N` | `200 OK` with a JSON array of at most N tasks in ascending ID order. The default is 20. A limit that is not an integer from 1 to 100 returns `400 Bad Request`. |
| `DELETE /tasks/{id}` | `204 No Content` with an empty body. Repeating the delete for the same ID also returns `204 No Content`. |

Run `go test ./...` from this directory to execute the smoke tests.
