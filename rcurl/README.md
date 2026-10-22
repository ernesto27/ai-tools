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

Additional request options:

- `--json '{"name":"Rust"}'` sends a literal JSON body and selects POST unless
  `-X` overrides it. It defaults both Content-Type and Accept to
  `application/json`; explicit `-H` values take precedence. JSON is neither
  validated nor read from files. `--json` conflicts with `-d`.
- `-u user:password` sends HTTP Basic authentication immediately. Split is at
  the first colon; a missing colon means an empty password, with no prompt.
  An explicit Authorization header conflicts with `-u`.
- `-f` makes final HTTP 4xx/5xx responses fail before printing headers or body
  or creating/truncating an output file. Without it, HTTP errors retain the
  existing successful exit and body output.
- `-I` sends HEAD and prints response status and headers to stdout. It conflicts
  with `-X`, `-d`, and `--json`. There is no response body; `-o` is ignored.
  `-i` does not duplicate the headers.
- `--retry N` allows N additional attempts (default 0, maximum 10), only for
  GET, HEAD, PUT, DELETE, OPTIONS, and TRACE. POST, PATCH, and custom methods
  are never retried. Connection errors, timeouts before response headers,
  and HTTP 408, 429, 500, 502, 503, 504 are transient. Body transfer errors
  are not retried because output may already have been written.
  Retry decisions happen before `-f` and before output; only the final response
  is printed. Each attempt has its own timeout and redirect limit.
  Delays use Retry-After seconds or HTTP dates (past dates mean zero);
  absent/invalid values use exponential backoff starting at 1 second, capped
  at 32 seconds. Total retry sleeping is limited to 60 seconds: if the next
  delay exceeds the remaining budget, the final response/error is returned
  without retrying early. Redirects retain reqwest's method handling.

```sh
rcurl --json '{"name":"Rust"}' -u user:password https://example.com/items
rcurl -X PUT --json '{"name":"Rust"}' --retry 3 -f https://example.com/items/1
rcurl -I -L https://example.com
rcurl --retry 2 -f -o page.html https://example.com
```
