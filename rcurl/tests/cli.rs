use std::{
    io::{Read, Write},
    net::TcpListener,
    process::{Command, Output},
    thread,
    time::Duration,
};

fn cli(args: &[&str]) -> Output {
    Command::new(env!("CARGO_BIN_EXE_rcurl"))
        .env("NO_PROXY", "*")
        .env("no_proxy", "*")
        .args(args)
        .output()
        .unwrap()
}

fn server(responses: Vec<&'static [u8]>) -> (String, thread::JoinHandle<Vec<Vec<u8>>>) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let url = format!("http://{}", listener.local_addr().unwrap());
    let handle = thread::spawn(move || {
        responses
            .into_iter()
            .map(|response| {
                let (mut stream, _) = listener.accept().unwrap();
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                let mut request = Vec::new();
                let mut buffer = [0; 1024];
                loop {
                    let n = stream.read(&mut buffer).unwrap();
                    assert!(n > 0);
                    request.extend_from_slice(&buffer[..n]);
                    if let Some(end) = request.windows(4).position(|w| w == b"\r\n\r\n") {
                        let headers = String::from_utf8_lossy(&request[..end]).to_lowercase();
                        let length = headers
                            .lines()
                            .find_map(|l| l.strip_prefix("content-length: "))
                            .map(|v| v.parse::<usize>().unwrap())
                            .unwrap_or(0);
                        if request.len() >= end + 4 + length {
                            break;
                        }
                    }
                }
                stream.write_all(response).unwrap();
                request
            })
            .collect()
    });
    (url, handle)
}

#[test]
fn default_get_and_binary_chunked_body() {
    let (url, server) = server(vec![b"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\nConnection: close\r\n\r\n3\r\na\0b\r\n2\r\ncd\r\n0\r\n\r\n"]);
    let output = cli(&[&url]);
    assert!(output.status.success());
    assert_eq!(output.stdout, b"a\0bcd");
    assert!(output.stderr.is_empty());
    assert!(server.join().unwrap()[0].starts_with(b"GET / HTTP/1.1\r\n"));
}

#[test]
fn methods_headers_and_body() {
    for method in [None, Some("PATCH")] {
        let (url, server) = server(vec![b"HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"]);
        let mut args = vec![
            url.as_str(),
            "-H",
            "X-Test: first",
            "-H",
            "X-Test: second",
            "-d",
            "hello",
        ];
        if let Some(method) = method {
            args.extend(["-X", method]);
        }
        assert!(cli(&args).status.success());
        let requests = server.join().unwrap();
        let request = String::from_utf8_lossy(&requests[0]);
        assert!(request.starts_with(&format!("{} / HTTP/1.1", method.unwrap_or("POST"))));
        assert!(request.contains("x-test: first\r\nx-test: second\r\n"));
        assert!(request.ends_with("hello"));
    }
}

#[test]
fn headers_stdout_and_body_file_even_for_http_errors() {
    let (url, server) = server(vec![
        b"HTTP/1.1 404 Not Found\r\nContent-Length: 4\r\nX-Test: yes\r\n\r\noops",
    ]);
    let path = std::env::temp_dir().join(format!("rcurl-test-{}.bin", std::process::id()));
    let output = cli(&[&url, "-i", "-o", path.to_str().unwrap()]);
    assert!(output.status.success());
    let headers = String::from_utf8(output.stdout).unwrap();
    assert!(headers.starts_with("HTTP/1.1 404 Not Found\r\n"));
    assert!(headers.contains("x-test: yes\r\n"));
    assert!(headers.ends_with("\r\n\r\n"));
    assert_eq!(std::fs::read(&path).unwrap(), b"oops");
    std::fs::remove_file(path).unwrap();
    server.join().unwrap();
}

#[test]
fn redirects_are_opt_in() {
    let redirect =
        b"HTTP/1.1 302 Found\r\nLocation: /final\r\nContent-Length: 0\r\nConnection: close\r\n\r\n";
    let (url, handle) = server(vec![redirect]);
    assert!(cli(&[&url]).status.success());
    assert_eq!(handle.join().unwrap().len(), 1);
    let (url, handle) = server(vec![
        redirect,
        b"HTTP/1.1 200 OK\r\nContent-Length: 4\r\n\r\ndone",
    ]);
    let output = cli(&[&url, "-L"]);
    assert!(output.status.success());
    assert_eq!(output.stdout, b"done");
    assert!(handle.join().unwrap()[1].starts_with(b"GET /final HTTP/1.1"));
}

#[test]
fn network_failure_and_timeout_are_errors() {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let url = format!("http://{}", listener.local_addr().unwrap());
    drop(listener);
    let output = cli(&[&url]);
    assert!(!output.status.success());
    assert!(output.stdout.is_empty());
    assert!(!output.stderr.is_empty());

    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let url = format!("http://{}", listener.local_addr().unwrap());
    let handle = thread::spawn(move || {
        let (_stream, _) = listener.accept().unwrap();
        thread::sleep(Duration::from_millis(300));
    });
    let output = cli(&[&url, "--timeout", "0.05", "--connect-timeout", "1"]);
    assert!(!output.status.success());
    assert!(output.stdout.is_empty());
    assert!(!output.stderr.is_empty());
    handle.join().unwrap();
}

#[test]
fn invalid_arguments_fail() {
    for args in [
        vec!["ftp://localhost"],
        vec!["http://localhost", "-H", "invalid"],
        vec!["http://localhost", "--timeout", "0"],
        vec!["http://localhost", "--timeout", "NaN"],
    ] {
        let output = cli(&args);
        assert!(!output.status.success());
        assert!(output.stdout.is_empty());
        assert!(!output.stderr.is_empty());
    }
}

#[test]
fn truncated_response_is_an_error() {
    let (url, handle) = server(vec![
        b"HTTP/1.1 200 OK\r\nContent-Length: 100\r\nConnection: close\r\n\r\nshort",
    ]);
    let output = cli(&[&url]);
    assert!(!output.status.success());
    assert!(!output.stderr.is_empty());
    handle.join().unwrap();
}
