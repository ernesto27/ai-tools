import { useEffect, useRef } from "react";
import { SectionHeading } from "./ui";

export function DebugOutput({ output, active }: { output: string; active: boolean }) {
  const panel = useRef<HTMLPreElement>(null);
  const follow = useRef(true);

  useEffect(() => {
    if (follow.current && panel.current) {
      panel.current.scrollTop = panel.current.scrollHeight;
    }
  }, [output]);

  return (
    <section aria-labelledby="run-output-h" className="flex min-w-0 flex-col gap-3">
      <div className="flex items-center justify-between gap-3">
        <SectionHeading id="run-output-h" plain>Debug output</SectionHeading>
        <span role="status" className="text-sm text-muted">{active ? "Live" : "Finished"}</span>
      </div>
      <pre ref={panel} tabIndex={0} aria-label="Live command output" onScroll={(event) => {
        const element = event.currentTarget;
        follow.current = element.scrollHeight - element.scrollTop - element.clientHeight < 32;
      }} className="max-h-80 min-h-32 overflow-auto rounded-md bg-panel p-4 font-mono text-md whitespace-pre-wrap break-words">
        {output}
      </pre>
      <span className="text-xs text-muted">Latest 200,000 characters · stdout and stderr</span>
    </section>
  );
}
