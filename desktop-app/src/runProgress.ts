import type { StageStatus } from "./hooks/useBranchRuns";
import type { RunLog } from "./hooks/useRunLogs";

export const STAGES = [
  { id: "implementation", label: "Implement", filename: "01-implementation.log" },
  { id: "codeReview", label: "Code review", filename: "02-codeReview.log" },
  { id: "securityReview", label: "Security review", filename: "03-securityReview.log" },
  { id: "riskClassification", label: "Risk classification", filename: "04-riskClassification.log" },
  { id: "reviewChanges", label: "Review changes", filename: "05-reviewChanges.log" },
] as const;

export function stageProgress(history: StageStatus[], logs: RunLog[], live: boolean, desktop: boolean) {
  const statuses = new Map(history.map((entry) => [entry.stage, entry.status]));
  const recorded = STAGES.filter((stage) => logs.some((log) => log.filename === stage.filename));
  const unfinished = recorded.filter((stage) => !statuses.has(stage.id));

  for (const stage of unfinished) {
    statuses.set(stage.id, desktop ? (live ? "RUNNING" : "INTERRUPTED") : "UNFINISHED");
  }

  const latestUnfinished = unfinished[unfinished.length - 1]?.id;
  const latestRecorded = recorded[recorded.length - 1]?.id;
  return { statuses, followStage: latestUnfinished ?? latestRecorded };
}
