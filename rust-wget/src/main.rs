use std::error::Error;
use std::io::{self, Write};
use std::path::{Path, PathBuf};
use std::process::ExitCode;

use clap::Parser;
use reqwest::Url;
use reqwest::blocking::{Client, Response};
use tempfile::NamedTempFile;

#[derive(Parser)]
#[command(version, about = "Download a file over HTTP or HTTPS")]
struct Args {
    /// URL to download
    url: Url,

    /// Save to FILE, or use '-' to write to standard output
    #[arg(short = 'O', long, value_name = "FILE")]
    output_document: Option<PathBuf>,

    /// Suppress the download summary
    #[arg(short, long)]
    quiet: bool,
}

fn main() -> ExitCode {
    match download(Args::parse()) {
        Ok(()) => ExitCode::SUCCESS,
        Err(error) => {
            eprintln!("rust-wget: {error}");
            let mut source = error.source();
            while let Some(cause) = source {
                eprintln!("  {cause}");
                source = cause.source();
            }
            ExitCode::FAILURE
        }
    }
}

fn download(args: Args) -> Result<(), Box<dyn Error>> {
    if !matches!(args.url.scheme(), "http" | "https") {
        return Err("only HTTP and HTTPS URLs are supported".into());
    }

    let filename = args
        .url
        .path_segments()
        .and_then(|mut segments| segments.next_back())
        .filter(|segment| !segment.is_empty())
        .unwrap_or("index.html")
        // A colon can make a path drive-relative or name an alternate stream on Windows.
        .replace(':', "%3A");

    let client = Client::builder()
        .user_agent(concat!("rust-wget/", env!("CARGO_PKG_VERSION")))
        .build()?;
    let mut response = client.get(args.url).send()?;
    if !response.status().is_success() {
        return Err(format!("HTTP {} from {}", response.status(), response.url()).into());
    }

    let output = args.output_document.as_deref();
    if output == Some(Path::new("-")) {
        let mut stdout = io::stdout().lock();
        io::copy(&mut response, &mut stdout)?;
        stdout.flush()?;
        return Ok(());
    }

    let destination = output.unwrap_or(Path::new(&filename));
    let (path, bytes) = save_file(&mut response, destination, output.is_none())
        .map_err(|error| format!("could not save {}: {error}", destination.display()))?;
    if !args.quiet {
        eprintln!("Saved {} ({bytes} bytes)", path.display());
    }
    Ok(())
}

fn save_file(
    response: &mut Response,
    destination: &Path,
    avoid_overwrite: bool,
) -> io::Result<(PathBuf, u64)> {
    let directory = destination
        .parent()
        .filter(|path| !path.as_os_str().is_empty())
        .unwrap_or_else(|| Path::new("."));
    let mut temp = NamedTempFile::new_in(directory)?;
    let bytes = io::copy(response, &mut temp)?;

    if !avoid_overwrite {
        temp.persist(destination).map_err(|error| error.error)?;
        return Ok((destination.to_owned(), bytes));
    }

    let mut candidate = destination.to_owned();
    let mut suffix = 0_u64;
    loop {
        match temp.persist_noclobber(&candidate) {
            Ok(_) => return Ok((candidate, bytes)),
            Err(error) if error.error.kind() == io::ErrorKind::AlreadyExists => {
                temp = error.file;
                suffix += 1;
                let mut name = destination.as_os_str().to_owned();
                name.push(format!(".{suffix}"));
                candidate = PathBuf::from(name);
            }
            Err(error) => return Err(error.error),
        }
    }
}
