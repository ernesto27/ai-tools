use std::fmt::Write as _;
use std::fs;
use std::io::ErrorKind;
use std::path::{Path, PathBuf};
use std::sync::{Mutex, MutexGuard};

use serde::Serialize;
use serde_json::Value;

use crate::factory_config::FactoryConfigFile;

static STORE_LOCK: Mutex<Vec<PathBuf>> = Mutex::new(Vec::new());

#[derive(Serialize)]
pub(crate) struct ProfileSummary {
    id: String,
    name: String,
}

impl ProfileSummary {
    fn new(name: String) -> Self {
        Self {
            id: name.clone(),
            name,
        }
    }
}

pub(crate) struct ProfileLaunch {
    path: PathBuf,
}

impl ProfileLaunch {
    pub(crate) fn path(&self) -> &Path {
        &self.path
    }
}

impl Drop for ProfileLaunch {
    fn drop(&mut self) {
        let mut active = STORE_LOCK.lock().unwrap_or_else(|error| error.into_inner());
        active.retain(|path| path != &self.path);
    }
}

pub(crate) struct ProfileStore {
    repo: PathBuf,
    directory: PathBuf,
}

impl ProfileStore {
    pub(crate) fn new(app_data: &Path, repo: &str) -> Self {
        Self {
            repo: PathBuf::from(repo),
            directory: repo_directory(app_data, repo),
        }
    }

    fn names(&self) -> Result<Vec<String>, String> {
        let entries = match fs::read_dir(&self.directory) {
            Ok(entries) => entries,
            Err(error) if error.kind() == ErrorKind::NotFound => return Ok(Vec::new()),
            Err(error) => return Err(error.to_string()),
        };
        let mut names = Vec::new();
        for entry in entries {
            let entry = entry.map_err(|error| error.to_string())?;
            if !entry
                .file_type()
                .map_err(|error| error.to_string())?
                .is_file()
            {
                continue;
            }
            let filename = entry.file_name();
            let Some(stem) = filename
                .to_str()
                .and_then(|name| name.strip_suffix(".json"))
            else {
                continue;
            };
            if valid_name(stem).is_ok_and(|name| name == stem) {
                names.push(stem.to_owned());
            }
        }
        names.sort_by_cached_key(|name| (name.to_lowercase(), name.clone()));
        Ok(names)
    }

    fn file_path(&self, name: &str) -> PathBuf {
        self.directory.join(format!("{name}.json"))
    }

    fn existing_path(&self, id: &str) -> Result<PathBuf, String> {
        self.names()?
            .into_iter()
            .find(|name| name == id)
            .map(|name| self.file_path(&name))
            .ok_or_else(|| "Factory profile does not exist.".into())
    }

    fn available_name(&self, id: Option<&str>, name: &str) -> Result<String, String> {
        let name = valid_name(name)?;
        let key = name.to_lowercase();
        if self
            .names()?
            .iter()
            .any(|entry| Some(entry.as_str()) != id && entry.to_lowercase() == key)
        {
            return Err("This profile name is already in use.".into());
        }
        Ok(name)
    }

    pub(crate) fn list(&self) -> Result<Vec<ProfileSummary>, String> {
        let _guard = lock_store()?;
        Ok(self.names()?.into_iter().map(ProfileSummary::new).collect())
    }

    pub(crate) fn load(&self, id: &str) -> Result<Value, String> {
        let _guard = lock_store()?;
        FactoryConfigFile::at(self.existing_path(id)?)
            .load()?
            .ok_or_else(|| "Factory profile does not exist.".into())
    }

    pub(crate) fn create(&self, name: &str) -> Result<ProfileSummary, String> {
        let _guard = lock_store()?;
        let name = self.available_name(None, name)?;
        let config = FactoryConfigFile::new(&self.repo)
            .load()?
            .ok_or("Save the repository configuration before creating a profile.")?;
        fs::create_dir_all(&self.directory).map_err(|error| error.to_string())?;
        FactoryConfigFile::at(self.file_path(&name)).create(&config)?;
        Ok(ProfileSummary::new(name))
    }

    pub(crate) fn update(&self, id: &str, config: &Value) -> Result<(), String> {
        let active = lock_store()?;
        let path = self.existing_path(id)?;
        ensure_available(&active, &path)?;
        FactoryConfigFile::at(path).update(config)
    }

    pub(crate) fn rename(&self, id: &str, name: &str) -> Result<ProfileSummary, String> {
        let active = lock_store()?;
        let source = self.existing_path(id)?;
        ensure_available(&active, &source)?;
        let name = self.available_name(Some(id), name)?;
        if name == id {
            return Ok(ProfileSummary::new(name));
        }

        let destination = self.file_path(&name);
        match fs::symlink_metadata(&destination) {
            // Case-only renames can resolve to the source on a case-insensitive filesystem.
            Ok(metadata) if metadata.is_file() && name.to_lowercase() == id.to_lowercase() => {}
            Ok(_) => return Err("This profile name is already in use.".into()),
            Err(error) if error.kind() == ErrorKind::NotFound => {}
            Err(error) => return Err(error.to_string()),
        }
        fs::rename(source, destination).map_err(|error| error.to_string())?;
        Ok(ProfileSummary::new(name))
    }

