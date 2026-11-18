import { memo, useId, useLayoutEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { cx, OutlineButton } from "./ui";

const markdownPlugins = [remarkGfm];

export const WalkthroughMarkdown = memo(function WalkthroughMarkdown({ content, className }: {
  content: string;
  className: string;
}) {
  return (
    <article className={cx("walkthrough-markdown", className)}>
      <Markdown remarkPlugins={markdownPlugins}>{content}</Markdown>
    </article>
  );
});

export function WalkthroughReader({ content }: { content: string }) {
  const { t } = useTranslation();
  const headingId = useId();
  const trigger = useRef<HTMLButtonElement>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const readingArea = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);

  useLayoutEffect(() => {
    if (open) {
      dialog.current?.showModal();
      readingArea.current?.focus();
    }
  }, [open]);

  return (
    <>
      <OutlineButton ref={trigger} aria-haspopup="dialog" onClick={() => setOpen(true)}>
        {t("expandWalkthrough")}
      </OutlineButton>
      {open && (
        <dialog
          ref={dialog}
          aria-labelledby={headingId}
          onClose={() => {
            setOpen(false);
            trigger.current?.focus({ preventScroll: true });
          }}
          className="fixed inset-0 m-auto h-[calc(100%-2rem)] max-h-none w-[calc(100%-2rem)] max-w-none overflow-hidden rounded-lg border border-field-line bg-panel text-ink shadow-2xl backdrop:bg-black/60"
        >
          <div className="flex h-full min-h-0 flex-col">
            <header className="flex shrink-0 items-center justify-between gap-4 border-b border-line px-5 py-3 sm:px-8">
              <h2 id={headingId} className="text-lg font-semibold text-ink-strong">{t("walkthrough")}</h2>
              <OutlineButton onClick={() => dialog.current?.close()}>{t("closeWalkthrough")}</OutlineButton>
            </header>
            <div
              ref={readingArea}
              role="region"
              aria-label={t("fileContents")}
              tabIndex={0}
              className="min-h-0 flex-1 overflow-auto overscroll-contain -outline-offset-1 focus-visible:outline-1 focus-visible:outline-muted"
            >
              <WalkthroughMarkdown content={content} className="mx-auto w-full max-w-[80ch] px-5 py-6 sm:px-8 sm:py-8" />
            </div>
          </div>
        </dialog>
      )}
    </>
  );
}
