use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::{mpsc, Arc, Mutex};
use std::time::{Duration, Instant};
use std::thread;

const MAX_HEADER_BYTES: usize = 64 * 1024;
const MAX_BODY_BYTES: usize = 1024 * 1024;
const MAX_TODO_COUNT: usize = 100;
const WORKER_COUNT: usize = 16;
const CONNECTION_QUEUE_CAPACITY: usize = 64;
const READ_TIMEOUT: Duration = Duration::from_secs(10);

#[derive(Clone)]
struct Todo {
    id: u64,
    title: String,
    completed: bool,
}

struct AppState {
    todos: Vec<Todo>,
    next_id: u64,
}

struct Request {
    method: String,
    path: String,
    body: String,
}

fn main() -> std::io::Result<()> {
    let listener = TcpListener::bind("0.0.0.0:8080")?;
    let state = Arc::new(Mutex::new(AppState {
        todos: Vec::new(),
        next_id: 1,
    }));
    println!("Todo service listening on http://0.0.0.0:8080");

    let (sender, receiver) = mpsc::sync_channel::<TcpStream>(CONNECTION_QUEUE_CAPACITY);
    let receiver = Arc::new(Mutex::new(receiver));
    for _ in 0..WORKER_COUNT {
        let receiver = Arc::clone(&receiver);
        let state = Arc::clone(&state);
        thread::spawn(move || loop {
            let stream = {
                let receiver = receiver.lock().unwrap_or_else(|poisoned| poisoned.into_inner());
                receiver.recv()
            };
            let Ok(stream) = stream else {
                break;
            };
            if let Err(error) = handle_connection(stream, state.clone()) {
                eprintln!("request failed: {error}");
            }
        });
    }

    for connection in listener.incoming() {
        match connection {
            Ok(stream) => {
                if let Err(error) = sender.send(stream) {
                    eprintln!("connection queue failed: {error}");
                    break;
                }
            }
            Err(error) => eprintln!("connection failed: {error}"),
        }
    }
    Ok(())
}

fn handle_connection(mut stream: TcpStream, state: Arc<Mutex<AppState>>) -> std::io::Result<()> {
    let Some(request) = read_request(&mut stream)? else {
        return Ok(());
    };
    let (status, body) = route(&request, &state);
    let reason = match status {
        200 => "OK",
        201 => "Created",
        400 => "Bad Request",
        503 => "Service Unavailable",
        404 => "Not Found",
        _ => "Internal Server Error",
    };
    write!(
        stream,
        "HTTP/1.1 {status} {reason}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
        body.len(),
        body
    )
}

fn read_request(stream: &mut TcpStream) -> std::io::Result<Option<Request>> {
    let deadline = Instant::now() + READ_TIMEOUT;
    let mut bytes = Vec::new();
    let mut chunk = [0; 4096];
    let header_end;
    loop {
        let count = read_before_deadline(stream, &mut chunk, deadline)?;
        if count == 0 {
            return Ok(None);
        }
        bytes.extend_from_slice(&chunk[..count]);
        if let Some(index) = find_bytes(&bytes, b"\r\n\r\n") {
            header_end = index + 4;
            if header_end > MAX_HEADER_BYTES {
                return Ok(None);
            }
            break;
        }
        if bytes.len() > MAX_HEADER_BYTES {
            return Ok(None);
        }
    }

    let headers = String::from_utf8_lossy(&bytes[..header_end]);
    let first_line = headers.lines().next().unwrap_or_default();
    let mut request_parts = first_line.split_whitespace();
    let Some(method) = request_parts.next() else {
        return Ok(None);
    };
    let Some(path) = request_parts.next() else {
        return Ok(None);
    };
    let method = method.to_owned();
    let path = path.to_owned();

    let content_length = headers
        .split("\r\n")
        .filter_map(|line| line.split_once(':'))
        .find(|(name, _)| name.eq_ignore_ascii_case("content-length"))
        .and_then(|(_, value)| value.trim().parse::<usize>().ok())
        .unwrap_or(0);
    if content_length > MAX_BODY_BYTES {
        return Err(std::io::Error::new(
            std::io::ErrorKind::InvalidData,
            "request body exceeds configured limit",
        ));
    }
    let request_end = header_end.checked_add(content_length).ok_or_else(|| {
        std::io::Error::new(std::io::ErrorKind::InvalidData, "request length overflow")
    })?;
    while bytes.len() < request_end {
        let count = read_before_deadline(stream, &mut chunk, deadline)?;
        if count == 0 {
            break;
        }
        let remaining = request_end - bytes.len();
        if count > remaining {
            bytes.extend_from_slice(&chunk[..remaining]);
            break;
        }
        bytes.extend_from_slice(&chunk[..count]);
    }
    let body_end = bytes.len().min(request_end);
    let body = String::from_utf8_lossy(&bytes[header_end..body_end]).into_owned();
    Ok(Some(Request { method, path, body }))
}

