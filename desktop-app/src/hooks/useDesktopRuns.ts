import { AppMessageError, toUiMessage, type UiMessage } from "../messages";
import type { BranchWorktree } from "../worktrees";
import { useEffect, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import type { FactoryConfig } from "../config";
import type { RunEntry } from "./useBranchRuns";
import type { FactoryRunMode } from "../router";
import { notifyRunCompletion } from "../notifications";
import { RUN_STATE } from "../runState";
import { readDesktopRuns, writeDesktopRuns, isRunTrackingUnavailable, type SavedDesktopRun } from "../desktopRunStorage";
import { useRunDiscovery } from "./useRunDiscovery";
import { isDesktopRunActive, type DesktopRun } from "../desktopRun";

export { isDesktopRunActive, type DesktopRun, type RunState } from "../desktopRun";

export { RUN_STATE } from "../runState";

type RunOutcome = typeof RUN_STATE.SUCCEEDED | typeof RUN_STATE.INTERRUPTED;

export interface StartDesktopRun {
  mode: FactoryRunMode;
  repo: string;
  repoPath: string;
  branch: string;
  config: FactoryConfig;
  createConfig: boolean;
  profileId: string | null;
  jiraUrl: string | null;
  imagePaths: string[];
}

export interface ResumeDesktopRun {
  repo: string;
  repoPath: string;
  branch: string;
  config: FactoryConfig;
  run: RunEntry;
  profileId: string | null;
}

export function useDesktopRuns(
  onBranchCreated: (path: string, worktree: BranchWorktree) => void,
  isDeletingWorktree: (path: string, branch: string) => boolean,
) {
  const [initial] = useState(() => {
    const saved = readDesktopRuns();
    const runs: Record<string, DesktopRun> = Object.fromEntries(saved.runs.map((run) => [run.id, {
      ...run, state: RUN_STATE.RUNNING, error: null, stopError: null,
      tracking: "restored", previousStopRequested: run.state === RUN_STATE.STOPPING,
    }]));
    return { runs, warning: saved.warning };
  });
  const sessions = useRef(initial.runs);
  const [runs, setRuns] = useState(initial.runs);
  const [supportsStop, setSupportsStop] = useState(false);
  const lifecycle = useRef({ active: true, leaving: false });

  useEffect(() => {
    if (initial.warning) console.warn("Run recovery storage:", initial.warning);
    lifecycle.current.active = true;
    function leave() {
      if (lifecycle.current.leaving) return;
      lifecycle.current.leaving = true;
      persist(sessions.current);
    }
    window.addEventListener("beforeunload", leave);
    window.addEventListener("pagehide", leave);
    return () => {
      lifecycle.current.active = false;
      window.removeEventListener("beforeunload", leave);
      window.removeEventListener("pagehide", leave);
    };
  }, []);

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

  function persist(next: Record<string, DesktopRun>): string | null {
    const saved: SavedDesktopRun[] = [];
    for (const run of Object.values(next)) {
      if (run.tracking === "unavailable" || (run.state !== RUN_STATE.RUNNING && run.state !== RUN_STATE.STOPPING)) continue;
      saved.push({
        id: run.id, repo: run.repo, repoPath: run.repoPath, branch: run.branch,
        runId: run.runId, directory: run.directory, state: run.previousStopRequested ? "stopping" : run.state, mode: run.mode,
        previousDirectories: run.previousDirectories,
      });
    }
    return writeDesktopRuns(saved);
  }

  function publish(next: Record<string, DesktopRun>) {
    if (!lifecycle.current.active || lifecycle.current.leaving) return;
    sessions.current = next;
    setRuns(next);
    const warning = persist(next);
    if (warning) console.warn("Run recovery storage:", warning);
  }

  function add(run: DesktopRun) {
    publish({ ...sessions.current, [run.id]: run });
  }

  function update(id: string, patch: Partial<DesktopRun>) {
    if (!lifecycle.current.active || lifecycle.current.leaving) return;
    const previous = sessions.current[id];
    if (!previous || previous.tracking === "unavailable") return;
    const next = { ...previous, ...patch };
    publish({ ...sessions.current, [id]: next });

    if (
      previous.tracking === "owned"
      && isDesktopRunActive(previous)
      && (next.state === RUN_STATE.SUCCEEDED || next.state === RUN_STATE.FAILED)
    ) {
      void notifyRunCompletion({
        repo: next.repo,
        branch: next.branch,
        state: next.state,
      });
    }
  }

  const discovery = useRunDiscovery(sessions, update, onBranchCreated, lifecycle);

  function executionFailed(id: string, error: unknown) {
    const message = error instanceof Error ? error.message : error;
    if (message === "This run is already tracked.") {
      // Tauri can retry a pending IPC request after a transport failure. The
      // original execution still owns this ID; its completion callback is lost.
      update(id, { state: RUN_STATE.RUNNING, tracking: "restored", error: null, stopError: null });
    } else {
      update(id, { state: RUN_STATE.FAILED, error: toUiMessage(error), stopError: null });
    }
  }

  // Native profile save or in-use failures happen before the CLI starts, so
  // a new run must not keep any run directory seen meanwhile.
  function profileLaunchFailed(id: string, error: unknown, patch: Partial<DesktopRun> = {}): boolean {
    if (typeof error !== "object" || error === null || !("profileError" in error)) return false;
    update(id, { ...patch, state: RUN_STATE.FAILED, error: String(error.profileError), stopError: null });
    return true;
  }

  function forget(id: string, stopError: UiMessage | null = null) {
    const run = sessions.current[id];
    if (!run || run.tracking !== "restored") return;
    publish({ ...sessions.current, [id]: {
      ...run, tracking: "unavailable", error: null, stopError,
    } });
  }

  function checkWorktreeDeletion(repoPath: string, branch: string) {
    if (isDeletingWorktree(repoPath, branch)) {
      throw new AppMessageError({ key: "worktreeDeletionInProgress" });
    }
  }

  function start(input: StartDesktopRun): string {
    checkWorktreeDeletion(input.repoPath, input.branch);
    const id = `desktop:${crypto.randomUUID()}`;
    add({
      id, repo: input.repo, repoPath: input.repoPath, branch: input.branch,
      runId: null, directory: null, state: RUN_STATE.STARTING, error: null,
      stopError: null, mode: input.mode, previousDirectories: [],
      tracking: "owned", previousStopRequested: false,
    });

    void (async () => {
      let dispatched = false;
      try {
        if (input.profileId === null) {
          await invoke(input.createConfig ? "create_factory_config" : "update_factory_config", {
            path: input.repoPath, config: input.config,
          });
        }
        const previous = await invoke<RunEntry[]>("list_branch_runs", {
          path: input.repoPath, branch: input.branch,
        });
        if (!lifecycle.current.active || lifecycle.current.leaving) return;
        update(id, { state: RUN_STATE.RUNNING, previousDirectories: previous.map((entry) => entry.directory) });
        dispatched = true;
        const completion = invoke<RunOutcome>(
          "start_tracked_factory_run",
          {
            runId: id,
            path: input.repoPath,
            jiraUrl: input.jiraUrl,
            mode: input.mode,
            imagePaths: input.imagePaths,
            profile: input.profileId === null ? null : { id: input.profileId, config: input.config },
          },
        );
        const outcome = await completion;
        update(id, { state: outcome, stopError: null });
      } catch (error) {
        if (profileLaunchFailed(id, error, { runId: null, directory: null })) dispatched = false;
        else executionFailed(id, error);
      } finally {
        // A short run can finish before the shared polling loop first sees it.
        if (dispatched) await discovery.finish(id);
      }
    })();
    return id;
  }

  function resume(input: ResumeDesktopRun): string {
    checkWorktreeDeletion(input.repoPath, input.branch);
    const id = `desktop:${crypto.randomUUID()}`;
    add({
      id, repo: input.repo, repoPath: input.repoPath, branch: input.branch,
      runId: input.run.name, directory: input.run.directory, state: RUN_STATE.STARTING, error: null,
      stopError: null, mode: "resume", previousDirectories: [],
      tracking: "owned", previousStopRequested: false,
    });

    void (async () => {
      let dispatched = false;
      try {
        if (input.profileId === null) {
          await invoke("update_factory_config", {
            path: input.repoPath, config: input.config,
          });
        }
        if (!lifecycle.current.active || lifecycle.current.leaving) return;
        update(id, { state: RUN_STATE.RUNNING });
        dispatched = true;
        const outcome = await invoke<RunOutcome>("resume_factory_run", {
          path: input.repoPath, branch: input.branch, runId: input.run.name,
          executionId: id,
          profile: input.profileId === null ? null : { id: input.profileId, config: input.config },
        });
        update(id, { state: outcome, stopError: null });
      } catch (error) {
        if (profileLaunchFailed(id, error)) dispatched = false;
        else executionFailed(id, error);
      } finally {
        if (dispatched) await discovery.finish(id);
      }
    })();
    return id;
  }

  async function stop(id: string): Promise<void> {
    const run = sessions.current[id];

    if (
      !supportsStop
      || !run
      || !isDesktopRunActive(run)
      || run.state !== RUN_STATE.RUNNING
    ) {
      return;
    }

    update(id, { state: RUN_STATE.STOPPING, stopError: null });

    try {
      await invoke<void>("stop_factory_run", { runId: id });
    } catch (error) {
      // A restored ID may belong to a finished run or a previous app instance.
      if (run.tracking === "restored" && isRunTrackingUnavailable(error)) {
        forget(id, { key: "couldNotStopRunError", values: { error: String(error) } });
        return;
      }
      // Execution may have finished while the Stop request was pending.
      if (sessions.current[id]?.state === RUN_STATE.STOPPING) {
        update(id, {
          state: RUN_STATE.RUNNING,
          stopError: { key: "couldNotStopRunError", values: { error: String(error) } },
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
    find: (repoPath: string, branch: string, runId: string) => Object.values(runs).reverse().find((run) =>
      run.repoPath === repoPath && run.branch === branch && (run.id === runId || (run.runId === runId
        && run.directory?.split(/[\\/]/).slice(-2)[0] === branch.replace(/[^a-zA-Z0-9._-]/g, "-")))),
  };
}

export type DesktopRuns = ReturnType<typeof useDesktopRuns>;
