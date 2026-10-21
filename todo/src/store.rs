use rusqlite::{params, Connection};

#[derive(Debug, Clone)]
pub struct Task {
    pub id: i64,
    pub title: String,
    pub notes: String,
    pub category: String,
    pub priority: String,
    pub completed: bool,
    pub created_at: String,
}
#[derive(Debug, Clone)]
pub struct Input {
    pub title: String,
    pub notes: String,
    pub category: String,
    pub priority: String,
    pub completed: bool,
}
impl Input {
    pub fn validate(&mut self) -> Result<(), String> {
        self.title = self.title.trim().to_string();
        if self.title.is_empty() || self.title.chars().count() > 200 {
            return Err("Title must contain 1–200 characters".into());
        }
        if self.notes.chars().count() > 2000 {
            return Err("Notes must be at most 2,000 characters".into());
        }
        if !["Personal", "Work", "Learning"].contains(&self.category.as_str()) {
            return Err("Invalid category".into());
        }
        if !["Low", "Medium", "High"].contains(&self.priority.as_str()) {
            return Err("Invalid priority".into());
        }
        Ok(())
    }
}
pub fn open(path: &str) -> rusqlite::Result<Connection> {
    let db = Connection::open(path)?;
    db.busy_timeout(std::time::Duration::from_secs(5))?;
    db.execute_batch("PRAGMA journal_mode=WAL; CREATE TABLE IF NOT EXISTS tasks (
      id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, notes TEXT NOT NULL DEFAULT '',
      category TEXT NOT NULL DEFAULT 'Personal', priority TEXT NOT NULL DEFAULT 'Medium',
      completed INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')));")?;
    Ok(db)
}
pub fn list(db: &Connection, status: &str) -> rusqlite::Result<Vec<Task>> {
    let mut stmt = db.prepare("SELECT id,title,notes,category,priority,completed,created_at FROM tasks WHERE (?1='all' OR (?1='active' AND completed=0) OR (?1='completed' AND completed=1)) ORDER BY completed,id DESC")?;
    let rows = stmt.query_map([status], |r| {
        Ok(Task {
            id: r.get(0)?,
            title: r.get(1)?,
            notes: r.get(2)?,
            category: r.get(3)?,
            priority: r.get(4)?,
            completed: r.get(5)?,
            created_at: r.get(6)?,
        })
    })?;
    rows.collect()
}
pub fn create(db: &Connection, input: &Input) -> rusqlite::Result<Task> {
    db.execute(
        "INSERT INTO tasks(title,notes,category,priority,completed) VALUES (?1,?2,?3,?4,?5)",
        params![
            input.title,
            input.notes,
            input.category,
            input.priority,
            input.completed
        ],
    )?;
    get(db, db.last_insert_rowid())
}
pub fn get(db: &Connection, id: i64) -> rusqlite::Result<Task> {
    list(db, "all")?
        .into_iter()
        .find(|t| t.id == id)
        .ok_or(rusqlite::Error::QueryReturnedNoRows)
}
pub fn update(db: &Connection, id: i64, input: &Input) -> rusqlite::Result<Task> {
    if db.execute(
        "UPDATE tasks SET title=?1,notes=?2,category=?3,priority=?4,completed=?5 WHERE id=?6",
        params![
            input.title,
            input.notes,
            input.category,
            input.priority,
            input.completed,
            id
        ],
    )? == 0
    {
        return Err(rusqlite::Error::QueryReturnedNoRows);
    }
    get(db, id)
}
pub fn delete(db: &Connection, id: i64) -> rusqlite::Result<()> {
    if db.execute("DELETE FROM tasks WHERE id=?1", [id])? == 0 {
        return Err(rusqlite::Error::QueryReturnedNoRows);
    }
    Ok(())
}
