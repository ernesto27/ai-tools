# Commands and flags

Run workflow commands from your project directory.

| Command | Behavior |
| --- | --- |
| `software-factory` | Implement using `run.branch`, then review code, review security, classify risk, and explain changes. |
| `software-factory continue` | Continue implementation and all enabled stages using `resume.branch`. |
| `software-factory resume <path>` | Recover a logged run from its saved stage status, appending to the same log folder; uses `resume.branch`. |
| `software-factory doctor` | Check `agent-sandbox`, `git`, and `docker` on `PATH`; no configuration required. |
| `software-factory update` | Install the latest release using the bundled installer. |
| `software-factory scheduled-task` | Create, edit, or delete [scheduled workflows](#scheduled-tasks) in your user crontab. |

| Flag | Purpose |
| --- | --- |
| `--jira URL` | Use a Jira Cloud issue instead of the local task; available for workflow commands. |
| `--file PATH` | Run the default workflow with one local prompt file and a new branch. |
| `-v`, `--version` | Print the release tag, or `dev` for a local build. |
| `-h`, `--help` | Show command help. |

`--config` and `-c` are not supported. Soft Factory always reads
`software-factory.json` from the project root, including embedded `agentSandbox`
settings. No sandbox configuration file is generated; supported settings
travel through flags. Use host authentication.

See [configuration](configuration.md) for JSON properties and
[reports](reports.md) for output locations and review statuses.

## Schedule a workflow {#scheduled-tasks}

On Linux or macOS, run these commands from the project directory containing
`software-factory.json`. The installed `software-factory` must be on your
`PATH` when you create or edit a schedule, and `agent-sandbox` must be on
cron's `PATH` when the job runs.

```sh
software-factory scheduled-task create --name weekday-morning --file task.txt --cron '30 8 * * 1-5'
software-factory scheduled-task edit --name weekday-morning --file task.txt --cron '0 9 * * 1-5'
software-factory scheduled-task delete --name weekday-morning
```

The first command schedules 8:30 a.m. on weekdays. The second changes it to
9:00 a.m. Names are unique within the project. Use a relative or absolute
`--file` path and quote the five-field `--cron` expression. To edit only the
prompt file, omit `--cron` to keep the existing time.

Each job starts in the project directory, reads its workflow settings from
`software-factory.json`, and uses the scheduled file as the task prompt. Each
run gets a new branch and uses it for all enabled stages.

Use `crontab -l` to see the installed jobs. Scheduled output, including
startup errors, is appended to
`~/.config/software-factory/scheduled-tasks/cron-output.log`. For stage logs
and summaries, see [reports](reports.md).
