mod store;
mod tui;
use axum::{
    extract::{Path, Query, State},
    http::{header, StatusCode},
    response::{Html, IntoResponse, Response},
    routing::get,
    Json, Router,
};
use std::sync::{Arc, Mutex};
use store::Input;
type Db = Arc<Mutex<rusqlite::Connection>>;
struct ApiError(StatusCode, String);
impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        (self.0, Json(serde_error(&self.1))).into_response()
    }
}
#[derive(serde::Serialize)]
struct ErrorBody {
    error: String,
}
fn serde_error(message: &str) -> ErrorBody {
    ErrorBody {
        error: message.into(),
    }
}
async fn database<T: Send + 'static>(
    db: Db,
    operation: impl FnOnce(&rusqlite::Connection) -> rusqlite::Result<T> + Send + 'static,
) -> Result<T, ApiError> {
    tokio::task::spawn_blocking(move || {
        let db = db.lock().map_err(|_| {
            ApiError(
                StatusCode::INTERNAL_SERVER_ERROR,
                "Database unavailable".into(),
            )
        })?;
        operation(&db).map_err(|e| match e {
            rusqlite::Error::QueryReturnedNoRows => {
                ApiError(StatusCode::NOT_FOUND, "Task not found".into())
            }
            _ => {
                eprintln!("Database error: {e}");
                ApiError(
                    StatusCode::INTERNAL_SERVER_ERROR,
                    "Could not save or load tasks".into(),
                )
            }
        })
    })
    .await
    .map_err(|_| {
        ApiError(
            StatusCode::INTERNAL_SERVER_ERROR,
            "Database worker unavailable".into(),
        )
    })?
}
#[derive(serde::Deserialize)]
struct Filter {
    status: Option<String>,
}
async fn list(
    State(db): State<Db>,
    Query(filter): Query<Filter>,
) -> Result<Json<Vec<store::Task>>, ApiError> {
    let status = filter.status.unwrap_or("all".into());
    if !["all", "active", "completed"].contains(&status.as_str()) {
        return Err(ApiError(StatusCode::BAD_REQUEST, "Invalid status".into()));
    }
    database(db, move |db| store::list(db, &status))
        .await
        .map(Json)
}
async fn create(
    State(db): State<Db>,
    Json(mut input): Json<Input>,
) -> Result<impl IntoResponse, ApiError> {
    input
        .validate()
        .map_err(|e| ApiError(StatusCode::BAD_REQUEST, e))?;
    Ok((
        StatusCode::CREATED,
        Json(database(db, move |db| store::create(db, &input)).await?),
    ))
}
async fn update(
    State(db): State<Db>,
    Path(id): Path<i64>,
    Json(mut input): Json<Input>,
) -> Result<Json<store::Task>, ApiError> {
    input
        .validate()
        .map_err(|e| ApiError(StatusCode::BAD_REQUEST, e))?;
    database(db, move |db| store::update(db, id, &input))
        .await
        .map(Json)
}
async fn delete(State(db): State<Db>, Path(id): Path<i64>) -> Result<StatusCode, ApiError> {
    database(db, move |db| store::delete(db, id)).await?;
    Ok(StatusCode::NO_CONTENT)
}
fn app(db: Db) -> Router {
    Router::new()
        .route(
            "/",
            get(|| async { Html(include_str!("../static/index.html")) }),
        )
        .route(
            "/app.css",
            get(|| async {
                (
                    [(header::CONTENT_TYPE, "text/css; charset=utf-8")],
                    include_str!("../static/app.css"),
                )
            }),
        )
        .route(
            "/app.js",
            get(|| async {
                (
                    [(header::CONTENT_TYPE, "text/javascript; charset=utf-8")],
                    include_str!("../static/app.js"),
                )
            }),
        )
        .route("/api/tasks", get(list).post(create))
        .route("/api/tasks/{id}", axum::routing::put(update).delete(delete))
        .with_state(db)
}
#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let path = std::env::var("DATABASE_PATH").unwrap_or("taskline.sqlite3".into());
    let db = store::open(&path)?;
    if std::env::args().any(|a| a == "--tui") {
        return tui::run(db);
    }
    let addr = std::env::var("BIND_ADDR").unwrap_or("127.0.0.1:3000".into());
    let listener = tokio::net::TcpListener::bind(&addr).await?;
    println!("Taskline → http://{addr}");
    axum::serve(listener, app(Arc::new(Mutex::new(db))))
        .with_graceful_shutdown(async {
            let _ = tokio::signal::ctrl_c().await;
        })
        .await?;
    Ok(())
}
#[cfg(test)]
mod tests {
    use super::*;
    use axum::{body::Body, http::Request};
    use http_body_util::BodyExt;
    use tower::ServiceExt;
    async fn request(
        app: &Router,
        method: &str,
        path: &str,
        body: &str,
    ) -> (StatusCode, serde_json::Value) {
        let response = app
            .clone()
            .oneshot(
                Request::builder()
                    .method(method)
                    .uri(path)
                    .header("content-type", "application/json")
                    .body(Body::from(body.to_string()))
                    .unwrap(),
            )
            .await
            .unwrap();
        let status = response.status();
        let bytes = response.into_body().collect().await.unwrap().to_bytes();
        (
            status,
            serde_json::from_slice(&bytes).unwrap_or(serde_json::Value::Null),
        )
    }
    #[tokio::test]
    async fn task_lifecycle_and_validation() {
        let app = app(Arc::new(Mutex::new(store::open(":memory:").unwrap())));
        assert_eq!(
            request(&app, "POST", "/api/tasks", r#"{"title":" "}"#)
                .await
                .0,
            StatusCode::BAD_REQUEST
        );
        let (status, task) =
            request(&app, "POST", "/api/tasks", r#"{"title":"Write tests"}"#).await;
        assert_eq!(status, StatusCode::CREATED);
        let path = format!("/api/tasks/{}", task["id"]);
        assert_eq!(
            request(
                &app,
                "PUT",
                &path,
                r#"{"title":"Tests written","completed":true}"#
            )
            .await
            .0,
            StatusCode::OK
        );
        assert_eq!(
            request(&app, "GET", "/api/tasks?status=active", "")
                .await
                .1
                .as_array()
                .unwrap()
                .len(),
            0
        );
        assert_eq!(
            request(&app, "GET", "/api/tasks?status=completed", "")
                .await
                .1[0]["title"],
            "Tests written"
        );
        assert_eq!(
            request(&app, "GET", "/api/tasks?status=invalid", "")
                .await
                .0,
            StatusCode::BAD_REQUEST
        );
        assert_eq!(
            request(&app, "DELETE", &path, "").await.0,
            StatusCode::NO_CONTENT
        );
        assert_eq!(
            request(&app, "DELETE", &path, "").await.0,
            StatusCode::NOT_FOUND
        );
    }
    #[test]
    fn storage_survives_reopen() {
        let path =
            std::env::temp_dir().join(format!("taskline-test-{}.sqlite3", std::process::id()));
        {
            let db = store::open(path.to_str().unwrap()).unwrap();
            store::create(
                &db,
                &Input {
                    title: "Persistent".into(),
                    notes: "".into(),
                    category: "Work".into(),
                    priority: "High".into(),
                    completed: false,
                },
            )
            .unwrap();
        }
        {
            let db = store::open(path.to_str().unwrap()).unwrap();
            assert_eq!(store::list(&db, "all").unwrap()[0].title, "Persistent");
        }
        std::fs::remove_file(path).unwrap();
    }
}
