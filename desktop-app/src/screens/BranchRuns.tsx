import { STAGE_NAMES, STATUS_COLOR, type Branch } from "../data";
import type { Navigate } from "../router";
import { Dot } from "../components/Dot";
import { Crumb, LinkButton, PrimaryButton, SectionHeading, TextButton, Title } from "../components/ui";

const PIP_LABEL: Record<string, string> = { s: "done", f: "failed", i: "interrupted", r: "running", "-": "not run" };
const PIP_COLOR: Record<string, string> = { s: "bg-ink", f: "bg-bad", i: "bg-warn", r: "bg-info", "-": "bg-line" };

interface Props {
  repo: string;
  branch: Branch;
  navigate: Navigate;
}

export function BranchRuns({ repo, branch, navigate }: Props) {
  const at = { repo, branch: branch.name };

  return (
    <div className="flex max-w-[1080px] flex-col gap-10">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1">
          <Crumb>{repo} / branch</Crumb>
          <Title>
            <span className="font-mono font-medium">{branch.name}</span>
          </Title>
        </div>
        <PrimaryButton onClick={() => navigate({ screen: "newRun", ...at, mode: "run" })}>New run</PrimaryButton>
      </header>

      <section aria-label="Branch" className="flex flex-col gap-7">
        <div className="flex flex-col gap-2.5">
          <span className="font-mono text-sm text-muted">
            {branch.task} · {branch.ahead} commits ahead · <span className="text-ok">{branch.add}</span>{" "}
            <span className="text-bad">{branch.del}</span> · {branch.pr}
          </span>
          <div className="flex flex-wrap items-center gap-5 pt-1.5 text-md">
            <LinkButton className="font-medium" onClick={() => navigate({ screen: "newRun", ...at, mode: "continue" })}>
              Continue
            </LinkButton>
            <TextButton>Open diff</TextButton>
            <TextButton>Set as resume branch</TextButton>
            <TextButton tone="danger">Remove worktree</TextButton>
          </div>
        </div>

        <div className="flex flex-col">
          <SectionHeading as="h3">
            {branch.runs.length} {branch.runs.length === 1 ? "run" : "runs"}
          </SectionHeading>
          {branch.runs.map((run) => (
            <div key={run.id} className="flex flex-wrap items-center gap-x-6 gap-y-2 border-b border-line py-4">
              <button
                type="button"
                className="group flex min-w-0 flex-[1_1_240px] flex-col gap-0.5 text-left"
                onClick={() => navigate({ screen: "run", ...at, runId: run.id })}
              >
                <span className="font-medium underline-offset-4 group-hover:underline">
                  {run.kind} · {run.when}
                </span>
                <span className="text-sm text-muted">{run.note}</span>
              </button>
              <span
                role="img"
                aria-label={run.pips
                  .split("")
                  .map((p, i) => `${STAGE_NAMES[i]} ${PIP_LABEL[p]}`)
                  .join(", ")}
                className="flex gap-[3px]"
              >
                {run.pips.split("").map((p, i) => (
                  <span key={i} className={`block h-1 w-3.5 rounded-[1px] ${PIP_COLOR[p]}`} />
                ))}
              </span>
              <span className="inline-flex w-run-status items-center gap-2 text-md">
                <Dot color={STATUS_COLOR[run.status]} />
                {run.status}
              </span>
              <span className="w-run-risk font-mono text-xs">{run.risk}</span>
              <span className="w-run-dur font-mono text-xs text-muted">{run.dur}</span>
              <span className="w-run-action text-right text-md">
                {run.resumable && <LinkButton onClick={() => navigate({ screen: "newRun", ...at, mode: "resume" })}>Resume</LinkButton>}
              </span>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}
