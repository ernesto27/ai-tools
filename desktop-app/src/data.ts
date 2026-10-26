// Hardcoded mock data for the first UI version.
// Shapes mirror what the backend (Tauri commands) will return later.

export type RunStatus = "succeeded" | "failed" | "interrupted" | "running";
export type BranchStatus = RunStatus | "idle";

/** One char per stage: s=done, f=failed, i=interrupted, r=running, -=not run */
export type Pips = string;

export interface Run {
  id: string;
  kind: "run" | "continue" | "resume";
  when: string;
  dur: string;
  status: RunStatus;
  risk: string;
  pips: Pips;
  note: string;
  resumable?: boolean;
}

export interface Branch {
  name: string;
  status: BranchStatus;
  task: string;
  ahead: number;
  add: string;
  del: string;
  pr: string;
  runs: Run[];
}

export interface Repo {
  name: string;
  status: BranchStatus;
  branches: Branch[];
}

export const STATUS_COLOR: Record<BranchStatus, string> = {
  succeeded: "var(--color-ok)",
  failed: "var(--color-bad)",
  interrupted: "var(--color-warn)",
  running: "var(--color-info)",
  idle: "var(--color-faint)",
};

export const STAGE_NAMES = ["implement", "code review", "security", "risk", "walkthrough"];

export const repos: Repo[] = [
  {
    name: "soft-factory",
    status: "running",
    branches: [
      {
        name: "factory-task",
        status: "running",
        task: "task.txt",
        ahead: 6,
        add: "+188",
        del: "−41",
        pr: "no PR",
        runs: [
          { id: "ft-3", kind: "run", when: "today 11:02", dur: "running", status: "running", risk: "—", pips: "sr---", note: "Code review, round 2 of 3" },
          { id: "ft-2", kind: "run", when: "today 09:47", dur: "6m 31s", status: "failed", risk: "—", pips: "ssf--", note: "Security review verification timed out", resumable: true },
          { id: "ft-1", kind: "continue", when: "Oct 22 15:20", dur: "14m 08s", status: "succeeded", risk: "LOW", pips: "sssss", note: "Walkthrough written" },
        ],
      },
      {
        name: "feature-logs",
        status: "interrupted",
        task: "task.txt",
        ahead: 11,
        add: "+412",
        del: "−87",
        pr: "no PR",
        runs: [
          { id: "fl-4", kind: "run", when: "Oct 22 18:02", dur: "22m 47s", status: "succeeded", risk: "MEDIUM", pips: "sssss", note: "1 security finding saved, not applied" },
          { id: "fl-3", kind: "resume", when: "Oct 22 16:31", dur: "3m 02s", status: "interrupted", risk: "—", pips: "ssi--", note: "Stopped during security review", resumable: true },
          { id: "fl-2", kind: "run", when: "Oct 22 16:10", dur: "9m 55s", status: "interrupted", risk: "—", pips: "si---", note: "Interrupted in code review" },
          { id: "fl-1", kind: "continue", when: "Oct 21 19:40", dur: "12m 18s", status: "failed", risk: "—", pips: "f----", note: "go build error in implementation" },
        ],
      },
      {
        name: "jira-SF-142",
        status: "succeeded",
        task: "Jira SF-142",
        ahead: 4,
        add: "+96",
        del: "−12",
        pr: "PR #48 open",
        runs: [
          { id: "sf-1", kind: "run", when: "Oct 21 10:15", dur: "31m 55s", status: "succeeded", risk: "HIGH", pips: "sssss", note: "Touches auth token handling" },
        ],
      },
    ],
  },
  {
    name: "agent-sandbox",
    status: "interrupted",
    branches: [
      {
        name: "api-key-flags",
        status: "interrupted",
        task: "task.txt",
        ahead: 2,
        add: "+54",
        del: "−9",
        pr: "no PR",
        runs: [
          { id: "as-1", kind: "run", when: "Oct 20 14:12", dur: "4m 40s", status: "interrupted", risk: "—", pips: "si---", note: "Interrupted in code review", resumable: true },
        ],
      },
    ],
  },
  { name: "billing-api", status: "idle", branches: [] },
];

// ---- Run detail ----

export type Tone = "ink" | "grey" | "warn" | "ok";

export interface StageLog {
  name: string;
  dur: string;
  verdict: string;
  tone: Tone;
  file: string;
  lines: [string, Tone][];
}

