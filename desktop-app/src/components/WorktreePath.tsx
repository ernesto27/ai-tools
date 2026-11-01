import { useEffect, useState } from "react";
import { writeText } from "@tauri-apps/plugin-clipboard-manager";
import { TextButton } from "./ui";

interface Props {
  path: string;
}

type CopyStatus = "idle" | "copying" | "copied" | { error: string };

export function WorktreePath({ path }: Props) {
  const [status, setStatus] = useState<CopyStatus>("idle");

  useEffect(() => {
    if (status !== "copied") return;
    const timer = setTimeout(() => setStatus("idle"), 2000);
    return () => clearTimeout(timer);
  }, [status]);

  async function copyPath() {
    setStatus("copying");
    try {
      await writeText(path);
      setStatus("copied");
    } catch (error) {
      setStatus({ error: `Could not copy path: ${String(error)}` });
    }
  }

  return (
    <div className="flex min-w-0 flex-col gap-1">
      <div className="flex min-w-0 items-start gap-2">
        <p className="min-w-0 font-mono text-xs break-all text-muted">{path}</p>
        <TextButton
          tone="muted"
          aria-label="Copy worktree path"
          title="Copy worktree path"
          disabled={status === "copying"}
          onClick={() => void copyPath()}
          className="flex size-7 shrink-0 items-center justify-center rounded-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-info disabled:opacity-40"
        >
          <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" className="size-4">
            <rect x="8" y="8" width="12" height="12" rx="2" />
            <path d="M16 8V4H4v12h4" />
          </svg>
        </TextButton>
      </div>
      <span role="status" className="text-xs text-muted">{status === "copied" ? "Copied" : ""}</span>
      {typeof status === "object" && (
        <p role="alert" className="text-xs break-words text-bad">{status.error}</p>
      )}
    </div>
  );
}
