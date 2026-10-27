import { useEffect, useMemo, useState } from "react";
import { AGENTS, CONFIG_STAGES } from "../data";
import { Checkbox, Crumb, Field, Input, OutlineButton, PrimaryButton, SectionHeading, Select, TextTabs, Title } from "../components/ui";
import { open } from "@tauri-apps/plugin-dialog";
import { invoke } from "@tauri-apps/api/core";
import { StatusMessage } from "../components/StatusMessage";
import { configToForm, formToConfig, mergeConfigEdits, parseFactoryConfig } from "../config";
import type { FactoryConfig, StageSettings, TaskSource } from "../config";

const TASK_SOURCES = [
  ["file", "File"],
  ["folder", "Folder"],
  ["drive", "Google Drive"],
] as const;

const TASK_SOURCE_TEXT: Record<TaskSource, { placeholder: string; hint: string }> = {
  file: { placeholder: "Path to the task file", hint: "run.filePrompt" },
  folder: { placeholder: "Folder with task.md and context files", hint: "task.md is the task; other files become documents" },
  drive: { placeholder: "Google Doc URL", hint: "Read with service_account.json" },
};

const FLAGS = [
  { key: "push", hint: "commit & push" },
  { key: "pr", hint: "open a PR" },
  { key: "hn", hint: "host network" },
] as const;

type FlagKey = (typeof FLAGS)[number]["key"];

const EFFORT_OPTIONS = ["low", "medium", "high", "xhigh"] as const;

const FORM_GRID = "grid grid-cols-1 gap-4 pt-5 pb-8 @min-[400px]:grid-cols-2 @min-[800px]:grid-cols-4";

interface Props {
  repo: string;
  repoPath: string;
}