fn read_before_deadline(
    stream: &mut TcpStream,
    buffer: &mut [u8],
    deadline: Instant,
) -> std::io::Result<usize> {
    let remaining = deadline.saturating_duration_since(Instant::now());
    if remaining.is_zero() {
        return Err(std::io::Error::new(
            std::io::ErrorKind::TimedOut,
            "request read deadline exceeded",
        ));
    }
    stream.set_read_timeout(Some(remaining))?;
    stream.read(buffer)
}

fn route(request: &Request, state: &Arc<Mutex<AppState>>) -> (u16, String) {
    if request.method == "GET" && request.path == "/health" {
        return (200, r#"{"status":"ok"}"#.to_owned());
    }

    let mut state = state.lock().unwrap_or_else(|poisoned| poisoned.into_inner());
    if request.path == "/todos" {
        return match request.method.as_str() {
            "GET" => (200, todos_json(&state.todos)),
            "POST" => {
                let Some(title) = json_string_field(&request.body, "title") else {
                    return (400, error_json("title must be a non-empty string"));
                };
                if title.trim().is_empty() {
                    return (400, error_json("title must be a non-empty string"));
                }
                if state.todos.len() >= MAX_TODO_COUNT {
                    return (503, error_json("todo capacity reached"));
                }
                let todo = Todo {
                    id: state.next_id,
                    title,
                    completed: false,
                };
                state.next_id += 1;
                state.todos.push(todo.clone());
                (201, todo_json(&todo))
            }
            _ => (404, error_json("not found")),
        };
    }

    let Some(id) = request
        .path
        .strip_prefix("/todos/")
        .and_then(|suffix| suffix.parse::<u64>().ok())
    else {
        return (404, error_json("not found"));
    };
    let Some(index) = state.todos.iter().position(|todo| todo.id == id) else {
        return (404, error_json("todo not found"));
    };

    match request.method.as_str() {
        "GET" => (200, todo_json(&state.todos[index])),
        "PUT" | "PATCH" => {
            if let Some(title) = json_string_field(&request.body, "title") {
                if title.trim().is_empty() {
                    return (400, error_json("title must be a non-empty string"));
                }
                state.todos[index].title = title;
            }
            if let Some(completed) = json_bool_field(&request.body, "completed") {
                state.todos[index].completed = completed;
            }
            (200, todo_json(&state.todos[index]))
        }
        "DELETE" => {
            state.todos.remove(index);
            (200, r#"{"deleted":true}"#.to_owned())
        }
        _ => (404, error_json("not found")),
    }
}

fn find_bytes(haystack: &[u8], needle: &[u8]) -> Option<usize> {
    haystack.windows(needle.len()).position(|window| window == needle)
}

fn json_string_field(input: &str, field: &str) -> Option<String> {
    let value = root_json_field(input, field)?;
    let bytes = value.as_bytes();
    if *bytes.first()? != b'"' {
        return None;
    }
    let mut value = String::new();
    let mut index = 1;
    while let Some(&byte) = bytes.get(index) {
        match byte {
            b'"' => return Some(value),
            b'\\' => {
                index += 1;
                match *bytes.get(index)? {
                    b'"' => value.push('"'),
                    b'\\' => value.push('\\'),
                    b'/' => value.push('/'),
                    b'b' => value.push('\u{0008}'),
                    b'f' => value.push('\u{000c}'),
                    b'n' => value.push('\n'),
                    b'r' => value.push('\r'),
                    b't' => value.push('\t'),
                    b'u' => {
                        let first = parse_json_hex_quad(bytes, index + 1)?;
                        index += 4;
                        let code_point = if (0xD800..=0xDBFF).contains(&first) {
                            if bytes.get(index + 1..index + 3)? != b"\\u" {
                                return None;
                            }
                            let second = parse_json_hex_quad(bytes, index + 3)?;
                            if !(0xDC00..=0xDFFF).contains(&second) {
                                return None;
                            }
                            index += 6;
                            0x10000 + (((first - 0xD800) as u32) << 10)
                                + (second - 0xDC00) as u32
                        } else if (0xDC00..=0xDFFF).contains(&first) {
                            return None;
                        } else {
                            first as u32
                        };
                        value.push(char::from_u32(code_point)?);
                    }
                    _ => return None,
                }
                index += 1;
            }
            0..=31 => return None,
            _ => {
                let character = std::str::from_utf8(&bytes[index..]).ok()?.chars().next()?;
                value.push(character);
                index += character.len_utf8();
            }
        }
    }
    None
}

fn parse_json_hex_quad(bytes: &[u8], start: usize) -> Option<u16> {
    let digits = bytes.get(start..start + 4)?;
    let mut value = 0u16;
    for &digit in digits {
        value = value.checked_mul(16)? + (digit as char).to_digit(16)? as u16;
    }
    Some(value)
}

fn json_bool_field(input: &str, field: &str) -> Option<bool> {
    let rest = root_json_field(input, field)?.trim_start();
    if rest.starts_with("true") {
        Some(true)
    } else if rest.starts_with("false") {
        Some(false)
    } else {
        None
    }
}

// Return the raw value for a member of the root object. Walking each value
// structurally prevents matching a similarly named member inside a nested
// object or inside a string.
fn root_json_field<'a>(input: &'a str, field: &str) -> Option<&'a str> {
    let bytes = input.as_bytes();
    let mut index = skip_json_whitespace(bytes, 0);
    if *bytes.get(index)? != b'{' {
        return None;
    }
    index += 1;
    let mut matched_value = None;
    let mut after_comma = false;

    loop {
        index = skip_json_whitespace(bytes, index);
        if *bytes.get(index)? == b'}' {
            if after_comma {
                return None;
            }
            index = skip_json_whitespace(bytes, index + 1);
            if index != bytes.len() {
                return None;
            }
            return matched_value;
        }
        if *bytes.get(index)? != b'"' {
            return None;
        }
        let key_end = json_string_end(bytes, index)?;
        let key = std::str::from_utf8(&bytes[index + 1..key_end - 1]).ok()?;
        index = skip_json_whitespace(bytes, key_end);
        if *bytes.get(index)? != b':' {
            return None;
        }
        index = skip_json_whitespace(bytes, index + 1);
        let value_start = index;
        let value_end = json_value_end(bytes, value_start)?;
        if key == field && matched_value.is_none() {
            matched_value = Some(std::str::from_utf8(&bytes[value_start..value_end]).ok()?);
        }
        after_comma = false;
        index = skip_json_whitespace(bytes, value_end);
        match *bytes.get(index)? {
            b',' => {
                index += 1;
                after_comma = true;
            }
            b'}' => {}
            _ => return None,
        }
    }
}

