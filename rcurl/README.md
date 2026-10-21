# rcurl

A basic curl-like Rust CLI using clap and reqwest. HTTP and HTTPS are supported;
TLS certificates and hostnames are verified using rustls with bundled public roots.
Response bodies are streamed without buffering the entire response in memory.

Build and install with a Rust toolchain:

```sh
cd rcurl
cargo build --release
cargo install --path .
```

Examples:

```sh
rcurl https://example.com
rcurl -X DELETE https://example.com/items/1
rcurl -H 'Accept: application/json' -H 'X-Client: rcurl' https://example.com
rcurl -d 'hello world' https://example.com/echo
rcurl -X PUT -H 'Content-Type: application/json' -d '{"name":"Rust"}' https://example.com/items/1
rcurl -i https://example.com
rcurl -o page.html https://example.com
rcurl -L --timeout 60 --connect-timeout 5 https://example.com
rcurl --help
```

GET is the default; `-d` supplies a literal body and selects POST unless `-X`
overrides the method. Headers can be repeated, including multiple values for the
same name. `-d` does not read files or automatically set a content type. For a
body starting with a dash, use `--data=-value`.

Redirects are disabled by default. `-L` follows up to 10 redirects using reqwest's
method and sensitive-header handling. `--timeout` limits the entire request,
including body transfer (default 30 seconds); `--connect-timeout` limits connection
establishment (default 10 seconds). Both accept positive fractional seconds.

Bodies go to stdout, or the file specified by `-o` (overwriting it). `-i` writes
the final response status and headers to stdout even with `-o`. Diagnostics go to
stderr. Network, timeout, argument, and output errors return a nonzero exit code.
HTTP error statuses such as 404 still print their bodies and exit successfully.
An interrupted transfer may leave partial output or a partial file. Standard
HTTP proxy environment variables are honored.

Run local HTTP integration tests and checks:

```sh
cargo fmt --check
cargo test
cargo clippy --all-targets -- -D warnings
```
