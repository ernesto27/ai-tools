import { useEffect, useRef, type RefObject } from "react";
import { invoke } from "@tauri-apps/api/core";
import { isDesktopRunActive, type DesktopRun } from "../desktopRun";
import { AppMessageError, type UiMessage } from "../messages";
import { RUN_STATE } from "../runState";
import type { BranchWorktree } from "../worktrees";
import type { RunEntry } from "./useBranchRuns";

function newRunEntry(entries: RunEntry[], run: DesktopRun): RunEntry | undefined {
  const previous = new Set(run.previousDirectories);
  const candidates = entries.filter((entry) => entry.name.startsWith(`${run.mode}-`) && !previous.has(entry.directory));
  if (candidates.length > 1) {
    throw new AppMessageError({ key: "multipleNewRunDirectoriesFoundOpenTheRunFromTheBranchList" });
  }
  return candidates[0];
}

export function useRunDiscovery(
  sessions: RefObject<Record<string, DesktopRun>>,
  update: (id: string, patch: Partial<DesktopRun>) => void,
  onBranchCreated: (path: string, branch: BranchWorktree) => void,
  lifecycle: RefObject<{ active: boolean; leaving: boolean }>,
) {
  const callbacks = useRef({ update, onBranchCreated });
  useEffect(() => { callbacks.current = { update, onBranchCreated }; }, [update, onBranchCreated]);
  const controller = useRef({
    active: false,
    generation: 0,
    foundBranches: new Set<string>(),
    pending: new Map<string, Promise<void>>(),
  });

  function currentRun(id: string, generation: number, final: boolean): DesktopRun | undefined {
    const state = controller.current;
    if (!state.active || state.generation !== generation || !lifecycle.current.active || lifecycle.current.leaving) return;
    const run = sessions.current[id];
    if (!run || run.tracking === "unavailable" || run.state === RUN_STATE.STARTING) return;
    if (final || isDesktopRunActive(run)) return run;
  }

  async function discover(id: string, generation: number, final: boolean) {
    let run = currentRun(id, generation, final);
    if (!run) return;
    if (!run.directory && run.mode !== "resume") {
      try {
        const entries = await invoke<RunEntry[]>("list_branch_runs", { path: run.repoPath, branch: run.branch });
        run = currentRun(id, generation, final);
        if (!run) return;
        const entry = run.directory ? undefined : newRunEntry(entries, run);
        if (entry) {
          callbacks.current.update(id, {
            runId: entry.name, directory: entry.directory,
            error: run.state === RUN_STATE.FAILED ? run.error : null,
          });
        }
      } catch (error) {
        run = currentRun(id, generation, final);
        if (run && isDesktopRunActive(run)) {
          const message: UiMessage = error instanceof AppMessageError ? error.descriptor
            : { key: "couldNotLocateRunLogsError", values: { error: String(error) } };
          callbacks.current.update(id, { error: message });
        }
      }
    }

    run = currentRun(id, generation, final);
    if (!run || controller.current.foundBranches.has(id)) return;
    try {
      const branches = await invoke<BranchWorktree[]>("list_branches", { path: run.repoPath });
      run = currentRun(id, generation, final);
      if (!run) return;
      const branchName = run.branch;
      const branch = branches.find((entry) => entry.branch === branchName);
      if (branch) {
        controller.current.foundBranches.add(id);
        callbacks.current.onBranchCreated(run.repoPath, branch);
      }
    } catch (error) {
      if (currentRun(id, generation, final)) console.error("Could not check run branch:", error);
    }
  }

  function enqueue(id: string, final = false): Promise<void> {
    const state = controller.current;
    const generation = state.generation;
    const previous = state.pending.get(id) ?? Promise.resolve();
    const task = previous.then(() => discover(id, generation, final)).catch((error) => {
      console.error("Could not discover run:", error);
    });
    state.pending.set(id, task);
    void task.then(() => {
      if (state.pending.get(id) === task) state.pending.delete(id);
    });
    return task;
  }

  useEffect(() => {
    const state = controller.current;
    state.active = true;
    const generation = ++state.generation;
    state.foundBranches.clear();
    state.pending.clear();
    let timer: ReturnType<typeof setTimeout> | undefined;

    async function poll() {
      const pending = Object.values(sessions.current).filter((run) =>
        currentRun(run.id, generation, false)
        && (!run.directory || !state.foundBranches.has(run.id)));
      await Promise.all(pending.map((run) => enqueue(run.id)));
      if (state.active && state.generation === generation && !lifecycle.current.leaving) {
        timer = setTimeout(() => void poll(), 1000);
      }
    }
    void poll();
    return () => {
      state.active = false;
      if (timer !== undefined) clearTimeout(timer);
    };
  }, [sessions, lifecycle]);

  return { finish: (id: string) => enqueue(id, true) };
}
