import { useState, type ReactNode } from "react";
import { runDetail, type Tone } from "../data";
import type { Navigate } from "../router";
import { Crumb, cx, LinkButton, PrimaryButton, Tabs, TextButton, Title } from "../components/ui";

type Tab = "log" | "summary" | "walk";

const TABS = [
  ["log", "Log"],
  ["summary", "Summary"],
  ["walk", "Walkthrough"],
] as const;

const TONE: Record<Tone, string> = { ink: "text-ink", grey: "text-muted", warn: "text-warn", ok: "text-ok" };

function Stat({ label, children, className }: { label: string; children: ReactNode; className?: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="text-sm text-muted">{label}</dt>
      <dd className={className ?? "text-2xl font-medium"}>{children}</dd>
    </div>
  );
}

const Code = ({ children }: { children: ReactNode }) => <code className="font-mono text-md">{children}</code>;

interface Props {
  repo: string;
  branch: string;
  navigate: Navigate;
}

export function RunDetail({ repo, branch, navigate }: Props) {
  const [sel, setSel] = useState(1);
  const [tab, setTab] = useState<Tab>("log");
  const d = runDetail;
  const cur = d.stages[sel];
  const file = tab === "log" ? cur.file : tab === "summary" ? "summary.log" : "05-reviewChanges.md";

  return (
    <div className="flex max-w-[1080px] flex-col gap-9">
      <header className="flex flex-col gap-2">
        <Crumb>
          {repo} /{" "}
          <LinkButton muted onClick={() => navigate({ screen: "branch", repo, branch })}>
            {branch}
          </LinkButton>
        </Crumb>
        <div className="flex flex-wrap items-end justify-between gap-4">
          <Title>{d.title}</Title>
          <div className="flex flex-wrap items-center gap-5 text-md">
            <TextButton>Open folder</TextButton>
            <TextButton>Open diff</TextButton>
            <PrimaryButton onClick={() => navigate({ screen: "newRun", repo, branch, mode: "continue" })}>Continue</PrimaryButton>
          </div>
        </div>
        <span className="font-mono text-sm text-muted">{d.meta}</span>
      </header>

      <dl className="grid grid-cols-[repeat(auto-fit,minmax(150px,1fr))] gap-4 border-y border-line py-5">
        <Stat label="Risk" className="text-2xl font-medium text-warn">
          {d.risk}
        </Stat>
        <Stat label="Code review">{d.codeReview}</Stat>
        <Stat label="Security review" className="text-2xl font-medium text-warn">
          {d.securityReview}
        </Stat>
        <Stat label="Changes" className="font-mono text-xl">
          <span className="text-ok">{d.add}</span> <span className="text-bad">{d.del}</span>{" "}
          <span className="text-md text-muted">{d.files} files</span>
        </Stat>
      </dl>

      <div className="flex flex-wrap items-start gap-10">
        <section aria-label="Stages" className="flex min-w-0 flex-[1_1_220px] flex-col gap-0.5">
          {d.stages.map((s, i) => (
            <button
              key={s.name}
              type="button"
              aria-pressed={sel === i}
              onClick={() => {
                setSel(i);
                setTab("log");
              }}
              className="flex min-h-13 w-full flex-col items-start gap-0.5 rounded-md px-2.5 py-2 text-left hover:bg-hover aria-pressed:bg-selected"
            >
              <span className="flex w-full justify-between gap-2">
                <span className="font-medium">{s.name}</span>
                <span className="font-mono text-xs text-muted">{s.dur}</span>
              </span>
              <span className={cx("text-sm", TONE[s.tone])}>{s.verdict}</span>
            </button>
          ))}
        </section>

        <section aria-label="Output" className="flex min-w-0 flex-[999_1_520px] flex-col gap-3">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-line">
            <Tabs label="Views" items={TABS} value={tab} onChange={setTab} size="sm" bordered={false} />
            <span className="font-mono text-xs text-muted">{file}</span>
          </div>

          {tab === "log" && (
            <div className="min-h-90 overflow-x-auto rounded-md bg-panel py-4 font-mono text-sm leading-[1.8]">
              {cur.lines.map(([text, tone], i) => (
                <div key={i} className="flex gap-4 px-4 whitespace-pre">
                  <span className="w-5 shrink-0 text-right text-faint select-none" aria-hidden="true">
                    {i + 1}
                  </span>
                  <span className={TONE[tone]}>{text}</span>
                </div>
              ))}
            </div>
          )}

          {tab === "summary" && (
            <div className="flex flex-col">
              {d.summary.map((e) => (
                <div key={e.name} className="grid grid-cols-4 gap-3 border-b border-line py-3 text-md">
                  <span className="font-medium">{e.name}</span>
                  <span>{e.agent}</span>
                  <span className="font-mono text-muted">{e.dur}</span>
                  <span className="text-muted">{e.note}</span>
                </div>
              ))}
            </div>
          )}

          {tab === "walk" && (
            <article className="flex max-w-[680px] flex-col gap-3.5 leading-[1.7]">
              <h3 className="mt-2 text-2xl font-semibold">Structured execution logs</h3>
              <p>
                Each invocation writes to its own folder under <Code>logs/&lt;branch&gt;/</Code>, with one numbered log per stage and a{" "}
                <Code>summary.log</Code> entry per attempt.
              </p>
              <ul className="flex list-disc flex-col gap-1 pl-[18px]">
                <li>
                  <Code>internal/logs/writer.go</Code> — invocation folder, sanitized names
                </li>
                <li>
                  <Code>internal/runner/stage.go</Code> — records each attempt
                </li>
                <li>
                  <Code>cmd/factory/resume.go</Code> — picks the next stage
                </li>
              </ul>
              <p className="text-warn">Unapplied security finding: log folder created with 0755. Suggested fix: 0700.</p>
            </article>
          )}
        </section>
      </div>
    </div>
  );
}
