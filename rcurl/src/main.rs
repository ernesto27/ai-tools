use clap::Parser;
use reqwest::{blocking::Client, header, redirect::Policy, Method, Url};
use std::{
    error::Error,
    fs::File,
    io::{self, Write},
    process::ExitCode,
    time::Duration,
};

#[derive(Parser)]
#[command(version, about = "A small streaming HTTP client")]
struct Args {
    /// HTTP or HTTPS URL
    url: Url,
    /// Request method (defaults to POST with -d, otherwise GET)
    #[arg(short = 'X', long = "request")]
    method: Option<Method>,
    /// Request header, in 'Name: value' form (repeatable)
    #[arg(short = 'H', long = "header", value_parser = parse_header)]
    headers: Vec<(header::HeaderName, header::HeaderValue)>,
    /// Literal request body (use --data=-value for a leading dash)
    #[arg(short = 'd', long = "data")]
    data: Option<String>,
    /// Include response status and headers on stdout
    #[arg(short = 'i', long = "include")]
    include: bool,
    /// Save response body to this file instead of stdout
    #[arg(short = 'o', long = "output")]
    output: Option<std::path::PathBuf>,
    /// Follow redirects (at most 10)
    #[arg(short = 'L', long = "location")]
    location: bool,
    /// Total request timeout in seconds, including body transfer
    #[arg(long, default_value = "30", value_parser = parse_seconds)]
    timeout: Duration,
    /// Connection timeout in seconds
    #[arg(long, default_value = "10", value_parser = parse_seconds)]
    connect_timeout: Duration,
}

fn parse_seconds(value: &str) -> Result<Duration, String> {
    let seconds: f64 = value.parse().map_err(|_| "expected positive seconds")?;
    let duration =
        Duration::try_from_secs_f64(seconds).map_err(|_| "expected finite positive seconds")?;
    if duration.is_zero() {
        return Err("timeout must be positive".into());
    }
    Ok(duration)
}

fn parse_header(value: &str) -> Result<(header::HeaderName, header::HeaderValue), String> {
    let (name, value) = value.split_once(':').ok_or("expected 'Name: value'")?;
    Ok((
        name.parse()
            .map_err(|e: header::InvalidHeaderName| e.to_string())?,
        value
            .trim()
            .parse()
            .map_err(|e: header::InvalidHeaderValue| e.to_string())?,
    ))
}

fn run(args: Args) -> Result<(), Box<dyn Error>> {
    if !matches!(args.url.scheme(), "http" | "https") {
        return Err("URL scheme must be http or https".into());
    }
    let client = Client::builder()
        .timeout(args.timeout)
        .connect_timeout(args.connect_timeout)
        .redirect(if args.location {
            Policy::limited(10)
        } else {
            Policy::none()
        })
        .build()?;
    let method = args.method.unwrap_or(if args.data.is_some() {
        Method::POST
    } else {
        Method::GET
    });
    let mut request = client.request(method, args.url);
    let mut headers = header::HeaderMap::new();
    for (name, value) in args.headers {
        headers.append(name, value);
    }
    request = request.headers(headers);
    if let Some(data) = args.data {
        request = request.body(data);
    }
    let mut response = request.send()?;
    let stdout = io::stdout();
    let mut stdout = stdout.lock();
    if args.include {
        writeln!(stdout, "{:?} {}\r", response.version(), response.status())?;
        for (name, value) in response.headers() {
            write!(stdout, "{name}: ")?;
            stdout.write_all(value.as_bytes())?;
            stdout.write_all(b"\r\n")?;
        }
        stdout.write_all(b"\r\n")?;
    }
    if let Some(path) = args.output {
        let mut file = File::create(path)?;
        io::copy(&mut response, &mut file)?;
        file.flush()?;
    } else {
        io::copy(&mut response, &mut stdout)?;
    }
    stdout.flush()?;
    Ok(())
}

fn main() -> ExitCode {
    match run(Args::parse()) {
        Ok(()) => ExitCode::SUCCESS,
        Err(error) => {
            eprintln!("rcurl: {error}");
            ExitCode::FAILURE
        }
    }
}
