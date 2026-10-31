use std::fs;
use std::io::Write;
use std::path::{Path, PathBuf};

use serde_json::Value;
use tempfile::NamedTempFile;

pub struct FactoryConfigFile {
    path: PathBuf,
}

impl FactoryConfigFile {
    pub fn new(repo_path: impl AsRef<Path>) -> Self {
        Self {
            path: repo_path.as_ref().join("software-factory.json"),
        }
    }

    pub fn load(&self) -> Result<Option<Value>, String> {
        let text = match fs::read_to_string(&self.path) {
            Ok(text) => text,
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
                return Ok(None);
            }
            Err(error) => {
                return Err(format!("Could not read {}: {error}", self.path.display()));
            }
        };

        let config: Value = serde_json::from_str(&text)
            .map_err(|error| format!("Invalid JSON in {}: {error}", self.path.display()))?;

        Self::validate(&config)?;

        Ok(Some(config))
    }

    pub fn create(&self, config: &Value) -> Result<(), String> {
        let temporary = self.prepare_write(config)?;

        temporary
            .persist_noclobber(&self.path)
            .map(|_| ())
            .map_err(|error| format!("Could not create {}: {error}", self.path.display()))
    }

    pub fn update(&self, config: &Value) -> Result<(), String> {
        let metadata = fs::metadata(&self.path)
            .map_err(|error| format!("Could not open {}: {error}", self.path.display()))?;

        if !metadata.is_file() {
            return Err(format!("{} is not a file.", self.path.display()));
        }

        let temporary = self.prepare_write(config)?;

        temporary
            .as_file()
            .set_permissions(metadata.permissions())
            .map_err(|error| format!("Could not preserve file permissions: {error}"))?;

        temporary
            .persist(&self.path)
            .map(|_| ())
            .map_err(|error| format!("Could not update {}: {error}", self.path.display()))
    }

    fn validate(config: &Value) -> Result<(), String> {
        if !config.is_object() {
            return Err("Factory configuration must be a JSON object.".into());
        }

        Ok(())
    }

    fn prepare_write(&self, config: &Value) -> Result<NamedTempFile, String> {
        Self::validate(config)?;

        let mut bytes = serde_json::to_vec_pretty(config).map_err(|error| error.to_string())?;
        bytes.push(b'\n');

        let directory = self
            .path
            .parent()
            .ok_or("Configuration path has no parent directory.")?;

        let mut temporary = NamedTempFile::new_in(directory)
            .map_err(|error| format!("Could not create temporary config: {error}"))?;

        temporary
            .write_all(&bytes)
            .map_err(|error| format!("Could not write temporary config: {error}"))?;

        temporary
            .as_file()
            .sync_all()
            .map_err(|error| format!("Could not sync temporary config: {error}"))?;

        Ok(temporary)
    }
}
