use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::{Arc, Mutex};
use std::thread;

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

    for connection in listener.incoming() {
        match connection {
            Ok(stream) => {
                let state = Arc::clone(&state);
                thread::spawn(move || {
                    if let Err(error) = handle_connection(stream, state) {
                        eprintln!("request failed: {error}");
                    }
                });
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
    let mut bytes = Vec::new();
    let mut chunk = [0; 4096];
    let header_end;
    loop {
        let count = stream.read(&mut chunk)?;
        if count == 0 {
            return Ok(None);
        }
        bytes.extend_from_slice(&chunk[..count]);
        if let Some(index) = find_bytes(&bytes, b"\r\n\r\n") {
            header_end = index + 4;
            break;
        }
        if bytes.len() > 64 * 1024 {
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
    while bytes.len() < header_end + content_length {
        let count = stream.read(&mut chunk)?;
        if count == 0 {
            break;
        }
        bytes.extend_from_slice(&chunk[..count]);
    }
    let body_end = bytes.len().min(header_end + content_length);
    let body = String::from_utf8_lossy(&bytes[header_end..body_end]).into_owned();
    Ok(Some(Request { method, path, body }))
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
    let key = format!("\"{field}\"");
    let mut rest = input.get(input.find(&key)? + key.len()..)?.trim_start();
    rest = rest.strip_prefix(':')?.trim_start();
    let mut chars = rest.chars();
    if chars.next()? != '"' {
        return None;
    }
    let mut value = String::new();
    let mut escaped = false;
    for character in chars {
        if escaped {
            value.push(match character {
                '"' => '"',
                '\\' => '\\',
                '/' => '/',
                'b' => '\u{0008}',
                'f' => '\u{000c}',
                'n' => '\n',
                'r' => '\r',
                't' => '\t',
                other => other,
            });
            escaped = false;
        } else if character == '\\' {
            escaped = true;
        } else if character == '"' {
            return Some(value);
        } else {
            value.push(character);
        }
    }
    None
}

fn json_bool_field(input: &str, field: &str) -> Option<bool> {
    let key = format!("\"{field}\"");
    let rest = input.get(input.find(&key)? + key.len()..)?.trim_start();
    let rest = rest.strip_prefix(':')?.trim_start();
    if rest.starts_with("true") {
        Some(true)
    } else if rest.starts_with("false") {
        Some(false)
    } else {
        None
    }
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
