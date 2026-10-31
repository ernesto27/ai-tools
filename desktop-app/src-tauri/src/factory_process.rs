use std::collections::HashMap;
use std::process::{Child, Command, ExitStatus, Stdio};
use std::sync::Mutex;

struct RunningProcess {
    child: Child,
    stop_requested: bool,
}

#[derive(Default)]
pub struct RunProcesses {
    entries: Mutex<HashMap<String, RunningProcess>>,
}

#[derive(Debug, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub enum RunOutcome {
    Succeeded,
    Interrupted,
}

impl RunProcesses {
    pub fn run(&self, run_id: String, command: Command) -> Result<RunOutcome, String> {
        self.start(run_id.clone(), command)?;

        loop {
            if let Some(status) = self.poll(&run_id)? {
                return self.finish(&run_id, status);
            }

            std::thread::sleep(std::time::Duration::from_millis(50));
        }
    }

    pub fn start(&self, run_id: String, mut command: Command) -> Result<(), String> {
        let mut entries = self
            .entries
            .lock()
            .map_err(|_| "Could not lock running processes.".to_string())?;

        if entries.contains_key(&run_id) {
            return Err("This run is already tracked.".to_string());
        }

        prepare(&mut command);

        let child = command
            .spawn()
            .map_err(|error| format!("Could not start software-factory: {error}"))?;

        entries.insert(
            run_id,
            RunningProcess {
                child,
                stop_requested: false,
            },
        );

        Ok(())
    }

    pub fn poll(&self, run_id: &str) -> Result<Option<ExitStatus>, String> {
        let mut entries = self
            .entries
            .lock()
            .map_err(|_| "Could not lock running processes.".to_string())?;

        let process = entries
            .get_mut(run_id)
            .ok_or_else(|| "This run is not tracked.".to_string())?;

        process
            .child
            .try_wait()
            .map_err(|error| format!("Could not check software-factory: {error}"))
    }

    pub fn finish(&self, run_id: &str, status: ExitStatus) -> Result<RunOutcome, String> {
        let mut entries = self
            .entries
            .lock()
            .map_err(|_| "Could not lock running processes.".to_string())?;

        let process = entries
            .remove(run_id)
            .ok_or_else(|| "This run is not tracked.".to_string())?;

        if status.success() {
            return Ok(RunOutcome::Succeeded);
        }

        if process.stop_requested {
            return Ok(RunOutcome::Interrupted);
        }

        Err(format!(
            "software-factory failed ({status}). Check the run log files."
        ))
    }

    #[cfg(unix)]
    pub fn request_stop(&self, run_id: &str) -> Result<(), String> {
        let mut entries = self
            .entries
            .lock()
            .map_err(|_| "Could not lock running processes.".to_string())?;

        let process = entries
            .get_mut(run_id)
            .ok_or_else(|| "This run is not tracked.".to_string())?;

        let status = process
            .child
            .try_wait()
            .map_err(|error| format!("Could not check software-factory: {error}"))?;

        if status.is_some() {
            return Err("This run has already finished.".to_string());
        }

        if process.stop_requested {
            return Ok(());
        }

        let pid = process.child.id() as libc::pid_t;

        // SAFETY: pid identifies our child; the lock prevents concurrent
        // polling or removal through this register while we signal it.
        if unsafe { libc::kill(pid, libc::SIGINT) } == -1 {
            return Err(format!(
                "Could not stop software-factory: {}",
                std::io::Error::last_os_error()
            ));
        }

        process.stop_requested = true;
        Ok(())
    }
}

fn prepare(command: &mut Command) {
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
}

#[cfg(test)]
mod tests {
    use super::*;

    #[cfg(unix)]
    #[test]
    fn tracked_run_completes_and_removes_entry() {
        let processes = RunProcesses::default();

        let mut command = Command::new("sh");
        command.args(["-c", "exit 0"]);

        let outcome = processes
            .run("test-run".to_string(), command)
            .expect("The process should complete successfully.");

        assert!(matches!(outcome, RunOutcome::Succeeded));

        let error = processes
            .poll("test-run")
            .expect_err("The completed run should no longer be tracked.");

        assert!(error.contains("This run is not tracked."));
    }

    #[cfg(unix)]
    #[test]
    fn tracked_run_can_be_interrupted() {
        use std::time::{Duration, Instant};

        let processes = RunProcesses::default();
        let run_id = "stop-test";

        let mut command = Command::new("sleep");
        command.arg("5");

        processes
            .start(run_id.to_string(), command)
            .expect("The process should start.");

        processes
            .request_stop(run_id)
            .expect("The stop request should succeed.");

        let deadline = Instant::now() + Duration::from_secs(2);

        let status = loop {
            if let Some(status) = processes
                .poll(run_id)
                .expect("Checking the process should succeed.")
            {
                break status;
            }

            assert!(
                Instant::now() < deadline,
                "The process did not stop in time."
            );

            std::thread::sleep(Duration::from_millis(10));
        };

        let outcome = processes
            .finish(run_id, status)
            .expect("The interrupted process should have an outcome.");

        assert!(matches!(outcome, RunOutcome::Interrupted));
        assert!(processes.poll(run_id).is_err());
    }

    #[test]
    fn reports_missing_executable() {
        let processes = RunProcesses::default();
        let directory = tempfile::tempdir().unwrap();
        let command = Command::new(directory.path().join("missing-executable"));

        let error = processes
            .run("missing-executable".to_string(), command)
            .unwrap_err();

        assert!(error.contains("Could not start software-factory"));
        assert!(processes.poll("missing-executable").is_err());
    }

    #[cfg(unix)]
    #[test]
    fn reports_exit_failure() {
        let processes = RunProcesses::default();
        let mut command = Command::new("sh");
        command.args(["-c", "printf output; printf error >&2; exit 7"]);

        let error = processes
            .run("failed-run".to_string(), command)
            .unwrap_err();

        assert!(error.contains('7'));
        assert!(processes.poll("failed-run").is_err());
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
        let outcome = RunProcesses::default()
            .run("detached-run".to_string(), command)
            .unwrap();
        assert!(matches!(outcome, RunOutcome::Succeeded));
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
