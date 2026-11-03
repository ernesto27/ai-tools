import { useTranslation } from "react-i18next";
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
  const { t } = useTranslation();
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
      ? { ok: true, text: t("git repository") }
      : { ok: false, text: t("not a git repository") },
    hasConfig
      ? { ok: true, text: t("software-factory.json found") }
      : { ok: false, text: t("no software-factory.json — create it in the repository's Config section") },
    ...(sandboxError
      ? [{ ok: false, text: sandboxError }]
      : requirements.map((r) => ({
          ok: r.installed,
          text: r.installed ? t("{{name}} installed", { name: r.name }) : t("{{name}} not installed", { name: r.name }),
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
        <Title>{t("Add repository")}</Title>
        <span className="text-base text-muted">{t("Point Soft Factory at a git repository on this machine.")}</span>
      </header>

      <section aria-labelledby="folder-h" className="flex flex-col gap-2.5">
        <SectionHeading id="folder-h" plain>
          {t("Folder")}
        </SectionHeading>
        <div className="flex gap-2">
          <label className="sr-only" htmlFor="folder">
            {t("Repository folder")}
          </label>
          <Input
            id="folder"
            mono
            large
            readOnly
            className="min-w-0 grow"
            value={path}
            placeholder={t("/path/to/repository")}
          />
          <OutlineButton onClick={browse}>{t("Browse")}</OutlineButton>
        </div>
      </section>

      {info && (
        <section aria-labelledby="chk-h" className="flex flex-col gap-1.5">
          <SectionHeading id="chk-h" plain className="mb-1">
            {t("Checks")}
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
          {t("Add repository")}
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
