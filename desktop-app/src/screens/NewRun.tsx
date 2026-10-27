import { useEffect, useState } from "react";
import { Channel, invoke } from "@tauri-apps/api/core";
import { applyTaskInputs, parseFactoryConfig, type FactoryConfig } from "../config";
import { resumableRuns, STATUS_COLOR } from "../data";
import { RunTaskInput, type TaskSource } from "../components/RunTaskInput";
import { StatusMessage } from "../components/StatusMessage";
import { DebugOutput } from "../components/DebugOutput";
import { useRunDebug } from "../hooks/useRunDebug";
import type { Navigate, NewRunMode } from "../router";
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
  ["run", "Run"],
  ["continue", "Continue"],
  ["resume", "Resume"],
] as const;

const MODE_DESC: Record<NewRunMode, string> = {
  run: "Full workflow: implement, review, classify risk, explain.",
  continue: "Rerun implementation and later stages on the existing resume.branch worktree.",
  resume: "Pick up an interrupted run from its log folder.",
};

interface Props {
  repo: string;
  branch: string;
  mode: NewRunMode;
  navigate: Navigate;
  repoPath: string;
}

interface RunOutputEvent {
  stream: "stdout" | "stderr";
  data: number[];
}

export function NewRun({ repo, repoPath, branch, mode, navigate }: Props) {
  const [source, setSource] = useState<TaskSource>("task");
  const [runBranch, setRunBranch] = useState("");
  const [taskValues, setTaskValues] = useState<Record<TaskSource, string>>({
    task: "", file: "", folder: "", jira: "",
  });
  const [res, setRes] = useState(0);
  const [loadedConfig, setLoadedConfig] = useState<FactoryConfig | null>(null);
  const [loadingConfig, setLoadingConfig] = useState(true);
  const [configError, setConfigError] = useState<string | null>(null);
  const [savingTask, setSavingTask] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saveMessage, setSaveMessage] = useState<string | null>(null);
  const { output: runOutput, reset: resetDebug, append: appendDebug, flush: flushDebug } = useRunDebug();

  useEffect(() => {
    let active = true;

    setLoadingConfig(true);
    setConfigError(null);
    setLoadedConfig(null);
    setSaveError(null);
    setSaveMessage(null);
    resetDebug();

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
        if (active) setConfigError(String(error));
      })
      .finally(() => {
        if (active) setLoadingConfig(false);
      });

    return () => {
      active = false;
    };
  }, [repoPath, resetDebug]);

  const taskConfig = applyTaskInputs(loadedConfig, {
    ...taskValues,
    ...(mode === "run" ? { branch: runBranch } : {}),
  });

  async function saveTaskInputs() {
    if (savingTask || loadingConfig || configError) return;

    setSavingTask(true);
    setSaveError(null);
    setSaveMessage(null);
    resetDebug("Saving configuration…\n");

    try {
      await invoke(loadedConfig ? "update_factory_config" : "create_factory_config", {
        path: repoPath,
        config: taskConfig,
      });
      setLoadedConfig(taskConfig);
      if (mode === "run") {
        appendDebug("Starting Software Factory…\n");
        const channel = new Channel<RunOutputEvent>();
        const decoders = { stdout: new TextDecoder(), stderr: new TextDecoder() };
        channel.onmessage = ({ stream, data }) => {
          appendDebug(data.length
            ? decoders[stream].decode(new Uint8Array(data), { stream: true })
            : decoders[stream].decode());
        };
        await invoke<void>("start_factory_run", {
          path: repoPath,
          jiraUrl: source === "jira" ? taskValues.jira.trim() : null,
          onOutput: channel,
        });
        appendDebug("\nRun completed.\n");
        setSaveMessage("Run completed.");
      } else {
        appendDebug("Task inputs saved.\n");
        setSaveMessage("Task inputs saved to software-factory.json.");
      }
    } catch (error) {
      appendDebug(`\n${String(error)}\n`);
      setSaveError(String(error));
    } finally {
      flushDebug();
      setSavingTask(false);
    }
  }

  const setMode = (m: NewRunMode) => navigate({ screen: "newRun", repo, branch, mode: m });

  const cta = mode === "run" ? "Start run" : mode === "continue" ? "Continue" : "Resume";

  return (
    <div className="@container flex h-full w-full max-w-[1440px] flex-col gap-8">
      <header className="flex flex-col gap-1">
        <Crumb>
          {repo} / {branch}
        </Crumb>
        <Title>New run</Title>
      </header>

      <div className="grid min-h-0 min-w-0 flex-1 grid-cols-1 items-start gap-8 @min-[1040px]:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)]">
        <div className="flex min-w-0 flex-col gap-8">
          <fieldset disabled={savingTask} className="m-0 min-w-0 border-0 p-0">
            <Tabs label="Mode" items={MODES} value={mode} onChange={setMode} />
          </fieldset>
          <p className="-mt-3 text-base text-muted">{MODE_DESC[mode]}</p>

          {(loadingConfig || (!configError && !loadedConfig)) && (
            <p role="status" className="text-sm text-muted">
              {loadingConfig
                ? "Loading configuration…"
                : "No configuration yet. Saving will create software-factory.json."}
            </p>
          )}

          {configError && (
            <p role="alert" className="text-sm text-bad">{configError}</p>
          )}

          {mode !== "resume" && (
            <div className="flex flex-col gap-3">
              <fieldset disabled={loadingConfig || savingTask || !!configError} className="m-0 flex min-w-0 flex-col gap-5 border-0 p-0">
                {mode === "run" && (
                  <Field label="Branch name" hint="Sets the shared branch for the workflow.">
                    <Input mono large className="w-full" placeholder="feature/my-task" value={runBranch}
                      onChange={(event) => {
                        setRunBranch(event.target.value);
                        setSaveMessage(null);
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
                    setSaveMessage(null);
                    setSaveError(null);
                  }}
                />
              </fieldset>
              {saveMessage && <StatusMessage message={saveMessage} />}
              {saveError && <p role="alert" className="text-sm whitespace-pre-wrap break-words text-bad">{saveError}</p>}
            </div>
          )}

          {mode === "resume" && (
            <section aria-labelledby="rs-h" className="flex flex-col">
              <SectionHeading id="rs-h">Interrupted runs</SectionHeading>
              {resumableRuns.map((r, i) => (
                <label key={r.folder} className="flex min-h-14 cursor-pointer items-center gap-3 border-b border-line">
                  <input type="radio" name="resume" className="m-0 size-4 accent-ink" checked={res === i} onChange={() => setRes(i)} />
                  <span className="flex grow flex-col">
                    <span className="font-medium">
                      {r.branch} · {r.when}
                    </span>
                    <span className="font-mono text-xs text-muted">
                      {r.last} → {r.next}
                    </span>
                  </span>
                  <span className="text-md" style={{ color: STATUS_COLOR[r.status] }}>
                    {r.status}
                  </span>
                </label>
              ))}
            </section>
          )}

          <div className="flex border-t border-line pt-4">
            <PrimaryButton
              size="lg"
              className="self-start"
              disabled={mode !== "resume" && (loadingConfig || savingTask || !!configError)}
              onClick={mode !== "resume" ? saveTaskInputs : undefined}
            >
              {savingTask ? (mode === "run" ? "Running…" : "Saving…") : cta}
            </PrimaryButton>
          </div>
          {runOutput !== null && (
            <DebugOutput output={runOutput} active={savingTask} />
          )}
        </div>

        {!loadingConfig && !configError && (
          <section aria-labelledby="config-preview-h" className="flex h-[70dvh] min-h-[320px] min-w-0 flex-col gap-3 @min-[1040px]:h-full @min-[1040px]:min-h-0">
            <SectionHeading id="config-preview-h" plain>software-factory.json preview</SectionHeading>
            <pre className="min-h-0 flex-1 overflow-auto rounded-md bg-panel p-4 font-mono text-md whitespace-pre-wrap break-words">
              <code>{JSON.stringify(mode === "resume" ? loadedConfig ?? {} : taskConfig, null, 2)}</code>
            </pre>
          </section>
        )}
      </div>
    </div>
  );
}