fn skip_json_whitespace(bytes: &[u8], mut index: usize) -> usize {
    while matches!(bytes.get(index), Some(b' ' | b'\n' | b'\r' | b'\t')) {
        index += 1;
    }
    index
}

fn json_string_end(bytes: &[u8], start: usize) -> Option<usize> {
    if *bytes.get(start)? != b'"' {
        return None;
    }
    let mut index = start + 1;
    while let Some(&byte) = bytes.get(index) {
        match byte {
            b'"' => return Some(index + 1),
            b'\\' => {
                index += 1;
                match *bytes.get(index)? {
                    b'"' | b'\\' | b'/' | b'b' | b'f' | b'n' | b'r' | b't' => index += 1,
                    b'u' => {
                        let first = parse_json_hex_quad(bytes, index + 1)?;
                        index += 4;
                        if (0xD800..=0xDBFF).contains(&first) {
                            if bytes.get(index + 1..index + 3)? != b"\\u" {
                                return None;
                            }
                            let second = parse_json_hex_quad(bytes, index + 3)?;
                            if !(0xDC00..=0xDFFF).contains(&second) {
                                return None;
                            }
                            index += 6;
                        } else if (0xDC00..=0xDFFF).contains(&first) {
                            return None;
                        }
                        index += 1;
                    }
                    _ => return None,
                }
            }
            0..=31 => return None,
            _ => index += 1,
        }
    }
    None
}

