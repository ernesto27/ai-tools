# Blog API

A small blog API built with Go's `net/http` package and SQLite. Posts are stored in `blog.db` in the current working directory by default. Set `DB_PATH` to choose another database file.

## Start the service

```sh
cd go-api
go run .
```

The service listens on port `8080`.

## Check health

```sh
curl -i http://localhost:8080/health
```

The endpoint returns HTTP 200 with `Content-Type: application/json` and this body:

```json
{"status":"ok"}
```

## Posts

- `GET /posts` lists posts.
- `POST /posts` creates a post with a JSON body such as `{"title":"First post","content":"Hello, blog!"}`.
- `GET /posts/{id}` reads one post.
- `DELETE /posts/{id}` deletes one post.

SQLite is provided by the pure-Go `modernc.org/sqlite` driver. Go downloads the module when you first run or build the service.
