# Blog API

This Go service stores blog posts in a local SQLite file. It creates the database and posts table on startup. Go 1.26 or newer is required. The SQLite driver is pure Go.

```sh
cd blog-api
go run .
```

The server listens on `127.0.0.1:8080` and uses `blog.db` in the current directory. Pass `-addr` and `-db` to change those defaults.

| Method | Path | Response |
| --- | --- | --- |
| `GET` | `/posts` | `200` with an array of posts, newest first |
| `POST` | `/posts` | `201` with the new post and its URL in `Location` |
| `GET` | `/posts/{id}` | `200` with the post |
| `PUT` | `/posts/{id}` | `200` with the replaced post |
| `DELETE` | `/posts/{id}` | `204` with an empty body |

List posts with `?limit=20&offset=0`. The limit defaults to 20 and accepts 1–100. The offset defaults to 0.

Create and update requests take a JSON object with non-empty `title` and `content` strings. Bodies are limited to 1 MiB. Each post includes an integer `id` and UTC timestamps named `created_at` and `updated_at`. Invalid input returns `400`, oversized bodies return `413`, and a missing post returns `404`.

```sh
curl -i http://127.0.0.1:8080/posts \
  -H 'Content-Type: application/json' \
  -d '{"title":"First post","content":"Hello, SQLite!"}'

curl http://127.0.0.1:8080/posts
```
