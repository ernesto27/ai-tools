use std::fs::File;
use std::io::{self, BufReader, Write};
use std::path::Path;
use std::process::ExitCode;

use clap::{CommandFactory, Parser, error::ErrorKind};
use loglens::cli::Cli;
use loglens::filtering::Filter;
use loglens::processing::analyze;
use loglens::reporting::write_report;

fn run(cli: Cli) -> Result<(), String> {
    let filter = Filter {
        levels: cli.level,
        text: cli.text,
        since: cli.since,
        until: cli.until,
    };
    let source = cli
        .file
        .as_ref()
        .filter(|path| path.as_path() != Path::new("-"));
    let summary = if let Some(path) = source {
        let file =
            File::open(path).map_err(|err| format!("cannot open {}: {err}", path.display()))?;
        analyze(
            BufReader::new(file),
            cli.format,
            &filter,
            cli.strict,
            cli.top,
        )
        .map_err(|err| format!("{}: {err}", path.display()))?
    } else {
        analyze(io::stdin().lock(), cli.format, &filter, cli.strict, cli.top)
            .map_err(|err| format!("standard input: {err}"))?
    };
    let mut stdout = io::BufWriter::new(io::stdout().lock());
    write_report(&mut stdout, &summary, cli.output)
        .and_then(|()| stdout.flush())
        .map_err(|err| format!("writing report to standard output: {err}"))
}

fn main() -> ExitCode {
    let cli = Cli::parse();
    if let Err(error) = cli.validate() {
        Cli::command()
            .error(ErrorKind::ValueValidation, error)
            .exit();
    }
    match run(cli) {
        Ok(()) => ExitCode::SUCCESS,
        Err(error) => {
            // Diagnostics must not panic if the receiving process closes stderr.
            let _ = writeln!(io::stderr().lock(), "loglens: {error}");
            ExitCode::FAILURE
        }
    }
}
