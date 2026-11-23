# Hello API

A minimal Go HTTP API using only the standard library. Requires Go 1.22 or newer.

From the repository root, start the server:

```sh
cd hello-api
go run .
```

In another terminal:

```sh
curl -i http://localhost:8080/
```

`GET /` returns HTTP 200 with `Content-Type: application/json` and:

```json
{"message":"Hello, world!"}
```

Other paths return HTTP 404. Unsupported methods at `/` return HTTP 405.
Stop the server with Ctrl+C.
