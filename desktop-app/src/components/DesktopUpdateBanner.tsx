import { useTranslation } from "react-i18next";
import type { DesktopUpdater } from "../hooks/useDesktopUpdater";
import { OutlineButton, PrimaryButton } from "./ui";

export function DesktopUpdateBanner({ updater }: {
  updater: DesktopUpdater;
}) {
  const { t } = useTranslation();
  if (!updater.update) return null;

  return (
    <div role="status" className="flex shrink-0 flex-wrap items-center gap-3 border-b border-line px-4 py-3">
      <span className="mr-auto text-sm text-ink">
        {t("desktopUpdateAvailable", { version: updater.update.version })}
      </span>
      <PrimaryButton
        disabled={updater.installing}
        onClick={() => void updater.install()}
      >
        {t(updater.installing ? "updatingDesktop" : "updateAndRestart")}
      </PrimaryButton>
      <OutlineButton disabled={updater.installing} onClick={updater.postpone}>
        {t("later")}
      </OutlineButton>
    </div>
  );
}
