# Your first run

Start with a small task in a local Git repository. The desktop app uses your installed CLIs to execute the workflow.

## Add a repository

1. Choose **Add repository** in the sidebar.
2. Select **Browse** and choose your repository folder.
3. Review the Git, configuration, and tool checks.
4. Choose **Add repository** to save it in the app.

A missing `software-factory.json` can be created in the repository's **Config** screen. The selected folder must be a Git repository. Saved repositories appear in the sidebar when you reopen the app.

## Configure the workflow

Open the repository menu and choose **Config**. Select your agent, model, and workflow settings, then save. See [Configuration](./configuration) for stage options and task sources.

## Start a task

1. Choose **New run** under the repository.
2. Keep the **Run** mode selected for a full workflow.
3. Enter the target branch name.
4. Provide a task as text, a file, a folder, or a Jira URL.
5. Review the `software-factory.json` preview and enabled stages.
6. Choose **Start run**.

The task source determines what is passed to Software Factory. File and folder inputs use native file pickers. Jira tasks require the CLI's [Jira configuration](https://ernesto27.github.io/ai-tools/soft-factory/jira).

![New run screen with task and configuration preview](/screenshots/new-run-dark.png)

## Follow the result

The run detail screen displays stage progress and output. Open **Log**, **Summary**, or **Walkthrough** to inspect available files. Review the changes and any reported failures before deciding what to do next.

Use [Runs and logs](./runs) to learn when to choose **Continue** or **Resume**.