fn json_value_end(bytes: &[u8], start: usize) -> Option<usize> {
    let start = skip_json_whitespace(bytes, start);
    match *bytes.get(start)? {
        b'"' => json_string_end(bytes, start),
        b'{' => {
            let mut index = skip_json_whitespace(bytes, start + 1);
            if *bytes.get(index)? == b'}' {
                return Some(index + 1);
            }
            loop {
                let key_end = json_string_end(bytes, index)?;
                index = skip_json_whitespace(bytes, key_end);
                if *bytes.get(index)? != b':' {
                    return None;
                }
                index = json_value_end(bytes, index + 1)?;
                index = skip_json_whitespace(bytes, index);
                match *bytes.get(index)? {
                    b',' => index = skip_json_whitespace(bytes, index + 1),
                    b'}' => return Some(index + 1),
                    _ => return None,
                }
            }
        }
        b'[' => {
            let mut index = skip_json_whitespace(bytes, start + 1);
            if *bytes.get(index)? == b']' {
                return Some(index + 1);
            }
            loop {
                index = json_value_end(bytes, index)?;
                index = skip_json_whitespace(bytes, index);
                match *bytes.get(index)? {
                    b',' => index = skip_json_whitespace(bytes, index + 1),
                    b']' => return Some(index + 1),
                    _ => return None,
                }
            }
        }
        _ => {
            let mut index = start;
            while let Some(&byte) = bytes.get(index) {
                if matches!(byte, b',' | b']' | b'}' | b' ' | b'\n' | b'\r' | b'\t') {
                    break;
                }
                index += 1;
            }
            let literal = bytes.get(start..index)?;
            if matches!(literal, b"true" | b"false" | b"null")
                || valid_json_number(literal)
            {
                Some(index)
            } else {
                None
            }
        }
    }
}

fn valid_json_number(bytes: &[u8]) -> bool {
    let mut index = 0;
    if bytes.get(index) == Some(&b'-') {
        index += 1;
    }
    match bytes.get(index) {
        Some(b'0') => index += 1,
        Some(b'1'..=b'9') => {
            index += 1;
            while matches!(bytes.get(index), Some(b'0'..=b'9')) {
                index += 1;
            }
        }
        _ => return false,
    }
    if bytes.get(index) == Some(&b'.') {
        index += 1;
        let fraction_start = index;
        while matches!(bytes.get(index), Some(b'0'..=b'9')) {
            index += 1;
        }
        if index == fraction_start {
            return false;
        }
    }
    if matches!(bytes.get(index), Some(b'e' | b'E')) {
        index += 1;
        if matches!(bytes.get(index), Some(b'+' | b'-')) {
            index += 1;
        }
        let exponent_start = index;
        while matches!(bytes.get(index), Some(b'0'..=b'9')) {
            index += 1;
        }
        if index == exponent_start {
            return false;
        }
    }
    index == bytes.len()
}

fn escape_json(value: &str) -> String {
    let mut escaped = String::with_capacity(value.len());
    for character in value.chars() {
        match character {
            '"' => escaped.push_str("\\\""),
            '\\' => escaped.push_str("\\\\"),
            '\n' => escaped.push_str("\\n"),
            '\r' => escaped.push_str("\\r"),
            '\t' => escaped.push_str("\\t"),
            character if character.is_control() => {
                escaped.push_str(&format!("\\u{:04x}", character as u32));
            }
            character => escaped.push(character),
        }
    }
    escaped
}

fn todo_json(todo: &Todo) -> String {
    format!(
        "{{\"id\":{},\"title\":\"{}\",\"completed\":{}}}",
        todo.id,
        escape_json(&todo.title),
        todo.completed
    )
}

fn todos_json(todos: &[Todo]) -> String {
    format!(
        "[{}]",
        todos.iter().map(todo_json).collect::<Vec<_>>().join(",")
    )
}

fn error_json(message: &str) -> String {
    format!("{{\"error\":\"{}\"}}", escape_json(message))
}
