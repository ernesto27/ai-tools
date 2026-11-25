pub struct Docker {}

#[derive(Debug, serde::Serialize, serde::Deserialize)]
pub struct Image {
    pub name: String,
}

impl Docker {
    pub fn get_images() -> Result<Vec<Image>, String> {
        let output = std::process::Command::new("docker")
            .args(["images", "--format", "{{.Repository}}:{{.Tag}}"])
            .output()
            .map_err(|err| format!("Could not run docker images: {err}"))?;

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(format!(
                "docker images failed ({}): {}",
                output.status,
                stderr.trim()
            ));
        }

        Ok(parse_images(&output.stdout))
    }
}

fn parse_images(stdout: &[u8]) -> Vec<Image> {
    String::from_utf8_lossy(stdout)
        .lines()
        .map(str::trim)
        .filter(|name| !name.is_empty() && !name.contains("<none>"))
        .map(|name| Image {
            name: name.to_owned(),
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::parse_images;

    #[test]
    fn keeps_tagged_images_and_skips_blank_and_untagged_rows() {
        let images = parse_images(b"ubuntu:24.04\n<none>:<none>\nrepo:<none>\n\n  redis:7 \t\n");
        let names: Vec<&str> = images.iter().map(|image| image.name.as_str()).collect();

        assert_eq!(names, vec!["ubuntu:24.04", "redis:7"]);
    }

    #[test]
    fn returns_no_images_for_empty_output() {
        assert!(parse_images(b"").is_empty());
    }
}
