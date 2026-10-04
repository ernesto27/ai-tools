---
name: review-changes
description: Review and explain every file and changed code section on the current Git branch, including committed, staged, unstaged, and untracked work. Use when asked for a complete branch-change walkthrough with inline diff lines, line references, why and how explanations, findings, and flow diagrams.
---

# Review changes

Produce a read-only, complete review of the current branch's changes. Explain what each change does, why it appears to be needed, how it fits the execution or data flow, and whether it introduces a concrete problem. Draw diagrams that help the reader follow the changed behavior.

## Establish the scope

- Honor an explicit base branch, commit, range, or file selection first. Otherwise identify the repository's default branch from local refs (for example `origin/HEAD`, then `main` or `master`) and use the merge base with `HEAD`. If the current branch *is* the default branch, use its upstream tracking ref when it exists. If no reliable base exists, report the ambiguity instead of silently reviewing only the last commit; ask for a base when interactive, or record the limitation and continue when running unattended.
- Check `git status --short`, `git branch -vv`, `git log --oneline <base>..HEAD`, `git diff --find-renames <base>...HEAD`, `git diff --cached`, `git diff`, and untracked paths. Do not fetch or change repository state merely to discover scope.
- When the caller supplies host Git evidence because the sandbox lacks Git metadata, use that evidence for status, base, and tracked patches instead of running Git commands. Inspect listed untracked files in the worktree to show their added lines.
- Build a file inventory from the union of committed, staged, unstaged, and untracked changes. Inspect untracked text files directly; `git diff` does not show them. Note deletions, renames, mode changes, generated files, and binaries even when no textual hunk exists.
- Explain the effective end state of the branch plus worktree, while keeping committed and uncommitted layers distinguishable. Avoid counting the same edit twice. Mention an intermediate committed change only if it matters to understanding a later reversal or risk.

## Analyze every change

- Read every changed hunk and enough surrounding code to understand its function. For changed functions, interfaces, configuration, or schemas, trace relevant callers and callees and check affected contracts. Use commit messages, task documents, and tests as evidence of intent; distinguish an inferred reason from a documented one.
- Cover every changed file and every substantive hunk. Explain adjacent lines together when they form one logical change, but do not use a broad summary to hide other edits. Identify cosmetic or generated changes briefly and explicitly.
- Show the actual added and removed code lines for each textual change in a fenced `diff` block beside its explanation. Preserve `+` and `-` markers and hunk line numbers. Include enough unchanged context to locate the edit, but do not replace changed lines with ellipses or paraphrases. For untracked text files, show their new lines as additions. For binaries or nontext assets, state that no line diff is available and describe the change from inspectable evidence.
- For each logical change, provide the path and exact current line number(s), what changed, why, how it works, and its effect on callers, data, errors, or users. For deleted code, cite the old path and line number or the diff hunk when no current line exists. Do not invent intent or behavior when the code does not establish it.
- Check correctness, edge cases, error handling, security, performance, tests, and project conventions where the edited code makes them relevant. State actionable findings with severity, location, trigger, impact, and a specific fix. Keep findings separate from neutral explanation. Report no findings when none are supported by evidence.
- Run focused read-only verification when it materially tests a concern. Report the command and result. Do not edit code as part of this review unless the user separately asks for fixes.

## Present the review

1. State the exact base, head, and included worktree layers, followed by a short behavior summary.
2. List every changed file with its status and role.
3. Walk through each file in file order. For every changed section, show its real diff lines first, then give line-linked explanations of *what*, *why*, and *how*. Group adjacent related lines without dropping any changed lines. Label the source of a diff when it matters (`committed`, `staged`, `unstaged`, or `untracked`), and make clear when later worktree edits supersede committed lines.
4. Put at least one diagram directly in the Markdown report file. Choose the diagram type that best explains the changed behavior, and use a descriptive heading and a fenced `text` block with plain text arrows, indentation, and labeled branches. Base every node and arrow on inspected code. Add another diagram only when it clarifies a distinct part of the change.
5. End with findings ordered by severity, verification performed, and any unresolved uncertainty. If the change is too large for one response, provide a complete file inventory, explain as many files as fit, and explicitly continue the remaining files in the next response rather than claiming full coverage.

Choose the diagram for the task:

- **Flowchart:** decisions, branches, and workflows; show the entry point through success and error paths.
- **Sequence diagram:** calls or messages between components; show participants and the order of interactions.
- **Architecture diagram:** services, modules, and data flow; show boundaries and the direction data moves.
- **State diagram:** lifecycle or status changes; show states and the events that trigger transitions.
- **Dependency graph:** package, module, or task dependencies; show which items depend on which others.

Use clickable `path:line` links when the interface supports them. Keep the detail proportional to each change while preserving complete coverage.
