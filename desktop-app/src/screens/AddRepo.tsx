import { useState } from "react";
import { AGENTS, knownFolders } from "../data";
import { Checkbox, Field, Input, LinkButton, OutlineButton, PrimaryButton, SectionHeading, Select, Title } from "../components/ui";

interface Props {
  onDone: () => void;
}

export function AddRepo({ onDone }: Props) {
  const [path, setPath] = useState(knownFolders[0].path);

  // Mock folder inspection; later this comes from a Tauri command.
  const found = knownFolders.find((f) => f.path === path.trim());
  const branch = found?.branch ?? "main";
  const hasConfig = found?.hasConfig ?? false;

  const checks = [
    { ok: true, text: `git repository · ${branch}` },
    hasConfig
      ? { ok: true, text: "software-factory.json found" }
      : { ok: false, text: "no software-factory.json — a starter will be created" },
    { ok: true, text: "agent-sandbox · git · docker on PATH" },
  ];

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
            className="min-w-0 grow"
            value={path}
            placeholder="/path/to/repository"
            onChange={(e) => setPath(e.target.value)}
          />
          <OutlineButton>Browse</OutlineButton>
        </div>
      </section>

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

      {!hasConfig && (
        <section aria-labelledby="starter-h" className="flex flex-col gap-4">
          <div className="flex flex-col gap-0.5 border-b border-line pb-3">
            <SectionHeading id="starter-h" plain>
              Starter software-factory.json
            </SectionHeading>
            <span className="text-sm text-muted">You can change everything later in Config.</span>
          </div>
          <div className="grid grid-cols-[repeat(auto-fit,minmax(180px,1fr))] gap-4">
            <Field label="Agent">
              <Select defaultValue="codex">
                {AGENTS.map((a) => (
                  <option key={a}>{a}</option>
                ))}
              </Select>
            </Field>
            <Field label="run.branch">
              <Input mono defaultValue="factory-task" />
            </Field>
            <Field label="Task file">
              <Input mono defaultValue="task.txt" />
            </Field>
          </div>
          <Checkbox className="text-base" defaultChecked>
            Add logs/ to .gitignore
          </Checkbox>
        </section>
      )}

      <div className="flex items-center gap-5 border-t border-line pt-2">
        <PrimaryButton size="lg" onClick={onDone}>
          Add repository
        </PrimaryButton>
        <LinkButton muted className="text-base" onClick={onDone}>
          Cancel
        </LinkButton>
      </div>
    </div>
  );
}
