import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import type { RunLog } from "../hooks/useRunLogs";
import { STAGES, type stageProgress } from "../runProgress";
import { statusClass, statusLabel } from "../runStatus";
import { formatStageDuration, latestStageTiming } from "../stageTiming";
import { cx } from "./ui";

interface Props {
  statuses: ReturnType<typeof stageProgress>["statuses"];
  logs: RunLog[];
  live: boolean;
}

export function StageSummary({ statuses, logs, live }: Props) {
  const { t } = useTranslation();
  const timings = useMemo(() => new Map(STAGES.map((stage) => {
    const log = logs.find((entry) => entry.filename === stage.filename);
    return [stage.id, log ? latestStageTiming(log.content) : null] as const;
  })), [logs]);
  const ticking = Array.from(timings.values()).some((timing) =>
    timing !== null && timing.finishedAt === null);
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    setNow(Date.now());
    if (!ticking) return;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [ticking]);

  return (
    <dl className="grid shrink-0 grid-cols-[repeat(auto-fit,minmax(150px,1fr))] gap-4 border-y border-line py-5">
      {STAGES.map((stage) => {
        const status = statuses.get(stage.id);
        const timing = timings.get(stage.id);
        const recorded = status !== undefined
          || logs.some((log) => log.filename === stage.filename);
        const duration = timing
          ? formatStageDuration((timing.finishedAt ?? now) - timing.startedAt)
          : recorded ? t("durationUnavailable") : null;
        return (
          <div key={stage.id} className="flex flex-col gap-0.5">
            <dt className="text-sm text-muted">{t(stage.label)}</dt>
            <dd className={cx("text-xl font-medium", statusClass(status))}>
              {!status && live ? t("pending") : statusLabel(status, t)}
            </dd>
            {duration !== null && (
              <dd aria-label={t("stageDuration", { stage: t(stage.label) })} className="text-sm text-muted tabular-nums">
                {duration}
              </dd>
            )}
          </div>
        );
      })}
    </dl>
  );
}
