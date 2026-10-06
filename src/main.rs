use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::{atomic::{AtomicU64, Ordering}, mpsc::sync_channel, Arc, Mutex};
use std::time::{Duration, Instant};

const MAX_REQUEST_BYTES: usize = 1024 * 1024;
const MAX_REQUEST_DURATION: Duration = Duration::from_secs(2);
const MAX_TODO_COUNT: usize = 10_000;
const MAX_TODO_TITLE_BYTES: usize = 512;
const MAX_JSON_DEPTH: usize = 64;

#[derive(Clone)]
struct Todo {
    id: u64,
    title: String,
    completed: bool,
}

#[derive(Clone)]
struct AppState {
    todos: Arc<Mutex<Vec<Todo>>>,
    next_id: Arc<AtomicU64>,
}

struct Request {
    method: String,
    path: String,
    body: String,
}

struct Response {
    status: u16,
    reason: &'static str,
    content_type: &'static str,
    body: String,
}

fn main() {
    let state = AppState {
        todos: Arc::new(Mutex::new(Vec::new())),
        next_id: Arc::new(AtomicU64::new(1)),
    };
    let listener = TcpListener::bind("0.0.0.0:8080").expect("failed to bind to port 8080");
    println!("Todo service listening on http://0.0.0.0:8080");
    let (sender, receiver) = sync_channel::<TcpStream>(32);
    let receiver = Arc::new(Mutex::new(receiver));
    for _ in 0..8 {
        let receiver = Arc::clone(&receiver);
        let state = state.clone();
        std::thread::spawn(move || loop {
            let stream = match receiver.lock().expect("connection queue lock poisoned").recv() {
                Ok(stream) => stream,
                Err(_) => break,
            };
            serve_connection(stream, state.clone());
        });
    }

    for connection in listener.incoming() {
        match connection {
            Ok(stream) => {
                let timeout = Some(Duration::from_secs(10));
                if stream.set_read_timeout(timeout).is_err() || stream.set_write_timeout(timeout).is_err() {
                    eprintln!("failed to configure connection timeout");
                    continue;
                }
                if sender.send(stream).is_err() {
                    break;
                }
            }
            Err(error) => eprintln!("failed to accept connection: {error}"),
        }
    }
}

fn serve_connection(mut stream: TcpStream, state: AppState) {
    let response = match read_request(&mut stream) {
        Ok(request) => route(request, &state),
        Err(()) => json_response(400, "Bad Request", "{\"error\":\"bad request\"}".to_owned()),
    };
    let _ = write!(
        stream,
        "HTTP/1.1 {} {}\r\nContent-Type: {}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
        response.status,
        response.reason,
        response.content_type,
        response.body.len(),
        response.body
    );
}

fn read_request(stream: &mut TcpStream) -> Result<Request, ()> {
    let deadline = Instant::now() + MAX_REQUEST_DURATION;
    let mut bytes = Vec::new();
    let mut chunk = [0u8; 4096];
    let header_end = loop {
        if let Some(index) = find_bytes(&bytes, b"\r\n\r\n") {
            break index + 4;
        }
        if bytes.len() >= MAX_REQUEST_BYTES {
            return Err(());
        }
        let count = read_before_deadline(stream, &mut chunk, deadline)?;
        if count == 0 {
            return Err(());
        }
        bytes.extend_from_slice(&chunk[..count]);
    };

    let headers = std::str::from_utf8(&bytes[..header_end]).map_err(|_| ())?;
    let mut lines = headers.split("\r\n");
    let mut request_line = lines.next().ok_or(())?.split_whitespace();
    let method = request_line.next().ok_or(())?.to_owned();
    let path = request_line.next().ok_or(())?.to_owned();
    if request_line.next().is_none() {
        return Err(());
    }
    let content_length = lines
        .filter_map(|line| line.split_once(':'))
        .find(|(name, _)| name.eq_ignore_ascii_case("content-length"))
        .map(|(_, value)| value.trim().parse::<usize>().map_err(|_| ()))
        .transpose()?
        .unwrap_or(0);
    if content_length > MAX_REQUEST_BYTES - header_end {
        return Err(());
    }
    let total_length = header_end + content_length;
    while bytes.len() < total_length {
        let remaining = total_length - bytes.len();
        let read_length = remaining.min(chunk.len());
        let count = read_before_deadline(stream, &mut chunk[..read_length], deadline)?;
        if count == 0 {
            return Err(());
        }
        bytes.extend_from_slice(&chunk[..count]);
    }
    let body = std::str::from_utf8(&bytes[header_end..total_length])
        .map_err(|_| ())?
        .to_owned();
    Ok(Request { method, path, body })
}

