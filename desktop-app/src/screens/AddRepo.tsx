import { useEffect, useState } from "react";
import { Input, OutlineButton, PrimaryButton, SectionHeading, Title } from "../components/ui";
import { open } from "@tauri-apps/plugin-dialog";
import { invoke } from "@tauri-apps/api/core";


interface Props {
  onDone: () => void;
}

interface FolderInfo {
  isGit: boolean;
  hasConfig: boolean;
}

interface Requirement {
  name: string;
  installed: boolean;
}


export function AddRepo({ onDone }: Props) {
  const [path, setPath] = useState("");
  const [info, setInfo] = useState<FolderInfo | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [requirements, setRequirements] = useState<Requirement[]>([]);
  const [sandboxError, setSandboxError] = useState<string | null>(null);

  useEffect(() => {
    invoke<Requirement[]>("check_requirements")
      .then(setRequirements)
      .catch((e) => setSandboxError(String(e)));
  }, []);

  const hasConfig = info?.hasConfig ?? false;

  const checks = [
    info?.isGit
      ? { ok: true, text: "git repository" }
      : { ok: false, text: "not a git repository" },
    hasConfig
      ? { ok: true, text: "software-factory.json found" }
      : { ok: false, text: "no software-factory.json — create it in the repository's Config section" },
    ...(sandboxError
      ? [{ ok: false, text: sandboxError }]
      : requirements.map((r) => ({
          ok: r.installed,
          text: r.installed ? `${r.name} installed` : `${r.name} not installed`,
        }))),
  ];

   async function browse() {
    const folder = await open({
      directory: true,
      multiple: false,
      defaultPath: path || undefined,
    });
     if (!folder) return;
    setError(null);
    setPath(folder);
    setInfo(await invoke<FolderInfo>("inspect_folder", { path: folder }));
  }

  async function addRepo() {
    try {
      await invoke("save_repo", { path });
      onDone();
    } catch (e) {
      setError(String(e));
    }
  }


  return (
    <div className="flex max-w-[680px] flex-col gap-9">
      <header className="flex flex-col gap-1">
        <Title>Add repository</Title>
        <span className="text-base text-muted">Point Soft Factory at a git repository on this machine.</span>
      </header>

      <section aria-labelledby="folder-h" className="flex flex-col gap-2.5">
        <SectionHeading id="folder-h" plain>
          Folder
        </SectionHeading>
        <div className="flex gap-2">
          <label className="sr-only" htmlFor="folder">
            Repository folder
          </label>
          <Input
            id="folder"
            mono
            large
            readOnly
            className="min-w-0 grow"
            value={path}
            placeholder="/path/to/repository"
          />
          <OutlineButton onClick={browse}>Browse</OutlineButton>
        </div>
      </section>

      {info && (
        <section aria-labelledby="chk-h" className="flex flex-col gap-1.5">
          <SectionHeading id="chk-h" plain className="mb-1">
            Checks
          </SectionHeading>
          {checks.map((c) => (
            <span key={c.text} className="font-mono text-sm whitespace-pre">
              <span className={c.ok ? "text-ok" : "text-warn"}>{c.ok ? "✓" : "!"}</span>
              {"  "}
              {c.text}
            </span>
          ))}
        </section>
      )}

      <div className="flex items-center gap-5 border-t border-line pt-2">
        <PrimaryButton size="lg" disabled={!info?.isGit} onClick={addRepo}>
          Add repository
        </PrimaryButton>
        {error && (
          <span role="alert" className="text-base text-warn">
            {error}
          </span>
        )}
      </div>
    </div>
  );
}
