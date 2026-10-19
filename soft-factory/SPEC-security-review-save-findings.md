# Security review findings without automatic fixes

Status: Approved

## Objective and configuration

Allow a user to inspect security findings and suggested fixes in the current
invocation's `summary.log` and decide which fixes to apply later.

Merge into existing local `software-factory.json`:

```json
{"stages":{"securityReview":{"saveFindings":true}}}
```

This Factory-owned boolean belongs only to security review. Omitted or false
preserves automatic review and correction. Reject explicitly supplied null,
string, numeric, array, and object values with an error identifying
`stages.securityReview.saveFindings`. Existing agent/model assignments and their
precedence remain valid. An explicitly disabled security stage remains skipped.

## Execution contract

When enabled, inspect the same task changes, supporting documents, bundled
security skill, and configured custom skill. Perform one review pass and report
actionable findings and suggested fixes. Do not dispatch a fixer or enter the
correction-and-reverification loop. Every participating reviewer receives the
same restrictions; custom skills cannot override them.

Do not edit implementation files, apply fixes, stage changes, create commits,
push, or open pull requests. Preserve pre-existing tracked, staged, unstaged,
and untracked task changes. Temporary reports and host logs are permitted and
must stay outside task-change evidence and commits. Scope these restrictions to
security review; other enabled stages retain their existing behavior.

After successful execution, continue enabled stages even when findings remain.
Actual failures and interruptions preserve normal stop/recovery behavior.
Execution status and verdict are independent: PASS means no actionable findings;
UNRESOLVED means findings intentionally left unapplied; BLOCKED means insufficient
review evidence or verification. SUCCEEDED does not mean vulnerability-free.

## Summary and evidence

Append the dedicated final security report under its own stage attempt before
the next stage. Preserve status, agent, model, duration, and execution errors.
Include verdict, checks performed, and verification gaps. Each finding includes
priority, affected location when available, trigger, impact, and suggested fix.
Suggestions are prose or illustrative snippets, never applied patches. Reports
must not contain secret values.

Explicitly state when there are no actionable findings. Missing, unreadable,
invalid, or incomplete reports must say `Security findings unavailable.`
Keep the ordinary change summary separate, for example
`No code changes made (saveFindings enabled).` Do not impose its two-line limit
on findings. Collect the dedicated final report rather than echoed prompts or
incidental terminal output. Preserve the detailed security log and archived
review evidence. Risk classification receives unapplied findings as evidence
and must not describe them as fixed.

Resume appends entries for executed attempts, preserves historical summaries,
and never reuses stale findings. A successful security stage skipped during
resume acquires no new entry. Logging failures warn and never enable fixes or
commits. Retain collected artifacts if saving the summary fails. Recovery still
requires its existing logs to be opened successfully.

Default, continue, and resume use existing summaries. Standalone review creates
and prints a new invocation log directory only when this mode and security
review are enabled. It does not run implementation or the final walkthrough.
Other standalone-review configurations preserve existing behavior. Recovery
of a logged standalone review also preserves its review-only stage scope.

## Pinned sandbox planning check

Inspected Agent Sandbox `v0.0.0-20261016150234-8042bb4d5f95` before implementation:

- `internal/sandbox/options.go`, `Options.FullPrompt`, requests staging and
  commits when either `PR` or `Push` is true. When both are false it explicitly
  instructs the agent not to stage, commit, or push.
- `publishResult` in `pullrequest.go` routes to `publish` in `sandbox.go` when
  `PR` is false, which skips publication when `Push` is also false. Host publication
  requires an already agent-committed worktree; there is no independent automatic
  host commit in the reporting path.
- `containerOptions` mounts Git administration data unconditionally. The two
  false settings select no-commit behavior; they are not filesystem write
  protection. The reporting prompt supplies the no-edit restriction to all
  participating agents. Host change evidence supplements direct inspection.

Override both publication booleans at the final argument boundary, including
conflicting inherited settings. No dependency upgrade or new dependency is
required. Reuse upstream exported configuration types and pointer semantics.

## Validation and boundaries

Use meaningful standard-library tests with controlled subprocesses and temporary
Git repositories, without Docker, credentials, or remote publication. Cover
configuration values and assignment precedence; reporting prompt restrictions
with a conflicting custom skill; final publication arguments; preserved HEAD,
index, dirty task files, and untracked task files; multiline, empty, invalid,
missing, clean, blocked, and unresolved reports; execution failures; continuing
risk classification; supported workflows and disabled stages; repeated attempts
and resume; and artifact retention when summary writing fails.

Run from the repository root:

```sh
go build -o /tmp/software-factory ./cmd/factory
go test ./...
go vet ./...
gofmt -w cmd internal
npm --prefix site-docs run docs:build
```

Do not add equivalent options to other stages, alter security priorities,
rewrite old summaries, discard user changes, expose secrets, or commit generated
reports/local configuration. Manual apply commands, automatically applying saved
suggestions, and a global read-only workflow are out of scope.
