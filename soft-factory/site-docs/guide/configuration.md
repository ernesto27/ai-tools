# Configuration

Create `software-factory.json` in the project root before running Soft Factory.
It is the single user-maintained workflow configuration source.

### `software-factory.json`: workflow and sandbox settings

The CLI always reads `software-factory.json` from the current working directory.
Rename an existing `config.json` to `software-factory.json`; there is no fallback
to the old filename, and `--config` / `-c` are no longer supported.

All property names use lower camelCase, including nested properties. These are all the options currently supported:

| Option | Purpose |
| --- | --- |
| `agentSandbox` | Embedded Agent Sandbox shared fields and `run`/`resume` sections; see transport details below. |
| `documents` | Optional list of local document paths. Paths resolve relative to this configuration file. Use `[]` or omit it when no local documents are needed. Blank paths are rejected. |
| `googleDrive` | Optional Google Drive settings. When present, at least one of `folders` or `files` must contain an entry. Omit this section when Drive is not needed. |
| `googleDrive.folders` | Optional list of exact folder names. Each name must identify one accessible folder. Blank names are rejected. |
| `googleDrive.files` | Optional list of full Google Docs URLs. Documents can be outside the configured folders. URLs are validated when configuration loads, before the Drive client starts. |
| `customSkills` | Optional project skills that extend the bundled stage skills. |
| `customSkills.codeReview` | Optional project skill name for the code review stage. |
| `customSkills.securityReview` | Optional project skill name for the security review stage. |
| `customSkills.riskClassification` | Optional project skill name for the risk classification stage. |
| `customSkills.reviewChanges` | Optional project skill name for the final review-changes walkthrough. |
| `stages` | Optional agent and model overrides for individual stages. See [Stage agents and models](#stage-agents-and-models). |
| `stages.<stage>.agent` | Optional agent override: `codex`, `claude`, `opencode`, or `pi`. |
| `stages.<stage>.model` | Optional model ID supported by the agent selected for that stage. |
| `disabledStages` | Optional list of stages to skip: `implementation`, `codeReview`, `securityReview`, `riskClassification`, or `reviewChanges`. See [Disabling stages](#disabling-stages). |

Minimal example:

```json
{
  "documents": [],
  "agentSandbox": {
    "agent": "codex",
    "branch": "factory-task",
    "run": {"filePrompt": "task.txt"},
    "resume": {"push": false},
    "pr": false
  }
}
```

To add project-specific review instructions, set skill names per stage. Skill
names keep their own spelling; only property names use camelCase:

```json
{
  "customSkills": {
    "codeReview": "test-code-review",
    "securityReview": "test-security-review",
    "riskClassification": "test-risk-classification",
    "reviewChanges": "test-review-changes"
  }
}
```

Each name must be one directory name, not a path. Soft Factory searches for
`.agents/skills/<name>/SKILL.md` first, then `.claude/skills/<name>/SKILL.md`,
relative to `software-factory.json`, and gives the first match to that stage
after its default skill. A configured skill for an enabled stage that is missing
or cannot be read stops the workflow before implementation with an error.

Example using both local documents and Google Drive:

```json
{
  "documents": ["requirements.md", "guidelines.md"],
  "googleDrive": {
    "folders": ["Project Documentation"],
    "files": ["https://docs.google.com/document/d/DOCUMENT_ID/edit?tab=t.0"]
  }
}
```

Google Drive requires a separate credentials JSON file named **`service_account.json`** in the project folder. This is the service-account key downloaded from Google Cloud, not `software-factory.json`. The filename and location are fixed in the current version.

The downloaded file must contain service-account credentials, including:

```json
{
  "type": "service_account",
  "client_email": "drive-reader@YOUR_PROJECT_ID.iam.gserviceaccount.com",
  "private_key": "..."
}
```



When `googleDrive` is configured, a missing or invalid credentials file stops the workflow. Without `googleDrive`, this file is not required. Keep the key private; `service_account*.json` files are ignored by Git.

Folder loading includes only Google Docs directly inside the selected folders. Individual URLs load the specified Google Docs regardless of their folder. See [Google Drive setup](google-drive.md) for download and sharing instructions.

#### Disabling stages

To skip stages without changing their order, list them in `disabledStages`.
Omitting the property or using `[]` runs every stage as before:

```json
{
  "documents": ["task.txt"],
  "disabledStages": ["securityReview", "riskClassification"]
}
```

| Stage | Default workflow | `review` | `continue` |
| --- | --- | --- | --- |
| `implementation` | Yes | No | Yes |
| `codeReview` | Yes | Yes | Yes |
| `securityReview` | Yes | Yes | Yes |
| `riskClassification` | Yes | Yes | Yes |
| `reviewChanges` | Yes | No | Yes |

Names are exact and case-sensitive. Kebab-case names such as
`security-review`, other spellings, blank or padded names, `null`, and
non-string entries are rejected before any agent runs, for example:

```text
Error: validate factory configuration: disabledStages[0]: unknown stage "securityReveiw"; allowed: implementation, codeReview, securityReview, riskClassification, reviewChanges
```

Duplicates are allowed. Listing a stage that the selected command never runs,
such as `implementation` with `review`, has no effect. Each skipped stage
prints `Skipping disabled stage: <name>` and, in the default workflow and
`continue`, adds that line to `summary.log`. Skipped stages start no agent,
create no stage log or report, and do not load their configured custom skill,
so a missing skill file does not block a disabled stage. Skill names are still
validated.

Disabling a stage never disables later stages. Risk classification uses only
the reports from reviews that ran in the current invocation and states when
review evidence is missing. Disabling `implementation` does not change branch
selection: the default workflow still uses `run.branch`, so that sandbox
worktree must already exist, and the final walkthrough still runs unless
`reviewChanges` is also disabled. Existing environment, branch, Jira, and
document checks still run when every stage is disabled; the command then
succeeds without starting an agent.

### Stage agents and models

Use `stages` in `software-factory.json` to choose an agent and model for each
stage. Each stage and each field is optional. For example:

```json
{
  "stages": {
    "implementation": {
      "agent": "codex",
      "model": "gpt-6.1-sol"
    },
    "codeReview": {
      "agent": "claude",
      "model": "claude-sonnet-4-6"
    },
    "securityReview": {
      "agent": "claude",
      "model": "claude-opus-4-6"
    },
    "riskClassification": {
      "model": "gpt-6.1-sol"
    },
    "reviewChanges": {
      "agent": "codex",
      "model": "gpt-6-luna"
    }
  }
}
```

This example assumes Codex is the default agent in `agentSandbox` for
risk classification. Authenticate both Codex and Claude before using the
mixed-agent example. Choose model IDs available to your account and supported
by the selected agent.

| Stage key | Applies to |
| --- | --- |
| `implementation` | Implementation in the default workflow and `continue`. |
| `codeReview` | Code review and corrections in every workflow. |
| `securityReview` | Security review and corrections in every workflow. |
| `riskClassification` | Final risk classification in every workflow. |
| `reviewChanges` | Final read-only walkthrough in the default workflow and `continue`. |

Use these exact camelCase keys. Overrides apply only when the stage runs;
`disabledStages` still skips a stage even when it has settings here.

Each nonempty `agent` or `model` overrides its corresponding value from
`agentSandbox` for that invocation. Omitted or empty fields retain
Agent Sandbox's configuration defaults independently: setting only `model`
keeps the configured agent, and setting only `agent` keeps the configured
model. When changing agents, ensure any inherited model is compatible with
the new agent, or set both fields.

The default workflow inherits `run` settings for implementation and `resume`
settings for later stages. `continue`, `review`, and `resume <path>` inherit `resume` settings.
Soft Factory resolves supported embedded settings into command-line arguments.
Stage agent and model overrides are independent and take priority over the
applicable section, then shared `agentSandbox` fields, then Agent Sandbox defaults.
Factory-generated prompts replace both section `query` and `filePrompt`; workflow
controls such as the risk stage's `push=false` and `pr=false` take priority too.
Explicit section false values, empty strings, and empty lists override shared
values. Lists are resolved before repeatable `--file` and `--image` flags are expanded.

### Sandbox settings transport

Factory passes supported `agentSandbox` settings as CLI flags and creates no
sandbox configuration JSON files. It keeps the project root as the subprocess
working directory, so relative task, image, and mounted-file paths resolve
from the project root. `--filePrompt` selects a task prompt, not a
configuration file; no `--config` flag is added.

Section `apiKey` and `reviewers` are temporarily ignored, even when nonempty.
They are never sent through flags, environment variables, or files, or logged.
Continue using Agent Sandbox's existing host authentication.
`agentSandbox` is required. If it is `null` or `{}`, Factory supplies no sandbox
settings and Agent Sandbox applies defaults and command requirements. A missing
local task path may still cause a contextual Factory file-read error. The
original local task always comes from `agentSandbox.run.filePrompt`, including
continuation, standalone review, and recovery, unless `--jira` supplies it.

### `agentSandbox`: defaults and task

The default workflow uses `run` settings for implementation and `resume`
settings for later stages. `continue`, `review`, and `resume <path>` use
`resume` settings.
The [stage overrides](#stage-agents-and-models) take priority over agent and
model defaults from this file.
Soft Factory selects one branch for every stage: `run.branch` for the default
workflow, or `resume.branch` for `review`, `continue`, and `resume <path>`.
It overrides the branch in each Agent Sandbox invocation. Set both branches to the same value when
you want to review or continue the task you just implemented.

Soft Factory decodes `agentSandbox` using Agent Sandbox's exported Go configuration
type and loads the unified configuration once per workflow. Factory reports
filesystem and JSON decoding/type errors and validates its own properties.
Agent Sandbox owns sandbox defaults, allowed agents/models, and command requirements.
Factory forwards supported settings through CLI flags; it does not reproduce the
Agent Sandbox JSON loader's strict unknown-field checks. Use camelCase keys such
as `filePrompt`, `baseImage`, and `commitMessage`.
Keep explicit branch names for Factory's worktree reports and execution logs.

Example for a local task:

```json
{
  "documents": [],
  "agentSandbox": {
    "run": {
      "agent": "codex",
      "branch": "factory-task",
      "filePrompt": "task.txt",
      "push": false,
      "pr": false
    },
    "resume": {
      "agent": "codex",
      "branch": "factory-task",
      "push": false,
      "pr": false
    }
  }
}
```

Choose a new `run.branch` for each new task. For `review` or `continue`, use an
existing sandbox branch in `resume.branch`; list available branches with
`agent-sandbox worktree-list`. Even for `continue`, the local task is read from
`run.filePrompt` unless you supply `--jira`.

Example for a Jira task, where `--jira` supplies the task instead of a local file:

```json
{
  "documents": [],
  "agentSandbox": {
    "run": {
      "agent": "codex",
      "branch": "eng-123",
      "push": false,
      "pr": false
    },
    "resume": {
      "agent": "codex",
      "branch": "eng-123",
      "push": false,
      "pr": false
    }
  }
}
```

Shared fields are `agent`, `model`, `effort`, `baseImage`, `branch`, `push`,
`pr`, `hn`, and `file`. A `run` or `resume` section can override each of them
and can also set `query`, `filePrompt`, `commitMessage`, and `image`. Factory
passes strings as individual arguments, booleans as `--push=false` /
`--pr=false` / `--hn=false` (or `true`), and lists as repeatable flags.
Unspecified options are omitted so Agent Sandbox retains its defaults.

Common settings in either section:

| Option | Purpose |
| --- | --- |
| `agent` | Agent to use: `codex`, `claude`, `opencode`, or `pi`. Reviews require subagent support. |
| `branch` | `run.branch` selects the default workflow's worktree; `resume.branch` selects the worktree for `review` and `continue`. All stages use that selected branch. |
| `filePrompt` | Local task file, relative to the working directory. Set it in `run` for local tasks; Soft Factory supplies the review prompts. |
| `model` | Optional model selection; otherwise use the agent's default. |
| `baseImage` | Optional container image with the tools your task needs, such as `golang:1.26-alpine` for Go tasks. |
| `apiKey`, `reviewers` | Temporarily ignored in both sections, even when configured. Values are never forwarded or logged. Use existing host authentication; API-key authentication and PR reviewer assignment are not supported by this transport. |
| `push` | Commit and push changes after a sandbox stage. The examples leave this disabled. |
| `pr` | Commit, push, and create or reuse a GitHub pull request after a sandbox stage. The examples leave this disabled. |
| `commitMessage` | Optional commit message when `push` or `pr` is enabled. |
| `hn` | Allow the container to access host services; defaults to `false`. |
| `file` | Read-only file mounts; a section list replaces the shared list, including an empty list. |
| `effort` | Optional effort setting, validated by Agent Sandbox. |
| `image` | Optional list of image paths to attach for agents that support them. |
| `query` | Inline prompt supported by `agent-sandbox`. For Soft Factory, use `run.filePrompt` or `--jira` to keep the original task available to every stage. |

Soft Factory supplies generated prompts for reviews and for tasks with supporting context or Jira input. Do not set a separate review prompt in `resume`.
