use crate::store::{self, Input, Task};
use crossterm::event::{self, Event, KeyCode, KeyEvent, KeyEventKind, KeyModifiers};
use ratatui::{
    layout::{Constraint, Layout},
    style::{Color, Style},
    widgets::{Block, List, ListItem, ListState, Paragraph, Wrap},
    Frame,
};
use rusqlite::Connection;

const FILTERS: [&str; 3] = ["all", "active", "completed"];

struct Editor {
    id: Option<i64>,
    input: Input,
    field: usize,
}

struct App {
    db: Connection,
    tasks: Vec<Task>,
    selection: ListState,
    filter: usize,
    editor: Option<Editor>,
    deleting: Option<i64>,
    message: String,
}

impl App {
    fn new(db: Connection) -> rusqlite::Result<Self> {
        let mut app = Self {
            db,
            tasks: Vec::new(),
            selection: ListState::default(),
            filter: 0,
            editor: None,
            deleting: None,
            message: String::new(),
        };
        app.refresh()?;
        Ok(app)
    }

    fn refresh(&mut self) -> rusqlite::Result<()> {
        self.tasks = store::list(&self.db, FILTERS[self.filter])?;
        self.selection.select(if self.tasks.is_empty() {
            None
        } else {
            Some(
                self.selection
                    .selected()
                    .unwrap_or(0)
                    .min(self.tasks.len() - 1),
            )
        });
        Ok(())
    }

    fn selected(&self) -> Option<&Task> {
        self.selection.selected().and_then(|i| self.tasks.get(i))
    }

    fn handle(&mut self, key: KeyEvent) -> Result<bool, Box<dyn std::error::Error>> {
        if key.kind == KeyEventKind::Release {
            return Ok(false);
        }
        if key.code == KeyCode::Char('c') && key.modifiers.contains(KeyModifiers::CONTROL) {
            return Ok(true);
        }
        if let Some(id) = self.deleting {
            match key.code {
                KeyCode::Char('y') => {
                    store::delete(&self.db, id)?;
                    self.deleting = None;
                    self.refresh()?;
                }
                KeyCode::Esc | KeyCode::Char('n') => self.deleting = None,
                _ => {}
            }
            return Ok(false);
        }
        if let Some(editor) = &mut self.editor {
            match key.code {
                KeyCode::Esc => {
                    self.editor = None;
                    self.message.clear();
                }
                KeyCode::Tab => editor.field = (editor.field + 1) % 4,
                KeyCode::BackTab => editor.field = (editor.field + 3) % 4,
                KeyCode::Enter => {
                    if let Err(error) = editor.input.validate() {
                        self.message = error;
                        return Ok(false);
                    }
                    if let Some(id) = editor.id {
                        store::update(&self.db, id, &editor.input)?;
                    } else {
                        store::create(&self.db, &editor.input)?;
                    }
                    self.editor = None;
                    self.message = "Saved".into();
                    self.refresh()?;
                }
                KeyCode::Left | KeyCode::Right if editor.field >= 2 => {
                    let values = if editor.field == 2 {
                        &["Personal", "Work", "Learning"]
                    } else {
                        &["Low", "Medium", "High"]
                    };
                    let value = if editor.field == 2 {
                        &mut editor.input.category
                    } else {
                        &mut editor.input.priority
                    };
                    let position = values.iter().position(|v| *v == value).unwrap_or(0);
                    let delta = if key.code == KeyCode::Left { 2 } else { 1 };
                    *value = values[(position + delta) % 3].into();
                }
                KeyCode::Backspace if editor.field < 2 => {
                    if editor.field == 0 {
                        &mut editor.input.title
                    } else {
                        &mut editor.input.notes
                    }
                    .pop();
                }
                KeyCode::Char(c)
                    if editor.field < 2
                        && !key
                            .modifiers
                            .intersects(KeyModifiers::CONTROL | KeyModifiers::ALT) =>
                {
                    let (text, limit) = if editor.field == 0 {
                        (&mut editor.input.title, 200)
                    } else {
                        (&mut editor.input.notes, 2000)
                    };
                    if !c.is_control() && text.chars().count() < limit {
                        text.push(c);
                    }
                }
                _ => {}
            }
            return Ok(false);
        }
        self.message.clear();
        match key.code {
            KeyCode::Char('q') | KeyCode::Esc => return Ok(true),
            KeyCode::Down | KeyCode::Char('j') => {
                if !self.tasks.is_empty() {
                    self.selection.select(Some(
                        (self.selection.selected().unwrap_or(0) + 1).min(self.tasks.len() - 1),
                    ));
                }
            }
            KeyCode::Up | KeyCode::Char('k') => {
                self.selection
                    .select(self.selection.selected().map(|i| i.saturating_sub(1)));
            }
            KeyCode::Char('f') => {
                self.filter = (self.filter + 1) % 3;
                self.selection.select(Some(0));
                self.refresh()?;
            }
            KeyCode::Char('n') => {
                self.editor = Some(Editor {
                    id: None,
                    input: Input {
                        title: String::new(),
                        notes: String::new(),
                        category: "Personal".into(),
                        priority: "Medium".into(),
                        completed: false,
                    },
                    field: 0,
                });
            }
            KeyCode::Char('e') => {
                if let Some(task) = self.selected() {
                    self.editor = Some(Editor {
                        id: Some(task.id),
                        input: task_input(task),
                        field: 0,
                    });
                }
            }
            KeyCode::Char(' ') => {
                if let Some(task) = self.selected() {
                    let mut input = task_input(task);
                    input.completed = !input.completed;
                    store::update(&self.db, task.id, &input)?;
                    self.refresh()?;
                }
            }
            KeyCode::Char('d') => self.deleting = self.selected().map(|t| t.id),
            _ => {}
        }
        Ok(false)
    }

