import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { formatStageDuration } from "../stageTiming";

interface Props {
  startedAt: number;
  finishedAt: number | null;
}

export function AuditDuration({ startedAt, finishedAt }: Props) {
  const { t } = useTranslation();
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (finishedAt !== null) return;
    setNow(Date.now());
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [startedAt, finishedAt]);

  return (
    <span aria-label={t("auditDuration")} className="text-sm text-muted tabular-nums">
      {formatStageDuration((finishedAt ?? now) - startedAt)}
    </span>
  );
}
