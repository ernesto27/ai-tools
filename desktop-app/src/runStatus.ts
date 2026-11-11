import type { TFunction } from "i18next";

export const STAGE_STATUS = {
  SUCCEEDED: "SUCCEEDED",
  FAILED: "FAILED",
  RUNNING: "RUNNING",
  STARTED: "STARTED",
  INTERRUPTED: "INTERRUPTED",
  UNFINISHED: "UNFINISHED",
  SKIPPED: "SKIPPED",
} as const;

const STATUS_CLASSES: Record<string, string> = {
  [STAGE_STATUS.SUCCEEDED]: "text-ok",
  [STAGE_STATUS.FAILED]: "text-bad",
  [STAGE_STATUS.RUNNING]: "text-info",
  [STAGE_STATUS.STARTED]: "text-info",
  [STAGE_STATUS.INTERRUPTED]: "text-warn",
  [STAGE_STATUS.UNFINISHED]: "text-warn",
  [STAGE_STATUS.SKIPPED]: "text-muted",
};

export function statusClass(status: string | undefined): string {
  return STATUS_CLASSES[status ?? ""] ?? "text-muted";
}

const STATUS_LABELS: Record<string, string> = {
  [STAGE_STATUS.SUCCEEDED]: "succeeded",
  [STAGE_STATUS.FAILED]: "failed",
  [STAGE_STATUS.RUNNING]: "running",
  [STAGE_STATUS.STARTED]: "started",
  [STAGE_STATUS.INTERRUPTED]: "interrupted",
  [STAGE_STATUS.UNFINISHED]: "unfinished",
  [STAGE_STATUS.SKIPPED]: "skipped",
};
const STAGE_LABELS: Record<string, string> = {
  implementation: "implementation", codeReview: "codeReview",
  securityReview: "securityReview", riskClassification: "riskClassification",
  reviewChanges: "reviewChanges",
};

export function statusLabel(status: string | undefined, t: TFunction, uppercaseEnglish = false): string {
  if (!status) return t("notRecorded");
  const label = Object.prototype.hasOwnProperty.call(STATUS_LABELS, status) ? STATUS_LABELS[status] : undefined;
  if (!label) return status;
  return t(uppercaseEnglish ? `${label}Uppercase` : label);
}

export function stageLabel(stage: string, t: TFunction): string {
  const label = Object.prototype.hasOwnProperty.call(STAGE_LABELS, stage) ? STAGE_LABELS[stage] : undefined;
  return label ? t(label) : stage;
}
