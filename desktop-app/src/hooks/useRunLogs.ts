import { useEffect, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import type { RunEntry } from "./useBranchRuns";

export interface RunLog {
  filename: string;
  content: string;
}

interface RunDetail {
  entry: RunEntry;
  logs: RunLog[];
}

export function useRunLogs(repoPath: string, branch: string, runId: string) {
  const [logs, setLogs] = useState<RunLog[]>([]);
  const [run, setRun] = useState<RunEntry | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    let active = true;

    setLogs([]);
    setRun(null);
    setLoading(true);
    setError(null);

    async function load() {
      try {
        const { entry, logs } = await invoke<RunDetail>("load_run_detail", {
          path: repoPath,
          branch,
          runId,
        });

        if (active) {
          setLogs(logs);
          setRun(entry);
        }
      } catch (error) {
        if (active) setError(String(error));
      } finally {
        if (active) setLoading(false);
      }
    }

    void load();

    return () => {
      active = false;
    };
  }, [repoPath, branch, runId, revision]);

  return {
    logs,
    run,
    loading,
    error,
    refresh: () => setRevision((current) => current + 1),
  };
}
