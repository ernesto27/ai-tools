mod cli;
mod store;

fn main() {
    if let Err(error) = cli::run(std::env::args().skip(1).collect()) {
        eprintln!("error: {error}");
        std::process::exit(1);
    }
}
