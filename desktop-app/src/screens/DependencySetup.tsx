import { useTranslation } from "react-i18next";
import { OutlineButton, PrimaryButton, Title, cx } from "../components/ui";
import type { Dependency, DependencyStatus } from "../hooks/useDependencySetup";

const statusKeys: Record<DependencyStatus, string> = {
  checking: "dependencyChecking",
  missing: "dependencyMissing",
  declined: "dependencyNotInstalled",
  installing: "dependencyInstalling",
  ready: "dependencyReady",
  error: "dependencyError",
};

interface Props {
  dependencies: readonly Dependency[];
  needsConfirmation: boolean;
  onAccept: () => void;
  onDecline: () => void;
  canRecheck: boolean;
  onRecheck: () => void;
}

export function DependencySetup({ dependencies, needsConfirmation, onAccept, onDecline, canRecheck, onRecheck }: Props) {
  const { t } = useTranslation();

  return (
    <main
      data-testid="dependency-setup"
      className="flex min-h-dvh items-center justify-center bg-canvas p-6 text-ink"
      aria-labelledby="dependency-setup-title"
    >
      <section className="w-full max-w-lg rounded-md border border-line bg-panel p-6">
        <div id="dependency-setup-title">
          <Title>{t("dependencySetupTitle")}</Title>
        </div>
        <p className="mt-3 text-sm text-muted">{t("dependencySetupDescription")}</p>
        <ul className="mt-6 space-y-4" aria-live="polite" aria-atomic="false">
          {dependencies.map((dependency) => {
            const busy = dependency.status === "checking" || dependency.status === "installing";
            return (
              <li key={dependency.id} data-testid={`dependency-${dependency.id}`}>
                <div className="flex items-center justify-between gap-4">
                  <span className="font-mono text-sm">{dependency.id}</span>
                  <span className="inline-flex items-center gap-2 text-sm">
                    <span
                      aria-hidden="true"
                      className={cx(
                        "size-2 shrink-0 rounded-full",
                        busy && "bg-info motion-safe:animate-pulse",
                        dependency.status === "ready" && "bg-ok",
                        (dependency.status === "missing" || dependency.status === "declined") && "bg-warn",
                        dependency.status === "error" && "bg-bad",
                      )}
                    />
                    {t(statusKeys[dependency.status])}
                  </span>
                </div>
                {dependency.status === "error" && (
                  <p role="alert" className="mt-2 whitespace-pre-wrap break-words text-sm text-bad">
                    {dependency.error}
                  </p>
                )}
              </li>
            );
          })}
        </ul>
        {needsConfirmation && (
          <div className="mt-6 border-t border-line pt-4">
            <p className="text-sm text-muted">{t("dependencyInstallConfirmation")}</p>
            <div className="mt-4 flex flex-wrap gap-3">
              <PrimaryButton onClick={onAccept}>{t("dependencyAcceptInstall")}</PrimaryButton>
              <OutlineButton onClick={onDecline}>{t("dependencyDeclineInstall")}</OutlineButton>
            </div>
          </div>
        )}
        {dependencies.some((dependency) => dependency.status === "declined") && (
          <p role="status" className="mt-6 text-sm text-muted">{t("dependencyInstallationDeclined")}</p>
        )}
        {dependencies.some((dependency) => dependency.status === "error" || dependency.status === "declined") && (
          <OutlineButton className="mt-4" disabled={!canRecheck} onClick={onRecheck}>
            {t("dependencyCheckAgain")}
          </OutlineButton>
        )}
      </section>
    </main>
  );
}
