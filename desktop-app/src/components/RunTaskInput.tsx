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

function TaskPathInput({ value, onChange, directory, label, placeholder, children, inputRef, onSelectImages }: TaskInputProps & {
  directory: boolean;
  label: string;
  placeholder: string;
  children?: ReactNode;
  inputRef?: (node: HTMLInputElement | null) => void;
  onSelectImages?: (paths: string[]) => void;
}) {
  const { t } = useTranslation();
  const [opening, setOpening] = useState(false);
  const [error, setError] = useState<UiMessage | null>(null);
  const latestChange = useRef(onChange);
  const latestImageSelection = useRef(onSelectImages);
  const mounted = useRef(false);

  useEffect(() => {
    latestChange.current = onChange;
    latestImageSelection.current = onSelectImages;
  }, [onChange, onSelectImages]);
  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; };
  }, []);

  async function browse() {
    if (opening) return;
    setError(null);
    setOpening(true);
    try {
      if (onSelectImages) {
        const paths = await open({
          title: t("selectRunImages"),
          directory: false,
          multiple: true,
          filters: [{ name: t("runImages"), extensions: ["png", "jpg", "jpeg", "webp"] }],
        });
        if (paths && mounted.current) latestImageSelection.current?.(paths);
        return;
      }
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
              readOnly={onSelectImages !== undefined}
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

function TaskFileInput({ value, onChange, kind = "task" }: {
  value: TaskFile[];
  onChange: (value: TaskFile[]) => void;
  kind?: "task" | "image";
}) {
  const { t } = useTranslation();
  const isImage = kind === "image";
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

  function setImageFiles(id: string, paths: string[]) {
    const index = value.findIndex((file) => file.id === id);
    if (index === -1) return;

    const existingPaths = new Set(value.filter((file) => file.id !== id).map((file) => file.path));
    const uniquePaths = [...new Set(paths)].filter((path) => !existingPaths.has(path));
    if (!uniquePaths.length) return;

    const [firstPath, ...additionalPaths] = uniquePaths;
    onChange([
      ...value.slice(0, index),
      { id, path: firstPath },
      ...additionalPaths.map((path) => ({ id: crypto.randomUUID(), path })),
      ...value.slice(index + 1),
    ]);
  }

  return (
    <div className="flex min-w-0 flex-col gap-3">
      {value.map((file, index) => (
        <TaskPathInput key={file.id} value={file.path}
          onChange={(path) => onChange(value.map((entry) => entry.id === file.id ? { ...entry, path } : entry))}
          directory={false}
          label={t(isImage ? "runImageNumber" : "taskFileNumber", { number: index + 1 })}
          placeholder={t(isImage ? "pathToYourRunImage" : "pathToYourTaskFile")}
          onSelectImages={isImage ? (paths) => setImageFiles(file.id, paths) : undefined}
          inputRef={(node) => {
            if (node) inputs.current.set(file.id, node);
            else inputs.current.delete(file.id);
          }}>
          <OutlineButton className="flex shrink-0 items-center justify-center px-2.5"
            onClick={() => removeFile(file.id)}
            aria-label={t(isImage ? "removeRunImage" : "removeTaskFile", { number: index + 1 })}
            title={t(isImage ? "removeRunImage" : "removeTaskFile", { number: index + 1 })}>
            <Icon name="remove" />
          </OutlineButton>
        </TaskPathInput>
      ))}
      <OutlineButton className="flex items-center gap-2 self-start" onClick={addFile}>
        <Icon name="plus" />{t(isImage ? "addRunImage" : "addTaskFile")}
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
type TaskView = TaskSource | "images";

const VIEW_LABELS: Record<TaskView, string> = {
  task: "task",
  file: "file",
  folder: "folder",
  images: "runImages",
  jira: "jiraUrl",
};

const viewKeys = Object.keys(VIEW_LABELS) as TaskView[];

const TEXT_INPUTS = {
  task: TaskTextInput,
  folder: TaskFolderInput,
  jira: JiraUrlInput,
} as const;

function TextSourceInput({ source, ...props }: TaskInputProps & { source: TextSource }) {
  const ActiveInput = TEXT_INPUTS[source];
  return <ActiveInput {...props} />;
}

export function RunTaskInput({
  source, onSourceChange, values, onChange, imageFiles, onImageFilesChange,
}: {
  source: TaskSource;
  onSourceChange: (source: TaskSource) => void;
  values: TaskValues;
  onChange: <Source extends TaskSource>(source: Source, value: TaskValues[Source]) => void;
  imageFiles: TaskFile[];
  onImageFilesChange: (files: TaskFile[]) => void;
}) {
  const { t } = useTranslation();
  const [showImages, setShowImages] = useState(false);
  const viewTabs = viewKeys.map((key) => [key, t(VIEW_LABELS[key])] as const);

  function selectView(view: TaskView) {
    setShowImages(view === "images");
    if (view !== "images") onSourceChange(view);
  }

  return (
    <section aria-labelledby="task-h" className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <SectionHeading id="task-h" plain>{t("task")}</SectionHeading>
        <TextTabs label={t("taskSource")} items={viewTabs} value={showImages ? "images" : source}
          onChange={selectView} wrap />
      </div>
      {showImages ? (
        <div role="region" aria-label={t("runImages")} className="flex min-w-0 flex-col gap-2">
          <p className="text-sm text-muted">{t("runImagesHint")}</p>
          <TaskFileInput key="images" kind="image" value={imageFiles} onChange={onImageFilesChange} />
        </div>
      ) : source === "file" ? (
        <TaskFileInput value={values.file} onChange={(value) => onChange("file", value)} />
      ) : (
        <TextSourceInput key={source} source={source} value={values[source]} onChange={(value) => onChange(source, value)} />
      )}
    </section>
  );
}
