import type { RunLog } from "./hooks/useRunLogs";
import { STAGES } from "./runProgress";

const SOURCES = [
  ...STAGES.map((stage) => ({
    tab: "log" as const,
    stage: stage.id,
    filename: stage.filename,
    label: stage.label,
  })),
  { tab: "summary", filename: "summary.log", label: "summary" },
] as const;

type LogSource = typeof SOURCES[number];

export interface RunLogMatch {
  source: LogSource;
  lineNumber: number;
  text: string;
}

interface RunLogMatchGroup {
  source: LogSource;
  matches: RunLogMatch[];
}

export function runLogLines(content: string) {
  return content.split(/\r?\n/);
}

export function searchRunLogs(logs: RunLog[], query: string): RunLogMatchGroup[] {
  const needle = query.trim().toLowerCase();
  if (!needle) return [];

  const contentByFilename = new Map(logs.map((log) => [log.filename, log.content]));
  const groups: RunLogMatchGroup[] = [];

  for (const source of SOURCES) {
    const content = contentByFilename.get(source.filename);
    if (content === undefined) continue;

    const matches: RunLogMatch[] = [];
    for (const [index, line] of runLogLines(content).entries()) {
      if (line.toLowerCase().includes(needle)) {
        matches.push({ source, lineNumber: index + 1, text: line });
      }
    }
    if (matches.length > 0) groups.push({ source, matches });
  }

  return groups;
}
