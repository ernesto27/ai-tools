use tauri::Manager;


// Learn more about Tauri commands at https://tauri.app/develop/calling-rust/
#[tauri::command]
fn greet(name: &str) -> String {
    format!("Hello, {}! You've been greeted from Rust!", name)
}

#[derive(serde::Serialize)]
#[serde(rename_all = "camelCase")]
struct FolderInfo {
    is_git: bool,
    has_config: bool,
}

#[derive(serde::Serialize, serde::Deserialize)]
struct SavedRepo {
    name: String,
    path: String,
}


#[tauri::command]
fn inspect_folder(path: String) -> FolderInfo {
    let root = std::path::Path::new(&path)
        .ancestors()
        .find(|dir| dir.join(".git").exists());
    FolderInfo {
        is_git: root.is_some(),
        has_config: std::path::Path::new(&path).join("software-factory.json").is_file(),
    }
}

#[tauri::command]
fn save_repo(app: tauri::AppHandle, path: String) -> Result<(), String> {
    let dir = app.path().app_data_dir().map_err(|e| e.to_string())?;
    std::fs::create_dir_all(&dir).map_err(|e| e.to_string())?;
    let file = dir.join("repos.json");

    let mut repos: Vec<SavedRepo> = match std::fs::read_to_string(&file) {
        Ok(text) => serde_json::from_str(&text).map_err(|e| e.to_string())?,
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => Vec::new(),
        Err(e) => return Err(e.to_string()),
    };

    if repos.iter().any(|r| r.path == path) {
        return Err("This repository is already added.".to_string());
    }

    let name = std::path::Path::new(&path)
        .file_name()
        .map(|n| n.to_string_lossy().into_owned())
        .unwrap_or_else(|| path.clone());
    repos.push(SavedRepo { name, path });

    let text = serde_json::to_string_pretty(&repos).map_err(|e| e.to_string())?;
    std::fs::write(&file, text).map_err(|e| e.to_string())
}

#[tauri::command]
async fn sandbox_help() -> Result<String, String> {
    let output = std::process::Command::new("agent-sandbox")
        .arg("-h")
        .output()
        .map_err(|e| format!("could not run agent-sandbox: {e}"))?;

    if !output.status.success() {
        return Err(String::from_utf8_lossy(&output.stderr).into_owned());
    }
    Ok(String::from_utf8_lossy(&output.stdout).into_owned())
}


#[derive(serde::Serialize, serde::Deserialize)]
struct Requirement {
    name: String,
    installed: bool,
}

#[tauri::command]
async fn check_requirements() -> Result<Vec<Requirement>, String> {
    let output = std::process::Command::new("agent-sandbox")
        .args(["doctor", "--json"])
        .output()
        .map_err(|e| format!("could not run agent-sandbox: {e}"))?;

    if !output.status.success() {
        return Err(String::from_utf8_lossy(&output.stderr).into_owned());
    }
    serde_json::from_slice(&output.stdout).map_err(|e| e.to_string())
}


#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_opener::init())
        .invoke_handler(tauri::generate_handler![greet, inspect_folder, save_repo, sandbox_help, check_requirements])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
