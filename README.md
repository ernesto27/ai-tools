# Blog API

A simple Go API using the standard library. Posts are stored in memory and are
lost when the server restarts. The API has no authentication.

Requires Go 1.22 or later. Start it from this directory:

```sh
go run .
```

The default address is `:8080`. Set `ADDR` to override it, for example
`ADDR=127.0.0.1:9000 go run .`.

| Method | Path | Behavior |
| --- | --- | --- |
| GET | `/posts` | List posts in ascending ID order (an empty list is `[]`) |
| POST | `/posts` | Create a post; returns 201 and a `Location` header |
| GET | `/posts/{id}` | Get a post |
| PUT | `/posts/{id}` | Replace a post's title and content |
| DELETE | `/posts/{id}` | Delete a post; returns 204 with no body |

Create and update requests accept a JSON object with nonblank `title` and
`content` strings. Leading and trailing whitespace is trimmed. Unknown fields,
multiple JSON values, and bodies over 1 MiB are rejected. Responses contain
`id`, `title`, `content`, `created_at`, and `updated_at` (UTC timestamps).
Errors return JSON such as `{"error":"post not found"}` with the corresponding
400, 404, 405, or 413 status.

```sh
curl -i http://localhost:8080/posts \
  -H 'Content-Type: application/json' \
  -d '{"title":"First post","content":"Hello, world!"}'
curl http://localhost:8080/posts
curl http://localhost:8080/posts/1
curl -X PUT http://localhost:8080/posts/1 \
  -H 'Content-Type: application/json' \
  -d '{"title":"Updated post","content":"New content"}'
curl -i -X DELETE http://localhost:8080/posts/1
```

Run checks with `go test ./...` and `go vet ./...`. To check for data races,
run `CGO_ENABLED=1 go test -race ./...` with a C compiler installed.