    fn draw(&mut self, frame: &mut Frame) {
        let areas = Layout::vertical([
            Constraint::Length(3),
            Constraint::Min(1),
            Constraint::Length(8),
            Constraint::Length(4),
        ])
        .split(frame.area());
        frame.render_widget(
            Paragraph::new(format!(
                "TASKLINE  |  {}  |  {} tasks",
                FILTERS[self.filter],
                self.tasks.len()
            ))
            .block(Block::bordered())
            .style(Style::default().fg(Color::Cyan)),
            areas[0],
        );
        let items: Vec<_> = self
            .tasks
            .iter()
            .map(|t| {
                ListItem::new(format!(
                    "[{}] {}  · {} · {}",
                    if t.completed { "x" } else { " " },
                    clean(&t.title),
                    clean(&t.category),
                    clean(&t.priority)
                ))
            })
            .collect();
        frame.render_stateful_widget(
            List::new(items)
                .block(Block::bordered().title("Tasks"))
                .highlight_symbol("> ")
                .highlight_style(Style::default().bg(Color::DarkGray)),
            areas[1],
            &mut self.selection,
        );
        let details = if let Some(editor) = &self.editor {
            let fields = [
                format!(
                    "Title: {}",
                    tail(
                        &editor.input.title,
                        areas[2].width.saturating_sub(13) as usize
                    )
                ),
                format!(
                    "Notes: {}",
                    tail(
                        &editor.input.notes,
                        areas[2].width.saturating_sub(13) as usize
                    )
                ),
                format!("Category: {}", editor.input.category),
                format!("Priority: {}", editor.input.priority),
            ];
            fields
                .iter()
                .enumerate()
                .map(|(i, text)| format!("{} {text}", if i == editor.field { ">" } else { " " }))
                .collect::<Vec<_>>()
                .join("\n")
        } else if let Some(task) = self.selected() {
            format!(
                "{}\n{} · {} · Created {}\n{}",
                clean(&task.title),
                clean(&task.category),
                clean(&task.priority),
                clean(&task.created_at),
                clean(&task.notes)
            )
        } else {
            "No tasks in this filter. Press n to create a task.".into()
        };
        frame.render_widget(
            Paragraph::new(details)
                .wrap(Wrap { trim: false })
                .block(Block::bordered().title(if self.editor.is_some() {
                    "Editor (text appends; Backspace removes)"
                } else {
                    "Details"
                })),
            areas[2],
        );
        let help = if self.deleting.is_some() {
            "Delete selected task? y confirm · n/Esc cancel"
        } else if self.editor.is_some() {
            "Tab/Shift-Tab field · ←/→ category/priority · Enter save · Esc cancel · Ctrl-C quit"
        } else {
            "↑/↓ j/k move · n new · e edit · Space complete/reopen\nd delete · f filter all/active/completed · q/Esc/Ctrl-C quit"
        };
        frame.render_widget(
            Paragraph::new(format!("{help}\n{}", self.message)).wrap(Wrap { trim: false }),
            areas[3],
        );
    }
}