export const runDetail = {
  title: "Add structured execution logs",
  meta: "run-2026-10-22_21-02-11 · Oct 22 18:02 · 22m 47s · task.txt",
  risk: "Medium",
  codeReview: "Pass",
  securityReview: "Unresolved",
  add: "+412",
  del: "−87",
  files: 9,
  stages: [
    {
      name: "Implement", dur: "6m 12s", verdict: "succeeded", tone: "ok", file: "01-implementation.log",
      lines: [["agent codex · model default", "grey"], ["branch feature-logs · worktree ready", "grey"], ["documents docs/logging.md, docs/arch.md", "grey"], ["create internal/logs/writer.go", "ink"], ["edit internal/runner/stage.go", "ink"], ["go build ./... ok", "ok"], ["go test ./... ok", "ok"], ["SUCCEEDED", "ok"]],
    },
    {
      name: "Code review", dur: "7m 40s", verdict: "pass · 2 rounds", tone: "ok", file: "02-codeReview.log",
      lines: [["round 1 of 3", "grey"], ["bug   writer.go:88  suffix not incremented on collision", "warn"], ["maint stage.go:41   duplicate status formatting", "warn"], ["maint resume.go:23  error message lacks path", "warn"], ["applying 3 corrections", "ink"], ["go test ./... ok", "ok"], ["round 2 of 3", "grey"], ["No actionable findings.", "ok"], ["PASS", "ok"]],
    },
    {
      name: "Security review", dur: "3m 05s", verdict: "unresolved", tone: "warn", file: "03-securityReview.log",
      lines: [["saveFindings=true · no changes applied", "grey"], ["scanning 9 changed files", "ink"], ["P2 internal/logs/writer.go:52", "warn"], ["   os.MkdirAll(dir, 0755)", "ink"], ["   logs may contain prompts; readable by other users", "ink"], ["   fix: use 0700", "ink"], ["UNRESOLVED", "warn"]],
    },
    {
      name: "Risk", dur: "1m 22s", verdict: "medium", tone: "warn", file: "04-riskClassification.log",
      lines: [["evidence: code review PASS, 1 unapplied finding", "grey"], ["scope: config loading, log writers, resume", "ink"], ["no schema or network changes", "ink"], ["MEDIUM", "warn"]],
    },
    {
      name: "Walkthrough", dur: "4m 28s", verdict: "succeeded", tone: "ok", file: "05-reviewChanges.log",
      lines: [["diff: 9 files, +412 −87", "grey"], ["writing 05-reviewChanges.md", "ink"], ["SUCCEEDED", "ok"]],
    },
  ] as StageLog[],
  summary: [
    { name: "Implement", agent: "codex", dur: "6m 12s", note: "Created log writer" },
    { name: "Code review", agent: "claude", dur: "7m 40s", note: "Pass after 2 rounds" },
    { name: "Security review", agent: "claude", dur: "3m 05s", note: "1 finding saved" },
    { name: "Risk", agent: "codex", dur: "1m 22s", note: "Medium" },
    { name: "Walkthrough", agent: "codex", dur: "4m 28s", note: "05-reviewChanges.md" },
  ],
};

// ---- New run ----

export const resumableRuns = [
  { branch: "factory-task", when: "today 09:47", folder: "logs/factory-task/run-2026-10-23_12-47-36.102884550", last: "securityReview,FAILED", next: "retry security review", status: "failed" as RunStatus },
  { branch: "feature-logs", when: "Oct 22 16:31", folder: "logs/feature-logs/run-2026-10-22_19-10-45.004217981", last: "codeReview,SUCCEEDED", next: "start security review", status: "interrupted" as RunStatus },
];

export const folderFiles = [
  { name: "task.md", role: "task" },
  { name: "design-notes.md", role: "document" },
  { name: "api-contract.yaml", role: "document" },
];

export const jsonSettings = [
  { k: "agent", v: "codex" },
  { k: "model", v: "default" },
  { k: "push", v: "false" },
  { k: "pr", v: "false" },
  { k: "stages", v: "5 of 5" },
  { k: "saveFindings", v: "false" },
  { k: "codeReview.agent", v: "claude" },
  { k: "documents", v: "2 + Drive" },
];

export const DEFAULT_TASK_TEXT =
  "Add structured execution logs.\n\nEach invocation should write to logs/<branch>/run-<timestamp>/ with one numbered log per stage and a summary.log entry per attempt.";

// ---- Config ----

export const AGENTS = ["codex", "claude", "opencode", "pi"];

export const CONFIG_STAGES = [
  { key: "implementation", label: "Implement", agent: "", skill: "" },
  { key: "codeReview", label: "Code review", agent: "claude", skill: "" },
  { key: "securityReview", label: "Security review", agent: "claude", skill: "" },
  { key: "riskClassification", label: "Risk", agent: "", skill: "" },
  { key: "reviewChanges", label: "Walkthrough", agent: "", skill: "" },
];

// ---- Add repository ----

export const knownFolders = [
  { name: "payments-ui", path: "~/code/work/payments-ui", branch: "main", hasConfig: false },
  { name: "docs-site", path: "~/code/work/docs-site", branch: "main", hasConfig: true },
  { name: "infra", path: "~/code/work/infra", branch: "develop", hasConfig: false },
  { name: "cli-helpers", path: "~/code/ai-enginering/cli-helpers", branch: "main", hasConfig: true },
];
