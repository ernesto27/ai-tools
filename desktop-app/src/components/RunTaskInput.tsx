import { useState } from "react";
import { open } from "@tauri-apps/plugin-dialog";
import { Field, Input, OutlineButton, SectionHeading, Textarea, TextTabs } from "./ui";

interface TaskInputProps {
  value: string;
  onChange: (value: string) => void;
}

function TaskTextInput({ value, onChange }: TaskInputProps) {
  return (
    <Field label="Task instructions">
      <Textarea rows={7} placeholder="Describe what you want to implement…" value={value}
        onChange={(event) => onChange(event.target.value)} />
    </Field>
  );
}

function TaskPathInput({ value, onChange, directory, label, placeholder }: TaskInputProps & {
  directory: boolean;
  label: string;
  placeholder: string;
}) {
  const [opening, setOpening] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function browse() {
    if (opening) return;
    setError(null);
    setOpening(true);
    try {
      const path = await open({ directory, multiple: false });
      if (path) onChange(path);
    } catch (error) {
      setError(`Could not open the browser: ${String(error)}`);
    } finally {
      setOpening(false);
    }
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-end gap-2">
        <div className="min-w-0 grow">
          <Field label={label}>
            <Input mono large value={value} placeholder={placeholder}
              onChange={(event) => onChange(event.target.value)} />
          </Field>
        </div>
        <OutlineButton onClick={browse} disabled={opening} aria-label={`Browse for ${label.toLowerCase()}`}>
          {opening ? "Opening…" : "Browse"}
        </OutlineButton>
      </div>
      {error && <p role="alert" className="text-sm text-bad">{error}</p>}
    </div>
  );
}

function TaskFileInput(props: TaskInputProps) {
  return <TaskPathInput {...props} directory={false} label="Task file" placeholder="Path to your task file" />;
}

function TaskFolderInput(props: TaskInputProps) {
  return <TaskPathInput {...props} directory label="Task folder" placeholder="Folder with task and context files" />;
}

function JiraUrlInput({ value, onChange }: TaskInputProps) {
  return (
    <Field label="Jira issue URL">
      <Input type="url" mono large placeholder="https://your-team.atlassian.net/browse/SF-142" value={value}
        onChange={(event) => onChange(event.target.value)} />
    </Field>
  );
}

const TASK_SOURCES = {
  task: { label: "Task", component: TaskTextInput },
  file: { label: "File", component: TaskFileInput },
  folder: { label: "Folder", component: TaskFolderInput },
  jira: { label: "Jira URL", component: JiraUrlInput },
} as const;

export type TaskSource = keyof typeof TASK_SOURCES;

const sourceKeys = Object.keys(TASK_SOURCES) as TaskSource[];
const sourceTabs = sourceKeys.map((key) => [key, TASK_SOURCES[key].label] as const);

export function RunTaskInput({ source, onSourceChange, values, onChange }: {
  source: TaskSource;
  onSourceChange: (source: TaskSource) => void;
  values: Record<TaskSource, string>;
  onChange: (source: TaskSource, value: string) => void;
}) {
  const ActiveInput = TASK_SOURCES[source].component;

  return (
    <section aria-labelledby="task-h" className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <SectionHeading id="task-h" plain>Task</SectionHeading>
        <TextTabs label="Task source" items={sourceTabs} value={source} onChange={onSourceChange} />
      </div>
      <ActiveInput key={source} value={values[source]} onChange={(value) => onChange(source, value)} />
    </section>
  );
}
