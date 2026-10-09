# Commands and flags

Run workflow commands from your project directory.

| Command | Behavior |
| --- | --- |
| `software-factory` | Implement using `run.branch`, then review code, review security, classify risk, and explain changes. |
| `software-factory review` | Review and correct existing changes using `resume.branch`, then classify risk. |
| `software-factory continue` | Continue implementation and all enabled stages using `resume.branch`. |
| `software-factory doctor` | Check `agent-sandbox`, `git`, and `docker` on `PATH`; no configuration required. |
| `software-factory update` | Install the latest release using the bundled installer. |

| Flag | Purpose |
| --- | --- |
| `--jira URL` | Use a Jira Cloud issue instead of the local task; available for workflow commands. |
| `-v`, `--version` | Print the release tag, or `dev` for a local build. |
| `-h`, `--help` | Show command help. |

`--config` and `-c` are not supported. Soft Factory always reads
`software-factory.json` from the current working directory.

See [configuration](configuration.md) for JSON properties and
[reports](reports.md) for output locations and review statuses.