    pub(crate) fn delete(&self, id: &str) -> Result<(), String> {
        let active = lock_store()?;
        let path = self.existing_path(id)?;
        ensure_available(&active, &path)?;
        fs::remove_file(path).map_err(|error| error.to_string())
    }

    pub(crate) fn prepare_launch(&self, id: &str, config: &Value) -> Result<ProfileLaunch, String> {
        let mut active = lock_store()?;
        let path = self.existing_path(id)?;
        ensure_available(&active, &path)?;
        FactoryConfigFile::at(path.clone()).update(config)?;
        active.push(path.clone());
        Ok(ProfileLaunch { path })
    }
}

fn lock_store() -> Result<MutexGuard<'static, Vec<PathBuf>>, String> {
    STORE_LOCK
        .lock()
        .map_err(|_| "Could not lock factory profiles.".into())
}

fn ensure_available(active: &[PathBuf], path: &Path) -> Result<(), String> {
    if active.iter().any(|current| current == path) {
        return Err("This profile is in use by a running workflow.".into());
    }
    Ok(())
}

fn repo_directory(app_data: &Path, repo: &str) -> PathBuf {
    let mut encoded = String::new();
    for byte in repo.bytes() {
        match byte {
            b'a'..=b'z' | b'0'..=b'9' => encoded.push(char::from(byte)),
            b'/' => encoded.push('-'),
            _ => write!(&mut encoded, "~{byte:02x}").expect("writing to a String cannot fail"),
        }
    }
    let mut directory = app_data.join("factory-profiles");
    for segment in encoded.as_bytes().chunks(96) {
        let segment = std::str::from_utf8(segment).expect("encoded paths use ASCII");
        directory.push(format!("repo{segment}"));
    }
    directory
}

