import { useEffect, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import type { RunEntry } from "./useBranchRuns";
import { isDesktopRunActive, type DesktopRun } from "./useDesktopRuns";

export interface RunLog {
  filename: string;
  content: string;
}

interface RunDetail {
  entry: RunEntry;
  logs: RunLog[];
}

export function useRunLogs(repoPath: string, branch: string, runId: string, session?: DesktopRun) {
  const [logs, setLogs] = useState<RunLog[]>([]);
  const [run, setRun] = useState<RunEntry | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [revision, setRevision] = useState(0);
  const pending = useRef<Promise<void>>(Promise.resolve());
  const resolvedRunId = session ? session.runId : runId;
  const live = session !== undefined && isDesktopRunActive(session);

  useEffect(() => {
    setLogs([]);
    setRun(null);
    setError(null);
    setLoading(true);
  }, [repoPath, branch, runId]);

  useEffect(() => {
    let active = true;
    let timer: ReturnType<typeof setTimeout> | undefined;
    if (!resolvedRunId) {
      setLoading(false);
      return;
    }
    function load() {
      // Serialize refreshes across effect restarts (completion and manual Refresh).
      pending.current = pending.current.then(async () => {
        if (!active) return;
        try {
          const detail = await invoke<RunDetail>("load_run_detail", {
            path: repoPath, branch, runId: resolvedRunId,
          });
          if (active) {
            setLogs(detail.logs);
            // Preserve last valid status snapshot if a writer was caught mid-line.
            setRun((previous) => detail.entry.error && previous
              ? { ...detail.entry, stages: previous.stages }
              : detail.entry);
            setError(null);
          }
        } catch (error) {
          if (active) setError(String(error));
        } finally {
          if (active) {
            setLoading(false);
            timer = setTimeout(load, 1000);
          }
        }
      });
    }
    load();
    return () => {
      active = false;
      if (timer !== undefined) clearTimeout(timer);
    };
  }, [repoPath, branch, runId, resolvedRunId, live, revision]);

  return { logs, run, loading, error, refresh: () => setRevision((value) => value + 1) };
}
