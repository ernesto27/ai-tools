# Runs and logs

Expand a repository and branch in the sidebar, then choose **Runs**. The branch screen shows its worktree path and recorded runs with their stage statuses. Use **Refresh** to reload the list and select a run to open its detail view.

## Read output

The detail screen combines stage status with the available output files:

| View | Content |
| --- | --- |
| Log | Output for the selected stage |
| Summary | `summary.log`, when present |
| Walkthrough | Rendered `05-reviewChanges.md`, when present |

Select a stage to read its output. During a desktop-started run, the detail view can follow the active stage. Use **Show folder** to reveal the run's log directory in your file manager.

![Completed run showing successful workflow stages, durations, and review changes output](/screenshots/completed-run-dark.png)

## Choose Run, Continue, or Resume

| Mode | Use it for |
| --- | --- |
| Run | A full workflow for a task on the configured run branch |
| Continue | Rerunning implementation and later stages in the existing `resume.branch` worktree |
| Resume | Picking up a failed or interrupted run from its log folder |

To continue, open the branch's **Continue** action and provide the next instructions. To resume, select **Resume** and choose a failed or interrupted run from the list. Review the configuration preview before launching either mode.

## Stop a running workflow

For an active run started in the current desktop session, choose **Stop** and confirm. The app waits for process cleanup and reports any stop error. Once stopped, you can use **Resume** to continue from the recorded logs.

Closing the desktop app is separate from stopping a workflow: a launched process can keep running after the app exits. Reopening the app lets you browse recorded runs, but does not restore the previous session's live process controls.
