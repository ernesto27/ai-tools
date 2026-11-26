import { useId, useLayoutEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { messageText, toUiMessage, type UiMessage } from "../messages";
import { Field, Input, OutlineButton, PrimaryButton } from "./ui";

interface Props {
  title: string;
  initialName: string;
  trigger: HTMLButtonElement;
  onSubmit: (name: string) => Promise<void>;
  onClose: () => void;
}

export function ProfileNameDialog({ title, initialName, trigger, onSubmit, onClose }: Props) {
  const { t } = useTranslation();
  const titleId = useId();
  const nameId = useId();
  const dialog = useRef<HTMLDialogElement>(null);
  const input = useRef<HTMLInputElement>(null);
  const submitting = useRef(false);
  const [name, setName] = useState(initialName);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<UiMessage | null>(null);

  useLayoutEffect(() => {
    dialog.current?.showModal();
    input.current?.focus();
  }, []);

  async function submit() {
    if (submitting.current || !name.trim()) return;
    submitting.current = true;
    setPending(true);
    setError(null);
    try {
      await onSubmit(name.trim());
      dialog.current?.close();
    } catch (failure) {
      setError(toUiMessage(failure));
    } finally {
      submitting.current = false;
      setPending(false);
    }
  }

  return (
    <dialog
      ref={dialog}
      aria-labelledby={titleId}
      onCancel={(event) => { if (submitting.current) event.preventDefault(); }}
      onClose={() => {
        onClose();
        if (trigger.isConnected) trigger.focus({ preventScroll: true });
      }}
      className="fixed inset-0 m-auto w-[calc(100%-2rem)] max-w-lg rounded-lg border border-field-line bg-panel p-5 text-ink shadow-2xl backdrop:bg-black/60"
    >
      <h2 id={titleId} className="text-lg font-semibold text-ink-strong">{title}</h2>
      <form className="mt-4 space-y-4" onSubmit={(event) => { event.preventDefault(); void submit(); }}>
        <Field label={t("factoryProfileName")} htmlFor={nameId}>
          <Input ref={input} id={nameId} required readOnly={pending} value={name}
            onChange={(event) => setName(event.target.value)} />
        </Field>
        {error && <p role="alert" className="text-sm break-words text-bad">{messageText(error, t)}</p>}
        <div className="flex flex-wrap justify-end gap-2">
          <OutlineButton type="button" disabled={pending} onClick={() => dialog.current?.close()}>{t("cancel")}</OutlineButton>
          <PrimaryButton type="submit" disabled={pending || !name.trim()}>{pending ? t("saving") : t("save")}</PrimaryButton>
        </div>
      </form>
    </dialog>
  );
}
