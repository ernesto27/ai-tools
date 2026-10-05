---
name: code-changes
description: Review a set of code changes for the person — an overview (one-line summary, ASCII flow diagram, risks, next steps), then every changed file with what it does in general, what changed, why, and what to watch out for, followed by its diff. Terse and technical. Use this whenever the person asks to review, explain, walk through or summarize changes ("what did you change?", "show me the diff", "review the changes", "explain these edits", "what does this commit do?"), right after finishing a coding task when they want to see what was done, or before they commit or open a PR. Works on the edits made in this conversation, the git working tree, staged changes, a commit or range, or specific paths. It explains changes; it is not a bug hunt — for finding defects, the code-review skill fits better.
---

# Review code changes

Give the person a review they can read in a minute and trust: what changed, file by file, why it changed, how the pieces connect, and what could go wrong. It is the same review the `review-changes` mod draws after each turn, produced on request.

This is a read-only task. Don't edit, format, stage or commit anything while reviewing; the person wants to see the changes as they are.

## 1. Find the change set

Pick the source from what the person asked for (any arguments they passed come after `ARGUMENTS:`):

| Asked for | Source |
| --- | --- |
| nothing specific, right after you changed code | the files you created, edited or wrote with shell commands in this conversation's latest task |
| "uncommitted", "working tree", or nothing in a git repo with no edits of yours in this conversation | the working tree against `HEAD`, untracked files included |
| "staged" | `--staged` |
| a commit, `HEAD~1`, `main..feature`, a PR branch | that commit or range |
| one or more paths | the above, narrowed to those paths |

In a git repository, `scripts/changes.sh` (next to this file) prints the whole change set at once — a per-file line count, then every file's unified diff, untracked files included as all-added:

```bash
bash <skill-dir>/scripts/changes.sh                    # working tree vs HEAD, untracked included
bash <skill-dir>/scripts/changes.sh --staged           # staged only
bash <skill-dir>/scripts/changes.sh HEAD~1             # one commit back to now
bash <skill-dir>/scripts/changes.sh main..feature      # a range
bash <skill-dir>/scripts/changes.sh -- src/api         # narrowed to paths (works with any of the above)
```

If it prints `NOT_A_GIT_REPO`, there is no baseline on disk to diff against. Use the conversation instead: list every file you created or changed in this task (Edit, Write, and shell commands such as heredocs, `sed -i`, `mv`, `rm`). For an edit, the diff is the old and new text of each Edit call; for a new file, the diff is its whole content as added lines; for a file a shell command changed with no record of its old content, say "changed by shell; no baseline" instead of inventing a diff.

Then read each changed file, or enough of it, to say what it is for in the project as a whole. A diff alone shows what moved; the review also has to say what the file does, and that needs the surrounding code.

## 2. Write the review

Use this structure, in this order. Headings matter: the person scans them.

````markdown
# Review: <N> files changed, +<added> −<removed>

## Summary
<one sentence: what was built or changed, and the main design choice>

## Flow
```text
<ASCII boxes and arrows, at most 70 columns wide: the flow of control or
data through the changed code, from entry to end; changed nodes marked *>
```

## Risks and things to check
- `file:function` — <risk> — <how to check it>
  (at most five)

## Next steps
- <at most three one-liners, most useful first>

## Files

### `<path>` · <new file | edited | deleted | renamed | changed by shell> · +<a> −<d>
- **What it does:** <the file's role in the project and its main functions or types; for a deleted file, what it did>
- **What changed:** <functions, types or signatures added, changed or removed>
- **Why:** <the reason, in a clause>
- **Watch out:** <one risk or check, or "—">

```diff
<this file's unified diff>
```
````

Repeat the `### <path>` block for every changed file, in the order the change set lists them.

### Style

Short fragments, no filler, no restating the request, no praise. Keep every technical detail: files, functions, types, signatures, routes, status codes, config keys and error names go in backticks. An identifier beats a sentence about it. Aim for under 150 words of overview and under 60 words per file, not counting diffs and the diagram.

**Why** comes from the conversation when you made the changes (the person's request, a bug you hit, a constraint they set). For changes you didn't make (someone's commit, a teammate's working tree), infer it from the code and commit messages, and say "likely" when you are inferring.

The flow diagram is the part people find most useful for multi-file changes: show how a request or piece of data actually moves through the changed code, not a list of files with arrows between them. For a one-file change with no meaningful flow, a two- or three-node diagram is fine.

### Large change sets

- A file's diff over about 150 lines: show the hunks that carry the change (new functions, changed signatures, logic) and note `… <n> more lines not shown` rather than pasting everything.
- More than about 15 files: keep a `###` block for each file whose code matters; group generated, lock, vendored or formatting-only files into one line at the end (`Also changed: package-lock.json, 3 snapshot files`).
- Binary files: list them with their size, no diff.

## Example

For a turn that added a count endpoint to a Go notes API:

````markdown
# Review: 2 files changed, +24 −1

## Summary
Added `GET /notes/count`, reusing `Store.List` filters so `?tag=` and `?q=` apply.

## Flow
```text
GET /notes/count ──► mux ──► *handleCount ──► *Store.Count ──► Store.List(filter)
                                   │
                                   └──► writeJSON {"count": n}
```

## Risks and things to check
- `store.go:Count` — builds the full filtered slice just to count it — fine at this size; add a counter if notes grow large.
- `handlers.go:handleCount` — not covered by a test — `curl 'localhost:8080/notes/count?tag=go'`.

## Next steps
- Add a table test for `handleCount` with and without `?tag=`.

## Files

### `internal/notes/handlers.go` · edited · +15 −1
- **What it does:** HTTP layer: routes on `http.ServeMux`, JSON helpers, request validation for notes.
- **What changed:** new `handleCount`; route `GET /notes/count` registered in `Routes`.
- **Why:** person asked for a note count with the existing filters.
- **Watch out:** route must stay registered before `GET /notes/{id}` patterns that could shadow it.

```diff
@@ -31,6 +31,7 @@ func (h *Handler) Routes(mux *http.ServeMux) {
 	mux.HandleFunc("GET /notes", h.handleList)
+	mux.HandleFunc("GET /notes/count", h.handleCount)
 	mux.HandleFunc("GET /notes/{id}", h.handleGet)
```

### `internal/notes/store.go` · edited · +9 −0
...
````
