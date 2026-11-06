import { toUiMessage, messageText, type UiMessage } from "../messages";
import { useTranslation } from "react-i18next";
import { useEffect, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { applyTaskInputs, parseFactoryConfig, withResumeBranch, type FactoryConfig } from "../config";
import { useBranchRuns } from "../hooks/useBranchRuns";
import { stageLabel, statusClass, statusLabel } from "../runStatus";
import { RunTaskInput, type TaskSource } from "../components/RunTaskInput";
import type { DesktopRuns } from "../hooks/useDesktopRuns";
import { RUN_MODE, type Navigate, type NewRunMode } from "../router";
import {
  Crumb,
  Field,
  Input,
  PrimaryButton,
  SectionHeading,
  Tabs,
  Title,
} from "../components/ui";

const MODES = [
  [RUN_MODE.RUN, "run"],
  [RUN_MODE.CONTINUE, "continue"],
  [RUN_MODE.RESUME, "resume"],
] as const;

const RESUMABLE_STATUSES = new Set(["FAILED", "INTERRUPTED"]);

const MODE_DESC: Record<NewRunMode, string> = {
  [RUN_MODE.RUN]: "fullWorkflowImplementReviewClassifyRiskExplain",
  [RUN_MODE.CONTINUE]: "rerunImplementationAndLaterStagesOnTheExistingResumeBranchWorktree",
  [RUN_MODE.RESUME]: "pickUpAnInterruptedRunFromItsLogFolder",
};

interface Props {
  desktopRuns: DesktopRuns;
  repo: string;
  branch: string;
  mode: NewRunMode;
  navigate: Navigate;
  repoPath: string;
}

export function NewRun({ repo, repoPath, branch, mode, navigate, desktopRuns }: Props) {
  const { t } = useTranslation();
  const [source, setSource] = useState<TaskSource>("task");
  const [runBranch, setRunBranch] = useState("");
  const [taskValues, setTaskValues] = useState<Record<TaskSource, string>>({
    task: "", file: "", folder: "", jira: "",
  });
  const [selectedRunName, setSelectedRunName] = useState<string | null>(null);
  const branchRuns = useBranchRuns(repoPath, branch);
  const resumableRuns = branchRuns.entries.filter((entry) => {
    const lastStatus = entry.stages[entry.stages.length - 1]?.status;
    return !entry.error && lastStatus !== undefined && RESUMABLE_STATUSES.has(lastStatus);
  }).reverse();
  const selectedRun = resumableRuns.find((entry) => entry.name === selectedRunName);
  const [loadedConfig, setLoadedConfig] = useState<FactoryConfig | null>(null);
  const [loadingConfig, setLoadingConfig] = useState(true);
  const [configError, setConfigError] = useState<UiMessage | null>(null);
  const [saveError, setSaveError] = useState<UiMessage | null>(null);

  useEffect(() => {
    let active = true;

    setLoadingConfig(true);
    setConfigError(null);
    setLoadedConfig(null);
    setSaveError(null);

    invoke<unknown>("load_factory_config", { path: repoPath })
      .then((value) => {
        if (!active) return;

        const config = value === null ? null : parseFactoryConfig(value);
        setLoadedConfig(config);
        setRunBranch(config?.agentSandbox?.branch ?? "");
        setTaskValues({
          task: config?.agentSandbox?.run?.query ?? "",
          file: config?.agentSandbox?.run?.filePrompt ?? "",
          folder: config?.taskFolder ?? "",
          jira: config?.jiraUrl ?? "",
        });
      })
      .catch((error) => {
        if (active) setConfigError(toUiMessage(error));
      })
      .finally(() => {
        if (active) setLoadingConfig(false);
      });

    return () => {
      active = false;
    };
  }, [repoPath]);

  const inputConfig = applyTaskInputs(loadedConfig, {
    ...taskValues,
    ...(mode === RUN_MODE.RUN ? { branch: runBranch } : {}),
  });

  const taskConfig = mode === RUN_MODE.CONTINUE
    ? withResumeBranch(inputConfig, branch)
    : inputConfig;

  const resumeConfig = withResumeBranch(loadedConfig, branch);

  function startRun() {
    if (mode === RUN_MODE.RESUME || loadingConfig || configError || desktopRuns.busy(repoPath)) return;
    setSaveError(null);

    try {
      const targetBranch = mode === RUN_MODE.RUN ? runBranch.trim() || branch : branch;
      const id = desktopRuns.start({
        mode,
        repo, repoPath, branch: targetBranch, config: taskConfig,
        createConfig: loadedConfig === null,
        jiraUrl: source === "jira" ? taskValues.jira.trim() : null,
      });
      navigate({ screen: "run", repo, branch: targetBranch, runId: id });
    } catch (error) {
      setSaveError(toUiMessage(error));
    }
  }

  function resumeRun() {
    if (!selectedRun || !loadedConfig || loadingConfig || configError
      || branchRuns.loading || branchRuns.error || desktopRuns.busy(repoPath)) return;
    setSaveError(null);

    try {
      const id = desktopRuns.resume({
        repo, repoPath, branch, config: resumeConfig, run: selectedRun,
      });
      navigate({ screen: "run", repo, branch, runId: id });
    } catch (error) {
      setSaveError(toUiMessage(error));
    }
  }

  const setMode = (m: NewRunMode) => navigate({ screen: "newRun", repo, branch, mode: m });

  const activeRun = desktopRuns.busy(repoPath);
  const resumeDisabled = !loadedConfig || !selectedRun || branchRuns.loading || !!branchRuns.error;
  const cta = mode === RUN_MODE.RUN ? "startRun" : mode === RUN_MODE.CONTINUE ? "continue" : "resume";

  return (
    <div className="@container flex h-full w-full min-w-0 flex-col gap-8">
      <header className="flex flex-col gap-1">
        <Crumb>
          {repo} / {branch}
        </Crumb>
        <Title>{t("newRun")}</Title>
      </header>

      <div className="grid min-h-0 min-w-0 flex-1 grid-cols-1 items-start gap-8 @min-[1040px]:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)]">
        <div className="flex min-w-0 flex-col gap-8">
          <fieldset className="m-0 min-w-0 border-0 p-0">
            <Tabs label={t("mode")} items={MODES.map(([key, label]) => [key, t(label)] as const)} value={mode} onChange={setMode} />
          </fieldset>
          <p className="-mt-3 text-base text-muted">{t(MODE_DESC[mode])}</p>

          {(loadingConfig || (!configError && !loadedConfig)) && (
            <p role="status" className="text-sm text-muted">
              {loadingConfig
                ? t("loadingConfiguration")
                : mode === RUN_MODE.RESUME
                  ? t("noConfigurationFoundConfigureThisRepositoryBeforeResumingARun")
                  : t("noConfigurationYetSavingWillCreateSoftwareFactoryJson")}
            </p>
          )}

          {configError && (
            <p role="alert" className="text-sm text-bad">{messageText(configError, t)}</p>
          )}

          {mode !== RUN_MODE.RESUME && (
            <div className="flex flex-col gap-3">
              <fieldset disabled={loadingConfig || !!configError} className="m-0 flex min-w-0 flex-col gap-5 border-0 p-0">
                {mode === RUN_MODE.RUN && (
                  <Field label={t("branchName")} hint={t("setsTheSharedBranchForTheWorkflow")}>
                    <Input mono large className="w-full" placeholder="feature/my-task" value={runBranch}
                      onChange={(event) => {
                        setRunBranch(event.target.value);
                        setSaveError(null);
                      }} />
                  </Field>
                )}
                <RunTaskInput
                  source={source}
                  onSourceChange={setSource}
                  values={taskValues}
                  onChange={(source, value) => {
                    setTaskValues((current) => ({ ...current, [source]: value }));
                    setSaveError(null);
                  }}
                />
              </fieldset>
            </div>
          )}

          {mode === RUN_MODE.RESUME && (
            <section aria-labelledby="rs-h" className="flex flex-col">
              <SectionHeading id="rs-h">{t("failedOrInterruptedRuns")}</SectionHeading>
              {branchRuns.loading && (
                <p role="status" className="py-4 text-muted">{t("loadingRuns")}</p>
              )}
              {branchRuns.error && (
                <p role="alert" className="py-4 text-bad">{branchRuns.error}</p>
              )}
              {!branchRuns.loading && !branchRuns.error && resumableRuns.length === 0 && (
                <p className="py-4 text-muted">{t("noFailedOrInterruptedRunsRecordedForThisBranch")}</p>
              )}
              {resumableRuns.map((entry) => {
                const last = entry.stages[entry.stages.length - 1];

                return (
                  <label key={entry.name} className="flex min-h-14 cursor-pointer items-center gap-3 border-b border-line py-3">
                    <input type="radio" name="resume" className="m-0 size-4 accent-ink"
                      checked={selectedRun?.name === entry.name}
                      onChange={() => setSelectedRunName(entry.name)} />
                    <span className="flex min-w-0 grow flex-col">
                      <span className="font-mono text-sm break-all">{entry.name}</span>
                      {entry.error ? (
                        <span className="text-sm text-bad">{entry.error}</span>
                      ) : (
                        <span className="text-sm text-muted">
                          {last ? t("lastRecordedStageStage", { stage: stageLabel(last.stage, t) }) : t("noStageResultsRecorded")}
                        </span>
                      )}
                    </span>
                    {last && <span className={statusClass(last.status)}>{statusLabel(last.status, t)}</span>}
                  </label>
                );
              })}
            </section>
          )}

          {saveError && <p role="alert" className="text-sm whitespace-pre-wrap break-words text-bad">{messageText(saveError, t)}</p>}

          {activeRun && (
            <button type="button" className="self-start text-sm underline" onClick={() => navigate({
              screen: "run", repo, branch: activeRun.branch, runId: activeRun.id,
            })}>
              {t("thisRepositoryHasAnActiveRunOpenActiveRun")}
            </button>
          )}
          <div className="flex border-t border-line pt-4">
            <PrimaryButton
              size="lg"
              className="self-start"
              disabled={!!activeRun || loadingConfig || !!configError || (mode === RUN_MODE.RESUME && resumeDisabled)}
              onClick={mode === RUN_MODE.RESUME ? resumeRun : startRun}
            >
              {t(cta)}
            </PrimaryButton>
          </div>
        </div>

        {!loadingConfig && !configError && (
          <section aria-labelledby="config-preview-h" className="flex h-[70dvh] min-h-[320px] min-w-0 flex-col gap-3 @min-[1040px]:h-full @min-[1040px]:min-h-0">
            <SectionHeading id="config-preview-h" plain>{t("softwareFactoryJsonPreview")}</SectionHeading>
            <pre className="min-h-0 flex-1 overflow-auto rounded-md bg-panel p-4 font-mono text-md whitespace-pre-wrap break-words">
              <code>{JSON.stringify(mode === RUN_MODE.RESUME ? resumeConfig : taskConfig, null, 2)}</code>
            </pre>
          </section>
        )}
      </div>
    </div>
  );
}
