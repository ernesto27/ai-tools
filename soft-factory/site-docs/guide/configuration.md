# Configuration

Create `software-factory.json` in the project folder and run Soft Factory there.
Start with this configuration:

```json
{
  "agentSandbox": {
    "agent": "codex",
    "branch": "fix-login",
    "push": false,
    "pr": false,
    "run": {
      "filePrompt": "task.txt"
    }
  }
}
```

Write your task in `task.txt`. The shared settings apply to implementation and
reviews. Choose a new branch for each new task.

| Section | Purpose |
| --- | --- |
| `agentSandbox` | Agent, branch, task file, and sandbox settings. Required. |
| `stages` | Optional settings for individual workflow stages. |
| `documents` | Optional local files to include with the task. |
| `googleDrive` | Optional Google Docs to include with the task. |

Use camelCase property names. The examples below are additions to this file;
keep your existing settings when adding them.

## Shared settings and overrides

Put settings in `agentSandbox` to share them. Add a `run` or `resume` section
only when you need different values:

- `run` controls implementation for a new task.
- `resume` controls later stages, `continue`, and recovery with `resume <path>`.
- Stage settings override the selected section, then shared settings.

The default workflow uses `run.branch`; continuation and recovery use
`resume.branch`. Both fall back to the shared `branch`. Every stage in a run
uses the same selected branch.

The local task always comes from `run.filePrompt`, including continuation and
recovery. To use a Jira issue instead, pass [`--jira`](jira.md).

### Sandbox options

These options can be shared or set inside `run` and `resume`:

| Option | Purpose |
| --- | --- |
| `agent` | `codex`, `claude`, `opencode`, or `pi`. Reviews need subagent support. |
| `branch` | Sandbox branch for the task. |
| `model` | Model supported by your selected agent and account. |
| `effort` | Effort setting supported by the agent. |
| `baseImage` | Docker image with the tools your task needs. |
| `push` | Commit and push changes after a stage. |
| `pr` | Commit, push, and create or reuse a GitHub pull request. |
| `hn` | Allow access to host services. Defaults to `false`. |
| `file` | List of read-only file mounts. |

Only `run` and `resume` also accept `filePrompt`, `query`, `commitMessage`,
and `image`. Use `run.filePrompt` for the original task; Factory supplies
prompts for later stages. `image` is a list of attachments for agents that
support them. `commitMessage` applies when publishing changes.

Section values replace shared values, including explicit `false`, empty
strings, and empty lists. Omitted options use Agent Sandbox defaults.

## Stage agents and models

Use `stages` to change individual steps:

```json
{
  "stages": {
    "implementation": {"effort": "high"},
    "codeReview": {"agent": "claude", "model": "claude-sonnet-4-6"}
  }
}
```

Authenticate each agent you use. When switching agents, choose a compatible
model too.

| Stage key | Step |
| --- | --- |
| `implementation` | Implement the task. |
| `codeReview` | Review code and apply corrections. |
| `securityReview` | Review security and apply corrections. |
| `riskClassification` | Classify the final risk. |
| `reviewChanges` | Write a read-only walkthrough. |

All five stages accept `agent`, `model`, `effort`, `customSkill`, and `disabled`.
Security review also accepts `saveFindings`.

Omitted or empty stage `agent` and `model` values keep the inherited settings.
For `effort`, omission or `null` inherits; `""` overrides with an empty value.
Agent Sandbox validates supported agents, models, and effort values.

## Custom skills

Add a skill name to any stage:

```json
{
  "stages": {
    "implementation": {"customSkill": "implementation-guide"},
    "codeReview": {"customSkill": "project-review"}
  }
}
```

Create `.agents/skills/<name>/SKILL.md` in your project. Factory also checks
`.claude/skills/<name>/SKILL.md`, using the first match. Supply a directory
name, not a path.

Skills add instructions to the task or the stage's default review instructions.
A missing or unreadable skill stops the workflow before agent execution when
its stage is due to run. Disabled stages do not load their skills.

## Disabling stages

Set `disabled: true` to skip a stage:

```json
{
  "stages": {
    "securityReview": {"disabled": true}
  }
}
```

Later stages still run. Skipped stages produce no stage log or report.
Risk classification notes missing review evidence.

If you skip implementation, the selected sandbox worktree must already exist.
If you skip every stage, configuration and context checks still run.

## Save security findings without corrections

To get suggested fixes without applying them:

```json
{
  "stages": {
    "securityReview": {"saveFindings": true}
  }
}
```

Security review makes one pass without editing task files, committing, pushing,
or opening a PR. Findings, suggested fixes, and verification gaps go into
`summary.log`. Risk classification receives the unapplied findings.
Other stages keep their usual behavior.

The default is `false`; `null` also means `false`. Findings do not stop later
stages, but execution errors do. See [reports](reports.md).

## Supporting documents

```json
{
  "documents": ["requirements.md"],
  "googleDrive": {
    "folders": ["Project Documentation"],
    "files": ["https://docs.google.com/document/d/DOCUMENT_ID/edit"]
  }
}
```

Local paths are relative to `software-factory.json`. Omit sections you do not
need. For Drive, provide folders, document URLs, or both. Folder names must
identify one accessible folder; only Google Docs directly inside it are loaded.

Drive requires `service_account.json` in the project folder and shared access
for that account. Keep credentials private. See [supporting documents](documents.md)
and [Google Drive setup](google-drive.md).

## Sandbox settings transport

Factory sends settings to Agent Sandbox as CLI flags. It creates no sandbox
configuration file. Relative paths keep the project folder as their base.
Factory's generated prompts and stage controls take priority over configured
values.

`apiKey` and `reviewers` are currently ignored in `run` and `resume`. They are
never forwarded or logged. Use host authentication.

::: tip Migrating an older configuration
Rename `config.json` to `software-factory.json`; `--config` and `-c` are no
longer supported. Move top-level `customSkills` values into
`stages.<stage>.customSkill`, and `disabledStages` entries into
`stages.<stage>.disabled: true`. Keep existing stage assignments.
:::
