import { useEffect, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import type { FactoryConfig } from "../config";
import type { RunEntry } from "./useBranchRuns";
import type { FactoryRunMode } from "../router";

export const RUN_STATE = {
  STARTING: "starting",
  RUNNING: "running",
  STOPPING: "stopping",
  SUCCEEDED: "succeeded",
  FAILED: "failed",
  INTERRUPTED: "interrupted",
} as const;

export type RunState = (typeof RUN_STATE)[keyof typeof RUN_STATE];
type RunOutcome = typeof RUN_STATE.SUCCEEDED | typeof RUN_STATE.INTERRUPTED;

export interface DesktopRun {
  id: string;
  repo: string;
  repoPath: string;
  branch: string;
  runId: string | null;
  directory: string | null;
  state: RunState;
  error: string | null;
  stopError: string | null;
}

export interface StartDesktopRun {
  mode: FactoryRunMode;
  repo: string;
  repoPath: string;
  branch: string;
  config: FactoryConfig;
  createConfig: boolean;
  jiraUrl: string | null;
}

export interface ResumeDesktopRun {
  repo: string;
  repoPath: string;
  branch: string;
  config: FactoryConfig;
  run: RunEntry;
}

export function isDesktopRunActive(run: DesktopRun): boolean {
  return run.state === RUN_STATE.STARTING
    || run.state === RUN_STATE.RUNNING
    || run.state === RUN_STATE.STOPPING;
}

export function useDesktopRuns() {
  const sessions = useRef<Record<string, DesktopRun>>({});
  const [runs, setRuns] = useState<Record<string, DesktopRun>>({});
  const [supportsStop, setSupportsStop] = useState(false);

  useEffect(() => {
    let active = true;

    invoke<boolean>("supports_factory_stop")
      .then((supported) => {
        if (active) setSupportsStop(supported);
      })
      .catch(() => {
        if (active) setSupportsStop(false);
      });

    return () => {
      active = false;
    };
  }, []);

  function update(id: string, patch: Partial<DesktopRun>) {
    sessions.current = {
      ...sessions.current,
      [id]: { ...sessions.current[id], ...patch },
    };
    setRuns(sessions.current);
  }

  function busy(repoPath: string) {
    return Object.values(sessions.current).find((run) =>
      run.repoPath === repoPath && isDesktopRunActive(run));
  }

  function start(input: StartDesktopRun): string {
    if (busy(input.repoPath)) throw new Error("This repository already has an active run.");
    const id = `desktop:${crypto.randomUUID()}`;
    update(id, {
      id, repo: input.repo, repoPath: input.repoPath, branch: input.branch,
      runId: null, directory: null, state: RUN_STATE.STARTING, error: null,
      stopError: null,
    });

    void (async () => {
      let stopped = false;
      let timer: ReturnType<typeof setTimeout> | undefined;
      let discovery = Promise.resolve();
      let knownDirectories: Set<string> | undefined;
      function discoverDirectory() {
        discovery = discovery.then(async () => {
          const previousDirectories = knownDirectories;
          if (!previousDirectories || sessions.current[id].directory) return;
          try {
            const entries = await invoke<RunEntry[]>("list_branch_runs", {
              path: input.repoPath, branch: input.branch,
            });
            const candidates = entries.filter((entry) =>
              entry.name.startsWith(`${input.mode}-`) && !previousDirectories.has(entry.directory));
            if (candidates.length > 1) {
              throw new Error("Multiple new run directories found. Open the run from the branch list.");
            }
            if (candidates.length === 1) {
              const entry = candidates[0];
              update(id, {
                directory: entry.directory, runId: entry.name,
                error: sessions.current[id].state === RUN_STATE.FAILED ? sessions.current[id].error : null,
              });
            }
          } catch (error) {
            if (isDesktopRunActive(sessions.current[id])) {
              update(id, { error: `Could not locate run logs: ${String(error)}` });
            }
          }
        });
        return discovery;
      }
      async function pollDirectory() {
        await discoverDirectory();
        if (!stopped && !sessions.current[id].directory) {
          timer = setTimeout(() => void pollDirectory(), 1000);
        }
      }
      try {
        await invoke(input.createConfig ? "create_factory_config" : "update_factory_config", {
          path: input.repoPath, config: input.config,
        });
        const previous = await invoke<RunEntry[]>("list_branch_runs", {
          path: input.repoPath, branch: input.branch,
        });
        knownDirectories = new Set(previous.map((entry) => entry.directory));
        update(id, { state: RUN_STATE.RUNNING });
        const completion = invoke<RunOutcome>(
          "start_tracked_factory_run",
          {
            runId: id,
            path: input.repoPath,
            jiraUrl: input.jiraUrl,
            mode: input.mode,
          },
        );
        void pollDirectory();
        const outcome = await completion;
        update(id, { state: outcome, stopError: null });
      } catch (error) {
        update(id, { state: RUN_STATE.FAILED, error: String(error), stopError: null });
      } finally {
        stopped = true;
        if (timer !== undefined) clearTimeout(timer);
        // A short run can finish before the first directory poll completes.
        await discoverDirectory();
      }
    })();
    return id;
  }

  function resume(input: ResumeDesktopRun): string {
    if (busy(input.repoPath)) throw new Error("This repository already has an active run.");
    const id = `desktop:${crypto.randomUUID()}`;
    update(id, {
      id, repo: input.repo, repoPath: input.repoPath, branch: input.branch,
      runId: input.run.name, directory: input.run.directory, state: RUN_STATE.STARTING, error: null,
      stopError: null,
    });

    void (async () => {
      try {
        await invoke("update_factory_config", {
          path: input.repoPath, config: input.config,
        });
        update(id, { state: RUN_STATE.RUNNING });
        const outcome = await invoke<RunOutcome>("resume_factory_run", {
          path: input.repoPath, branch: input.branch, runId: input.run.name,
          executionId: id,
        });
        update(id, { state: outcome, stopError: null });
      } catch (error) {
        update(id, { state: RUN_STATE.FAILED, error: String(error), stopError: null });
      }
    })();
    return id;
  }

  async function stop(id: string): Promise<void> {
    const run = sessions.current[id];

    if (
      !supportsStop
      || !run
      || run.state !== RUN_STATE.RUNNING
    ) {
      return;
    }

    update(id, { state: RUN_STATE.STOPPING, stopError: null });

    try {
      await invoke<void>("stop_factory_run", { runId: id });
    } catch (error) {
      // Execution may have finished while the Stop request was pending.
      if (sessions.current[id]?.state === RUN_STATE.STOPPING) {
        update(id, {
          state: RUN_STATE.RUNNING,
          stopError: `Could not stop run: ${String(error)}`,
        });
      }
    }
  }

  return {
    runs,
    supportsStop,
    start,
    resume,
    stop,
    busy,
    find: (repoPath: string, branch: string, runId: string) => Object.values(runs).reverse().find((run) =>
      run.repoPath === repoPath && (run.id === runId || (run.runId === runId
        && run.directory?.split(/[\\/]/).slice(-2)[0] === branch.replace(/[^a-zA-Z0-9._-]/g, "-")))),
  };
}

export type DesktopRuns = ReturnType<typeof useDesktopRuns>;
