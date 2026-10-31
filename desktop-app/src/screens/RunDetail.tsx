import { useEffect, useLayoutEffect, useRef, useState } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { revealItemInDir } from "@tauri-apps/plugin-opener";
import { useRunLogs } from "../hooks/useRunLogs";
import { isDesktopRunActive, RUN_STATE, type DesktopRuns } from "../hooks/useDesktopRuns";
import { STAGES, stageProgress } from "../runProgress";
import { RUN_MODE, type Navigate } from "../router";
import { statusClass, statusLabel } from "../runStatus";
import { Crumb, cx, LinkButton, OutlineButton, PrimaryButton, Tabs, TextButton, Title } from "../components/ui";

type Tab = "log" | "summary" | "walk";

const TABS = [
  ["log", "Log"],
  ["summary", "Summary"],
  ["walk", "Walkthrough"],
] as const;

interface Props {
  desktopRuns: DesktopRuns;
  repo: string;
  repoPath: string;
  branch: string;
  runId: string;
  navigate: Navigate;
}

export function RunDetail({ repo, repoPath, branch, runId, navigate, desktopRuns }: Props) {
  const session = desktopRuns.find(repoPath, branch, runId);
  const live = session !== undefined && isDesktopRunActive(session);
  const showStop = desktopRuns.supportsStop
    && session !== undefined
    && live;
  const { logs, run, loading, error, refresh } = useRunLogs(repoPath, branch, runId, session);
  const interrupted = session?.state === RUN_STATE.INTERRUPTED
    || (!live && run?.stages[run.stages.length - 1]?.status === "INTERRUPTED");
  const title = session?.runId ?? (
    session
      ? session.state === RUN_STATE.STOPPING ? "Stopping run"
        : interrupted ? "Stopped run"
        : live ? "Starting run"
        : session.state === RUN_STATE.FAILED ? "Failed run"
        : "Completed run"
      : runId
  );
  const [selectedStage, setSelectedStage] = useState<string>(STAGES[0].id);
  const [tab, setTab] = useState<Tab>("log");
  const [folderError, setFolderError] = useState<string | null>(null);
  const [stopDialogError, setStopDialogError] = useState<string | null>(null);
  const stopConfirmationPending = useRef(false);

  const [following, setFollowing] = useState(true);
  const outputPanel = useRef<HTMLDivElement>(null);
  const progress = stageProgress(run?.stages ?? [], logs, live, session !== undefined);

  useEffect(() => {
    if (following && progress.followStage) {
      setSelectedStage(progress.followStage);
      setTab("log");
    }
  }, [following, progress.followStage]);

  const stages = STAGES.filter((stage) =>
    logs.some((log) => log.filename === stage.filename)
    || run?.stages.some((entry) => entry.stage === stage.id),
  );
  const currentStage = stages.find((stage) => stage.id === selectedStage) ?? stages[0];
  const filename = tab === "summary" ? "summary.log"
    : tab === "walk" ? "05-reviewChanges.md" : currentStage?.filename;
  const output = logs.find((log) => log.filename === filename);

  useLayoutEffect(() => {
    if (outputPanel.current) {
      outputPanel.current.scrollTop = outputPanel.current.scrollHeight;
    }
  }, [filename, output?.content]);

  async function stopRun() {
    if (!session || session.state !== RUN_STATE.RUNNING || stopConfirmationPending.current) return;

    stopConfirmationPending.current = true;
    setStopDialogError(null);
    try {
      // Tauri's dialog plugin makes window.confirm asynchronous.
      const confirmed = await window.confirm("Stop this run? You can resume it later.");
      if (confirmed) {
        await desktopRuns.stop(session.id);
      }
    } catch (error) {
      setStopDialogError(`Could not confirm stopping the run: ${String(error)}`);
    } finally {
      stopConfirmationPending.current = false;
    }
  }

  async function openFolder() {
    if (!run) return;
    setFolderError(null);
    try {
      await revealItemInDir(run.directory);
    } catch (error) {
      setFolderError(String(error));
    }
  }

  return (
    <div className="flex h-full min-h-0 w-full flex-col gap-6 overflow-hidden xl:gap-9" aria-busy={loading}>
      <header className="flex min-w-0 shrink-0 flex-col gap-2">
        <Crumb>
          {repo} /{" "}
          <LinkButton muted onClick={() => navigate({ screen: "branch", repo, branch })}>
            {branch}
          </LinkButton>
        </Crumb>
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div className="min-w-0 break-words"><Title>{title}</Title></div>
          <div className="flex flex-wrap items-center gap-5 text-md">
            <TextButton disabled={!run} onClick={() => void openFolder()} className="disabled:opacity-40">
              Show folder
            </TextButton>
            <OutlineButton disabled={loading} onClick={refresh}>
              {loading ? "Loading…" : "Refresh"}
            </OutlineButton>
            {showStop && (
              <OutlineButton
                disabled={session.state !== RUN_STATE.RUNNING}
                onClick={() => void stopRun()}
                className="text-bad disabled:cursor-not-allowed disabled:opacity-40"
              >
                {session.state === RUN_STATE.STOPPING ? "Stopping…" : "Stop"}
              </OutlineButton>
            )}
            <PrimaryButton disabled={!!desktopRuns.busy(repoPath)} onClick={() => navigate({
              screen: "newRun", repo, branch,
              mode: interrupted ? RUN_MODE.RESUME : RUN_MODE.CONTINUE,
            })}>
              {interrupted ? "Resume" : "Continue"}
            </PrimaryButton>
          </div>
        </div>
      </header>

      {session && (
        <div className="shrink-0 space-y-2">
          <p role="status" className="text-sm text-muted">
            {session.state === RUN_STATE.STARTING ? "Starting run…"
              : session.state === RUN_STATE.STOPPING ? "Stopping run… Waiting for cleanup."
              : live ? (session.directory ? "Live · updates every second" : "Waiting for execution log directory…")
              : session.state === RUN_STATE.SUCCEEDED ? "Run completed."
              : session.state === RUN_STATE.INTERRUPTED ? "Run stopped. Use Resume to continue."
              : "Run failed."}
          </p>
          {session.error && <p role="alert" className="whitespace-pre-wrap break-words text-bad">{session.error}</p>}
          {session.stopError && (
            <p role="alert" className="whitespace-pre-wrap break-words text-bad">{session.stopError}</p>
          )}
        </div>
      )}

      {loading && <p role="status" className="text-muted">Loading run logs…</p>}
      {error && <p role="alert" className="break-words text-bad">{error}</p>}
      {folderError && <p role="alert" className="break-words text-bad">Could not show folder: {folderError}</p>}
      {stopDialogError && <p role="alert" className="break-words text-bad">{stopDialogError}</p>}

      {!loading && (
        <>
          {run?.error && <p role="alert" className="break-words text-bad">Could not load stage statuses: {run.error}</p>}

          <dl className="grid shrink-0 grid-cols-[repeat(auto-fit,minmax(150px,1fr))] gap-4 border-y border-line py-5">
            {STAGES.map((stage) => {
              const status = progress.statuses.get(stage.id);
              return (
                <div key={stage.id} className="flex flex-col gap-0.5">
                  <dt className="text-sm text-muted">{stage.label}</dt>
                  <dd className={cx("text-xl font-medium", statusClass(status))}>
                    {!status && live ? "Pending" : statusLabel(status)}
                  </dd>
                </div>
              );
            })}
          </dl>

          {logs.length === 0 && <p role="status" className="text-muted">No log files recorded for this run yet.</p>}

          <div className="grid min-h-0 flex-1 grid-cols-[minmax(140px,220px)_minmax(0,1fr)] gap-6 lg:gap-10">
            <section aria-label="Stages" className="flex min-h-0 min-w-0 flex-col gap-0.5 overflow-y-auto">
              {stages.length === 0 && <p className="text-sm text-muted">No stages recorded yet.</p>}
              {stages.map((stage) => {
                const status = progress.statuses.get(stage.id);
                return (
                  <button
                    key={stage.id}
                    type="button"
                    aria-pressed={currentStage?.id === stage.id}
                    onClick={() => {
                      setFollowing(false);
                      setSelectedStage(stage.id);
                      setTab("log");
                    }}
                    className="flex min-h-13 w-full shrink-0 flex-col items-start gap-0.5 rounded-md px-2.5 py-2 text-left hover:bg-hover aria-pressed:bg-selected"
                  >
                    <span className="font-medium">{stage.label}</span>
                    <span className={cx("text-sm", statusClass(status))}>
                      {!status && live ? "Pending" : statusLabel(status)}
                    </span>
                  </button>
                );
              })}
            </section>

            <section aria-label="Output" className="flex min-h-0 min-w-0 flex-col gap-3 overflow-hidden">
              <div className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b border-line">
                <Tabs label="Views" items={TABS} value={tab} onChange={(next) => { setFollowing(false); setTab(next); }} size="sm" bordered={false} />
                {live && (
                  <TextButton aria-pressed={following} onClick={() => {
                    setFollowing(true);
                    setTab("log");
                    if (progress.followStage) setSelectedStage(progress.followStage);
                    if (outputPanel.current) {
                      outputPanel.current.scrollTop = outputPanel.current.scrollHeight;
                    }
                  }}>
                    {following ? "Following live" : "Follow live"}
                  </TextButton>
                )}
                <span className="font-mono text-xs break-all text-muted">{filename ?? "No stage selected"}</span>
              </div>

              <div
                ref={outputPanel}
                key={filename ?? "no-file"}
                role="region"
                aria-label={filename ? `Contents of ${filename}` : "File contents"}
                tabIndex={0}
                className="min-h-0 flex-1 overflow-auto overscroll-contain rounded-md bg-panel"
              >
                {!output ? (
                  <p role="status" className="rounded-md bg-panel p-4 text-muted">
                    {tab === "summary" ? "No summary recorded for this run yet."
                      : tab === "walk" ? "No walkthrough recorded for this run yet."
                      : "No log recorded for this stage yet."}
                  </p>
                ) : output.content.length === 0 ? (
                  <p role="status" className="rounded-md bg-panel p-4 text-muted">This file is empty.</p>
                ) : tab === "walk" ? (
                  <article className="walkthrough-markdown p-5">
                    <Markdown remarkPlugins={[remarkGfm]}>{output.content}</Markdown>
                  </article>
                ) : (
                  <div className="py-4 font-mono text-sm leading-[1.8]">
                    {output.content.replace(/\r\n/g, "\n").split("\n").map((line, index) => (
                      <div key={index} className="flex gap-4 px-4 whitespace-pre">
                        <span className="w-8 shrink-0 text-right text-faint select-none" aria-hidden="true">{index + 1}</span>
                        <span>{line}</span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </section>
          </div>
        </>
      )}
    </div>
  );
}