fn read_before_deadline(stream: &mut TcpStream, buffer: &mut [u8], deadline: Instant) -> Result<usize, ()> {
    let remaining = deadline.checked_duration_since(Instant::now()).ok_or(())?;
    stream.set_read_timeout(Some(remaining)).map_err(|_| ())?;
    stream.read(buffer).map_err(|_| ())
}

fn find_bytes(haystack: &[u8], needle: &[u8]) -> Option<usize> {
    haystack.windows(needle.len()).position(|window| window == needle)
}

fn route(request: Request, state: &AppState) -> Response {
    match (request.method.as_str(), request.path.as_str()) {
        ("GET", "/health") => json_response(200, "OK", "{\"status\":\"ok\"}".to_owned()),
        ("GET", "/todos") => {
            let todos = state.todos.lock().expect("todo list lock poisoned");
            json_response(200, "OK", todos_json(&todos))
        }
        ("POST", "/todos") => create_todo(&request.body, state),
        _ if request.path.starts_with("/todos/") => {
            let id = match request.path[7..].parse::<u64>() {
                Ok(id) => id,
                Err(_) => return json_response(404, "Not Found", "{\"error\":\"not found\"}".to_owned()),
            };
            match request.method.as_str() {
                "PUT" => update_todo(id, &request.body, state),
                "DELETE" => delete_todo(id, state),
                _ => json_response(405, "Method Not Allowed", "{\"error\":\"method not allowed\"}".to_owned()),
            }
        }
        _ => json_response(404, "Not Found", "{\"error\":\"not found\"}".to_owned()),
    }
}

fn create_todo(body: &str, state: &AppState) -> Response {
    let title = match json_string_field(body, "title") {
        Some(title) if !title.trim().is_empty() && title.trim().len() <= MAX_TODO_TITLE_BYTES => title.trim().to_owned(),
        Some(title) if title.trim().len() > MAX_TODO_TITLE_BYTES => {
            return json_response(400, "Bad Request", "{\"error\":\"title is too long\"}".to_owned());
        }
        _ => return json_response(400, "Bad Request", "{\"error\":\"title is required\"}".to_owned()),
    };
    let mut todos = state.todos.lock().expect("todo list lock poisoned");
    if todos.len() >= MAX_TODO_COUNT {
        return json_response(503, "Service Unavailable", "{\"error\":\"todo limit reached\"}".to_owned());
    }
    let todo = Todo {
        id: state.next_id.fetch_add(1, Ordering::Relaxed),
        title,
        completed: false,
    };
    todos.push(todo.clone());
    json_response(201, "Created", todo_json(&todo))
}

fn update_todo(id: u64, body: &str, state: &AppState) -> Response {
    let title = match json_field_value(body, "title") {
        Some(JsonField::String(value)) if value.trim().len() <= MAX_TODO_TITLE_BYTES => Some(value),
        Some(JsonField::String(_)) => return json_response(400, "Bad Request", "{\"error\":\"title is too long\"}".to_owned()),
        Some(_) => return json_response(400, "Bad Request", "{\"error\":\"title must be a string\"}".to_owned()),
        None => None,
    };
    let completed = match json_field_value(body, "completed") {
        Some(JsonField::Bool(value)) => Some(value),
        Some(_) => return json_response(400, "Bad Request", "{\"error\":\"completed must be a boolean\"}".to_owned()),
        None => None,
    };
    if (title.is_none() && completed.is_none()) || title.as_ref().is_some_and(|v| v.trim().is_empty()) {
        return json_response(400, "Bad Request", "{\"error\":\"invalid todo update\"}".to_owned());
    }
    let mut todos = state.todos.lock().expect("todo list lock poisoned");
    let Some(todo) = todos.iter_mut().find(|todo| todo.id == id) else {
        return json_response(404, "Not Found", "{\"error\":\"not found\"}".to_owned());
    };
    if let Some(title) = title {
        todo.title = title.trim().to_owned();
    }
    if let Some(completed) = completed {
        todo.completed = completed;
    }
    json_response(200, "OK", todo_json(todo))
}

