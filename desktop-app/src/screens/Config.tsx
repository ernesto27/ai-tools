import { useMemo, useState } from "react";
import { AGENTS, CONFIG_STAGES, configDocuments } from "../data";
import { Checkbox, Crumb, Field, Input, PrimaryButton, SectionHeading, Select, TextTabs, Title } from "../components/ui";

type TaskSource = "file" | "folder" | "drive";

const TASK_SOURCES = [
  ["file", "File"],
  ["folder", "Folder"],
  ["drive", "Google Drive"],
] as const;

const TASK_SOURCE_TEXT: Record<TaskSource, { placeholder: string; hint: string }> = {
  file: { placeholder: "Path to the task file, relative to the project", hint: "run.filePrompt" },
  folder: { placeholder: "Folder with task.md and context files", hint: "task.md is the task; other files become documents" },
  drive: { placeholder: "Google Doc URL", hint: "Read with service_account.json" },
};

const FLAGS = [
  { key: "push", hint: "commit & push" },
  { key: "pr", hint: "open a PR" },
  { key: "hn", hint: "host network" },
] as const;

type FlagKey = (typeof FLAGS)[number]["key"];

const FORM_GRID = "grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-4 pt-5 pb-8";

interface Props {
  repo: string;
  branch: string;
}

export function Config({ repo, branch }: Props) {
  const [agent, setAgent] = useState("codex");
  const [flags, setFlags] = useState<Record<FlagKey, boolean>>({ push: false, pr: false, hn: false });
  const [off, setOff] = useState<Record<string, boolean>>({});
  const [saveFindings, setSaveFindings] = useState(false);
  const [taskSrc, setTaskSrc] = useState<TaskSource>("file");
  const [taskVals, setTaskVals] = useState<Record<TaskSource, string>>({
    file: "task.txt",
    folder: "tasks/logging",
    drive: "https://docs.google.com/document/d/[DOC_ID]",
  });
  const [documents, setDocuments] = useState(configDocuments);
  const [dirty, setDirty] = useState(false);

  // Every edit marks the form dirty until Save is pressed.
  const edit = <T,>(fn: (v: T) => void) => (v: T) => {
    fn(v);
    setDirty(true);
  };

  const json = useMemo(() => {
    const disabled = CONFIG_STAGES.map((s) => s.key).filter((k) => off[k]);
    const cfg: Record<string, unknown> = {
      documents,
      googleDrive: {
        folders: ["Engineering Specs"],
        ...(taskSrc === "drive" ? { files: [taskVals.drive] } : {}),
      },
      agentSandbox: {
        agent,
        ...flags,
        branch: "factory-task",
        run: taskSrc === "file" ? { filePrompt: taskVals.file } : {},
      },
      stages: {
        codeReview: { agent: "claude", customSkill: "go-review" },
        securityReview: { agent: "claude", saveFindings },
      },
    };
    if (taskSrc === "folder") cfg.taskFolder = taskVals.folder;
    if (disabled.length) cfg.disabledStages = disabled;
    return JSON.stringify(cfg, null, 2);
  }, [agent, flags, off, saveFindings, taskSrc, taskVals, documents]);

  return (
    <div className="flex max-w-[1080px] flex-col gap-9">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1">
          <Crumb>
            {repo} / {branch}
          </Crumb>
          <Title>Config</Title>
        </div>
        <div className="flex items-center gap-4 text-md">
          <span className="text-muted">{dirty ? "Unsaved changes" : "Saved"}</span>
          <PrimaryButton onClick={() => setDirty(false)}>Save</PrimaryButton>
        </div>
      </header>

      <div className="flex flex-wrap items-start gap-12">
        <div className="flex min-w-0 flex-[999_1_640px] flex-col">
          <SectionHeading>Agent</SectionHeading>
          <div className={FORM_GRID}>
            <Field label="Agent">
              <Select value={agent} onChange={(e) => edit(setAgent)(e.target.value)}>
                {AGENTS.map((a) => (
                  <option key={a} value={a}>
                    {a}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="Model">
              <Input placeholder="agent default" onChange={() => setDirty(true)} />
            </Field>
            <Field label="Base image">
              <Input mono placeholder="optional" onChange={() => setDirty(true)} />
            </Field>
          </div>

          <SectionHeading>Branch &amp; task</SectionHeading>
          <div className={FORM_GRID}>
            <Field label="branch" hint="agentSandbox.branch · used by run, continue and resume">
              <Input mono defaultValue="factory-task" onChange={() => setDirty(true)} />
            </Field>
            <div className="flex flex-col gap-1.5 text-sm text-muted">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span id="task-lbl">Task</span>
                <TextTabs label="Task source" items={TASK_SOURCES} value={taskSrc} onChange={edit(setTaskSrc)} size="sm" />
              </div>
              <Input
                mono
                aria-labelledby="task-lbl"
                value={taskVals[taskSrc]}
                placeholder={TASK_SOURCE_TEXT[taskSrc].placeholder}
                onChange={(e) => edit(setTaskVals)({ ...taskVals, [taskSrc]: e.target.value })}
              />
              <span className="text-xs">{TASK_SOURCE_TEXT[taskSrc].hint}</span>
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
          <div className="flex flex-col pb-8">
            {CONFIG_STAGES.map((s) => (
              <div key={s.key} className="flex flex-wrap items-center gap-x-4 gap-y-2 border-b border-line py-2.5">
                <Checkbox
                  className="grow basis-stage-label text-base"
                  checked={!off[s.key]}
                  onChange={() => edit(setOff)({ ...off, [s.key]: !off[s.key] })}
                >
                  <span className="font-medium">{s.label}</span>
                </Checkbox>
                <span className="w-stage-agent text-md text-muted">{s.agent || `${agent} (inherited)`}</span>
                <span className="w-stage-skill font-mono text-xs text-muted">{s.skill}</span>
                <span className="w-stage-extra">
                  {s.key === "securityReview" && (
                    <Checkbox tight className="text-sm" checked={saveFindings} onChange={() => edit(setSaveFindings)(!saveFindings)}>
                      saveFindings
                    </Checkbox>
                  )}
                </span>
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
            <div className="flex min-h-11 items-center justify-between border-b border-line">
              Drive · Engineering Specs
              <span className="font-sans text-sm text-muted">service account ✓</span>
            </div>
            <button type="button" className="min-h-10 self-start font-sans text-md underline-offset-4 hover:underline">
              + Add document
            </button>
          </div>
        </div>

        <aside aria-labelledby="json-h" className="sticky top-0 flex min-w-0 flex-[1_1_340px] flex-col gap-2.5">
          <h2 id="json-h" className="font-mono text-sm font-normal text-muted">
            software-factory.json
          </h2>
          <pre className="overflow-x-auto rounded-md bg-panel p-4 font-mono text-sm leading-[1.7] whitespace-pre">{json}</pre>
        </aside>
      </div>
    </div>
  );
}
