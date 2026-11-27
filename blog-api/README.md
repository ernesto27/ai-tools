# Blog API

A small JSON blog API written in Go with SQLite persistence. Go 1.22 or newer is required.

```sh
cd blog-api
go run .
```

The server listens on `:8080` and creates `blog.db` in the working directory. Set `ADDR` and `DATABASE_PATH` to change these defaults. Keep the database file outside a publicly served directory.

| Method | Path | Description |
| --- | --- | --- |
| GET | `/posts` | List posts, newest ID first |
| POST | `/posts` | Create a post |
| GET | `/posts/{id}` | Get a post |
| PUT | `/posts/{id}` | Replace a post's title and content |
| DELETE | `/posts/{id}` | Delete a post |

Create or replace a post with JSON such as `{"title":"Hello","content":"My first post"}`. Both fields must be non-empty strings. Responses include the ID and UTC creation and update timestamps. Creation returns 201 and a `Location` header; deletion returns 204. Invalid input returns 400, and a missing post returns 404, with a JSON `error` message.

```sh
curl -X POST http://localhost:8080/posts -H 'Content-Type: application/json' -d '{"title":"Hello","content":"My first post"}'
curl http://localhost:8080/posts
```

Run tests with `go test ./...`. This API does not include authentication; protect it before exposing it to untrusted clients.
