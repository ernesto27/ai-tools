import { messageText, type UiMessage } from "../messages";
import { useTranslation } from "react-i18next";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { open } from "@tauri-apps/plugin-dialog";
import { Icon } from "./Icon";
import { Field, Input, OutlineButton, SectionHeading, Textarea, TextTabs } from "./ui";

interface TaskInputProps {
  value: string;
  onChange: (value: string) => void;
}

function TaskTextInput({ value, onChange }: TaskInputProps) {
  const { t } = useTranslation();
  return (
    <Field label={t("taskInstructions")}>
      <Textarea rows={7} placeholder={t("describeWhatYouWantToImplement")} value={value}
        onChange={(event) => onChange(event.target.value)} />
    </Field>
  );
}

function TaskPathInput({ value, onChange, directory, label, placeholder, children, inputRef }: TaskInputProps & {
  directory: boolean;
  label: string;
  placeholder: string;
  children?: ReactNode;
  inputRef?: (node: HTMLInputElement | null) => void;
}) {
  const { t } = useTranslation();
  const [opening, setOpening] = useState(false);
  const [error, setError] = useState<UiMessage | null>(null);
  const latestChange = useRef(onChange);
  const mounted = useRef(false);

  useEffect(() => { latestChange.current = onChange; }, [onChange]);
  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; };
  }, []);

  async function browse() {
    if (opening) return;
    setError(null);
    setOpening(true);
    try {
      const path = await open({ directory, multiple: false });
      if (path && mounted.current) latestChange.current(path);
    } catch (error) {
      setError({ key: "couldNotOpenTheBrowserError", values: { error: String(error) } });
    } finally {
      setOpening(false);
    }
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-end gap-2">
        <div className="min-w-0 grow">
          <Field label={label}>
            <Input ref={inputRef} mono large className="w-full min-w-0" value={value} placeholder={placeholder}
              onChange={(event) => onChange(event.target.value)} />
          </Field>
        </div>
        <OutlineButton onClick={browse} disabled={opening} aria-label={t("browseForLabel", { label })}>
          {opening ? t("opening") : t("browse")}
        </OutlineButton>
        {children}
      </div>
      {error && <p role="alert" className="text-sm text-bad">{messageText(error, t)}</p>}
    </div>
  );
}

export interface TaskFile {
  id: string;
  path: string;
}

export function createTaskFiles(paths: string[]): TaskFile[] {
  return (paths.length ? paths : [""]).map((path) => ({ id: crypto.randomUUID(), path }));
}

export interface TaskValues {
  task: string;
  file: TaskFile[];
  folder: string;
  jira: string;
}

function TaskFileInput({ value, onChange }: {
  value: TaskFile[];
  onChange: (value: TaskFile[]) => void;
}) {
  const { t } = useTranslation();
  const inputs = useRef(new Map<string, HTMLInputElement>());
  const focusId = useRef<string | null>(null);

  useEffect(() => {
    if (focusId.current) {
      inputs.current.get(focusId.current)?.focus();
      focusId.current = null;
    }
  }, [value]);

  function addFile() {
    const file = createTaskFiles([""])[0];
    focusId.current = file.id;
    onChange([...value, file]);
  }

  function removeFile(id: string) {
    const index = value.findIndex((file) => file.id === id);
    const remaining = value.filter((file) => file.id !== id);
    const next = remaining.length ? remaining : createTaskFiles([""]);
    focusId.current = next[Math.min(index, next.length - 1)].id;
    onChange(next);
  }

  return (
    <div className="flex min-w-0 flex-col gap-3">
      {value.map((file, index) => (
        <TaskPathInput key={file.id} value={file.path}
          onChange={(path) => onChange(value.map((entry) => entry.id === file.id ? { ...entry, path } : entry))}
          directory={false} label={t("taskFileNumber", { number: index + 1 })}
          placeholder={t("pathToYourTaskFile")}
          inputRef={(node) => {
            if (node) inputs.current.set(file.id, node);
            else inputs.current.delete(file.id);
          }}>
          <OutlineButton className="flex shrink-0 items-center justify-center px-2.5"
            onClick={() => removeFile(file.id)} aria-label={t("removeTaskFile", { number: index + 1 })}
            title={t("removeTaskFile", { number: index + 1 })}>
            <Icon name="remove" />
          </OutlineButton>
        </TaskPathInput>
      ))}
      <OutlineButton className="flex items-center gap-2 self-start" onClick={addFile}>
        <Icon name="plus" />{t("addTaskFile")}
      </OutlineButton>
    </div>
  );
}

function TaskFolderInput(props: TaskInputProps) {
  const { t } = useTranslation();
  return <TaskPathInput {...props} directory label={t("taskFolder")} placeholder={t("folderWithTaskAndContextFiles")} />;
}

function JiraUrlInput({ value, onChange }: TaskInputProps) {
  const { t } = useTranslation();
  return (
    <Field label={t("jiraIssueUrl")}>
      <Input type="url" mono large placeholder="https://your-team.atlassian.net/browse/SF-142" value={value}
        onChange={(event) => onChange(event.target.value)} />
    </Field>
  );
}

export type TaskSource = keyof TaskValues;
type TextSource = Exclude<TaskSource, "file">;

const SOURCE_LABELS: Record<TaskSource, string> = {
  task: "task",
  file: "file",
  folder: "folder",
  jira: "jiraUrl",
};

const sourceKeys = Object.keys(SOURCE_LABELS) as TaskSource[];

const TEXT_INPUTS = {
  task: TaskTextInput,
  folder: TaskFolderInput,
  jira: JiraUrlInput,
} as const;

function TextSourceInput({ source, ...props }: TaskInputProps & { source: TextSource }) {
  const ActiveInput = TEXT_INPUTS[source];
  return <ActiveInput {...props} />;
}

export function RunTaskInput({ source, onSourceChange, values, onChange }: {
  source: TaskSource;
  onSourceChange: (source: TaskSource) => void;
  values: TaskValues;
  onChange: <Source extends TaskSource>(source: Source, value: TaskValues[Source]) => void;
}) {
  const { t } = useTranslation();
  const sourceTabs = sourceKeys.map((key) => [key, t(SOURCE_LABELS[key])] as const);

  return (
    <section aria-labelledby="task-h" className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <SectionHeading id="task-h" plain>{t("task")}</SectionHeading>
        <TextTabs label={t("taskSource")} items={sourceTabs} value={source} onChange={onSourceChange} />
      </div>
      {source === "file"
        ? <TaskFileInput value={values.file} onChange={(value) => onChange("file", value)} />
        : <TextSourceInput key={source} source={source} value={values[source]} onChange={(value) => onChange(source, value)} />}
    </section>
  );
}
