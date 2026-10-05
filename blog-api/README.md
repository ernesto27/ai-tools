# Blog API

A small REST API for managing blog posts. It uses Go's standard library and
stores posts in a JSON file, so it can run without an external database.

## Run

```sh
cd blog-api
go run ./cmd/blog-api
```

The server listens on `:8080` by default. Set `PORT` to change the port and
`BLOG_DATA_FILE` to choose where the post data is stored. The default data file
is `data/posts.json` under the current working directory.

## Endpoints

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/health` | Liveness check |
| `GET` | `/api/posts` | List posts; accepts `limit`, `offset`, and `q` |
| `POST` | `/api/posts` | Create a post |
| `GET` | `/api/posts/{id}` | Get a post |
| `PUT` | `/api/posts/{id}` | Replace a post's editable fields |
| `DELETE` | `/api/posts/{id}` | Delete a post |

Create and update requests use JSON with `title`, `content`, and optional
`excerpt`, `author`, and `tags` fields. If `excerpt` is omitted, it is derived
from the content. Slugs are generated from titles and kept unique. Responses
use JSON; errors have the form `{"error":"..."}`.

Example:

```sh
curl -i http://localhost:8080/api/posts \
  -H 'Content-Type: application/json' \
  -d '{"title":"Hello world","content":"My first post","author":"Ada"}'
```
