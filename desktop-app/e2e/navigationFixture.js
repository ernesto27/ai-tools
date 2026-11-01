import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdirSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";
import { basename, join } from "node:path";

export function prepareNavigationFixture(fixture, command) {
  const repoName = "navigation-repository";
  const branch = "navigation-check";
  const runId = "run-2026-11-01_12-00-00";
  const logText = "Prepared navigation fixture: implementation failed.";
  const repoDirectory = join(fixture, repoName);
  mkdirSync(repoDirectory);
  const repo = realpathSync(repoDirectory);
  command("git", ["init", "--initial-branch=main", repo]);
  writeFileSync(join(repo, "README.md"), "# Desktop navigation fixture\n");
  writeFileSync(join(repo, "software-factory.json"), JSON.stringify({
    documents: [],
    agentSandbox: {
      agent: "codex",
      branch,
      run: { query: "Desktop navigation fixture task" },
      resume: { branch },
    },
  }, null, 2) + "\n");
  command("git", ["-C", repo, "add", "README.md", "software-factory.json"]);
  command("git", [
    "-C", repo,
    "-c", "user.name=Desktop UI Test",
    "-c", "user.email=desktop-ui-test@example.invalid",
    "-c", "commit.gpgsign=false",
    "-c", "core.hooksPath=/dev/null",
    "commit", "-m", "Initial desktop navigation fixture",
  ]);
  const worktreeDirectory = join(fixture, "navigation-worktree");
  command("git", ["-C", repo, "worktree", "add", "-b", branch, worktreeDirectory]);
  const worktree = realpathSync(worktreeDirectory);

  const dataHome = join(fixture, "data");
  const configHome = join(fixture, "config");
  const appData = join(dataHome, "com.ernesto.desktop-app");
  const sandboxData = join(configHome, "agent-sandbox");
  mkdirSync(appData, { recursive: true });
  mkdirSync(sandboxData, { recursive: true });
  writeFileSync(join(appData, "repos.json"), JSON.stringify([
    { name: repoName, path: repo },
  ], null, 2) + "\n");
  writeFileSync(join(sandboxData, "worktrees.jsonl"), JSON.stringify({
    repo,
    path: worktree,
    branch,
    base_branch: "main",
    created: "2026-11-01T12:00:00Z",
  }) + "\n");
  const environment = {
    ...process.env,
    XDG_DATA_HOME: dataHome,
    XDG_CONFIG_HOME: configHome,
    GDK_BACKEND: "x11",
  };
  const entries = JSON.parse(command(
    "agent-sandbox", ["worktree-list", "--json", "--repo", repo],
    { env: environment },
  ));
  assert.deepEqual(entries, [{ branch, path: worktree }],
    "The real agent-sandbox must return the prepared worktree.");

  const fingerprint = createHash("sha256").update(repo).digest("hex").slice(0, 12);
  const logsParent = join(homedir(), ".config", "software-factory", "logs");
  const projectLogs = join(logsParent, `${basename(repo)}-${fingerprint}`);
  mkdirSync(logsParent, { recursive: true });
  let ownsProjectLogs = false;
  try {
    // An existing directory must never become owned by this fixture.
    mkdirSync(projectLogs);
    ownsProjectLogs = true;
    const runDirectory = join(projectLogs, branch, runId);
    mkdirSync(runDirectory, { recursive: true });
    writeFileSync(join(runDirectory, "stage_status.txt"), "implementation,FAILED\n");
    writeFileSync(join(runDirectory, "01-implementation.log"), `${logText}\n`);
    return { repoName, repo, worktree, branch, runId, logText, environment, projectLogs };
  } catch (error) {
    if (ownsProjectLogs) rmSync(projectLogs, { recursive: true, force: true });
    throw error;
  }
}
