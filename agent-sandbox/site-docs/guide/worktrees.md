# Worktrees and continuing work

A worktree is a second working directory for your repository. Each sandbox
task has its own branch and files, so you can work in your original directory
while the agent edits another.

## Find a task

Run from anywhere inside the original repository:

```bash
agent-sandbox worktree-list
```

This lists only worktrees recorded by `agent-sandbox` for that repository.
It does not manage worktrees you created manually with Git.

On Linux, worktrees normally live under:

```text
~/.config/agent-sandbox/worktrees/<repo>-<hash>/<branch>
```


## Open and review

```bash
agent-sandbox worktree-editor -b fix-login
```

This requires VS Code's `code` command in `PATH`. With another editor, open
the listed directory directly. Inspect the worktree's files and diff, then
run the project's checks there.

## Continue a task

```bash
agent-sandbox resume -b fix-login -a codex -q "add a regression test for the fix"
```

`resume` requires a recorded branch for the current repository and keeps
both committed and uncommitted work. You can choose a different agent for
the next invocation. Credentials and other execution options are resolved
again; the `resume` JSON section is used instead of `run`.

Only one session may hold a given worktree at a time. Wait for it to finish
or cancel it before resuming or deleting that task.

## Remove a completed task

Run deletion commands from your original repository, outside the worktree
being deleted:

```bash
agent-sandbox worktree-delete -b fix-login
```

This removes the worktree **and its local branch**. It refuses uncommitted
changes unless you pass `--force`. Retain or merge the work before deleting
the branch, even if its worktree is clean.

::: warning Deletion discards work
`worktree-delete -b fix-login --force` also discards uncommitted changes.
`worktree-delete-all` removes all registered worktrees for the current
repository, their branches, and uncommitted changes after confirmation.
`worktree-delete-all --yes` skips that confirmation.
:::

Deletion does not remove remote branches or GitHub PRs.
