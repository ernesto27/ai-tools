# Blog API

The Blog API is a Go HTTP service that stores posts in a local SQLite file. It uses Go's standard HTTP router and a pure-Go SQLite driver. Install Go 1.26 or newer to run it.

```sh
cd blog-api
go run .
```

The server creates the database and posts table on first run. Reuse the same database path across restarts to keep your posts.

| Environment variable | Default | Purpose |
| --- | --- | --- |
| `BLOG_ADDR` | `127.0.0.1:8080` | HTTP listen address |
| `BLOG_DB_PATH` | `blog.db` | SQLite file relative to the working directory, or an absolute path |

The database file's parent directory must exist. The API is intended for local use and has no authentication.

## Endpoints

| Method | Path | Result |
| --- | --- | --- |
| `GET` | `/posts` | `200` with a JSON array, newest first |
| `POST` | `/posts` | `201` with the created post and its URL in `Location` |
| `GET` | `/posts/{id}` | `200` with the post |
| `PUT` | `/posts/{id}` | `200` with the post after replacing its title and content |
| `DELETE` | `/posts/{id}` | `204` with an empty response |

`GET /posts` accepts `limit` from 1 to 100 (default 20) and `offset` starting at 0 (default 0). Each post has an integer `id`, `title`, `content`, `created_at`, and `updated_at`. Timestamps use RFC 3339 in UTC.

Send `POST` and `PUT` requests as `application/json` with nonblank string fields `title` and `content`. Bodies may be up to 1 MiB. Invalid input returns `400`, oversized bodies return `413`, and an unsupported content type returns `415`. Requests for a missing post return `404`.

```sh
curl -i http://127.0.0.1:8080/posts \
  -H 'Content-Type: application/json' \
  -d '{"title":"First post","content":"Hello from SQLite."}'

curl 'http://127.0.0.1:8080/posts?limit=10&offset=0'
```
