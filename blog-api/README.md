# Blog API

A small JSON REST API for blog posts, backed by SQLite. Requires Go 1.26.8 or newer.

```sh
cd blog-api
go run .
```

The server listens on `:8080` and creates `blog.db` in the current directory. Set `ADDR` and `DATABASE_PATH` to override those defaults. The database schema is created on startup.

| Method | Path | Description |
| --- | --- | --- |
| GET | /posts | List posts, newest ID first |
| POST | /posts | Create a post (201 and Location header) |
| GET | /posts/{id} | Get a post |
| PUT | /posts/{id} | Replace a post's title and content |
| DELETE | /posts/{id} | Delete a post (204) |

Create and update accept a JSON object with non-empty `title` and `content` strings. Responses include `id`, `created_at`, and `updated_at` (UTC timestamps). Errors return a JSON `error` message; invalid input returns 400 and missing posts return 404.

```sh
curl -X POST http://localhost:8080/posts -H 'Content-Type: application/json' -d '{"title":"Hello","content":"My first post"}'
curl http://localhost:8080/posts
```

Run tests with `go test ./...`.
