import { invoke } from "@tauri-apps/api/core";
import { open } from "@tauri-apps/plugin-dialog";
import { useCallback, useEffect, useId, useLayoutEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { DeleteConfirmationDialog } from "../components/DeleteConfirmationDialog";
import { Icon } from "../components/Icon";
import { Crumb, Field, Input, OutlineButton, PrimaryButton, Select, TextButton, Title } from "../components/ui";
import { AppMessageError } from "../messages";

interface ScheduledTask {
  name: string;
  filePrompt: string;
  cron?: string;
}

type Frequency = "keep" | "daily" | "weekdays" | "weekly" | "custom";
type Preset = Exclude<Frequency, "keep" | "custom">;
type LoadState = "loading" | "ready" | "error";
const optionOrder: Frequency[] = ["keep", "daily", "weekdays", "weekly", "custom"];
const days = [1, 2, 3, 4, 5, 6, 0];
const hours = timeOptions(24);
const minutes = timeOptions(60);

function timeOptions(count: number) {
  return Array.from({ length: count }, (_, index) => String(index).padStart(2, "0"));
}

function dayLabel(day: number, locale: string): string {
  return new Intl.DateTimeFormat(locale, { weekday: "long", timeZone: "UTC" })
    .format(new Date(Date.UTC(2026, 0, 4 + day)));
}

function preset(cron: string | undefined): { frequency: Preset; time: string; day: number } | null {
  const match = cron?.match(/^(\d{1,2}) (\d{1,2}) \* \* (\*|1-5|[0-7])$/);
  if (!match || Number(match[1]) > 59 || Number(match[2]) > 23) return null;
  const [, minute, hour, weekday] = match;
  return {
    frequency: weekday === "*" ? "daily" : weekday === "1-5" ? "weekdays" : "weekly",
    time: hour.padStart(2, "0") + ":" + minute.padStart(2, "0"),
    day: weekday === "*" || weekday === "1-5" ? 1 : Number(weekday) % 7,
  };
}

interface Selection {
  task: ScheduledTask;
  trigger: HTMLButtonElement;
}

export function Schedules({ repo, repoPath }: { repo: string; repoPath: string }) {
  const { t, i18n } = useTranslation();
  const [tasks, setTasks] = useState<ScheduledTask[]>([]);
  const [load, setLoad] = useState<LoadState>("loading");
  const [listError, setListError] = useState("");
  const [status, setStatus] = useState<"scheduleSaved" | "scheduleDeleted" | null>(null);
  const [editor, setEditor] = useState<{ task?: ScheduledTask; trigger: HTMLButtonElement } | null>(null);
  const [deletion, setDeletion] = useState<Selection | null>(null);
  const request = useRef(0);
  const newButton = useRef<HTMLButtonElement>(null);
  const locale = i18n.resolvedLanguage ?? i18n.language;

  const reload = useCallback(async () => {
    const id = ++request.current;
    setLoad("loading");
    setListError("");
    try {
      const rows = await invoke<ScheduledTask[]>("scheduled_list_factory_runs", { path: repoPath });
      if (id !== request.current) return;
      setTasks(rows);
      setLoad("ready");
    } catch (error) {
      if (id !== request.current) return;
      setListError(String(error));
      setLoad("error");
    }
  }, [repoPath]);

  useEffect(() => {
    void reload();
    return () => { request.current++; };
  }, [reload]);

  async function write(command: string, payload: { name: string; file?: string; cron?: string }) {
    const id = request.current;
    setStatus(null);
    try {
      await invoke<void>(command, { ...payload, path: repoPath });
    } finally {
      if (id === request.current) await reload();
    }
  }

  function timing(task: ScheduledTask) {
    if (!task.cron) return t("scheduleTimingUnavailable");
    const value = preset(task.cron);
    if (!value) return <code>{task.cron}</code>;
    const [hour, minute] = value.time.split(":").map(Number);
    const time = new Intl.DateTimeFormat(locale, {
      hour: "numeric", minute: "2-digit", timeZone: "UTC",
    }).format(new Date(Date.UTC(2000, 0, 1, hour, minute)));
    const frequency = value.frequency === "weekly"
      ? dayLabel(value.day, locale)
      : t("scheduleFrequency." + value.frequency);
    return t("scheduleRunsAt", { frequency, time });
  }

  return (
    <section className="mx-auto flex max-w-[1080px] flex-col gap-4">
      <header>
        <Crumb>{repo} / {t("schedules")}</Crumb>
        <div className="mt-2 flex flex-wrap items-center justify-between gap-3">
          <Title>{t("schedules")}</Title>
          <PrimaryButton ref={newButton} disabled={load !== "ready"} onClick={(event) => {
            setStatus(null);
            setEditor({ trigger: event.currentTarget });
          }}>
            <Icon name="plus" width="18" height="18" />
            {t("scheduleNew")}
          </PrimaryButton>
        </div>
        <p className="mt-3 whitespace-pre-line text-sm text-muted">{t("scheduleDescription")}</p>
      </header>
      {load === "loading" && <p role="status">{t("loading")}</p>}
      {load === "error" && (
        <div className="flex flex-wrap items-center gap-2">
          <p role="alert" className="break-words text-bad">{t("scheduleListError", { error: listError })}</p>
          <OutlineButton onClick={() => void reload()}>{t("retry")}</OutlineButton>
        </div>
      )}
      {load === "ready" && tasks.length === 0 && <p>{t("scheduleEmpty")}</p>}
      {tasks.length > 0 && (
        <>
          <div className="overflow-hidden rounded-lg border border-line">
            <div aria-hidden="true" className="hidden grid-cols-[minmax(0,1fr)_minmax(0,1fr)_8rem] gap-3 border-b border-line bg-panel px-4 py-3 text-sm font-medium text-muted sm:grid">
              <span>{t("scheduleNameAndPrompt")}</span>
              <span>{t("scheduleTiming")}</span>
            </div>
            <ul aria-label={t("schedules")} className="divide-y divide-line">
              {tasks.map((task) => (
                <li key={task.name} className="grid items-center gap-3 p-4 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_8rem]">
                  <div className="min-w-0 wrap-anywhere">
                    <strong className="block text-ink-strong">{task.name}</strong>
                    <code className="text-sm text-muted">{task.filePrompt}</code>
                  </div>
                  <div className="wrap-anywhere">{timing(task)}</div>
                  <div className="flex flex-wrap gap-3">
                    <TextButton disabled={load !== "ready"} aria-label={t("scheduleEditName", { name: task.name })} onClick={(event) => {
                      setStatus(null);
                      setEditor({ task, trigger: event.currentTarget });
                    }}>{t("scheduleEdit")}</TextButton>
                    <TextButton disabled={load !== "ready"} tone="danger" aria-label={t("scheduleDeleteName", { name: task.name })} onClick={(event) => {
                      setStatus(null);
                      setDeletion({ task, trigger: event.currentTarget });
                    }}>{t("delete")}</TextButton>
                  </div>
                </li>
              ))}
            </ul>
          </div>
          <p className="text-sm text-muted">{t("scheduleTimezone")}</p>
        </>
      )}
      {status && <p role="status">{t(status)}</p>}
      {editor && (
        <ScheduleEditor
          task={editor.task}
          repoPath={repoPath}
          names={tasks.map(({ name }) => name)}
          canWrite={load === "ready"}
          onRetry={() => void reload()}
          load={load}
          onClose={() => {
            setEditor(null);
            (editor.trigger.isConnected ? editor.trigger : newButton.current)?.focus();
          }}
          onSave={async (name, file, cron) => {
            await write(editor.task ? "scheduled_edit_factory_run" : "scheduled_create_factory_run", {
              name, file, ...(cron === undefined ? {} : { cron }),
            });
            setStatus("scheduleSaved");
          }}
        />
      )}
      {deletion && (
        <DeleteConfirmationDialog
          name={deletion.task.name}
          title={t("scheduleDelete")}
          description={t("scheduleConfirmDelete", { name: deletion.task.name })}
          pendingLabel={t("scheduleDeleting")}
          trigger={deletion.trigger}
          onDelete={async () => {
            if (load !== "ready") {
              throw new AppMessageError({ key: "scheduleRetryBeforeWrite" });
            }
            await write("scheduled_delete_factory_run", { name: deletion.task.name });
            setStatus("scheduleDeleted");
          }}
          onClose={() => {
            setDeletion(null);
            if (!deletion.trigger.isConnected) newButton.current?.focus();
          }}
        />
      )}
    </section>
  );
}

interface EditorProps {
  task?: ScheduledTask;
  repoPath: string;
  names: string[];
  canWrite: boolean;
  load: LoadState;
  onRetry: () => void;
  onClose: () => void;
  onSave: (name: string, file: string, cron?: string) => Promise<void>;
}

function ScheduleEditor({ task, repoPath, names, canWrite, load, onRetry, onClose, onSave }: EditorProps) {
  const { t, i18n } = useTranslation();
  const dialog = useRef<HTMLDialogElement>(null);
  const active = useRef(false);
  const titleId = useId();
  const fileId = useId();
  const value = preset(task?.cron);
  const [name, setName] = useState(task?.name ?? "");
  const [file, setFile] = useState(task?.filePrompt ?? "");
  const [frequency, setFrequency] = useState<Frequency>(
    value?.frequency ?? (task ? (task.cron ? "custom" : "keep") : "weekdays"),
  );
  const [day, setDay] = useState(value?.day ?? 1);
  const [time, setTime] = useState(value?.time ?? "09:00");
  const [hour, minute] = time.split(":");
  const [expression, setExpression] = useState(value ? "" : task?.cron ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const locale = i18n.resolvedLanguage ?? i18n.language;

  useLayoutEffect(() => {
    dialog.current?.showModal();
  }, []);

  async function browse() {
    if (active.current) return;
    active.current = true;
    setBusy(true);
    setError("");
    try {
      const path = await open({
        title: t("schedulePromptFile"), directory: false, multiple: false, defaultPath: repoPath,
      });
      if (path) setFile(path);
    } catch (cause) {
      setError(t("scheduleBrowseError", { error: String(cause) }));
    } finally {
      active.current = false;
      setBusy(false);
    }
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (active.current || !canWrite) return;
    const trimmedName = name.trim();
    if (!trimmedName) return setError(t("scheduleNameRequired"));
    if (!task && names.includes(trimmedName)) return setError(t("scheduleNameDuplicate"));
    if (!file) return setError(t("scheduleFileRequired"));
    let cron: string | undefined;
    if (frequency === "custom") {
      if (/[\r\n]/.test(expression) || expression.trim().split(/\s+/).length !== 5) {
        return setError(t("scheduleCronInvalid"));
      }
      cron = expression.trim().split(/\s+/).join(" ");
    } else if (frequency !== "keep") {
      const [hour, minute] = time.split(":").map(Number);
      const weekday = frequency === "daily" ? "*" : frequency === "weekdays" ? "1-5" : day;
      cron = [minute, hour, "*", "*", weekday].join(" ");
    }

    active.current = true;
    setBusy(true);
    setError("");
    try {
      const samePreset = value && value.frequency === frequency && value.time === time
        && (frequency !== "weekly" || value.day === day);
      const sameCron = cron === task?.cron;
      await onSave(task?.name ?? trimmedName, file, samePreset || sameCron ? undefined : cron);
      dialog.current?.close();
    } catch (cause) {
      setError(t("scheduleWriteError", { error: String(cause) }));
    } finally {
      active.current = false;
      setBusy(false);
    }
  }

  return (
    <dialog ref={dialog} aria-labelledby={titleId} onClose={onClose}
      onCancel={(event) => { if (active.current) event.preventDefault(); }}
      className="fixed inset-0 m-auto max-h-[calc(100%-2rem)] w-[calc(100%-2rem)] max-w-lg overflow-auto rounded-lg border border-field-line bg-panel p-5 text-ink shadow-2xl backdrop:bg-black/60"
    >
      <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
        <h2 id={titleId} className="text-lg font-semibold">{t(task ? "scheduleEditTitle" : "scheduleNew")}</h2>
        <Field label={t("scheduleName")}><Input value={name} readOnly={Boolean(task)} disabled={busy} onChange={(event) => setName(event.target.value)} /></Field>
        <div className="flex flex-col gap-1.5">
          <span id={fileId} className="text-sm text-muted">{t("schedulePromptFile")}</span>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <output aria-labelledby={fileId} className="min-w-0 wrap-anywhere text-sm">{file || t("scheduleNoFile")}</output>
            <OutlineButton disabled={busy} onClick={() => void browse()}>{t("browse")}</OutlineButton>
          </div>
        </div>
        <Field label={t("scheduleFrequencyLabel")}>
          <Select name="scheduleFrequency" value={frequency} disabled={busy} onChange={(event) => setFrequency(event.target.value as Frequency)}>
            {optionOrder.filter((option) => option !== "keep" || (task && !task.cron)).map((option) => (
              <option key={option} value={option}>{t("scheduleFrequency." + option)}</option>
            ))}
          </Select>
        </Field>
        {frequency === "weekly" && (
          <Field label={t("scheduleDay")}>
            <Select value={day} disabled={busy} onChange={(event) => setDay(Number(event.target.value))}>
              {days.map((value) => <option key={value} value={value}>{dayLabel(value, locale)}</option>)}
            </Select>
          </Field>
        )}
        {(frequency === "daily" || frequency === "weekdays" || frequency === "weekly") && (
          <fieldset className="grid grid-cols-2 gap-3">
            <legend className="mb-1.5 text-sm text-muted">{t("scheduleTime")}</legend>
            <Field label={t("scheduleHour")}>
              <Select name="scheduleHour" value={hour} disabled={busy}
                onChange={(event) => setTime(`${event.target.value}:${minute}`)}>
                {hours.map((value) => <option key={value} value={value}>{value}</option>)}
              </Select>
            </Field>
            <Field label={t("scheduleMinute")}>
              <Select name="scheduleMinute" value={minute} disabled={busy}
                onChange={(event) => setTime(`${hour}:${event.target.value}`)}>
                {minutes.map((value) => <option key={value} value={value}>{value}</option>)}
              </Select>
            </Field>
          </fieldset>
        )}
        {frequency === "custom" && (
          <Field label={t("scheduleCron")} hint={t("scheduleCronHint")}>
            <Input mono value={expression} disabled={busy} onChange={(event) => setExpression(event.target.value)} />
          </Field>
        )}
        {error && <p role="alert" className="whitespace-pre-wrap wrap-anywhere text-bad">{error}</p>}
        <footer className="flex flex-wrap justify-end gap-2">
          {load === "error" && <OutlineButton disabled={busy} onClick={onRetry}>{t("retry")}</OutlineButton>}
          <OutlineButton disabled={busy} onClick={() => dialog.current?.close()}>{t("cancel")}</OutlineButton>
          <PrimaryButton type="submit" disabled={busy || !canWrite}>{t(task ? "scheduleSave" : "scheduleCreate")}</PrimaryButton>
        </footer>
      </form>
    </dialog>
  );
}
