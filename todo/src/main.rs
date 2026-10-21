mod store;
mod tui;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let path = std::env::var("DATABASE_PATH").unwrap_or("taskline.sqlite3".into());
    tui::run(store::open(&path)?)
}
