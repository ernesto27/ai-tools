# Blog API

A small Go blog API built with Gin and backed by a local SQLite database.

## Requirements

- Go 1.22 or later

## Start the service

From this directory, download the Go modules and start the API:

```sh
go mod tidy
go run .
```

The service listens on port `8080` and creates `blog.db` in the current directory. Set `PORT` to change the listening port or `DB_PATH` to choose a different SQLite file.

## Check health

```sh
curl -i http://localhost:8080/health
```

The response is HTTP 200 with `Content-Type: application/json` and body `{"status":"ok"}`.

## Blog endpoints

- `GET /api/posts` lists posts.
- `POST /api/posts` creates a post with `{"title":"...","content":"..."}`.
- `GET /api/posts/:id` retrieves a post.
- `PUT /api/posts/:id` updates a post with `{"title":"...","content":"..."}`.
- `DELETE /api/posts/:id` deletes a post.
