use axum::{
    extract::{Path, State},
    http::StatusCode,
    response::IntoResponse,
    routing::{get, post},
    Json, Router,
};
use serde::{Deserialize, Serialize};
use std::{
    net::SocketAddr,
    sync::{
        atomic::{AtomicU64, Ordering},
        Arc, Mutex,
    },
};

#[derive(Clone)]
struct AppState {
    todos: Arc<Mutex<Vec<Todo>>>,
    next_id: Arc<AtomicU64>,
}

#[derive(Clone, Serialize)]
struct Todo {
    id: u64,
    title: String,
    completed: bool,
}

#[derive(Deserialize)]
struct CreateTodo {
    title: String,
}

#[derive(Deserialize)]
struct UpdateTodo {
    title: Option<String>,
    completed: Option<bool>,
}

#[derive(Serialize)]
struct Health {
    status: &'static str,
}

#[tokio::main]
async fn main() {
    let state = AppState {
        todos: Arc::new(Mutex::new(Vec::new())),
        next_id: Arc::new(AtomicU64::new(1)),
    };
    let app = Router::new()
        .route("/health", get(health))
        .route("/todos", get(list_todos).post(create_todo))
        .route("/todos/{id}", axum::routing::put(update_todo).delete(delete_todo))
        .with_state(state);

    let address = SocketAddr::from(([0, 0, 0, 0], 8080));
    let listener = tokio::net::TcpListener::bind(address)
        .await
        .expect("failed to bind to port 8080");
    println!("Todo service listening on http://{address}");
    axum::serve(listener, app)
        .await
        .expect("todo service failed");
}

async fn health() -> Json<Health> {
    Json(Health { status: "ok" })
}

async fn list_todos(State(state): State<AppState>) -> Json<Vec<Todo>> {
    let todos = state.todos.lock().expect("todo list lock poisoned");
    Json(todos.clone())
}

async fn create_todo(
    State(state): State<AppState>,
    Json(input): Json<CreateTodo>,
) -> Result<impl IntoResponse, StatusCode> {
    let title = input.title.trim();
    if title.is_empty() {
        return Err(StatusCode::BAD_REQUEST);
    }

    let mut todos = state.todos.lock().expect("todo list lock poisoned");
    let todo = Todo {
        id: state.next_id.fetch_add(1, Ordering::Relaxed),
        title: title.to_owned(),
        completed: false,
    };
    todos.push(todo.clone());
    Ok((StatusCode::CREATED, Json(todo)))
}

async fn update_todo(
    State(state): State<AppState>,
    Path(id): Path<u64>,
    Json(input): Json<UpdateTodo>,
) -> Result<Json<Todo>, StatusCode> {
    if input.title.as_ref().is_some_and(|title| title.trim().is_empty()) {
        return Err(StatusCode::BAD_REQUEST);
    }

    let mut todos = state.todos.lock().expect("todo list lock poisoned");
    let todo = todos
        .iter_mut()
        .find(|todo| todo.id == id)
        .ok_or(StatusCode::NOT_FOUND)?;
    if let Some(title) = input.title {
        todo.title = title.trim().to_owned();
    }
    if let Some(completed) = input.completed {
        todo.completed = completed;
    }
    Ok(Json(todo.clone()))
}

async fn delete_todo(
    State(state): State<AppState>,
    Path(id): Path<u64>,
) -> StatusCode {
    let mut todos = state.todos.lock().expect("todo list lock poisoned");
    let Some(index) = todos.iter().position(|todo| todo.id == id) else {
        return StatusCode::NOT_FOUND;
    };
    todos.remove(index);
    StatusCode::NO_CONTENT
}
