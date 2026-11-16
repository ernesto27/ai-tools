use std::sync::{
    Mutex,
    atomic::{AtomicBool, Ordering},
};
use std::time::Duration;
#[cfg(target_os = "linux")]
use tauri::Manager;
use tauri::{AppHandle, State};
use tauri_plugin_updater::{Update, UpdaterExt};

#[derive(Default)]
pub struct DesktopUpdates {
    checked: AtomicBool,
    candidate: Mutex<Option<Update>>,
}

#[derive(serde::Serialize)]
pub struct AvailableUpdate {
    version: String,
}

fn supported_installation(_app: &AppHandle) -> bool {
    if cfg!(debug_assertions) {
        return false;
    }
    #[cfg(all(target_os = "linux", target_arch = "x86_64"))]
    {
        _app.env()
            .appimage
            .is_some_and(|path| std::path::Path::new(&path).is_file())
    }
    #[cfg(all(target_os = "macos", target_arch = "aarch64"))]
    {
        std::env::current_exe().ok().is_some_and(|path| {
            path.parent()
                .and_then(|p| p.parent())
                .and_then(|p| p.parent())
                .is_some_and(|bundle| bundle.extension().is_some_and(|ext| ext == "app"))
        })
    }
    #[cfg(not(any(
        all(target_os = "linux", target_arch = "x86_64"),
        all(target_os = "macos", target_arch = "aarch64")
    )))]
    {
        false
    }
}

// Check the installation and its directory before downloading and again before
// installing. This avoids offering privileged updates for shared/read-only apps.
fn writable_installation(_app: &AppHandle) -> Result<(), String> {
    #[cfg(unix)]
    {
        #[cfg(target_os = "linux")]
        let path = _app.env().appimage.map(std::path::PathBuf::from);
        #[cfg(target_os = "macos")]
        let path = std::env::current_exe().ok().and_then(|path| {
            path.parent()?
                .parent()?
                .parent()
                .map(std::path::Path::to_path_buf)
        });
        #[cfg(not(any(target_os = "linux", target_os = "macos")))]
        let path: Option<std::path::PathBuf> = None;
        let path = path.ok_or("Unsupported desktop installation.")?;
        check_writable_path(&path)
    }
    #[cfg(not(unix))]
    {
        Err("Unsupported desktop installation.".into())
    }
}

#[cfg(unix)]
fn check_writable_path(path: &std::path::Path) -> Result<(), String> {
    use std::ffi::CString;
    use std::os::unix::ffi::OsStrExt;

    let parent = path.parent().ok_or("Missing installation directory.")?;
    for (target, mode) in [(path, libc::W_OK), (parent, libc::W_OK | libc::X_OK)] {
        let target = CString::new(target.as_os_str().as_bytes())
            .map_err(|_| "Invalid installation path.")?;
        // SAFETY: target is NUL terminated and remains valid for this call.
        if unsafe { libc::access(target.as_ptr(), mode) } != 0 {
            return Err("Desktop installation is not writable.".into());
        }
    }
    Ok(())
}

#[tauri::command]
pub async fn check_desktop_update(
    app: AppHandle,
    state: State<'_, DesktopUpdates>,
) -> Result<Option<AvailableUpdate>, String> {
    if !supported_installation(&app) || state.checked.swap(true, Ordering::AcqRel) {
        return Ok(None);
    }
    writable_installation(&app)?;
    let update = app
        .updater_builder()
        .timeout(Duration::from_secs(20))
        .build()
        .map_err(|error| error.to_string())?
        .check()
        .await
        .map_err(|error| error.to_string())?;
    let metadata = update.as_ref().map(|update| AvailableUpdate {
        version: update.version.clone(),
    });
    *state
        .candidate
        .lock()
        .map_err(|_| "Could not lock desktop update.")? = update;
    Ok(metadata)
}

#[tauri::command]
pub async fn install_desktop_update(
    app: AppHandle,
    state: State<'_, DesktopUpdates>,
) -> Result<(), String> {
    let mut update = state
        .candidate
        .lock()
        .map_err(|_| "Could not lock desktop update.")?
        .take()
        .ok_or("No desktop update is available.")?;
    writable_installation(&app)?;
    update.timeout = Some(Duration::from_secs(15 * 60));
    let bytes = update
        .download(|_, _| {}, || {})
        .await
        .map_err(|error| error.to_string())?;
    writable_installation(&app)?;
    update.install(bytes).map_err(|error| error.to_string())?;
    app.restart();
}

#[cfg(all(test, unix))]
mod tests {
    use super::*;
    use std::os::unix::fs::PermissionsExt;

    #[test]
    fn rejects_missing_or_read_only_installations() {
        let directory = tempfile::tempdir().unwrap();
        let path = directory.path().join("Soft Factory.AppImage");
        assert!(check_writable_path(&path).is_err());
        std::fs::write(&path, "installed app").unwrap();
        assert!(check_writable_path(&path).is_ok());
        // Root bypasses Unix file permission checks.
        if unsafe { libc::geteuid() } == 0 {
            return;
        }
        std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o444)).unwrap();
        assert!(check_writable_path(&path).is_err());
        assert_eq!(std::fs::read_to_string(&path).unwrap(), "installed app");
    }

    #[test]
    fn requires_permission_to_replace_the_directory_entry() {
        let directory = tempfile::tempdir().unwrap();
        let path = directory.path().join("Soft Factory.AppImage");
        std::fs::write(&path, "installed app").unwrap();
        if unsafe { libc::geteuid() } == 0 {
            return;
        }
        std::fs::set_permissions(directory.path(), std::fs::Permissions::from_mode(0o555)).unwrap();
        let result = check_writable_path(&path);
        std::fs::set_permissions(directory.path(), std::fs::Permissions::from_mode(0o755)).unwrap();
        assert!(result.is_err());
    }
}