fn delete_todo(id: u64, state: &AppState) -> Response {
    let mut todos = state.todos.lock().expect("todo list lock poisoned");
    let Some(index) = todos.iter().position(|todo| todo.id == id) else {
        return json_response(404, "Not Found", "{\"error\":\"not found\"}".to_owned());
    };
    todos.remove(index);
    json_response(204, "No Content", String::new())
}

fn json_response(status: u16, reason: &'static str, body: String) -> Response {
    Response { status, reason, content_type: "application/json", body }
}

fn todos_json(todos: &[Todo]) -> String {
    format!("[{}]", todos.iter().map(todo_json).collect::<Vec<_>>().join(","))
}

fn todo_json(todo: &Todo) -> String {
    format!(
        "{{\"id\":{},\"title\":{},\"completed\":{}}}",
        todo.id,
        json_quote(&todo.title),
        todo.completed
    )
}

fn json_quote(value: &str) -> String {
    let mut out = String::from("\"");
    for ch in value.chars() {
        match ch {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            ch if ch <= '\u{1f}' => out.push_str(&format!("\\u{:04x}", ch as u32)),
            ch => out.push(ch),
        }
    }
    out.push('"');
    out
}

fn json_string_field(input: &str, field: &str) -> Option<String> {
    match json_field_value(input, field) {
        Some(JsonField::String(value)) => Some(value),
        _ => None,
    }
}

enum JsonField {
    String(String),
    Bool(bool),
    Other,
}

fn json_field_value(input: &str, field: &str) -> Option<JsonField> {
    let bytes = input.as_bytes();
    if !valid_json(bytes) {
        return None;
    }
    let mut index = skip_ws(bytes, 0);
    if bytes.get(index) != Some(&b'{') {
        return None;
    }
    index += 1;
    loop {
        index = skip_ws(bytes, index);
        if bytes.get(index) == Some(&b'}') {
            return None;
        }
        let (key, after_key) = parse_json_string(bytes, index)?;
        index = skip_ws(bytes, after_key);
        if bytes.get(index) != Some(&b':') {
            return None;
        }
        index = skip_ws(bytes, index + 1);
        if key == field {
            if bytes.get(index) == Some(&b'"') {
                return parse_json_string(bytes, index).map(|(value, _)| JsonField::String(value));
            }
            if bytes.get(index..index + 4) == Some(b"true") {
                return Some(JsonField::Bool(true));
            }
            if bytes.get(index..index + 5) == Some(b"false") {
                return Some(JsonField::Bool(false));
            }
            return Some(JsonField::Other);
        }
        index = skip_json_value(bytes, index)?;
        index = skip_ws(bytes, index);
        match bytes.get(index) {
            Some(b',') => index += 1,
            Some(b'}') => return None,
            _ => return None,
        }
    }
}

fn skip_ws(bytes: &[u8], mut index: usize) -> usize {
    while matches!(bytes.get(index), Some(b' ' | b'\t' | b'\r' | b'\n')) {
        index += 1;
    }
    index
}

fn parse_json_string(bytes: &[u8], start: usize) -> Option<(String, usize)> {
    if bytes.get(start) != Some(&b'"') { return None; }
    let mut result = String::new();
    let mut index = start + 1;
    while let Some(&byte) = bytes.get(index) {
        match byte {
            b'"' => return Some((result, index + 1)),
            b'\\' => {
                index += 1;
                match *bytes.get(index)? {
                    b'"' => result.push('"'),
                    b'\\' => result.push('\\'),
                    b'/' => result.push('/'),
                    b'b' => result.push('\u{0008}'),
                    b'f' => result.push('\u{000c}'),
                    b'n' => result.push('\n'),
                    b'r' => result.push('\r'),
                    b't' => result.push('\t'),
                    b'u' => {
                        let digits_start = index + 1;
                        let first = parse_hex4(bytes, digits_start)?;
                        let code = if (0xd800..=0xdbff).contains(&first) {
                            let escape_start = digits_start + 4;
                            if bytes.get(escape_start..escape_start + 2)? != b"\\u" {
                                return None;
                            }
                            let second = parse_hex4(bytes, escape_start + 2)?;
                            if !(0xdc00..=0xdfff).contains(&second) {
                                return None;
                            }
                            index = escape_start + 5;
                            0x10000 + (((first as u32 - 0xd800) << 10) | (second as u32 - 0xdc00))
                        } else {
                            if (0xdc00..=0xdfff).contains(&first) {
                                return None;
                            }
                            index = digits_start + 3;
                            first as u32
                        };
                        let ch = char::from_u32(code)?;
                        result.push(ch);
                    }
                    _ => return None,
                }
            }
            0..=31 => return None,
            _ => {
                let rest = std::str::from_utf8(bytes.get(index..)?).ok()?;
                let ch = rest.chars().next()?;
                result.push(ch);
                index += ch.len_utf8() - 1;
            }
        }
        index += 1;
    }
    None
}

