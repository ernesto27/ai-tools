# rust-wget

`rust-wget` downloads a single HTTP or HTTPS URL. It follows redirects and streams the response to a file or standard output.

Run these commands from this directory.

```sh
cargo build --locked
./target/debug/rust-wget https://example.org/
./target/debug/rust-wget -O local.html https://example.org/
./target/debug/rust-wget -O - https://example.org/
./target/debug/rust-wget --help
```

By default, the filename comes from the last path segment of the supplied URL. A path ending in `/` saves as `index.html`. If that filename exists, the downloader tries suffixes such as `.1` and `.2`.

Use `-O` or `--output-document` to choose a filename. An explicit output file is replaced after a successful download. Use `-O -` to stream to standard output and `-q` or `--quiet` to suppress the save summary. HTTP and transfer errors exit with a nonzero status.
