import { useId, useLayoutEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { AppMessageError, messageText, type UiMessage } from "../messages";
import { OutlineButton } from "./ui";

interface Props {
  name: string;
  title: string;
  description: string;
  pendingLabel: string;
  trigger: HTMLButtonElement;
  onDelete: () => Promise<void>;
  forceDelete?: {
    label: string;
    warning: string;
    readError: (error: unknown) => string | undefined;
    onDelete: () => Promise<void>;
  };
  onClose: () => void;
}

export function DeleteConfirmationDialog({ name, title, description, pendingLabel, trigger, onDelete, forceDelete, onClose }: Props) {
  const { t } = useTranslation();
  const titleId = useId();
  const descriptionId = useId();
  const dialog = useRef<HTMLDialogElement>(null);
  const cancel = useRef<HTMLButtonElement>(null);
  const deleting = useRef(false);
  const [pending, setPending] = useState(false);
  const [failure, setFailure] = useState<{ message: UiMessage; canForce: boolean } | null>(null);
  const forceAction = failure?.canForce ? forceDelete : undefined;
  const canForce = Boolean(forceAction);

  useLayoutEffect(() => {
    dialog.current?.showModal();
    cancel.current?.focus();
  }, []);

  useLayoutEffect(() => {
    if (canForce) cancel.current?.focus();
  }, [canForce]);

  async function confirm(deleteAction: () => Promise<void>) {
    if (deleting.current) return;
    deleting.current = true;
    setPending(true);
    setFailure(null);
    try {
      await deleteAction();
      dialog.current?.close();
    } catch (error) {
      const dirtyError = deleteAction === onDelete ? forceDelete?.readError(error) : undefined;
      setFailure({
        message: error instanceof AppMessageError ? error.descriptor : {
          key: "couldNotDeleteNameError",
          values: { name, error: dirtyError ?? String(error) },
        },
        canForce: dirtyError !== undefined,
      });
    } finally {
      deleting.current = false;
      setPending(false);
    }
  }

  return (
    <dialog
      ref={dialog}
      aria-labelledby={titleId}
      aria-describedby={descriptionId}
      onCancel={(event) => { if (deleting.current) event.preventDefault(); }}
      onClose={() => {
        onClose();
        if (trigger.isConnected) trigger.focus({ preventScroll: true });
      }}
      className="fixed inset-0 m-auto w-[calc(100%-2rem)] max-w-lg rounded-lg border border-field-line bg-panel p-5 text-ink shadow-2xl backdrop:bg-black/60"
    >
      <h2 id={titleId} className="text-lg font-semibold text-ink-strong">{title}</h2>
      <p id={descriptionId} className="mt-2 break-words text-sm text-muted">
        {description}
      </p>
      {failure && (
        <div role="alert" className="mt-3 whitespace-pre-wrap break-words text-sm text-bad">
          <p>{messageText(failure.message, t)}</p>
          {forceAction && <p className="mt-2 font-medium">{forceAction.warning}</p>}
        </div>
      )}
      <div className="mt-5 flex flex-wrap justify-end gap-2">
        <OutlineButton ref={cancel} disabled={pending} onClick={() => dialog.current?.close()}>
          {t("cancel")}
        </OutlineButton>
        <OutlineButton disabled={pending} className="text-bad!" onClick={() => void confirm(forceAction?.onDelete ?? onDelete)}>
          {pending ? pendingLabel : forceAction?.label ?? title}
        </OutlineButton>
      </div>
    </dialog>
  );
}
