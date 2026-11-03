import { useTranslation } from "react-i18next";
import { WorktreePath } from "../components/WorktreePath";
import { RUN_MODE, type Navigate } from "../router";
import { stageLabel, statusClass, statusLabel } from "../runStatus";
import { useBranchRuns, type RunEntry } from "../hooks/useBranchRuns";
import { Crumb, LinkButton, OutlineButton, PrimaryButton, SectionHeading, Title } from "../components/ui";

interface Props {
  repo: string;
  repoPath: string;
  branch: string;
  worktreePath?: string;
  navigate: Navigate;
}

function RunRow({ entry, onOpen }: { entry: RunEntry; onOpen: () => void }) {
  const { t } = useTranslation();
  return (
    <article className="relative flex min-w-0 flex-col gap-3 border-b border-line p-4 hover:bg-hover focus-within:bg-hover">
      <button
        type="button"
        aria-label={t("Open run {{name}}", { name: entry.name })}
        onClick={onOpen}
        className="absolute inset-0 cursor-pointer rounded-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-info"
      />
      <h3 className="font-mono text-md break-words">{entry.name}</h3>
      <p className="font-mono text-xs break-all text-muted">{entry.directory}</p>
      {entry.error && <p role="alert" className="text-sm break-words text-bad">{entry.error}</p>}
      {!entry.error && entry.stages.length === 0 && (
        <p className="text-sm text-muted">{t("No stage results recorded yet.")}</p>
      )}
      <dl className="grid grid-cols-[minmax(0,1fr)_auto] gap-x-6 gap-y-2 text-md">
        {entry.stages.map((stage, index) => (
          <StageRow key={`${stage.stage}:${index}`} name={stage.stage} status={stage.status} />
        ))}
      </dl>
    </article>
  );
}

function StageRow({ name, status }: { name: string; status: string }) {
  const { t } = useTranslation();
  return (
    <>
      <dt>{stageLabel(name, t)}</dt>
      <dd className={statusClass(status)}>{statusLabel(status, t, true)}</dd>
    </>
  );
}

export function BranchRuns({ repo, repoPath, branch, worktreePath, navigate }: Props) {
  const { t } = useTranslation();
  const { entries, loading, error, refresh } = useBranchRuns(repoPath, branch);
  const at = { repo, branch };

  return (
    <div className="flex max-w-[1080px] flex-col gap-8" aria-busy={loading}>
      <header className="flex min-w-0 flex-wrap items-end justify-between gap-4">
        <div className="flex min-w-0 max-w-full flex-1 flex-col gap-1">
          <Crumb>{repo} / {t("branch")}</Crumb>
          <Title><span className="font-mono font-medium">{branch}</span></Title>
          {worktreePath !== undefined && <WorktreePath key={worktreePath} path={worktreePath} />}
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <OutlineButton onClick={refresh} disabled={loading}>{loading ? t("Loading…") : t("Refresh")}</OutlineButton>
          <PrimaryButton onClick={() => navigate({ screen: "newRun", ...at, mode: RUN_MODE.RUN })}>{t("New run")}</PrimaryButton>
        </div>
      </header>
      <div className="flex items-center gap-5 text-md">
        <LinkButton onClick={() => navigate({ screen: "newRun", ...at, mode: RUN_MODE.CONTINUE })}>{t("Continue")}</LinkButton>
      </div>
      <section aria-labelledby="runs-heading" className="min-w-0">
        <SectionHeading id="runs-heading">
          {loading ? t("Runs") : t("runCount", { count: entries.length })}
        </SectionHeading>
        {loading && <p role="status" className="py-4 text-muted">{t("Loading runs…")}</p>}
        {error && <p role="alert" className="py-4 text-bad">{error}</p>}
        {!loading && !error && entries.length === 0 && (
          <p className="py-4 text-muted">{t("No runs recorded for this branch.")}</p>
        )}
        {entries.map((entry) => (
          <RunRow
            key={entry.directory}
            entry={entry}
            onOpen={() => navigate({ screen: "run", ...at, runId: entry.name })}
          />
        ))}
      </section>
    </div>
  );
}
