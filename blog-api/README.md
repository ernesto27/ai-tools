# Blog API

A small JSON blog API written in Go with SQLite persistence. Requires Go 1.26.8 or newer.

```sh
cd blog-api
go run .
```

The server listens on `:8080` and creates `blog.db` in the current directory. Set `ADDR` and `DATABASE_PATH` to override these defaults.

| Method | Path | Description |
| --- | --- | --- |
| GET | /posts | List posts, newest ID first |
| POST | /posts | Create a post |
| GET | /posts/{id} | Get a post |
| PUT | /posts/{id} | Replace a post's title and content |
| DELETE | /posts/{id} | Delete a post |

Create or replace a post with JSON containing nonempty `title` and `content` strings:

```sh
curl -X POST http://localhost:8080/posts -H 'Content-Type: application/json' -d '{"title":"Hello","content":"My first post"}'
curl http://localhost:8080/posts
```

Creation returns `201` and a `Location` header; deletion returns `204`. Invalid input returns `400`, and a missing post returns `404`. Error responses contain an `error` string. Timestamps are UTC RFC 3339 strings.
