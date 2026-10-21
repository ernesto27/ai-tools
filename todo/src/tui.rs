use crate::store::{self, Input};
use crossterm::{
    cursor,
    event::{self, Event, KeyCode},
    execute,
    terminal::{self, ClearType},
};
use std::io::{self, Write};
struct Terminal;
impl Drop for Terminal {
    fn drop(&mut self) {
        let _ = terminal::disable_raw_mode();
        let _ = execute!(io::stdout(), terminal::LeaveAlternateScreen, cursor::Show);
    }
}
pub fn run(db: rusqlite::Connection) -> Result<(), Box<dyn std::error::Error>> {
    terminal::enable_raw_mode()?;
    let _guard = Terminal;
    execute!(io::stdout(), terminal::EnterAlternateScreen, cursor::Hide)?;
    let mut selected = 0usize;
    let mut filter = 0;
    let mut message = String::new();
    loop {
        let tasks = store::list(&db, ["all", "active", "completed"][filter])?;
        selected = selected.min(tasks.len().saturating_sub(1));
        execute!(
            io::stdout(),
            cursor::MoveTo(0, 0),
            terminal::Clear(ClearType::All)
        )?;
        print!("\x1b[1;32m TASKLINE / focus on what matters\x1b[0m\r\n\r\n Filter: {}    {} tasks\r\n\r\n",["All","Active","Completed"][filter],tasks.len());
        let (_, height) = terminal::size()?;
        let visible = height.saturating_sub(10) as usize;
        let start = selected.saturating_sub(visible.saturating_sub(1));
        for (i, task) in tasks.iter().enumerate().skip(start).take(visible) {
            let title: String = task
                .title
                .chars()
                .map(|c| if c.is_control() { ' ' } else { c })
                .take(60)
                .collect();
            print!(
                " {} [{}] {}  · {}\r\n",
                if i == selected { ">" } else { " " },
                if task.completed { "x" } else { " " },
                title,
                task.priority
            );
        }
        print!(
            "\r\n n new  e edit  space complete  d delete  f filter  ↑/↓ move  q quit\r\n {}",
            message
        );
        io::stdout().flush()?;
        if let Event::Key(key) = event::read()? {
            if key.kind != event::KeyEventKind::Press {
                continue;
            }
            message.clear();
            match key.code {
                KeyCode::Char('q') => break,
                KeyCode::Down => selected = (selected + 1).min(tasks.len().saturating_sub(1)),
                KeyCode::Up => selected = selected.saturating_sub(1),
                KeyCode::Char('f') => {
                    filter = (filter + 1) % 3;
                    selected = 0;
                }
                KeyCode::Char('n') | KeyCode::Char('e') => {
                    let editing = key.code == KeyCode::Char('e');
                    if editing && tasks.is_empty() {
                        continue;
                    }
                    let title = prompt("Title (blank cancels): ")?;
                    if title.trim().is_empty() {
                        continue;
                    }
                    let old = if editing { tasks.get(selected) } else { None };
                    let mut input = Input {
                        title,
                        notes: old.map(|t| t.notes.clone()).unwrap_or_default(),
                        category: old.map(|t| t.category.clone()).unwrap_or("Personal".into()),
                        priority: old.map(|t| t.priority.clone()).unwrap_or("Medium".into()),
                        completed: old.map(|t| t.completed).unwrap_or(false),
                    };
                    if let Err(error) = input.validate() {
                        message = error;
                        continue;
                    }
                    if let Some(task) = old {
                        store::update(&db, task.id, &input)?;
                    } else {
                        store::create(&db, &input)?;
                    }
                }
                KeyCode::Char(' ') => {
                    if let Some(t) = tasks.get(selected) {
                        store::update(
                            &db,
                            t.id,
                            &Input {
                                title: t.title.clone(),
                                notes: t.notes.clone(),
                                category: t.category.clone(),
                                priority: t.priority.clone(),
                                completed: !t.completed,
                            },
                        )?;
                    }
                }
                KeyCode::Char('d') => {
                    if let Some(t) = tasks.get(selected) {
                        if prompt("Delete this task? Type yes: ")?.trim() == "yes" {
                            store::delete(&db, t.id)?;
                        }
                    }
                }
                _ => {}
            }
        }
    }
    Ok(())
}
fn prompt(label: &str) -> io::Result<String> {
    terminal::disable_raw_mode()?;
    execute!(io::stdout(), cursor::Show)?;
    print!("\r\n{label}");
    io::stdout().flush()?;
    let mut input = String::new();
    let result = io::stdin().read_line(&mut input);
    terminal::enable_raw_mode()?;
    execute!(io::stdout(), cursor::Hide)?;
    result?;
    Ok(input.trim().into())
}
