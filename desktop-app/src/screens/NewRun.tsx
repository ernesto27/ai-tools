import { useState } from "react";
import { AGENTS, DEFAULT_TASK_TEXT, folderFiles, jsonSettings, resumableRuns, STATUS_COLOR } from "../data";
import type { Navigate, NewRunMode } from "../router";
import {
  Checkbox,
  Crumb,
  Field,
  Input,
  LinkButton,
  OutlineButton,
  PrimaryButton,
  SectionHeading,
  Select,
  Tabs,
  Textarea,
  TextTabs,
  Title,
} from "../components/ui";

const MODES = [
  ["run", "Run"],
  ["continue", "Continue"],
  ["resume", "Resume"],
] as const;

const MODE_DESC: Record<NewRunMode, string> = {
  run: "Full workflow on run.branch: implement, review, classify risk, explain.",
  continue: "Rerun implementation and later stages on the existing resume.branch worktree.",
  resume: "Pick up an interrupted run from its log folder.",
};

type Source = "local" | "folder" | "jira";
const SOURCES = [
  ["local", "Local file"],
  ["folder", "Folder"],
  ["jira", "Jira"],
] as const;

type SettingsMode = "json" | "custom";
const SETTINGS_MODES = [
  ["json", "From software-factory.json"],
  ["custom", "Custom for this run"],
] as const;

const STAGES = ["Implement", "Code review", "Security", "Risk", "Walkthrough"];
const JIRA_URL = "https://your-team.atlassian.net/browse/SF-142";

interface Props {
  repo: string;
  branch: string;
  mode: NewRunMode;
  navigate: Navigate;
}

export function NewRun({ repo, branch, mode, navigate }: Props) {
  const [src, setSrc] = useState<Source>("local");
  const [res, setRes] = useState(0);
  const [settings, setSettings] = useState<SettingsMode>("json");

  const setMode = (m: NewRunMode) => navigate({ screen: "newRun", repo, branch, mode: m });

  let command: string, runBranch: string, cta: string, stagesLabel: string;
  if (mode === "resume") {
    const r = resumableRuns[res];
    command = `software-factory resume ${r.folder}`;
    runBranch = r.branch;
    cta = "Resume";
    stagesLabel = r.next;
  } else {
    command = "software-factory" + (mode === "continue" ? " continue" : "") + (src === "jira" ? ` --jira ${JIRA_URL}` : "");
    runBranch = mode === "continue" ? branch : "factory-task";
    cta = mode === "continue" ? "Continue" : "Start run";
    stagesLabel = "All 5";
  }

  return (
    <div className="flex max-w-[720px] flex-col gap-8">
      <header className="flex flex-col gap-1">
        <Crumb>
          {repo} / {branch}
        </Crumb>
        <Title>New run</Title>
      </header>

      <Tabs label="Mode" items={MODES} value={mode} onChange={setMode} />
      <p className="-mt-3 text-base text-muted">{MODE_DESC[mode]}</p>

      {mode !== "resume" && (
        <section aria-labelledby="task-h" className="flex flex-col gap-2.5">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <SectionHeading id="task-h" plain>
              Task
            </SectionHeading>
            <TextTabs label="Task source" items={SOURCES} value={src} onChange={setSrc} />
          </div>

          {src === "local" && (
            <>
              <label className="sr-only" htmlFor="task-text">
                Task text (task.txt)
              </label>
              <Textarea id="task-text" rows={7} defaultValue={DEFAULT_TASK_TEXT} />
              <span className="font-mono text-xs text-muted">task.txt · saved before the run starts</span>
            </>
          )}

          {src === "jira" && (
            <>
              <label className="sr-only" htmlFor="jira-url">
                Jira issue URL
              </label>
              <Input id="jira-url" type="url" mono large defaultValue={JIRA_URL} />
              <span className="text-sm text-muted">[Issue title loads here] · passed as --jira</span>
            </>
          )}

          {src === "folder" && (
            <>
              <div className="flex gap-2">
                <label className="sr-only" htmlFor="task-folder">
                  Task folder
                </label>
                <Input
                  id="task-folder"
                  mono
                  large
                  className="min-w-0 grow"
                  defaultValue="tasks/logging"
                  placeholder="folder with task and context files"
                />
                <OutlineButton>Browse</OutlineButton>
              </div>
              <div className="flex flex-col">
                {folderFiles.map((f) => (
                  <div key={f.name} className="flex min-h-10 items-center justify-between gap-3 border-b border-line">
                    <span className="font-mono text-md">{f.name}</span>
                    <span className="text-sm text-muted">{f.role}</span>
                  </div>
                ))}
              </div>
              <span className="text-sm text-muted">task.md becomes run.filePrompt · other files are added as documents</span>
            </>
          )}
        </section>
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

      <section aria-labelledby="set-h" className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <SectionHeading id="set-h" plain>
            Settings
          </SectionHeading>
          <TextTabs label="Settings source" items={SETTINGS_MODES} value={settings} onChange={setSettings} />
        </div>

        {settings === "json" && (
          <>
            <dl className="grid grid-cols-[repeat(auto-fit,minmax(150px,1fr))] gap-x-6 gap-y-3 rounded-md bg-panel px-4 py-3.5 text-md">
              {jsonSettings.map((v) => (
                <div key={v.k} className="flex flex-col gap-0.5">
                  <dt className="font-mono text-xs text-muted">{v.k}</dt>
                  <dd className="font-mono">{v.v}</dd>
                </div>
              ))}
            </dl>
            <span className="text-sm text-muted">
              Read from software-factory.json ·{" "}
              <LinkButton muted onClick={() => navigate({ screen: "config", repo, branch })}>
                Edit in Config
              </LinkButton>
            </span>
          </>
        )}

        {settings === "custom" && (
          <>
            <div className="grid grid-cols-[repeat(auto-fit,minmax(180px,1fr))] gap-4">
              <Field label="Agent">
                <Select defaultValue="codex">
                  {AGENTS.map((a) => (
                    <option key={a}>{a}</option>
                  ))}
                </Select>
              </Field>
              <Field label="Model">
                <Input placeholder="agent default" />
              </Field>
            </div>
            <div className="flex flex-wrap gap-x-6 gap-y-1 text-base">
              {["push", "pr", "saveFindings"].map((k) => (
                <Checkbox key={k} tight>
                  {k}
                </Checkbox>
              ))}
            </div>
            <div className="flex flex-wrap gap-x-6 gap-y-1 text-base">
              <span className="w-full text-sm text-muted">Stages</span>
              {STAGES.map((s) => (
                <Checkbox key={s} tight defaultChecked>
                  {s}
                </Checkbox>
              ))}
            </div>
            <Checkbox tight className="text-base">
              Also save these to software-factory.json
            </Checkbox>
          </>
        )}
      </section>

      <section aria-label="Summary" className="flex flex-col gap-3.5 border-t border-line pt-2">
        <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-1.5 text-base">
          <dt className="text-muted">Branch</dt>
          <dd className="font-mono text-md">{runBranch}</dd>
          <dt className="text-muted">Stages</dt>
          <dd>{stagesLabel}</dd>
          <dt className="text-muted">Checks</dt>
          <dd className="text-ok">All passed</dd>
        </dl>
        <code className="rounded-md bg-panel px-3.5 py-3 font-mono text-md break-all">$ {command}</code>
        <PrimaryButton size="lg" className="self-start">
          {cta}
        </PrimaryButton>
      </section>
    </div>
  );
}
