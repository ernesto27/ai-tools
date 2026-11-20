import { useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import type { RunLog } from "../hooks/useRunLogs";
import { STAGES, type stageProgress } from "../runProgress";
import { STAGE_STATUS, statusClass, statusLabel } from "../runStatus";
import { formatStageDuration, latestStageTiming } from "../stageTiming";
import { cx } from "./ui";

interface Props {
  statuses: ReturnType<typeof stageProgress>["statuses"];
  logs: RunLog[];
  live: boolean;
  selectedStage?: string;
  onSelectStage: (stage: string) => void;
  renderActions?: (stage: (typeof STAGES)[number]) => ReactNode;
}

export function StageSummary({ statuses, logs, live, selectedStage, onSelectStage, renderActions }: Props) {
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
        const active = live && (status === STAGE_STATUS.RUNNING || status === STAGE_STATUS.STARTED);
        const timing = timings.get(stage.id);
        const recorded = status !== undefined
          || logs.some((log) => log.filename === stage.filename);
        const duration = timing
          ? formatStageDuration((timing.finishedAt ?? now) - timing.startedAt)
          : recorded ? t("durationUnavailable") : null;
        return (
          <div key={stage.id} className="flex min-w-0 flex-col gap-2">
            <button
              type="button"
              disabled={!recorded}
              aria-pressed={selectedStage === stage.id}
              onClick={() => onSelectStage(stage.id)}
              className={cx(
                "grid min-w-0 items-center gap-x-2 gap-y-0.5 rounded-md px-2.5 py-2 text-left enabled:hover:bg-hover aria-pressed:bg-selected disabled:cursor-default",
                active ? "grid-cols-[minmax(0,1fr)_minmax(0,30%)] inset-ring-1 inset-ring-info/25" : "grid-cols-1",
              )}
            >
              <span className="col-span-full text-sm text-muted">{t(stage.label)}</span>
              <span className={cx("col-span-full text-xl font-medium wrap-anywhere", statusClass(status))}>
                {!status && live ? t("pending") : statusLabel(status, t)}
              </span>
              {duration !== null && (
                <span aria-label={t("stageDuration", { stage: t(stage.label) })} className="text-sm text-muted tabular-nums">
                  {duration}
                </span>
              )}
              {active && (
                <span aria-hidden="true" className="stage-activity w-full max-w-22 justify-self-end">
                  {Array.from({ length: 9 }, (_, index) => (
                    <span key={index} style={{ animationDelay: `${index * -140}ms` }} />
                  ))}
                </span>
              )}
            </button>
            {renderActions?.(stage)}
          </div>
        );
      })}
    </div>
  );
}