fn parse_hex4(bytes: &[u8], start: usize) -> Option<u16> {
    let digits = std::str::from_utf8(bytes.get(start..start + 4)?).ok()?;
    u16::from_str_radix(digits, 16).ok()
}

fn valid_json(bytes: &[u8]) -> bool {
    let Some(end) = parse_json_value_at_depth(bytes, skip_ws(bytes, 0), 0) else {
        return false;
    };
    skip_ws(bytes, end) == bytes.len()
}

fn parse_json_value(bytes: &[u8], start: usize) -> Option<usize> {
    parse_json_value_at_depth(bytes, start, 0)
}

fn parse_json_value_at_depth(bytes: &[u8], start: usize, depth: usize) -> Option<usize> {
    if depth > MAX_JSON_DEPTH {
        return None;
    }
    match *bytes.get(start)? {
        b'"' => parse_json_string(bytes, start).map(|(_, end)| end),
        b'{' => {
            let mut index = skip_ws(bytes, start + 1);
            if bytes.get(index) == Some(&b'}') {
                return Some(index + 1);
            }
            loop {
                let (_, after_key) = parse_json_string(bytes, index)?;
                index = skip_ws(bytes, after_key);
                if bytes.get(index) != Some(&b':') {
                    return None;
                }
                index = skip_ws(bytes, index + 1);
                index = parse_json_value_at_depth(bytes, index, depth + 1)?;
                index = skip_ws(bytes, index);
                match bytes.get(index) {
                    Some(b',') => index = skip_ws(bytes, index + 1),
                    Some(b'}') => return Some(index + 1),
                    _ => return None,
                }
            }
        }
        b'[' => {
            let mut index = skip_ws(bytes, start + 1);
            if bytes.get(index) == Some(&b']') {
                return Some(index + 1);
            }
            loop {
                index = parse_json_value_at_depth(bytes, index, depth + 1)?;
                index = skip_ws(bytes, index);
                match bytes.get(index) {
                    Some(b',') => index = skip_ws(bytes, index + 1),
                    Some(b']') => return Some(index + 1),
                    _ => return None,
                }
            }
        }
        b't' if bytes.get(start..start + 4)? == b"true" => Some(start + 4),
        b'f' if bytes.get(start..start + 5)? == b"false" => Some(start + 5),
        b'n' if bytes.get(start..start + 4)? == b"null" => Some(start + 4),
        b'-' | b'0'..=b'9' => parse_json_number(bytes, start),
        _ => None,
    }
}

fn parse_json_number(bytes: &[u8], start: usize) -> Option<usize> {
    let mut index = start;
    if bytes.get(index) == Some(&b'-') {
        index += 1;
    }
    match bytes.get(index)? {
        b'0' => index += 1,
        b'1'..=b'9' => {
            index += 1;
            while bytes.get(index).is_some_and(u8::is_ascii_digit) {
                index += 1;
            }
        }
        _ => return None,
    }
    if bytes.get(index) == Some(&b'.') {
        index += 1;
        if !bytes.get(index).is_some_and(u8::is_ascii_digit) {
            return None;
        }
        while bytes.get(index).is_some_and(u8::is_ascii_digit) {
            index += 1;
        }
    }
    if matches!(bytes.get(index), Some(b'e' | b'E')) {
        index += 1;
        if matches!(bytes.get(index), Some(b'+' | b'-')) {
            index += 1;
        }
        if !bytes.get(index).is_some_and(u8::is_ascii_digit) {
            return None;
        }
        while bytes.get(index).is_some_and(u8::is_ascii_digit) {
            index += 1;
        }
    }
    Some(index)
}

fn skip_json_value(bytes: &[u8], start: usize) -> Option<usize> {
    parse_json_value(bytes, start)
}
