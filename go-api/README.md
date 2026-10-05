# Blog API

A small JSON API built with Gin and SQLite. It stores blog posts in `blog.db` in
the current directory by default.

## Requirements

- Go 1.22 or newer
- A C compiler for the SQLite driver (`github.com/mattn/go-sqlite3`)

## Start the service

From this directory, download the Go modules and start the API:

```sh
go mod tidy
go run .
```

The service listens on port `8080`. Set `DATABASE_PATH` to use another SQLite
database file.

## Check health

```sh
curl -i http://localhost:8080/health
```

The endpoint returns HTTP 200 and `{"status":"ok"}` as JSON.

## Posts API

- `GET /posts` lists posts.
- `POST /posts` creates a post; send `{"title":"Hello","body":"First post"}`.
- `GET /posts/:id` reads a post.
- `PUT /posts/:id` replaces its title and body using the same JSON shape.
- `DELETE /posts/:id` deletes a post.
