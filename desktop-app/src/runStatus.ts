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
  SUCCEEDED: "succeeded", FAILED: "failed", RUNNING: "running", STARTED: "started",
  INTERRUPTED: "interrupted", UNFINISHED: "unfinished", SKIPPED: "skipped",
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