fn valid_name(input: &str) -> Result<String, String> {
    let name = input.trim();
    let reserved_base = name
        .split('.')
        .next()
        .unwrap_or("")
        .trim_end()
        .to_ascii_uppercase();
    let device_name = matches!(
        reserved_base.as_str(),
        "CON" | "PRN" | "AUX" | "NUL" | "CONIN$" | "CONOUT$"
    ) || ["COM", "LPT"].iter().any(|prefix| {
        reserved_base.strip_prefix(*prefix).is_some_and(|suffix| {
            matches!(
                suffix,
                "1" | "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9" | "¹" | "²" | "³"
            )
        })
    });
    if name.is_empty()
        || name.chars().count() > 80
        || name.len() > 240
        || name.ends_with('.')
        || device_name
        || name
            .chars()
            .any(|c| c.is_control() || "<>:\"/\\|?*".contains(c))
    {
        return Err(
            "Enter a profile name with 1 to 80 characters that can be used as a filename.".into(),
        );
    }
    if matches!(
        name.to_lowercase().as_str(),
        "repository default" | "predeterminado del repositorio"
    ) {
        return Err("This profile name is already in use.".into());
    }
    Ok(name.to_owned())
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn persists_an_independent_profile_for_one_repo() {
        let root = tempfile::tempdir().unwrap();
        let repo = root.path().join("team name/project");
        let other = root.path().join("team-name/project");
        fs::create_dir_all(&repo).unwrap();
        fs::create_dir_all(&other).unwrap();
        let app_data = root.path().join("desktop");
        let store = ProfileStore::new(&app_data, repo.to_str().unwrap());
        let other_store = ProfileStore::new(&app_data, other.to_str().unwrap());
        assert_ne!(store.directory, other_store.directory);

        let default = FactoryConfigFile::new(&repo);
        let original = json!({"agentSandbox": {"agent": "codex"}, "extra": {"kept": true}});
        assert!(store.list().unwrap().is_empty());
        assert!(store.create("Missing").is_err());
        default.create(&original).unwrap();
        let root_file = repo.join("software-factory.json");
        let original_bytes = fs::read(&root_file).unwrap();

        let created = store.create(" Quick review ").unwrap();
        assert_eq!(created.name, "Quick review");
        let profile_path = store.directory.join("Quick review.json");
        assert_eq!(
            FactoryConfigFile::at(profile_path.clone()).load().unwrap(),
            Some(original.clone())
        );
        assert!(store.create("quick REVIEW").is_err());
        assert!(store.load("QUICK REVIEW").is_err());
        assert!(other_store.load(&created.id).is_err());
        assert!(other_store.list().unwrap().is_empty());
        assert!(store.update(&created.id, &json!([])).is_err());
        assert_eq!(store.load(&created.id).unwrap(), original);

        let mut changed = original.clone();
        changed["agentSandbox"]["effort"] = json!("low");
        store.update(&created.id, &changed).unwrap();
        store.create("alternate").unwrap();
        assert_eq!(store.load("alternate").unwrap(), original);
        assert_eq!(store.rename(&created.id, "Fast").unwrap().id, "Fast");
        assert!(!profile_path.exists());
        assert!(store.directory.join("Fast.json").exists());
        assert_eq!(store.rename("Fast", "fast").unwrap().id, "fast");
        assert!(store.load("Fast").is_err());

        let reopened = ProfileStore::new(&app_data, repo.to_str().unwrap());
        let names: Vec<_> = reopened
            .list()
            .unwrap()
            .into_iter()
            .map(|profile| profile.name)
            .collect();
        assert_eq!(names, ["alternate", "fast"]);
        assert_eq!(reopened.load("fast").unwrap(), changed);
        assert_eq!(fs::read(&root_file).unwrap(), original_bytes);
        fs::remove_file(root_file).unwrap();
        assert_eq!(reopened.load("fast").unwrap(), changed);

        reopened.delete("fast").unwrap();
        reopened.delete("alternate").unwrap();
        assert!(store.list().unwrap().is_empty());
        assert!(store.load("fast").is_err());
    }

    #[test]
    fn validates_portable_profile_names() {
        for invalid in [
            "../escape",
            "a\\b",
            "a:b",
            "a\0b",
            "Trailing.",
            " ",
            "COM1",
            "lpt².backup",
            "Repository DEFAULT",
            "Predeterminado del repositorio",
        ] {
            assert!(valid_name(invalid).is_err(), "{invalid:?}");
        }
        assert!(valid_name(&"a".repeat(80)).is_ok());
        assert!(valid_name(&"a".repeat(81)).is_err());
        assert!(valid_name(&"界".repeat(80)).is_ok());
        assert!(valid_name(&"🖍".repeat(61)).is_err());
        assert_eq!(valid_name("  Ágil  2  ").unwrap(), "Ágil  2");
    }

    #[test]
    fn encodes_registered_paths_without_losing_distinctions() {
        let data = Path::new("/data");
        assert_eq!(
            repo_directory(data, "/home/ernesto/my-repo"),
            data.join("factory-profiles/repo-home-ernesto-my~2drepo"),
        );
        assert_ne!(
            repo_directory(data, "/some/repo"),
            repo_directory(data, "/some-repo")
        );
        assert_ne!(
            repo_directory(data, "/home/Ernesto"),
            repo_directory(data, "/home/ernesto")
        );
        assert_eq!(
            repo_directory(data, &format!("/{}", "a".repeat(110))),
            data.join("factory-profiles")
                .join(format!("repo-{}", "a".repeat(95)))
                .join(format!("repo{}", "a".repeat(15))),
        );
    }

    #[test]
    fn launch_saves_to_the_profile_and_holds_its_path_until_completion() {
        let root = tempfile::tempdir().unwrap();
        let repo = root.path().join("repo");
        fs::create_dir(&repo).unwrap();
        let default = FactoryConfigFile::new(&repo);
        let original = json!({"agentSandbox": {"agent": "codex"}});
        default.create(&original).unwrap();
        let store = ProfileStore::new(root.path(), repo.to_str().unwrap());
        store.create("Quick review").unwrap();
        store.create("Other").unwrap();
        let edited = json!({"agentSandbox": {"branch": "qa", "resume": {"branch": "qa"}}});

        assert!(store.prepare_launch("Quick review", &json!([])).is_err());
        assert_eq!(store.load("Quick review").unwrap(), original);
        let launch = store.prepare_launch("Quick review", &edited).unwrap();
        assert_eq!(launch.path(), store.directory.join("Quick review.json"));
        assert_eq!(store.load("Quick review").unwrap(), edited);
        for result in [
            store.prepare_launch("Quick review", &original).map(drop),
            store.update("Quick review", &original),
            store.rename("Quick review", "Renamed").map(drop),
            store.delete("Quick review"),
        ] {
            assert_eq!(
                result.unwrap_err(),
                "This profile is in use by a running workflow."
            );
        }
        store.update("Other", &edited).unwrap();
        assert_eq!(default.load().unwrap(), Some(original.clone()));
        drop(launch);
        store.update("Quick review", &original).unwrap();
    }

    #[test]
    fn malformed_profile_is_reported_and_left_intact() {
        let temp = tempfile::tempdir().unwrap();
        let repo = temp.path().join("repo");
        let store = ProfileStore::new(temp.path(), repo.to_str().unwrap());
        fs::create_dir_all(&store.directory).unwrap();
        let path = store.directory.join("Broken.json");
        fs::write(&path, "{broken").unwrap();
        assert_eq!(store.list().unwrap()[0].name, "Broken");
        assert!(store.load("Broken").is_err());
        assert_eq!(fs::read_to_string(path).unwrap(), "{broken");
    }
}
