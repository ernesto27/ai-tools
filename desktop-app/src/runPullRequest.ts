import type { StageStatus } from "./hooks/useBranchRuns";
import type { RunLog } from "./hooks/useRunLogs";

const PUBLISHED_PR = /^(?:Pull request ready for review|Existing pull request(?: \(draft\))?|PR description updated|No net session changes\. PR description unchanged): (\S+)[\t ]*$/;
const GITHUB_PR = /^https:\/\/github\.com\/([A-Za-z0-9_.-]+)\/([A-Za-z0-9_.-]+)\/pull\/[1-9]\d*$/;

export function implementationPullRequestUrl(history: StageStatus[], logs: RunLog[]): string | null {
  const implementation = [...history].reverse().find((entry) => entry.stage === "implementation");
  if (implementation?.status !== "SUCCEEDED") return null;
  const content = logs.find((log) => log.filename === "01-implementation.log")?.content;
  if (!content) return null;

  const lines = content.split(/\r?\n/);
  for (let index = lines.length - 1; index >= 0; index -= 1) {
    const candidate = PUBLISHED_PR.exec(lines[index])?.[1];
    if (!candidate) continue;
    const match = GITHUB_PR.exec(candidate);
    if (!match || [match[1], match[2]].some((part) => part === "." || part === "..")) continue;
    return candidate;
  }
  return null;
}
