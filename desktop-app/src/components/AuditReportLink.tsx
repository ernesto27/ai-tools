import { useState } from "react";
import { useTranslation } from "react-i18next";
import { openPath } from "@tauri-apps/plugin-opener";
import { LinkButton } from "./ui";

export function AuditReportLink({ path }: { path: string }) {
  const { t } = useTranslation();
  const [error, setError] = useState<string | null>(null);
  const filename = path.split(/[\\/]/).pop() ?? path;

  async function openReport() {
    setError(null);
    try {
      await openPath(path);
    } catch (error) {
      setError(String(error));
    }
  }

  return (
    <div className="w-full min-w-0 text-xs">
      <div role="status">
        <LinkButton
          title={path}
          aria-label={t("auditReportSaved", { path: filename })}
          className="max-w-full break-all text-left text-info focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-info"
          onClick={() => void openReport()}
        >
          {filename}
        </LinkButton>
      </div>
      {error && (
        <p role="alert" className="mt-2 break-words text-bad">
          {t("couldNotOpenAuditReportError", { error })}
        </p>
      )}
    </div>
  );
}
