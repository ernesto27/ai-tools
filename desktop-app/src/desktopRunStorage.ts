export const DESKTOP_RUNS_KEY = "soft-factory.desktop-runs";

export interface SavedDesktopRun {
  id: string;
  repo: string;
  repoPath: string;
  branch: string;
  runId: string | null;
  directory: string | null;
  state: "running" | "stopping";
  mode: "run" | "continue" | "resume";
  previousDirectories: string[];
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isSavedRun(value: unknown): value is SavedDesktopRun {
  if (!isObject(value)) return false;
  return typeof value.id === "string"
    && /^desktop:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value.id)
    && typeof value.repo === "string" && value.repo.length > 0
    && typeof value.repoPath === "string" && value.repoPath.length > 0
    && typeof value.branch === "string" && value.branch.length > 0
    && (value.runId === null || (typeof value.runId === "string" && value.runId.length > 0))
    && (value.directory === null || (typeof value.directory === "string" && value.directory.length > 0))
    && ((value.runId === null) === (value.directory === null))
    && (value.state === "running" || value.state === "stopping")
    && (value.mode === "run" || value.mode === "continue" || value.mode === "resume")
    && (value.mode !== "resume" || value.directory !== null)
    && Array.isArray(value.previousDirectories)
    && value.previousDirectories.every((path) => typeof path === "string");
}

export function readDesktopRuns(): { runs: SavedDesktopRun[]; warning: string | null } {
  try {
    const text = window.localStorage.getItem(DESKTOP_RUNS_KEY);
    if (text === null) return { runs: [], warning: null };
    const value: unknown = JSON.parse(text);
    if (!isObject(value) || value.version !== 1 || !Array.isArray(value.runs)) {
      return { runs: [], warning: "Invalid or unsupported run recovery data was ignored." };
    }
    const valid = value.runs.filter(isSavedRun);
    const counts = new Map<string, number>();
    for (const run of valid) counts.set(run.id, (counts.get(run.id) ?? 0) + 1);
    const runs = valid.filter((run) => counts.get(run.id) === 1);
    return {
      runs,
      warning: runs.length === value.runs.length ? null : "Invalid or unsupported run recovery data was ignored.",
    };
  } catch {
    return { runs: [], warning: "Could not read run recovery data." };
  }
}

export function writeDesktopRuns(runs: SavedDesktopRun[]): string | null {
  try {
    if (runs.length) window.localStorage.setItem(DESKTOP_RUNS_KEY, JSON.stringify({ version: 1, runs }));
    else window.localStorage.removeItem(DESKTOP_RUNS_KEY);
    return null;
  } catch {
    return "Could not persist run recovery data.";
  }
}

export function isRunTrackingUnavailable(error: unknown): boolean {
  const message = error instanceof Error ? error.message : error;
  return message === "This run is not tracked." || message === "This run has already finished.";
}
