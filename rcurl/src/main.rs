use clap::Parser;
use reqwest::{blocking::Client, header, redirect::Policy, Method, Url};
use std::{
    error::Error,
    fs::File,
    io::{self, Write},
    process::ExitCode,
    time::{Duration, SystemTime},
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
    /// Literal JSON body; defaults Content-Type and Accept to application/json
    #[arg(long, conflicts_with = "data")]
    json: Option<String>,
    /// HTTP Basic credentials: user:password (missing password means empty)
    #[arg(short = 'u', long = "user")]
    user: Option<String>,
    /// Fail on HTTP 4xx/5xx without writing response headers or body
    #[arg(short = 'f', long = "fail")]
    fail: bool,
    /// Send HEAD and print response headers; conflicts with method and body
    #[arg(short = 'I', long = "head", conflicts_with_all = ["method", "data", "json"])]
    head: bool,
    /// Additional attempts for transient failures (0-10; idempotent methods only)
    #[arg(long, default_value = "0", value_parser = clap::value_parser!(u32).range(0..=10))]
    retry: u32,
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

fn retry_after(value: &str) -> Option<Duration> {
    value
        .parse::<u64>()
        .map(Duration::from_secs)
        .ok()
        .or_else(|| {
            httpdate::parse_http_date(value).ok().map(|date| {
                date.duration_since(SystemTime::now())
                    .unwrap_or(Duration::ZERO)
            })
        })
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
    let method = args.method.unwrap_or(if args.head {
        Method::HEAD
    } else if args.data.is_some() || args.json.is_some() {
        Method::POST
    } else {
        Method::GET
    });
    let idempotent = matches!(
        method,
        Method::GET | Method::HEAD | Method::PUT | Method::DELETE | Method::OPTIONS | Method::TRACE
    );
    let mut request = client.request(method, args.url);
    let mut headers = header::HeaderMap::new();
    for (name, value) in args.headers {
        headers.append(name, value);
    }
    if args.json.is_some() {
        headers
            .entry(header::CONTENT_TYPE)
            .or_insert(header::HeaderValue::from_static("application/json"));
        headers
            .entry(header::ACCEPT)
            .or_insert(header::HeaderValue::from_static("application/json"));
    }
    if let Some(user) = args.user {
        if headers.contains_key(header::AUTHORIZATION) {
            return Err("-u conflicts with an explicit Authorization header".into());
        }
        let (name, password) = user.split_once(':').unwrap_or((&user, ""));
        request = request.basic_auth(name, Some(password));
    }
    request = request.headers(headers);
    if let Some(data) = args.data.or(args.json) {
        request = request.body(data);
    }
    let mut retries = 0;
    let mut wait_budget = Duration::from_secs(60);
    let mut response = loop {
        let result = request
            .try_clone()
            .ok_or("request cannot be replayed")?
            .send();
        let transient = match &result {
            Ok(response) => matches!(
                response.status().as_u16(),
                408 | 429 | 500 | 502 | 503 | 504
            ),
            Err(error) => error.is_timeout() || error.is_connect(),
        };
        if !idempotent || !transient || retries >= args.retry {
            break result?;
        }
        let delay = result
            .as_ref()
            .ok()
            .and_then(|response| {
                response
                    .headers()
                    .get(header::RETRY_AFTER)?
                    .to_str()
                    .ok()
                    .and_then(retry_after)
            })
            .unwrap_or_else(|| Duration::from_secs(1 << retries.min(5)));
        if delay > wait_budget {
            break result?;
        }
        drop(result);
        std::thread::sleep(delay);
        wait_budget -= delay;
        retries += 1;
    };
    if args.fail && (response.status().is_client_error() || response.status().is_server_error()) {
        return Err(format!("HTTP {}", response.status()).into());
    }
    let stdout = io::stdout();
    let mut stdout = stdout.lock();
    if args.include || args.head {
        writeln!(stdout, "{:?} {}\r", response.version(), response.status())?;
        for (name, value) in response.headers() {
            write!(stdout, "{name}: ")?;
            stdout.write_all(value.as_bytes())?;
            stdout.write_all(b"\r\n")?;
        }
        stdout.write_all(b"\r\n")?;
    }
    if args.head {
        stdout.flush()?;
        return Ok(());
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn retry_after_accepts_seconds_and_dates() {
        assert_eq!(retry_after("2"), Some(Duration::from_secs(2)));
        assert_eq!(
            retry_after("Thu, 01 Jan 1970 00:00:00 GMT"),
            Some(Duration::ZERO)
        );
        let future = SystemTime::now() + Duration::from_secs(120);
        let delay = retry_after(&httpdate::fmt_http_date(future)).unwrap();
        assert!(delay > Duration::from_secs(118) && delay <= Duration::from_secs(120));
        assert_eq!(retry_after("invalid"), None);
        assert_eq!(retry_after("-1"), None);
    }
}