fn clean(text: &str) -> String {
    text.chars()
        .map(|c| if c.is_control() { ' ' } else { c })
        .collect()
}

// Keep the append position visible without letting long fields hide other controls.
fn tail(text: &str, width: usize) -> String {
    let text = clean(text);
    if ratatui::text::Line::from(text.as_str()).width() <= width {
        return text;
    }
    let mut rest = text.as_str();
    while !rest.is_empty() && ratatui::text::Line::from(rest).width() + 1 > width {
        rest = &rest[rest.chars().next().unwrap().len_utf8()..];
    }
    format!("…{rest}")
}

fn task_input(task: &Task) -> Input {
    Input {
        title: task.title.clone(),
        notes: task.notes.clone(),
        category: task.category.clone(),
        priority: task.priority.clone(),
        completed: task.completed,
    }
}

struct Restore;
impl Drop for Restore {
    fn drop(&mut self) {
        ratatui::restore();
    }
}

pub fn run(db: Connection) -> Result<(), Box<dyn std::error::Error>> {
    let mut app = App::new(db)?;
    let mut terminal = ratatui::try_init()?;
    let _restore = Restore;
    loop {
        terminal.draw(|frame| app.draw(frame))?;
        if let Event::Key(key) = event::read()? {
            match app.handle(key) {
                Ok(true) => break,
                Ok(false) => {}
                Err(error) => app.message = format!("Could not save or load tasks: {error}"),
            }
        }
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use ratatui::{backend::TestBackend, Terminal};

    fn key(app: &mut App, code: KeyCode) {
        assert!(!app.handle(KeyEvent::new(code, KeyModifiers::NONE)).unwrap());
    }

    fn text(app: &mut App, text: &str) {
        for c in text.chars() {
            key(app, KeyCode::Char(c));
        }
    }

    #[test]
    fn keyboard_lifecycle_and_filters() {
        let mut app = App::new(store::open(":memory:").unwrap()).unwrap();
        key(&mut app, KeyCode::Char('n'));
        key(&mut app, KeyCode::Enter);
        assert!(app.message.contains("Title"));
        assert!(app.tasks.is_empty());
        text(&mut app, "Write tests");
        key(&mut app, KeyCode::Tab);
        text(&mut app, "Notes");
        key(&mut app, KeyCode::Tab);
        key(&mut app, KeyCode::Right);
        key(&mut app, KeyCode::Tab);
        key(&mut app, KeyCode::Right);
        key(&mut app, KeyCode::Enter);
        assert_eq!(app.tasks[0].category, "Work");
        assert_eq!(app.tasks[0].priority, "High");
        key(&mut app, KeyCode::Char('e'));
        text(&mut app, " ✓");
        key(&mut app, KeyCode::Backspace);
        text(&mut app, "done");
        key(&mut app, KeyCode::Enter);
        assert_eq!(app.tasks[0].title, "Write tests done");
        assert_eq!(app.tasks[0].notes, "Notes");
        key(&mut app, KeyCode::Char(' '));
        assert!(app.tasks[0].completed);
        key(&mut app, KeyCode::Char('f'));
        assert!(app.tasks.is_empty());
        key(&mut app, KeyCode::Char('e'));
        key(&mut app, KeyCode::Char('d'));
        key(&mut app, KeyCode::Down);
        assert!(app.editor.is_none());
        assert!(app.deleting.is_none());
        key(&mut app, KeyCode::Char('f'));
        assert_eq!(app.tasks.len(), 1);
        key(&mut app, KeyCode::Char(' '));
        assert!(app.tasks.is_empty());
        key(&mut app, KeyCode::Char('f'));
        key(&mut app, KeyCode::Char('e'));
        text(&mut app, "discard");
        key(&mut app, KeyCode::Esc);
        assert_eq!(app.tasks[0].title, "Write tests done");
        key(&mut app, KeyCode::Char('d'));
        key(&mut app, KeyCode::Char('n'));
        assert_eq!(app.tasks.len(), 1);
        key(&mut app, KeyCode::Char('d'));
        key(&mut app, KeyCode::Char('y'));
        assert!(app.tasks.is_empty());
        assert!(app
            .handle(KeyEvent::new(KeyCode::Char('q'), KeyModifiers::NONE))
            .unwrap());
    }

    #[test]
    fn rendering_help_editor_and_small_terminal() {
        let mut app = App::new(store::open(":memory:").unwrap()).unwrap();
        let mut terminal = Terminal::new(TestBackend::new(100, 30)).unwrap();
        terminal.draw(|frame| app.draw(frame)).unwrap();
        let screen = terminal
            .backend()
            .buffer()
            .content
            .iter()
            .map(|c| c.symbol())
            .collect::<String>();
        assert!(screen.contains("No tasks"));
        assert!(screen.contains("Space complete/reopen"));
        key(&mut app, KeyCode::Char('n'));
        terminal.draw(|frame| app.draw(frame)).unwrap();
        let screen = terminal
            .backend()
            .buffer()
            .content
            .iter()
            .map(|c| c.symbol())
            .collect::<String>();
        assert!(screen.contains("Enter save"));
        assert!(screen.contains("Category: Personal"));
        let mut tiny = Terminal::new(TestBackend::new(20, 8)).unwrap();
        tiny.draw(|frame| app.draw(frame)).unwrap();
    }

    #[test]
    fn scrolling_long_editor_and_failed_save() {
        let mut app = App::new(store::open(":memory:").unwrap()).unwrap();
        for i in 0..40 {
            key(&mut app, KeyCode::Char('n'));
            text(&mut app, &format!("Task {i}"));
            key(&mut app, KeyCode::Enter);
        }
        for _ in 0..39 {
            key(&mut app, KeyCode::Down);
        }
        let mut terminal = Terminal::new(TestBackend::new(80, 24)).unwrap();
        terminal.draw(|frame| app.draw(frame)).unwrap();
        let screen = terminal
            .backend()
            .buffer()
            .content
            .iter()
            .map(|c| c.symbol())
            .collect::<String>();
        assert!(screen.contains("> [ ] Task 0"));
        key(&mut app, KeyCode::Char('e'));
        key(&mut app, KeyCode::Tab);
        text(&mut app, &"界".repeat(500));
        terminal.draw(|frame| app.draw(frame)).unwrap();
        let screen = terminal
            .backend()
            .buffer()
            .content
            .iter()
            .map(|c| c.symbol())
            .collect::<String>();
        assert!(screen.contains("Priority: Medium"));
        assert!(screen.contains("Enter save"));
        app.db.execute("DROP TABLE tasks", []).unwrap();
        assert!(app
            .handle(KeyEvent::new(KeyCode::Enter, KeyModifiers::NONE))
            .is_err());
        assert_eq!(
            app.editor.as_ref().unwrap().input.notes.chars().count(),
            500
        );
    }

    #[test]
    fn sqlite_survives_reopen_and_missing_records_fail() {
        let path = std::env::temp_dir().join(format!(
            "taskline-{}-{}.sqlite3",
            std::process::id(),
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_nanos()
        ));
        {
            let mut app = App::new(store::open(path.to_str().unwrap()).unwrap()).unwrap();
            key(&mut app, KeyCode::Char('n'));
            text(&mut app, "Persistent");
            key(&mut app, KeyCode::Enter);
        }
        {
            let db = store::open(path.to_str().unwrap()).unwrap();
            assert_eq!(store::list(&db, "all").unwrap()[0].title, "Persistent");
            assert!(store::delete(&db, -1).is_err());
            assert!(
                store::update(&db, -1, &task_input(&store::list(&db, "all").unwrap()[0])).is_err()
            );
        }
        std::fs::remove_file(path).unwrap();
    }
}
