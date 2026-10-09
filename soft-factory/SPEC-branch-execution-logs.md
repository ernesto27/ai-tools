# Group execution logs by branch

Status: Approved by the user on 2026-10-09.

## Objective

Group a sandbox branch's initial workflow and subsequent continues under one
branch folder, with command and start time identifying each invocation.
This changes the directory layout of future execution logs only.

## Required behavior

- The default workflow resolves `run.branch` using existing branch resolution
  and creates `logs/<sanitized-branch>/run-<UTC-timestamp>/`.
- `software-factory continue` resolves `resume.branch` using existing branch
  resolution and creates `logs/<sanitized-branch>/continue-<UTC-timestamp>/`.
- Commands selecting the same branch reuse the same parent folder. Create it
  when needed, including when a continue is the first invocation.
- Every invocation reserves a new child directory. Repeated runs and any number
  of continues are allowed. Never overwrite or append to earlier artifacts.
- Preserve the UTC timestamp format `2006-01-02_15-04-05.000000000` and collision
  suffixes `-2`, `-3`, and so on.
- Preserve branch sanitization: ASCII letters, digits, `.`, `_`, and `-` remain;
  other characters become `-`. `feature/logs` becomes one `feature-logs` folder.
- Keep `logs/` relative to the CLI working directory, directories at `0700`,
  and existing log file permissions unchanged.
- Store `summary.log`, stage logs, and `05-review-changes.md` in the invocation
  child. Preserve names, contents, and lifecycle behavior.
- `Execution logs: ...` prints the child path. All path consumers, including
  walkthrough saving, use that path.
- Preserve warning-and-continue behavior when execution logging is unavailable.
- `software-factory review` has no execution-log directory.

Example:

```text
logs/
└── disable-stages-pipeline2/
    ├── run-2026-10-08_19-38-48.007046183/
    │   ├── summary.log
    │   ├── 01-implementation.log
    │   └── ...
    ├── continue-2026-10-08_20-09-49.425026374/
    │   ├── summary.log
    │   └── ...
    └── continue-2026-10-08_21-15-30.123456789/
        ├── summary.log
        └── ...
```

## Implementation and validation

Use Go 1.25.3 or later, existing standard-library filesystem and time APIs, and
Cobra v1.10.2. No new dependencies are needed. Keep allocation in
`internal/executionlog` and command selection in CLI orchestration. Wrap
filesystem failures with contextual errors. Update the README and documentation
site, using the site's existing build workflow.

Use temporary directories and controlled Git fixtures to verify run and continue
prefixes, shared parents, continue-first allocation, independent files, branch
sanitization, same-timestamp collision suffixes, unchanged flat logs, artifact
paths, logging failures, and review-only behavior. Do not launch real workflows
that execute agents or modify worktrees for automated verification.

Run from `soft-factory`:

```sh
gofmt -w cmd internal
go build -o /tmp/software-factory ./cmd/factory
go test ./...
go vet ./...
```

From `soft-factory/site-docs`, run `npm ci` and `npm run docs:build`.

## Boundaries

Preserve branch resolution, pipeline behavior, timestamp precision, logging
lifecycle, permissions, and error behavior. Never move, delete, or reorganize
existing logs, merge execution files, or modify unrelated user changes. Do not
commit credentials or runtime log contents.

Other commands, branch encoding redesign, log root configuration, new
dependencies, CI changes, migrations, retention policies, indexes, and layout
flags are outside this feature. Distinct branch names that sanitize to the same
name retain existing normalization behavior.

## Success criteria

A run and two continues on one branch produce one parent with three correctly
prefixed, isolated invocation children. Same-timestamp allocations cannot
overwrite earlier logs. Future paths and documentation use the hierarchy;
existing flat logs, review-only behavior, and pipeline execution remain
unchanged. Go build, tests, vet, and the documentation build pass.

## Open questions

None.
