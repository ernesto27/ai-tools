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

export function statusLabel(status: string | undefined): string {
  if (!status) return "Not recorded";
  return status.charAt(0) + status.slice(1).toLowerCase().replace(/_/g, " ");
}
