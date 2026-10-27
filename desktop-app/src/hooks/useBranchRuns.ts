import { useEffect, useState } from "react";
import { invoke } from "@tauri-apps/api/core";

export interface StageStatus {
  stage: string;
  status: string;
}

export interface RunEntry {
  name: string;
  directory: string;
  stages: StageStatus[];
  error: string | null;
}

export function useBranchRuns(repoPath: string, branch: string) {
  const [entries, setEntries] = useState<RunEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    let active = true;
    let timer: ReturnType<typeof setTimeout> | undefined;

    setLoading(true);
    setError(null);
    setEntries([]);

    async function load() {
      try {
        const entries = await invoke<RunEntry[]>("list_branch_runs", { path: repoPath, branch });
        if (active) {
          setEntries(entries);
          setError(null);
        }
      } catch (error) {
        if (active) setError(String(error));
      } finally {
        if (active) {
          setLoading(false);
          timer = setTimeout(load, 2000);
        }
      }
    }

    void load();

    return () => {
      active = false;
      if (timer !== undefined) clearTimeout(timer);
    };
  }, [repoPath, branch, revision]);

  return {
    entries,
    loading,
    error,
    refresh: () => setRevision((current) => current + 1),
  };
}
