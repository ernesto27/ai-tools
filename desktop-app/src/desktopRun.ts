import type { UiMessage } from "./messages";
import { RUN_STATE } from "./runState";

export type RunState = (typeof RUN_STATE)[keyof typeof RUN_STATE];

export interface DesktopRun {
  id: string;
  repo: string;
  repoPath: string;
  branch: string;
  runId: string | null;
  directory: string | null;
  state: RunState;
  error: UiMessage | null;
  stopError: UiMessage | null;
  mode: "run" | "continue" | "resume";
  previousDirectories: string[];
  tracking: "owned" | "restored" | "unavailable";
  previousStopRequested: boolean;
}

export function isDesktopRunActive(run: DesktopRun): boolean {
  return run.tracking !== "unavailable" && (run.state === RUN_STATE.STARTING
    || run.state === RUN_STATE.RUNNING
    || run.state === RUN_STATE.STOPPING);
}