export function Config({ repo, repoPath }: Props) {
  const [agent, setAgent] = useState("codex");
  const [model, setModel] = useState("");
  const [effort, setEffort] = useState("medium");
  const [branch, setBranch] = useState("factory-task");
  const [baseImage, setBaseImage] = useState("");
  const [stageSettings, setStageSettings] = useState<Record<string, StageSettings>>(() =>
    Object.fromEntries(CONFIG_STAGES.map((s) => [s.key, { agent: s.agent, model: "", effort: "medium", customSkill: s.skill }])),
  );
  const [flags, setFlags] = useState<Record<FlagKey, boolean>>({ push: false, pr: false, hn: false });
  const [off, setOff] = useState<Record<string, boolean>>({});
  const [saveFindings, setSaveFindings] = useState(false);
  const [taskSrc, setTaskSrc] = useState<TaskSource>("file");
  const [taskPickerError, setTaskPickerError] = useState<string | null>(null);
  const [taskPickerOpen, setTaskPickerOpen] = useState(false);
  const [taskVals, setTaskVals] = useState<Record<TaskSource, string>>({
    file: "task.txt",
    folder: "tasks/logging",
    drive: "https://docs.google.com/document/d/[DOC_ID]",
  });
  const [documents, setDocuments] = useState<string[]>([]);
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saveMessage, setSaveMessage] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [loadAttempt, setLoadAttempt] = useState(0);
  const [loadedConfig, setLoadedConfig] = useState<FactoryConfig | null>(null);
  const [initialConfig, setInitialConfig] = useState<FactoryConfig | null>(null);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setLoadError(null);
    setSaveError(null);
    setSaveMessage(null);

    invoke<unknown>("load_factory_config", { path: repoPath })
      .then((value) => {
        if (!active) return;
        const config = value === null ? null : parseFactoryConfig(value);
        const form = configToForm(config);
        setAgent(form.agent);
        setModel(form.model);
        setEffort(form.effort);
        setBranch(form.branch);
        setBaseImage(form.baseImage);
        setStageSettings(form.stageSettings);
        setFlags(form.flags);
        setOff(form.off);
        setSaveFindings(form.saveFindings);
        setTaskSrc(form.taskSrc);
        setTaskVals(form.taskVals);
        setDocuments(form.documents);
        setLoadedConfig(config);
        setInitialConfig(formToConfig(form, config));
        setSaved(config !== null);
        setDirty(false);
      })
      .catch((error) => {
        if (active) setLoadError(String(error));
      })
      .finally(() => {
        if (active) setLoading(false);
      });

    return () => { active = false; };
  }, [repoPath, loadAttempt]);

  // Every edit marks the form dirty until Save is pressed.
  const edit = <T,>(fn: (v: T) => void) => (v: T) => {
    fn(v);
    setDirty(true);
    setSaveMessage(null);
  };

  const editStage = (key: string, field: keyof StageSettings, value: string) => {
    setStageSettings((current) => ({ ...current, [key]: { ...current[key], [field]: value } }));
    setDirty(true);
    setSaveMessage(null);
  };

  async function addDocument() {
    const path = await open({
      directory: false,
      multiple: false,
    });

    if (!path) return;

    setDocuments((current) =>
      current.includes(path) ? current : [...current, path],
    );
    setDirty(true);
    setSaveMessage(null);
  }

  async function browseTask() {
    const source = taskSrc;
    if (source === "drive" || taskPickerOpen) return;

    setTaskPickerError(null);
    setTaskPickerOpen(true);
    try {
      const path = await open({
        directory: source === "folder",
        multiple: false,
      });
      if (!path) return;

      setTaskVals((current) => ({ ...current, [source]: path }));
      setDirty(true);
      setSaveMessage(null);
    } catch (error) {
      setTaskPickerError(String(error));
    } finally {
      setTaskPickerOpen(false);
    }
  }

  const formConfig = useMemo(() => formToConfig({ agent, model, effort, branch, baseImage, stageSettings, flags, off, saveFindings, taskSrc, taskVals, documents }, loadedConfig),
    [agent, model, effort, branch, baseImage, stageSettings, flags, off, saveFindings, taskSrc, taskVals, documents, loadedConfig]);
  const config = loadedConfig && initialConfig ? mergeConfigEdits(loadedConfig, initialConfig, formConfig) : formConfig;
  const json = JSON.stringify(config, null, 2);

  async function saveConfig() {
    if (saving || loading || loadError) return;
    setSaving(true);
    setSaveError(null);
    setSaveMessage(null);

    try {
      await invoke(loadedConfig ? "update_factory_config" : "create_factory_config", {
        path: repoPath,
        config,
      });
      setLoadedConfig(config);
      setInitialConfig(formConfig);
      setSaved(true);
      setDirty(false);
      setSaveMessage(`software-factory.json saved in ${repoPath}`);
    } catch (error) {
      setSaveError(String(error));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="config-form flex w-full min-w-0 flex-col gap-9">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1">
          <Crumb>{repo} · {repoPath}</Crumb>
          <Title>Config</Title>
        </div>
        <div className="flex items-center gap-4 text-md">
          <span role="status" className="text-muted">{loading ? "Loading…" : loadError ? "Could not load" : saving ? "Saving…" : dirty ? "Unsaved changes" : saved ? "Saved" : "Not saved"}</span>
          <PrimaryButton disabled={saving || loading || !!loadError} onClick={saveConfig}>{saving ? "Saving…" : "Save"}</PrimaryButton>
        </div>
      </header>
      {saveMessage && <StatusMessage message={saveMessage} />}
      {saveError && <p role="alert" className="text-sm text-bad">{saveError}</p>}
      {loadError && (
        <div role="alert" className="flex flex-wrap items-center gap-3 text-sm text-bad">
          <span>{loadError}</span>
          <OutlineButton onClick={() => setLoadAttempt((attempt) => attempt + 1)}>Retry</OutlineButton>
        </div>
      )}

      <div className="flex flex-wrap items-start gap-8">
        <fieldset disabled={saving || loading || !!loadError} className="@container m-0 flex min-w-0 flex-[1.2_1_520px] flex-col border-0 p-0">
          <SectionHeading>Agent defaults</SectionHeading>
          <div className={FORM_GRID}>
            <Field label="Agent">
              <Select value={agent} onChange={(e) => edit(setAgent)(e.target.value)}>
                {!AGENTS.includes(agent) && <option value={agent}>{agent || "Agent default"}</option>}
                {AGENTS.map((a) => (
                  <option key={a} value={a}>
                    {a}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="Model">
              <Input value={model} placeholder="agent default" onChange={(e) => edit(setModel)(e.target.value)} />
            </Field>
            <Field label="Effort">
              <Select value={effort} onChange={(e) => edit(setEffort)(e.target.value)}>
                {!EFFORT_OPTIONS.some((value) => value === effort) && <option value={effort}>{effort}</option>}
                {EFFORT_OPTIONS.map((value) => (
                  <option key={value} value={value}>{value}</option>
                ))}
              </Select>
            </Field>
            <Field label="Base image">
              <Input mono value={baseImage} placeholder="optional" onChange={(e) => edit(setBaseImage)(e.target.value)} />
            </Field>
          </div>

          <SectionHeading>Branch &amp; task</SectionHeading>
          <div className="grid grid-cols-1 items-start gap-5 pt-5 pb-8 @min-[600px]:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
            <Field label="Branch">
              <Input mono value={branch} onChange={(e) => edit(setBranch)(e.target.value)} />
            </Field>
            <div className="flex flex-col gap-1.5 text-sm text-muted">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span id="task-lbl">Task source</span>
                <TextTabs label="Task source" items={TASK_SOURCES} value={taskSrc} onChange={(source) => {
                  edit(setTaskSrc)(source);
                  setTaskPickerError(null);
                }} size="sm" />
              </div>
              <div className="flex gap-2">
                <Input
                  mono
                  className="min-w-0 flex-1"
                  aria-labelledby="task-lbl"
                  readOnly={taskSrc !== "drive"}
                  value={taskVals[taskSrc]}
                  placeholder={TASK_SOURCE_TEXT[taskSrc].placeholder}
                  onChange={(e) => edit(setTaskVals)({ ...taskVals, [taskSrc]: e.target.value })}
                />
                {taskSrc !== "drive" && (
                  <OutlineButton className="shrink-0" disabled={taskPickerOpen} onClick={browseTask}>
                    Browse
                  </OutlineButton>
                )}
              </div>
              <span className="text-xs">{TASK_SOURCE_TEXT[taskSrc].hint}</span>
              {taskPickerError && <span role="alert" className="text-sm text-bad">{taskPickerError}</span>}
            </div>
          </div>

          <SectionHeading>Git</SectionHeading>
          <div className="flex flex-wrap gap-x-8 gap-y-2 pt-4 pb-8 text-base">
            {FLAGS.map((f) => (
              <Checkbox key={f.key} checked={flags[f.key]} onChange={() => edit(setFlags)({ ...flags, [f.key]: !flags[f.key] })}>
                <span className="font-mono">{f.key}</span>
                <span className="text-sm text-muted">{f.hint}</span>
              </Checkbox>
            ))}
          </div>

          <SectionHeading>Stages</SectionHeading>
          <p className="pt-3 pb-5 text-sm text-muted">Set overrides for each stage. Empty agent and model values use the defaults above.</p>
          <div className="flex flex-col gap-4 pb-8">
            {CONFIG_STAGES.map((s) => (
              <div key={s.key} className={`flex flex-col gap-4 rounded-lg border border-line p-5 ${off[s.key] ? "bg-canvas" : "bg-field"}`}>
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <h3 className="text-base font-medium">{s.label}</h3>
                  <div className="flex flex-wrap items-center gap-4">
                    {s.key === "securityReview" && (
                      <Checkbox tight className="text-sm" disabled={!!off[s.key]} checked={saveFindings} onChange={() => edit(setSaveFindings)(!saveFindings)}>
                        Save findings
                      </Checkbox>
                    )}
                    <Checkbox
                      tight
                      className="text-sm"
                      aria-label={`Disable ${s.label}`}
                      checked={!!off[s.key]}
                      onChange={() => edit(setOff)({ ...off, [s.key]: !off[s.key] })}
                    >
                      Disabled
                    </Checkbox>
                  </div>
                </div>
                <fieldset disabled={!!off[s.key]} aria-label={`${s.label} settings`} className="m-0 grid min-w-0 grid-cols-1 gap-4 border-0 p-0 disabled:opacity-50 @min-[400px]:grid-cols-2 @min-[800px]:grid-cols-[minmax(0,1fr)_minmax(0,1.3fr)_minmax(0,0.8fr)_minmax(0,1.2fr)]">
                  <Field label="Agent">
                    <Select className="min-w-0 w-full" value={stageSettings[s.key].agent} onChange={(e) => editStage(s.key, "agent", e.target.value)}>
                      <option value="">Use shared agent</option>
                      {stageSettings[s.key].agent && !AGENTS.includes(stageSettings[s.key].agent) && <option value={stageSettings[s.key].agent}>{stageSettings[s.key].agent}</option>}
                      {AGENTS.map((a) => <option key={a} value={a}>{a}</option>)}
                    </Select>
                  </Field>
                  <Field label="Model">
                    <Input className="min-w-0 w-full" value={stageSettings[s.key].model} onChange={(e) => editStage(s.key, "model", e.target.value)} />
                  </Field>
                  <Field label="Effort">
                    <Select className="min-w-0 w-full" value={stageSettings[s.key].effort} onChange={(e) => editStage(s.key, "effort", e.target.value)}>
                      {!EFFORT_OPTIONS.some((value) => value === stageSettings[s.key].effort) && <option value={stageSettings[s.key].effort}>{stageSettings[s.key].effort}</option>}
                      {EFFORT_OPTIONS.map((value) => (
                        <option key={value} value={value}>{value}</option>
                      ))}
                    </Select>
                  </Field>
                  {s.key !== "implementation" && (
                    <Field label="Custom skill">
                      <Input mono className="min-w-0 w-full" value={stageSettings[s.key].customSkill} onChange={(e) => editStage(s.key, "customSkill", e.target.value)} />
                    </Field>
                  )}
                </fieldset>
              </div>
            ))}
          </div>

          <SectionHeading>Documents</SectionHeading>
          <div className="flex flex-col font-mono text-md">
            {documents.map((doc) => (
              <div key={doc} className="flex min-h-11 items-center justify-between border-b border-line">
                {doc}
                <button
                  type="button"
                  aria-label={`Remove ${doc}`}
                  className="size-9 text-xl text-muted hover:text-ink"
                  onClick={() => edit(setDocuments)(documents.filter((d) => d !== doc))}
                >
                  ×
                </button>
              </div>
            ))}
            {documents.length === 0 && <p className="py-3 font-sans text-sm text-muted">No documents added.</p>}
            <button
              type="button"
              onClick={addDocument}
              className="min-h-10 self-start font-sans text-md underline-offset-4 hover:underline"
            >
              + Add document
            </button>
          </div>
        </fieldset>

        <aside aria-labelledby="json-h" className="sticky top-0 flex min-w-0 flex-[1_1_440px] flex-col gap-2.5">
          <h2 id="json-h" className="font-mono text-sm font-normal text-muted">
            software-factory.json
          </h2>
          <pre className="overflow-x-auto rounded-md bg-panel p-4 font-mono text-sm leading-[1.7] whitespace-pre">{loading ? "Loading configuration…" : loadError ? "Configuration unavailable." : json}</pre>
        </aside>
      </div>
    </div>
  );
}
