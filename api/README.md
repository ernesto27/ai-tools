# Go API

A standalone HTTP API using only the Go standard library. Requires Go 1.22 or later.

From this directory, start the server:

```sh
go run .
```

The default port is `8080`. Override it with `PORT=9090 go run .`.
The server listens on all interfaces and shuts down gracefully on SIGINT or SIGTERM.

Check its health:

```sh
curl -i http://localhost:8080/health
```

`GET /health` returns HTTP `200`, `Content-Type: application/json`, and:

```json
{"status":"ok"}
```

`HEAD /health` also returns `200` with no response body. Other methods return
`405`, and unknown paths return `404`. This endpoint reports that the HTTP
server is running; it does not check external dependencies.

Run checks and build (the binary is written outside the repository):

```sh
go test -race ./...
go vet ./...
go build -o /tmp/health-api .
```
