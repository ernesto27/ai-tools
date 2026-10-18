# Configuration

Create `software-factory.json` in the project root before running Soft Factory.

**Breaking change:** `software-factory.json` is now the only maintained workflow
configuration. To migrate manually, move the entire old `agent-sandbox.json`
object into the top-level `agentSandbox` property, keeping `documents`,
`customSkills`, `disabledStages`, and `stages` unchanged. The old file is never
read as input: there is no fallback, merge, or automatic migration.

Factory loads this configuration once per workflow. It writes the original
embedded JSON to a temporary `./agent-sandbox.json` with mode `0600` in the
project directory, runs the existing Agent Sandbox binary there, then removes
the generated file. Any pre-existing regular file is backed up without parsing
and restored with its original contents and permissions, even after failures
or normal cancellation. Symlinks and non-regular paths are refused.

Keep `software-factory.json`, `agent-sandbox.json`, and
`.soft-factory-sandbox-runtime/` in your project's `.gitignore`; configuration
can contain secrets. The private runtime directory holds the workflow lock,
backup, and recovery information. A competing or interrupted run stops with
recovery instructions. After forced termination, first stop Factory and all
its sandbox children, inspect the runtime directory's `owner` file and the
generated path, preserve `original.json`, and manually restore that backup
before removing the runtime directory. If there was no original file, remove
the generated file after confirming ownership. Never remove recovery artifacts
while a workflow is active. Do not run a separate manual `agent-sandbox`
workflow in the same directory while Factory owns its runtime configuration.

### `software-factory.json`: workflow and sandbox settings

The CLI always reads `software-factory.json` from the current working directory.
Rename an existing `config.json` to `software-factory.json`; there is no fallback
to the old filename, and `--config` / `-c` are no longer supported.

All property names use lower camelCase, including nested properties. These are all the options currently supported:

| Option | Purpose |
| --- | --- |
| `agentSandbox` | Embedded Agent Sandbox configuration, using its shared fields and `run`/`resume` sections. Agent Sandbox owns validation and defaults. |
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

Minimal local-task example:

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
Soft Factory passes overrides as command-line arguments. Branch, task,
authentication, and other sandbox settings come from the same loaded
`agentSandbox` snapshot. The generated runtime file transports all fields,
including JSON-only fields; `-f` / `--filePrompt` still selects a task prompt.

### `agentSandbox`: defaults and task

The default workflow uses `run` settings for implementation and `resume`
settings for later stages. `continue`, `review`, and `resume <path>` use
`resume` settings.
The [stage overrides](#stage-agents-and-models) take priority over agent and
model defaults from this embedded object.
Soft Factory selects one branch for every stage: `run.branch` for the default
workflow, or `resume.branch` for `review`, `continue`, and `resume <path>`.
It overrides the branch in each Agent Sandbox invocation. Set both branches to the same value when
you want to review or continue the task you just implemented.

Soft Factory decodes `agentSandbox` using Agent Sandbox's exported Go configuration
type. Factory reports JSON/type errors and filesystem failures; Agent Sandbox
validates sandbox settings and command requirements when each stage starts.
Omitting `agentSandbox` or setting it to `null` delivers `{}` to Agent Sandbox.
Factory adds no required-sandbox check, though reading a local task without a
usable `agentSandbox.run.filePrompt` can report a contextual filesystem error.
Explicit Factory flags (including stage agent/model overrides and workflow
controls) take priority, followed by the applicable section, then shared fields.
Agent Sandbox supplies defaults. Explicit `false` and empty section values
retain their upstream meaning. Relative task and sandbox paths resolve from
the project root; the temporary transport does not rebase paths.
Use camelCase keys such as `filePrompt`,
`baseImage`, `apiKey`, and `commitMessage`; old hyphenated keys are unsupported.
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

Common settings in either section:

| Option | Purpose |
| --- | --- |
| `agent` | Agent to use: `codex`, `claude`, `opencode`, or `pi`. Reviews require subagent support. |
| `branch` | `run.branch` selects the default workflow's worktree; `resume.branch` selects the worktree for `review` and `continue`. All stages use that selected branch. |
| `filePrompt` | Local task file, relative to the working directory. Set it in `run` for local tasks; Soft Factory supplies the review prompts. |
| `model` | Optional model selection; otherwise use the agent's default. |
| `baseImage` | Optional container image with the tools your task needs, such as `golang:1.26-alpine` for Go tasks. |
| `apiKey` | Optional API key for Codex or Claude. Otherwise, use the agent's existing host authentication. |
| `push` | Commit and push changes after a sandbox stage. The examples leave this disabled. |
| `pr` | Commit, push, and create or reuse a GitHub pull request after a sandbox stage. The examples leave this disabled. |
| `commitMessage` | Optional commit message when `push` or `pr` is enabled. |
| `hn` | Allow the container to access host services; defaults to `false`. |
| `image` | Optional list of image paths to attach for agents that support them. |
| `query` | Inline prompt supported by `agent-sandbox`. For Soft Factory, use `run.filePrompt` or `--jira` to keep the original task available to every stage. |

Soft Factory supplies generated prompts for reviews and for tasks with supporting context or Jira input. The original local task always comes from `agentSandbox.run.filePrompt`, including `continue`, `review`, and recovery `resume`. Jira overrides the task while supporting documents still apply. Do not set a separate review prompt in `resume`.
