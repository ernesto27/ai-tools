import { useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { join } from "@tauri-apps/api/path";
import { RUN_STATE } from "../runState";

interface AuditTarget {
  repoPath: string;
  directory: string;
  filename: string;
}

interface DesktopAudit {
  executionId: string;
  state: (typeof RUN_STATE)[keyof typeof RUN_STATE];
  reportPath: string | null;
  error: string | null;
  stopError: string | null;
  startedAt: number;
  finishedAt: number | null;
}

function targetKey(target: AuditTarget): string {
  return JSON.stringify([target.repoPath, target.directory, target.filename]);
}

// App owns this hook so pending commands and their results survive screen changes.
export function useDesktopAudits() {
  const sessions = useRef<Record<string, DesktopAudit>>({});
  const [audits, setAudits] = useState<Record<string, DesktopAudit>>({});

  function update(key: string, patch: Partial<DesktopAudit>) {
    sessions.current = { ...sessions.current, [key]: { ...sessions.current[key], ...patch } };
    setAudits(sessions.current);
  }

  function start(target: AuditTarget) {
    const key = targetKey(target);
    const current = sessions.current[key];
    if (current?.state === RUN_STATE.RUNNING || current?.state === RUN_STATE.STOPPING) return;
    const executionId = `audit:${crypto.randomUUID()}`;
    const startedAt = Date.now();
    update(key, {
      executionId, state: RUN_STATE.RUNNING, reportPath: null, error: null,
      stopError: null, startedAt, finishedAt: null,
    });

    void (async () => {
      try {
        const logPath = await join(target.directory, target.filename);
        const outcome = await invoke<typeof RUN_STATE.SUCCEEDED | typeof RUN_STATE.INTERRUPTED>("audit_factory_run", {
          path: target.repoPath,
          executionId,
          logPath,
        });
        if (outcome === RUN_STATE.INTERRUPTED) {
          update(key, { state: RUN_STATE.INTERRUPTED, stopError: null, finishedAt: Date.now() });
          return;
        }
        // software-factory archives the HTML beside the input log with this name.
        const reportPath = await join(target.directory, target.filename.replace(/\.log$/, "-audit.html"));
        update(key, { state: RUN_STATE.SUCCEEDED, reportPath, stopError: null, finishedAt: Date.now() });
      } catch (error) {
        update(key, { state: RUN_STATE.FAILED, error: String(error), stopError: null, finishedAt: Date.now() });
      }
    })();
  }

  async function stop(target: AuditTarget): Promise<void> {
    const key = targetKey(target);
    const audit = sessions.current[key];
    if (!audit || audit.state !== RUN_STATE.RUNNING) return;

    update(key, { state: RUN_STATE.STOPPING, stopError: null });
    try {
      await invoke<void>("stop_factory_run", { runId: audit.executionId });
    } catch (error) {
      // The audit may finish, or a retry may start, before this request settles.
      const current = sessions.current[key];
      if (current?.executionId === audit.executionId && current.state === RUN_STATE.STOPPING) {
        update(key, { state: RUN_STATE.RUNNING, stopError: String(error) });
      }
    }
  }

  return {
    start,
    stop,
    find: (target: AuditTarget) => audits[targetKey(target)],
  };
}

export type DesktopAudits = ReturnType<typeof useDesktopAudits>;
