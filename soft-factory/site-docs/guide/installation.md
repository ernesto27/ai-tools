# Installation

## What you need

Soft Factory supports **Linux x86_64**.

- Git installed and a project to work on.
- [Agent Sandbox installed and configured](https://ernesto27.github.io/ai-tools/installation.html).
- Docker running and your chosen coding agent signed in.

## Install Soft Factory

Open a terminal and run:

```bash
curl -fsSL https://raw.githubusercontent.com/ernesto27/ai-tools/master/soft-factory/install.sh | bash
```

The installer installs the latest version. Follow any instructions it shows
so you can run `software-factory` from your terminal.

## Check your setup

```bash
software-factory --version
software-factory doctor
```

The first command shows your installed version. The second checks for
Agent Sandbox, Git, and Docker. If it reports a missing tool, install that
tool before starting your first task.

## Update

To get the latest version:

```bash
software-factory update
```

Next, [configure your first task](first-run.md).
