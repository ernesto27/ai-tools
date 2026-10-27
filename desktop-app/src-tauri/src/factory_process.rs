use std::io::Read;
use std::process::{Command, Stdio};

#[derive(Clone, serde::Serialize)]
pub struct RunOutput {
    stream: &'static str,
    data: Vec<u8>,
}

fn forward_output(
    mut reader: impl Read,
    stream: &'static str,
    emit: &impl Fn(RunOutput),
) -> Result<(), String> {
    let mut buffer = [0; 4096];
    loop {
        let count = reader
            .read(&mut buffer)
            .map_err(|e| format!("Could not read {stream}: {e}"))?;
        if count == 0 {
            // Empty data marks EOF so the UI can flush this stream's UTF-8 decoder.
            emit(RunOutput {
                stream,
                data: Vec::new(),
            });
            return Ok(());
        }
        emit(RunOutput {
            stream,
            data: buffer[..count].to_vec(),
        });
    }
}

pub fn run(mut command: Command, emit: impl Fn(RunOutput) + Sync) -> Result<(), String> {
    let mut child = command
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .map_err(|e| format!("Could not start software-factory: {e}"))?;
    let stdout = child.stdout.take().expect("stdout was configured as piped");
    let stderr = child.stderr.take().expect("stderr was configured as piped");

    std::thread::scope(|scope| {
        let stdout_reader = scope.spawn(|| forward_output(stdout, "stdout", &emit));
        let stderr_reader = scope.spawn(|| forward_output(stderr, "stderr", &emit));
        let status = child.wait();
        let stdout_result = stdout_reader.join();
        let stderr_result = stderr_reader.join();

        stdout_result.map_err(|_| "stdout reader failed".to_string())??;
        stderr_result.map_err(|_| "stderr reader failed".to_string())??;
        let status = status.map_err(|e| format!("Could not wait for software-factory: {e}"))?;
        if !status.success() {
            return Err(format!(
                "software-factory failed ({status}). See debug output."
            ));
        }
        Ok(())
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Mutex;

    #[test]
    fn forwards_bytes_without_waiting_for_a_newline() {
        let mut bytes = Vec::new();
        let collect = Mutex::new(&mut bytes);
        forward_output(&b"partial \xf0\x9f\x98\x80"[..], "stdout", &|output| {
            assert_eq!(output.stream, "stdout");
            collect.lock().unwrap().extend(output.data);
        })
        .unwrap();
        assert_eq!(bytes, b"partial \xf0\x9f\x98\x80");
    }

    #[cfg(unix)]
    #[test]
    fn streams_before_process_exit() {
        let directory = tempfile::tempdir().unwrap();
        let gate = directory.path().join("received");
        let mut command = Command::new("sh");
        command.arg("-c").arg(
            "printf early; i=0; while [ ! -f \"$1\" ]; do i=$((i+1)); [ \"$i\" -lt 200 ] || exit 9; sleep 0.01; done; printf late"
        ).arg("stream-test").arg(&gate);
        let bytes = Mutex::new(Vec::new());
        run(command, |output| {
            bytes.lock().unwrap().extend(output.data);
            std::fs::write(&gate, "received").unwrap();
        })
        .unwrap();
        assert_eq!(*bytes.lock().unwrap(), b"earlylate");
    }

    #[cfg(unix)]
    #[test]
    fn drains_both_pipes_and_preserves_failure_output() {
        let mut command = Command::new("sh");
        command.arg("-c").arg("i=0; while [ \"$i\" -lt 10000 ]; do printf output; printf error >&2; i=$((i+1)); done; exit 7");
        let counts = Mutex::new((0, 0));
        let error = run(command, |output| {
            let mut counts = counts.lock().unwrap();
            match output.stream {
                "stdout" => counts.0 += output.data.len(),
                "stderr" => counts.1 += output.data.len(),
                _ => panic!("unexpected stream"),
            }
        })
        .unwrap_err();
        assert_eq!(*counts.lock().unwrap(), (60000, 50000));
        assert!(error.contains('7'));
    }

    #[test]
    fn reports_missing_executable() {
        let directory = tempfile::tempdir().unwrap();
        let command = Command::new(directory.path().join("missing-executable"));
        assert!(
            run(command, |_| {})
                .unwrap_err()
                .contains("Could not start software-factory")
        );
    }
}
