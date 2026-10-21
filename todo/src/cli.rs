use crate::store::{self, Input, Task};
use std::error::Error;

const HELP: &str = "Taskline — command-line task manager

Usage: taskline <command> [arguments]

Commands:
  add <title> [--notes TEXT] [--category VALUE] [--priority VALUE]
  list [all|active|completed]       List tasks (default: all)
  show <id>                         Show task details
  edit <id> [--title TEXT] [--notes TEXT] [--category VALUE] [--priority VALUE]
  complete <id>                     Mark a task completed
  reopen <id>                       Mark a task active
  delete <id>                       Delete a task immediately
  help                              Show this help

Categories: Personal, Work, Learning. Priorities: Low, Medium, High.
Defaults: Personal category and Medium priority.
Set DATABASE_PATH to select a database (default: taskline.sqlite3).
Quote titles and values containing spaces. Commands never prompt.";

pub fn run(args: Vec<String>) -> Result<(), Box<dyn Error>> {
    let Some(command) = args.first().map(String::as_str) else {
        println!("{HELP}");
        return Ok(());
    };
    if matches!(command, "help" | "--help" | "-h") {
        if args.len() != 1 {
            return Err("help takes no arguments".into());
        }
        println!("{HELP}");
        return Ok(());
    }
    // Validate the entire command before opening or changing a database.
    let mut input = Input {
        title: String::new(),
        notes: String::new(),
        category: "Personal".into(),
        priority: "Medium".into(),
        completed: false,
    };
    let mut fields = Vec::new();
    let mut id = 0;
    let mut status = "all";
    match command {
        "add" | "edit" => {
            let value = args
                .get(1)
                .ok_or("expected a title or task ID; see taskline help")?;
            if command == "add" {
                input.title = value.clone();
            } else {
                id = parse_id(value)?;
            }
            let (options, remainder) = args[2..].as_chunks::<2>();
            for pair in options {
                let field = pair[0].as_str();
                if fields.contains(&field) {
                    return Err(format!("duplicate option: {field}").into());
                }
                match field {
                    "--title" if command == "edit" => input.title = pair[1].clone(),
                    "--notes" => input.notes = pair[1].clone(),
                    "--category" => input.category = pair[1].clone(),
                    "--priority" => input.priority = pair[1].clone(),
                    _ => return Err(format!("unknown option: {field}").into()),
                }
                fields.push(field);
            }
            if !remainder.is_empty() {
                return Err("each option requires a value".into());
            }
            if command == "edit" && fields.is_empty() {
                return Err("edit requires at least one option".into());
            }
            let mut validation = input.clone();
            if command == "edit" && !fields.contains(&"--title") {
                validation.title = "unchanged".into();
            }
            validation.validate()?;
        }
        "list" => {
            if args.len() > 2 {
                return Err("usage: taskline list [all|active|completed]".into());
            }
            status = args.get(1).map(String::as_str).unwrap_or("all");
            if !["all", "active", "completed"].contains(&status) {
                return Err("status must be all, active, or completed".into());
            }
        }
        "show" | "complete" | "reopen" | "delete" => {
            if args.len() != 2 {
                return Err(format!("usage: taskline {command} <id>").into());
            }
            id = parse_id(&args[1])?;
        }
        _ => return Err(format!("unknown command: {command}; see taskline help").into()),
    }
    let path = std::env::var("DATABASE_PATH").unwrap_or("taskline.sqlite3".into());
    let db = store::open(&path)?;
    match command {
        "list" => {
            let tasks = store::list(&db, status)?;
            if tasks.is_empty() {
                println!("No tasks.");
            }
            for task in tasks {
                println!(
                    "{}\t{}\t{}\t{}\t{}",
                    task.id,
                    if task.completed {
                        "completed"
                    } else {
                        "active"
                    },
                    clean(&task.category),
                    clean(&task.priority),
                    clean(&task.title)
                );
            }
        }
        "add" => {
            input.validate()?;
            show(&store::create(&db, &input)?);
        }
        _ => {
            let task = store::get(&db, id).map_err(|error| match error {
                rusqlite::Error::QueryReturnedNoRows => format!("task {id} not found"),
                _ => error.to_string(),
            })?;
            match command {
                "show" => show(&task),
                "delete" => {
                    store::delete(&db, id)?;
                    println!("Deleted task {id}.");
                }
                _ => {
                    if !fields.contains(&"--title") {
                        input.title = task.title;
                    }
                    if !fields.contains(&"--notes") {
                        input.notes = task.notes;
                    }
                    if !fields.contains(&"--category") {
                        input.category = task.category;
                    }
                    if !fields.contains(&"--priority") {
                        input.priority = task.priority;
                    }
                    input.completed = match command {
                        "complete" => true,
                        "reopen" => false,
                        _ => task.completed,
                    };
                    input.validate()?;
                    show(&store::update(&db, id, &input)?);
                }
            }
        }
    }
    Ok(())
}

fn parse_id(value: &str) -> Result<i64, Box<dyn Error>> {
    value
        .parse::<i64>()
        .ok()
        .filter(|id| *id > 0)
        .ok_or_else(|| "task ID must be a positive integer".into())
}

fn clean(value: &str) -> String {
    value
        .chars()
        .map(|c| if c.is_control() { ' ' } else { c })
        .collect()
}

fn show(task: &Task) {
    println!(
        "ID: {}\nTitle: {}\nStatus: {}\nCategory: {}\nPriority: {}\nCreated: {}\nNotes: {}",
        task.id,
        clean(&task.title),
        if task.completed {
            "completed"
        } else {
            "active"
        },
        clean(&task.category),
        clean(&task.priority),
        clean(&task.created_at),
        clean(&task.notes)
    );
}
