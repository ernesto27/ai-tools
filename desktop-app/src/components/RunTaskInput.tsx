import { messageText, type UiMessage } from "../messages";
import { useTranslation } from "react-i18next";
import { useState } from "react";
import { open } from "@tauri-apps/plugin-dialog";
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

function TaskPathInput({ value, onChange, directory, label, placeholder }: TaskInputProps & {
  directory: boolean;
  label: string;
  placeholder: string;
}) {
  const { t } = useTranslation();
  const [opening, setOpening] = useState(false);
  const [error, setError] = useState<UiMessage | null>(null);

  async function browse() {
    if (opening) return;
    setError(null);
    setOpening(true);
    try {
      const path = await open({ directory, multiple: false });
      if (path) onChange(path);
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
            <Input mono large value={value} placeholder={placeholder}
              onChange={(event) => onChange(event.target.value)} />
          </Field>
        </div>
        <OutlineButton onClick={browse} disabled={opening} aria-label={t("browseForLabel", { label })}>
          {opening ? t("opening") : t("browse")}
        </OutlineButton>
      </div>
      {error && <p role="alert" className="text-sm text-bad">{messageText(error, t)}</p>}
    </div>
  );
}

function TaskFileInput(props: TaskInputProps) {
  const { t } = useTranslation();
  return <TaskPathInput {...props} directory={false} label={t("taskFile")} placeholder={t("pathToYourTaskFile")} />;
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

const TASK_SOURCES = {
  task: { label: "task", component: TaskTextInput },
  file: { label: "file", component: TaskFileInput },
  folder: { label: "folder", component: TaskFolderInput },
  jira: { label: "jiraUrl", component: JiraUrlInput },
} as const;

export type TaskSource = keyof typeof TASK_SOURCES;

const sourceKeys = Object.keys(TASK_SOURCES) as TaskSource[];

export function RunTaskInput({ source, onSourceChange, values, onChange }: {
  source: TaskSource;
  onSourceChange: (source: TaskSource) => void;
  values: Record<TaskSource, string>;
  onChange: (source: TaskSource, value: string) => void;
}) {
  const { t } = useTranslation();
  const sourceTabs = sourceKeys.map((key) => [key, t(TASK_SOURCES[key].label)] as const);
  const ActiveInput = TASK_SOURCES[source].component;

  return (
    <section aria-labelledby="task-h" className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <SectionHeading id="task-h" plain>{t("task")}</SectionHeading>
        <TextTabs label={t("taskSource")} items={sourceTabs} value={source} onChange={onSourceChange} />
      </div>
      <ActiveInput key={source} value={values[source]} onChange={(value) => onChange(source, value)} />
    </section>
  );
}
