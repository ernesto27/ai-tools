import { invoke } from "@tauri-apps/api/core";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Icon } from "./Icon";
import { OutlineButton } from "./ui";

type ChangedFile = { path: string; patch: string | null };
type DiffState =
  | { status: "idle" | "loading" }
  | { status: "failed"; error: string }
  | { status: "ready"; files: ChangedFile[] };

interface Props {
  repoPath: string;
  branch: string;
  active: boolean;
  live: boolean;
}

export function RunChanges({ repoPath, branch, active, live }: Props) {
  const { t } = useTranslation();
  const [state, setState] = useState<DiffState>({ status: "idle" });
  const [selected, setSelected] = useState(0);

  if (!active) return null;

  async function load() {
    setState({ status: "loading" });
    try {
      const files = await invoke<ChangedFile[]>("load_git_diff", { path: repoPath, branch });
      setSelected(0);
      setState({ status: "ready", files });
    } catch (error) {
      setState({ status: "failed", error: String(error) });
    }
  }

  const { status } = state;
  return (
    <>
      <div className="flex shrink-0 flex-wrap items-center gap-3">
        <OutlineButton disabled={live || status === "loading"} onClick={() => void load()}>
          {t("loadLatestCommitChanges")}
        </OutlineButton>
      </div>
      {live ? <p role="status" className="text-muted">{t("changesWaiting")}</p>
        : status === "loading" ? <p role="status" className="text-muted">{t("changesLoading")}</p>
        : status === "failed" ? (
          <p role="alert" className="whitespace-pre-wrap break-words text-bad">
            {t("changesLoadError", { error: state.error })}
          </p>
        )
        : status === "ready" && (state.files.length
          ? <FileDiff files={state.files} selected={selected} setSelected={setSelected} />
          : <p role="status" className="text-muted">{t("noChangesFound")}</p>)}
    </>
  );
}

function FileDiff({ files, selected, setSelected }: {
  files: ChangedFile[];
  selected: number;
  setSelected: (index: number) => void;
}) {
  const { t } = useTranslation();
  const file = files[selected];
  const entries = useMemo(() => groupFiles(files), [files]);

  const patch = useMemo(() => file.patch === null ? null : parsePatch(file.patch), [file.patch]);

  return (
    <div className="grid min-h-0 min-w-0 flex-1 grid-rows-[minmax(0,1fr)_minmax(0,2fr)] gap-3 md:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] md:grid-rows-1">
      <nav aria-label={t("changedFiles")} className="min-h-0 overflow-auto rounded-md bg-panel p-2">
        <FileList entries={entries} selected={selected} onSelect={setSelected} />
      </nav>
      <section key={selected} aria-label={t("changesForFile", { filename: file.path })} tabIndex={0}
        className="min-h-0 min-w-0 overflow-auto rounded-md bg-panel">
        <header className="sticky left-0 top-0 z-10 flex items-baseline justify-between gap-3 border-b border-line bg-panel px-3 py-2 font-mono text-sm">
          <h3 className="min-w-0 break-all">{file.path}</h3>
          {patch && (patch.added > 0 || patch.removed > 0) && (
            <span className="flex shrink-0 gap-2 tabular-nums"
              aria-label={t("diffLineTotals", { added: patch.added, removed: patch.removed })}>
              <span className="text-ok">+{patch.added}</span>
              <span className="text-bad">−{patch.removed}</span>
            </span>
          )}
        </header>
        {patch ? (
          <pre className="w-max min-w-full py-1 font-mono text-sm leading-6"><code className="block">
            {patch.rows.map((row, index) => (
              <span key={index} data-diff-line={row.kind}
                className={"grid grid-cols-[3.5rem_3.5rem_1.5rem_1fr] " + diffRowTone[row.kind]}>
                {"text" in row ? (
                  <>
                    <span className="border-r border-line px-2 text-right text-muted tabular-nums select-none">{row.old}</span>
                    <span className="border-r border-line px-2 text-right text-muted tabular-nums select-none">{row.new}</span>
                    <span className="text-center select-none">{row.marker}</span>
                    <span className="pr-3 text-ink">{row.text}</span>
                  </>
                ) : <span className="col-span-4 px-3">{row.label}</span>}
              </span>
            ))}
          </code></pre>
        ) : <p role="status" className="px-3 py-2 text-muted">{t("diffTooLarge")}</p>}
      </section>
    </div>
  );
}

