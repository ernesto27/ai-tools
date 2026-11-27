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

interface RunLogSearchResult {
  groups: RunLogMatchGroup[];
  total: number;
  shown: number;
}

// Keeps the result list cheap to re-render while logs refresh every second.
const RUN_LOG_MATCH_LIMIT = 200;

export function runLogLines(content: string) {
  return content.split(/\r?\n/);
}

export function searchRunLogs(logs: RunLog[], query: string): RunLogSearchResult {
  const needle = query.trim().toLowerCase();
  const groups: RunLogMatchGroup[] = [];
  let total = 0;
  if (!needle) return { groups, total, shown: 0 };

  const contentByFilename = new Map(logs.map((log) => [log.filename, log.content]));

  for (const source of SOURCES) {
    const content = contentByFilename.get(source.filename);
    if (content === undefined) continue;

    const matches: RunLogMatch[] = [];
    for (const [index, line] of runLogLines(content).entries()) {
      if (!line.toLowerCase().includes(needle)) continue;
      total += 1;
      if (total <= RUN_LOG_MATCH_LIMIT) matches.push({ source, lineNumber: index + 1, text: line });
    }
    if (matches.length > 0) groups.push({ source, matches });
  }

  return { groups, total, shown: Math.min(total, RUN_LOG_MATCH_LIMIT) };
}
