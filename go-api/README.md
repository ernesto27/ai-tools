# Blog API

A small Go HTTP API for blog posts. The service uses Gin for HTTP routing and stores posts in a local SQLite database.

## Start the service

From this directory, run:

```sh
go mod tidy
go run .
```

The server listens on port `8080` and creates `blog.db` in the current directory. Set `DATABASE_PATH` to use a different database file.

## Check health

```sh
curl -i http://localhost:8080/health
```

The endpoint responds with HTTP 200, `Content-Type: application/json`, and `{"status":"ok"}`.

## Posts API

- `GET /posts` lists posts.
- `POST /posts` creates a post from `{"title":"...","content":"..."}`.
- `GET /posts/:id` retrieves one post.
- `PUT /posts/:id` replaces a post's title and content.
- `DELETE /posts/:id` deletes a post.
