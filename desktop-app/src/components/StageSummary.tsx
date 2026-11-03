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
  selectedStage?: string;
  onSelectStage: (stage: string) => void;
}

export function StageSummary({ statuses, logs, live, selectedStage, onSelectStage }: Props) {
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
    <div aria-label={t("stages")} className="grid shrink-0 grid-cols-[repeat(auto-fit,minmax(150px,1fr))] gap-4 border-y border-line py-5">
      {STAGES.map((stage) => {
        const status = statuses.get(stage.id);
        const timing = timings.get(stage.id);
        const recorded = status !== undefined
          || logs.some((log) => log.filename === stage.filename);
        const duration = timing
          ? formatStageDuration((timing.finishedAt ?? now) - timing.startedAt)
          : recorded ? t("durationUnavailable") : null;
        return (
          <button
            key={stage.id}
            type="button"
            disabled={!recorded}
            aria-pressed={selectedStage === stage.id}
            onClick={() => onSelectStage(stage.id)}
            className="flex min-w-0 flex-col gap-0.5 rounded-md px-2.5 py-2 text-left enabled:hover:bg-hover aria-pressed:bg-selected disabled:cursor-default"
          >
            <span className="text-sm text-muted">{t(stage.label)}</span>
            <span className={cx("text-xl font-medium", statusClass(status))}>
              {!status && live ? t("pending") : statusLabel(status, t)}
            </span>
            {duration !== null && (
              <span aria-label={t("stageDuration", { stage: t(stage.label) })} className="text-sm text-muted tabular-nums">
                {duration}
              </span>
            )}
            {live && (status === "RUNNING" || status === "STARTED") && (
              <span aria-hidden="true" className="stage-activity mt-2">
                {Array.from({ length: 9 }, (_, index) => (
                  <span key={index} style={{ animationDelay: `${index * -140}ms` }} />
                ))}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