type DiffRow =
  | { kind: "hunk" | "note"; label: string }
  | { kind: "addition" | "deletion" | "context"; old?: number; new?: number; marker: string; text: string };

const diffRowTone = {
  hunk: "bg-info/10 text-info",
  note: "text-muted",
  context: "",
  addition: "bg-ok/10 text-ok",
  deletion: "bg-bad/10 text-bad",
};

function parsePatch(patch: string) {
  const rows: DiffRow[] = [];
  const metadata: DiffRow[] = [];
  let oldLine = 0;
  let newLine = 0;
  let added = 0;
  let removed = 0;

  for (const line of patch.replace(/\n$/, "").split("\n")) {
    const hunk = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/.exec(line);
    if (hunk) {
      oldLine = Number(hunk[1]);
      newLine = Number(hunk[2]);
      rows.push({ kind: "hunk", label: line });
    } else if (rows.length === 0) {
      metadata.push({ kind: "note", label: line });
    } else if (line.startsWith("+")) {
      rows.push({ kind: "addition", new: newLine++, marker: "+", text: line.slice(1) });
      added++;
    } else if (line.startsWith("-")) {
      rows.push({ kind: "deletion", old: oldLine++, marker: "−", text: line.slice(1) });
      removed++;
    } else if (line.startsWith(" ") || line === "") {
      rows.push({ kind: "context", old: oldLine++, new: newLine++, marker: "", text: line.slice(1) });
    } else {
      rows.push({ kind: "note", label: line });
    }
  }

  return { rows: rows.length ? rows : metadata, added, removed };
}

type FileEntry = { name: string; path: string; index: number };
type FolderEntry = { name: string; children: FileEntries };
type FileEntries = Map<string, FileEntry | FolderEntry>;

function groupFiles(files: ChangedFile[]): FileEntries {
  const root: FileEntries = new Map();
  files.forEach(({ path }, index) => {
    const parts = path.split("/");
    const name = parts.pop()!;
    let entries = root;
    for (const folderName of parts) {
      const key = `folder:${folderName}`;
      const existing = entries.get(key);
      const folder = existing && "children" in existing ? existing : { name: folderName, children: new Map() };
      entries.set(key, folder);
      entries = folder.children;
    }
    // Git can report a file replaced by a folder at the same path.
    entries.set(`file:${index}`, { name, path, index });
  });
  return root;
}

interface FileListProps {
  entries: FileEntries;
  selected: number;
  onSelect: (index: number) => void;
  nested?: boolean;
}

const fileRow = "flex w-full min-w-0 items-center gap-2 rounded-sm px-2 py-1.5 text-left font-mono text-sm hover:bg-selected focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-info";

function FileList({ entries, selected, onSelect, nested }: FileListProps) {
  return (
    <ul className={nested ? "ml-4 border-l border-line pl-1" : undefined}>
      {Array.from(entries, ([key, entry]) => (
        <li key={key}>
          {"children" in entry ? <FolderRow entry={entry} selected={selected} onSelect={onSelect} /> : (
            <button type="button" title={entry.path} aria-current={entry.index === selected ? "true" : undefined}
              className={`${fileRow} aria-[current=true]:bg-selected`}
              onClick={() => onSelect(entry.index)}>
              <Icon name="file" className="size-4 shrink-0 text-muted" />
              <span className="truncate">{entry.name}</span>
            </button>
          )}
        </li>
      ))}
    </ul>
  );
}

function FolderRow({ entry, selected, onSelect }: { entry: FolderEntry } & Pick<FileListProps, "selected" | "onSelect">) {
  const [expanded, setExpanded] = useState(true);
  return (
    <>
      <button type="button" title={entry.name} aria-expanded={expanded} className={fileRow}
        onClick={() => setExpanded(!expanded)}>
        <Icon name={expanded ? "folder-open" : "folder"} className="size-4 shrink-0 text-muted" />
        <span className="truncate">{entry.name}</span>
      </button>
      <div hidden={!expanded}>
        <FileList entries={entry.children} selected={selected} onSelect={onSelect} nested />
      </div>
    </>
  );
}
