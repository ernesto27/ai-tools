export interface StageTiming {
  startedAt: number;
  finishedAt: number | null;
}

const SEPARATOR = "==================================================";

function timestamp(value: string | undefined): number | null {
  if (!value || !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/.test(value)) {
    return null;
  }
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : null;
}

export function latestStageTiming(content: string): StageTiming | null {
  const text = content.replace(/\r\n/g, "\n");
  // The execution header also has a Started field. Only use stage metadata.
  const heading = new RegExp(`^${SEPARATOR}\\nSTAGE [1-5]: [A-Z ]+\\n${SEPARATOR}\\n`, "gm");
  let attempt: string | null = null;
  let match: RegExpExecArray | null;
  while ((match = heading.exec(text)) !== null) {
    attempt = text.slice(match.index + match[0].length);
  }
  if (attempt === null) return null;

  const startedAt = timestamp(/^Started: ([^\n]*)(?:\n|$)/.exec(attempt)?.[1]);
  if (startedAt === null) return null;
  const result = /^--- RESULT ---\nStatus: [A-Z_]+\nFinished: ([^\n]*)$/m.exec(attempt);
  const finishedAt = timestamp(result?.[1]);
  if (finishedAt !== null && finishedAt < startedAt) return null;
  return { startedAt, finishedAt };
}

export function formatStageDuration(milliseconds: number): string {
  const total = Math.floor(Math.max(0, milliseconds) / 1000);
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = total % 60;
  const paddedSeconds = String(seconds).padStart(2, "0");
  if (hours > 0) {
    return `${hours}h ${String(minutes).padStart(2, "0")}m ${paddedSeconds}s`;
  }
  return minutes > 0 ? `${minutes}m ${paddedSeconds}s` : `${seconds}s`;
}
