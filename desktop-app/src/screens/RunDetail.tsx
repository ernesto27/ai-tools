import { messageText, type UiMessage } from "../messages";
import { useTranslation } from "react-i18next";
import { useEffect, useId, useLayoutEffect, useMemo, useRef, useState } from "react";
import { openUrl, revealItemInDir } from "@tauri-apps/plugin-opener";
import { implementationPullRequestUrl } from "../runPullRequest";
import { useRunLogs } from "../hooks/useRunLogs";
import type { DesktopAudits } from "../hooks/useDesktopAudits";
import { isDesktopRunActive, RUN_STATE, type DesktopRuns } from "../hooks/useDesktopRuns";
import { runLogLines, searchRunLogs, type RunLogMatch } from "../runLogSearch";
import { STAGES, stageProgress } from "../runProgress";
import { STAGE_STATUS } from "../runStatus";
import { RUN_MODE, type Navigate } from "../router";
import { StageSummary } from "../components/StageSummary";
import { AuditDuration } from "../components/AuditDuration";
import { AuditReportLink } from "../components/AuditReportLink";
import { RunChanges } from "../components/RunChanges";
import { WalkthroughMarkdown, WalkthroughReader } from "../components/Walkthrough";
import { Crumb, Input, LinkButton, OutlineButton, PrimaryButton, Tabs, TextButton, Title } from "../components/ui";

type Tab = "log" | "summary" | "walk" | "changes";

const TABS = [
  ["log", "log"],
  ["summary", "summary"],
  ["walk", "walkthrough"],
  ["changes", "changes"],
] as const;

interface Props {
  desktopRuns: DesktopRuns;
  desktopAudits: DesktopAudits;
  repo: string;
  repoPath: string;
  branch: string;
  runId: string;
  navigate: Navigate;
}

