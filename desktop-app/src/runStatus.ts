import type { TFunction } from "i18next";

const STATUS_CLASSES: Record<string, string> = {
  SUCCEEDED: "text-ok",
  FAILED: "text-bad",
  RUNNING: "text-info",
  STARTED: "text-info",
  INTERRUPTED: "text-warn",
  UNFINISHED: "text-warn",
  SKIPPED: "text-muted",
};

export function statusClass(status: string | undefined): string {
  return STATUS_CLASSES[status ?? ""] ?? "text-muted";
}

const STATUS_LABELS: Record<string, string> = {
  SUCCEEDED: "Succeeded", FAILED: "Failed", RUNNING: "Running", STARTED: "Started",
  INTERRUPTED: "Interrupted", UNFINISHED: "Unfinished", SKIPPED: "Skipped",
};
const STAGE_LABELS: Record<string, string> = {
  implementation: "Implementation", codeReview: "Code review",
  securityReview: "Security review", riskClassification: "Risk classification",
  reviewChanges: "Review changes",
};

export function statusLabel(status: string | undefined, t: TFunction, uppercaseEnglish = false): string {
  if (!status) return t("Not recorded");
  const label = Object.prototype.hasOwnProperty.call(STATUS_LABELS, status) ? STATUS_LABELS[status] : undefined;
  if (!label) return status;
  return uppercaseEnglish ? t(status) : t(label);
}

export function stageLabel(stage: string, t: TFunction): string {
  const label = Object.prototype.hasOwnProperty.call(STAGE_LABELS, stage) ? STAGE_LABELS[stage] : undefined;
  return label ? t(label) : stage;
}
