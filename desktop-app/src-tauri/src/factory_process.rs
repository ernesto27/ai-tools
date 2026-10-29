use std::process::{Command, Stdio};

pub fn run(mut command: Command) -> Result<(), String> {
    command
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null());

    #[cfg(unix)]
    {
        use std::os::unix::process::CommandExt;
        // SAFETY: setsid is async-signal-safe; this hook does not allocate or lock.
        unsafe {
            command.pre_exec(|| {
                if libc::setsid() == -1 {
                    return Err(std::io::Error::last_os_error());
                }
                Ok(())
            });
        }
    }

    let mut child = command
        .spawn()
        .map_err(|error| format!("Could not start software-factory: {error}"))?;

    // Observe completion while the desktop is open. Exiting the desktop drops
    // this waiter without terminating the child in its independent session.
    let status = child
        .wait()
        .map_err(|error| format!("Could not wait for software-factory: {error}"))?;
    if !status.success() {
        return Err(format!(
            "software-factory failed ({status}). Check the run log files."
        ));
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn reports_missing_executable() {
        let directory = tempfile::tempdir().unwrap();
        let command = Command::new(directory.path().join("missing-executable"));
        assert!(
            run(command)
                .unwrap_err()
                .contains("Could not start software-factory")
        );
    }

    #[cfg(unix)]
    #[test]
    fn reports_exit_failure() {
        let mut command = Command::new("sh");
        command.args(["-c", "printf output; printf error >&2; exit 7"]);
        assert!(run(command).unwrap_err().contains('7'));
    }

    #[cfg(unix)]
    #[test]
    fn detached_parent_fixture() {
        let Some(directory) = std::env::var_os("FACTORY_DETACH_TEST_DIRECTORY") else {
            return;
        };
        let mut command = Command::new("sh");
        command
            .arg("-c")
            .arg(
                r#"
            printf '%s' "$$" > "$1/ready.tmp"
            mv "$1/ready.tmp" "$1/ready"
            i=0
            while [ ! -f "$1/continue" ]; do
                i=$((i+1)); [ "$i" -lt 500 ] || exit 9; sleep 0.01
            done
            printf output; printf error >&2
            printf finished > "$1/finished.tmp"
            mv "$1/finished.tmp" "$1/finished"
        "#,
            )
            .arg("detach-test")
            .arg(directory);
        run(command).unwrap();
    }

    #[cfg(unix)]
    #[test]
    fn child_survives_parent_exit_in_its_own_session() {
        use std::time::{Duration, Instant};

        fn wait_for_file(path: &std::path::Path) {
            let deadline = Instant::now() + Duration::from_secs(8);
            while !path.exists() {
                assert!(
                    Instant::now() < deadline,
                    "Timed out waiting for {}",
                    path.display()
                );
                std::thread::sleep(Duration::from_millis(10));
            }
        }

        let directory = tempfile::tempdir().unwrap();
        let mut parent = Command::new(std::env::current_exe().unwrap())
            .args(["--exact", "factory_process::tests::detached_parent_fixture"])
            .env("FACTORY_DETACH_TEST_DIRECTORY", directory.path())
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn()
            .unwrap();
        let ready = directory.path().join("ready");
        wait_for_file(&ready);
        let pid: libc::pid_t = std::fs::read_to_string(ready).unwrap().parse().unwrap();
        // SAFETY: getsid only queries the session of the live fixture child.
        assert_eq!(unsafe { libc::getsid(pid) }, pid);
        parent.kill().unwrap();
        parent.wait().unwrap();
        std::fs::write(directory.path().join("continue"), "").unwrap();
        let finished = directory.path().join("finished");
        wait_for_file(&finished);
        assert_eq!(std::fs::read_to_string(finished).unwrap(), "finished");
    }
}
