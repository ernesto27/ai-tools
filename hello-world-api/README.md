# Hello World API

A minimal Go HTTP API using only the standard library. Requires Go 1.22 or later.

From this directory, start the server:

```sh
go run .
```

In another terminal, call the API:

```sh
curl http://localhost:8080/
```

`GET /` returns HTTP 200 with `Content-Type: application/json` and this body:

```json
{"message":"Hello, world!"}
```

Unknown paths return HTTP 404. Unsupported methods on `/` return HTTP 405;
`HEAD /` is also supported. Stop the server with Ctrl+C.