export function RunDetail({ repo, repoPath, branch, runId, navigate, desktopRuns, desktopAudits }: Props) {
  const { t } = useTranslation();
  const session = desktopRuns.find(repoPath, branch, runId);
  const live = session !== undefined && isDesktopRunActive(session);
  const trackingUnavailable = session?.tracking === "unavailable";
  const showStop = desktopRuns.supportsStop
    && session !== undefined
    && live;
  const { logs, run, loading, error } = useRunLogs(repoPath, branch, runId, session);
  const pullRequestUrl = useMemo(
    () => implementationPullRequestUrl(run?.stages ?? [], logs),
    [run?.stages, logs],
  );
  const [pullRequestError, setPullRequestError] = useState<string | null>(null);
  useEffect(() => {
    setPullRequestError(null);
  }, [pullRequestUrl]);
  const interrupted = session?.state === RUN_STATE.INTERRUPTED
    || (!live && run?.stages[run.stages.length - 1]?.status === "INTERRUPTED");
  const title = session?.runId ?? (
    session
      ? trackingUnavailable ? t("run")
        : session.state === RUN_STATE.STOPPING ? t("stoppingRun")
        : interrupted ? t("stoppedRun")
        : live ? t("startingRun")
        : session.state === RUN_STATE.FAILED ? t("failedRun")
        : t("completedRun")
      : runId
  );
  const [selectedStage, setSelectedStage] = useState<string>(STAGES[0].id);
  const [tab, setTab] = useState<Tab>("log");
  const [logSearchOpen, setLogSearchOpen] = useState(false);
  const logSearchTitle = useId();
  const logSearchTrigger = useRef<HTMLButtonElement>(null);
  const logSearchDialog = useRef<HTMLDialogElement>(null);
  const logSearchInput = useRef<HTMLInputElement>(null);
  const [logQuery, setLogQuery] = useState("");
  const [selectedMatch, setSelectedMatch] = useState<RunLogMatch | null>(null);
  const query = logQuery.trim();
  const logSearch = useMemo(
    () => logSearchOpen ? searchRunLogs(logs, query) : null,
    [logSearchOpen, logs, query],
  );
  const matchGroups = logSearch?.groups ?? [];
  const [folderError, setFolderError] = useState<string | null>(null);
  const [stopDialogError, setStopDialogError] = useState<UiMessage | null>(null);
  const stopConfirmationPending = useRef(false);

  const [following, setFollowing] = useState(true);
  const outputPanel = useRef<HTMLDivElement>(null);
  const progress = stageProgress(run?.stages ?? [], logs, live, session !== undefined && !trackingUnavailable);

  useLayoutEffect(() => {
    if (logSearchOpen) {
      logSearchDialog.current?.showModal();
      logSearchInput.current?.focus();
    }
  }, [logSearchOpen]);

  useEffect(() => {
    if (following && progress.followStage) {
      setSelectedStage(progress.followStage);
      setTab("log");
    }
  }, [following, progress.followStage]);

  const stages = STAGES.filter((stage) =>
    logs.some((log) => log.filename === stage.filename)
    || run?.stages.some((entry) => entry.stage === stage.id)
    || selectedMatch?.source.filename === stage.filename,
  );
  const currentStage = stages.find((stage) => stage.id === selectedStage) ?? stages[0];
  const filename = tab === "changes" ? undefined
    : tab === "summary" ? "summary.log"
    : tab === "walk" ? "05-reviewChanges.md" : currentStage?.filename;
  const output = logs.find((log) => log.filename === filename);
  const selectedLine = selectedMatch?.source.filename === filename
    ? selectedMatch?.lineNumber
    : undefined;

  useLayoutEffect(() => {
    const panel = outputPanel.current;
    if (panel && selectedLine === undefined) {
      panel.scrollTop = panel.scrollHeight;
    }
  }, [filename, output?.content, selectedLine]);

  useLayoutEffect(() => {
    const panel = outputPanel.current;
    if (!panel || selectedLine === undefined) return;

    const row = panel.querySelector<HTMLElement>(`[data-log-line="${selectedLine}"]`);
    if (!row) return;

    panel.scrollTop += row.getBoundingClientRect().top - panel.getBoundingClientRect().top
      - (panel.clientHeight - row.offsetHeight) / 2;
    row.focus({ preventScroll: true });
  }, [filename, selectedLine, selectedMatch]);

  function closeLogSearch() {
    logSearchDialog.current?.close();
    setLogSearchOpen(false);
  }

  function dismissLogSearch() {
    closeLogSearch();
    logSearchTrigger.current?.focus({ preventScroll: true });
  }

  function openLogMatch(match: RunLogMatch) {
    closeLogSearch();
    setFollowing(false);
    if (match.source.tab === "log") setSelectedStage(match.source.stage);
    setTab(match.source.tab);
    setSelectedMatch({ ...match });
  }

  async function stopRun() {
    if (!session || session.state !== RUN_STATE.RUNNING || stopConfirmationPending.current) return;

    stopConfirmationPending.current = true;
    setStopDialogError(null);
    try {
      // Tauri's dialog plugin makes window.confirm asynchronous.
      const confirmed = await window.confirm(t("stopThisRunYouCanResumeItLater"));
      if (confirmed) {
        await desktopRuns.stop(session.id);
      }
    } catch (error) {
      setStopDialogError({ key: "couldNotConfirmStoppingTheRunError", values: { error: String(error) } });
    } finally {
      stopConfirmationPending.current = false;
    }
  }

  async function openPullRequest() {
    if (!pullRequestUrl) return;
    setPullRequestError(null);
    try {
      await openUrl(pullRequestUrl);
    } catch (error) {
      setPullRequestError(String(error));
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
          <div className="min-w-0 max-w-full space-y-2 break-words">
            <Title>{title}</Title>
            {pullRequestUrl && (
              <LinkButton
                data-testid="run-pull-request"
                className="block max-w-full break-all text-left text-md focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-info"
                onClick={() => void openPullRequest()}
              >
                {pullRequestUrl}
              </LinkButton>
            )}
          </div>
          <div className="flex flex-wrap items-center gap-5 text-md">
            <TextButton disabled={!run} onClick={() => void openFolder()} className="disabled:opacity-40">
              {t("showFolder")}
            </TextButton>
            {showStop && (
              <OutlineButton
                disabled={session.state !== RUN_STATE.RUNNING}
                onClick={() => void stopRun()}
                className="text-bad disabled:cursor-not-allowed disabled:opacity-40"
              >
                {session.state === RUN_STATE.STOPPING ? t("stopping") : t("stop")}
              </OutlineButton>
            )}
            <PrimaryButton onClick={() => navigate({
              screen: "newRun", repo, branch,
              mode: interrupted ? RUN_MODE.RESUME : RUN_MODE.CONTINUE,
            })}>
              {interrupted ? t("resume") : t("continue")}
            </PrimaryButton>
          </div>
        </div>
      </header>

      {session && (
        <div className="shrink-0 space-y-2">
          {!trackingUnavailable && <p role="status" className="text-sm text-muted">
            {session.state === RUN_STATE.STARTING ? t("startingRunProgress")
              : session.state === RUN_STATE.STOPPING ? t("stoppingRunWaitingForCleanup")
              : live ? (session.directory ? t("liveUpdatesEverySecond") : t("waitingForExecutionLogDirectory"))
              : session.state === RUN_STATE.SUCCEEDED ? t("runCompleted")
              : session.state === RUN_STATE.INTERRUPTED ? t("runStoppedUseResumeToContinue")
              : t("runFailed")}
          </p>}
          {session.error && <p role="alert" className="whitespace-pre-wrap break-words text-bad">{messageText(session.error, t)}</p>}
          {session.stopError && (
            <p role="alert" className="whitespace-pre-wrap break-words text-bad">{messageText(session.stopError, t)}</p>
          )}
        </div>
      )}

      {loading && <p role="status" className="text-muted">{t("loadingRunLogs")}</p>}
      {error && <p role="alert" className="break-words text-bad">{error}</p>}
      {folderError && <p role="alert" className="break-words text-bad">{t("couldNotShowFolderError", { error: folderError })}</p>}
      {pullRequestError && (
        <p role="alert" className="break-words text-bad">
          {t("couldNotOpenPullRequestError", { error: pullRequestError })}
        </p>
      )}
      {stopDialogError && <p role="alert" className="break-words text-bad">{messageText(stopDialogError, t)}</p>}

      {!loading && (
        <>
          {run?.error && <p role="alert" className="break-words text-bad">{t("couldNotLoadStageStatusesError", { error: run.error })}</p>}

          <StageSummary
            statuses={progress.statuses}
            logs={logs}
            live={live}
            selectedStage={currentStage?.id}
            onSelectStage={(stage) => {
              setFollowing(false);
              setSelectedMatch(null);
              setSelectedStage(stage);
              setTab("log");
            }}
            renderActions={(stage) => {
              if (!stage.supportsAudit || progress.statuses.get(stage.id) !== STAGE_STATUS.SUCCEEDED) return null;
              const target = run ? { repoPath, directory: run.directory, filename: stage.filename } : null;
              const audit = target ? desktopAudits.find(target) : undefined;
              const auditing = audit?.state === RUN_STATE.RUNNING;
              const stopping = audit?.state === RUN_STATE.STOPPING;
              const canStop = auditing && desktopRuns.supportsStop;
              const hasLog = logs.some((log) => log.filename === stage.filename);
              return (
                <div className="flex min-w-0 flex-col items-start gap-2 px-2.5" data-testid={`audit-${stage.id}`}>
                  <OutlineButton
                    disabled={stopping || (auditing ? !desktopRuns.supportsStop
                      : !target || !hasLog)}
                    aria-label={t(stopping ? "stoppingAuditStage" : canStop ? "stopAuditStage"
                      : auditing ? "auditingStage" : "auditStage", { stage: t(stage.label) })}
                    aria-busy={auditing || stopping}
                    onClick={() => {
                      if (!target) return;
                      if (canStop) void desktopAudits.stop(target);
                      else desktopAudits.start(target);
                    }}
                  >
                    {t(stopping ? "stopping" : canStop ? "stop" : auditing ? "auditing" : "audit")}
                  </OutlineButton>
                  {audit && <AuditDuration startedAt={audit.startedAt} finishedAt={audit.finishedAt} />}
                  {audit?.reportPath && (
                    <AuditReportLink key={audit.reportPath} path={audit.reportPath} />
                  )}
                  {audit?.state === RUN_STATE.FAILED && (
                    <p role="alert" className="w-full break-words text-sm text-bad">
                      {t("auditFailed", { error: audit.error })}
                    </p>
                  )}
                  {audit?.stopError && (
                    <p role="alert" className="w-full break-words text-sm text-bad">
                      {t("couldNotStopAuditError", { error: audit.stopError })}
                    </p>
                  )}
                  {audit?.state === RUN_STATE.INTERRUPTED && (
                    <p role="status" className="text-sm text-muted">{t("auditInterrupted")}</p>
                  )}
                </div>
              );
            }}
          />

          {logs.length === 0 && <p role="status" className="text-muted">{t("noLogFilesRecordedForThisRunYet")}</p>}
        </>
      )}

      <section aria-label={t("output")} className="flex min-h-0 min-w-0 flex-1 flex-col gap-3 overflow-hidden">
        <div className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b border-line">
          <Tabs label={t("views")} items={TABS.map(([key, label]) => [key, t(label)] as const)} value={tab} onChange={(next) => {
            setFollowing(false);
            setSelectedMatch(null);
            setTab(next);
          }} size="sm" bordered={false} />
          <OutlineButton
            ref={logSearchTrigger}
            aria-haspopup="dialog"
            disabled={loading}
            onClick={() => setLogSearchOpen(true)}
          >
            {t("searchRunLogs")}
          </OutlineButton>
          {live && (
            <TextButton aria-pressed={following} onClick={() => {
              setSelectedMatch(null);
              setFollowing(true);
              setTab("log");
              if (progress.followStage) setSelectedStage(progress.followStage);
              if (outputPanel.current) {
                outputPanel.current.scrollTop = outputPanel.current.scrollHeight;
              }
            }}>
              {following ? t("followingLive") : t("followLive")}
            </TextButton>
          )}
          {tab !== "changes" && (
            <div className="flex min-w-0 flex-wrap items-center gap-3">
              <span className="font-mono text-xs break-all text-muted">{filename ?? t("noStageSelected")}</span>
              {tab === "walk" && output?.content && <WalkthroughReader content={output.content} />}
            </div>
          )}
        </div>

        {logSearchOpen && (
          <dialog
            ref={logSearchDialog}
            aria-labelledby={logSearchTitle}
            onCancel={(event) => {
              event.preventDefault();
              dismissLogSearch();
            }}
            className="fixed inset-0 m-auto max-h-[calc(100dvh-2rem)] w-[calc(100%-2rem)] max-w-2xl flex-col gap-4 overflow-hidden rounded-lg border border-field-line bg-panel p-4 text-ink shadow-2xl open:flex backdrop:bg-[#0009] sm:p-5"
          >
            <div className="flex shrink-0 items-center justify-between gap-4">
              <h2 id={logSearchTitle} className="text-lg font-semibold text-ink-strong">{t("searchRunLogs")}</h2>
              <OutlineButton onClick={dismissLogSearch}>{t("closeRunLogSearch")}</OutlineButton>
            </div>
            <Input
              ref={logSearchInput}
              type="search"
              aria-label={t("searchRunLogs")}
              placeholder={t("searchRunLogs")}
              value={logQuery}
              onChange={(event) => setLogQuery(event.currentTarget.value)}
              onKeyDown={(event) => {
                if (event.key !== "Escape" || event.nativeEvent.isComposing) return;
                // Search inputs can consume Escape to clear their value.
                event.preventDefault();
                dismissLogSearch();
              }}
              className="w-full shrink-0"
            />
            {query && (
              matchGroups.length === 0 ? (
                <p role="status" className="shrink-0 text-sm text-muted">{t("noMatchingLogLines")}</p>
              ) : (
                <>
                  <div
                    role="region"
                    aria-label={t("runLogSearchResults")}
                    className="min-h-0 overflow-auto overscroll-contain rounded-md border border-line p-2"
                  >
                    {matchGroups.map(({ source, matches }) => (
                      <div key={source.filename} className="space-y-1 pb-2 last:pb-0">
                        <h3 className="px-2 text-sm font-medium text-muted">{t(source.label)}</h3>
                        <ul>
                          {matches.map((match) => (
                            <li key={match.lineNumber}>
                              <button
                                type="button"
                                data-log-match={`${source.filename}:${match.lineNumber}`}
                                className="flex w-full min-w-0 items-baseline gap-3 rounded-sm px-2 py-1 text-left font-mono text-sm hover:bg-selected focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-info"
                                onClick={() => openLogMatch(match)}
                              >
                                <span className="shrink-0 text-muted">{t("runLogLineNumber", { line: match.lineNumber })}</span>
                                <span className="min-w-0 overflow-hidden text-ellipsis whitespace-pre">{match.text}</span>
                              </button>
                            </li>
                          ))}
                        </ul>
                      </div>
                    ))}
                  </div>
                  {logSearch && logSearch.total > logSearch.shown && (
                    <p className="shrink-0 text-sm text-muted">
                      {t("runLogSearchTruncated", { shown: logSearch.shown, total: logSearch.total })}
                    </p>
                  )}
                </>
              )
            )}
          </dialog>
        )}

        <RunChanges
          repoPath={repoPath}
          branch={branch}
          active={tab === "changes"}
          live={live}
        />
        {!loading && tab !== "changes" && (
          <div
            ref={outputPanel}
            key={filename ?? "no-file"}
            role="region"
            aria-label={filename ? t("contentsOfFilename", { filename }) : t("fileContents")}
            tabIndex={0}
            className="min-h-0 flex-1 overflow-auto overscroll-contain rounded-md bg-panel"
          >
            {!output ? (
              <p role="status" className="rounded-md bg-panel p-4 text-muted">
                {tab === "summary" ? t("noSummaryRecordedForThisRunYet")
                  : tab === "walk" ? t("noWalkthroughRecordedForThisRunYet")
                  : t("noLogRecordedForThisStageYet")}
              </p>
            ) : output.content.length === 0 ? (
              <p role="status" className="rounded-md bg-panel p-4 text-muted">{t("thisFileIsEmpty")}</p>
            ) : tab === "walk" ? (
              <WalkthroughMarkdown content={output.content} className="p-5" />
            ) : (
              <div className="py-4 font-mono text-sm leading-[1.8]">
                {runLogLines(output.content).map((line, index) => (
                  <div
                    key={index}
                    data-log-line={index + 1}
                    tabIndex={-1}
                    className={`flex gap-4 px-4 whitespace-pre focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-info ${selectedLine === index + 1 ? "bg-info/10" : ""}`}
                  >
                    <span className="w-8 shrink-0 text-right text-faint select-none" aria-hidden="true">{index + 1}</span>
                    <span>{line}</span>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </section>
    </div>
  );
}
