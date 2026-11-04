import { messageText, type UiMessage } from "../messages";
import { useTranslation } from "react-i18next";
import type { BranchWorktree } from "../worktrees";
import { useCallback, useEffect, useState, type ComponentProps } from "react";
import { STATUS_COLOR } from "../data";
import { RUN_MODE, type Navigate, type Route } from "../router";
import { Dot } from "./Dot";
import { cx } from "./ui";
import { RepoContextMenu } from "./RepoContextMenu";
import { Icon } from "./Icon";
import { version } from "../../package.json";

interface Props {
  repos: SavedRepo[];
  branchesByPath: Record<string, BranchWorktree[]>;
  onRefreshBranches: (path: string) => Promise<void>;
  route: Route;
  navigate: Navigate;
  onRemoveRepo: (repo: SavedRepo) => Promise<void>;
}

export interface SavedRepo {
  name: string;
  path: string;
}

function NavItem({ tall, className, ...props }: ComponentProps<"button"> & { tall?: boolean }) {
  return (
    <button
      type="button"
      className={cx(
        "nav-item flex w-full items-center gap-2 rounded-md px-2 text-left text-muted hover:bg-hover hover:text-ink",
        tall ? "min-h-9" : "min-h-8",
        "aria-expanded:text-ink aria-[current=page]:bg-selected aria-[current=page]:font-medium aria-[current=page]:text-ink",
        className,
      )}
      {...props}
    />
  );
}

export function Sidebar({ repos, branchesByPath, onRefreshBranches, route, navigate, onRemoveRepo }: Props) {
  const { t } = useTranslation();
  const activeRepo = "repo" in route ? route.repo : null;
  const activeBranch = "branch" in route ? route.branch : null;
  const [openRepo, setOpenRepo] = useState("");
  const [openBranch, setOpenBranch] = useState<{ repo: string; branch: string } | null>(null);
  const [repoMenu, setRepoMenu] = useState<{
    repo: SavedRepo;
    x: number;
    y: number;
    trigger: HTMLButtonElement;
  } | null>(null);
  const [removeError, setRemoveError] = useState<UiMessage | null>(null);
  const closeRepoMenu = useCallback(() => setRepoMenu(null), []);

  const deleteFromMenu = () => {
    if (!repoMenu) return;
    const { repo } = repoMenu;
    closeRepoMenu();
    setRemoveError(null);
    void onRemoveRepo(repo).catch((error: unknown) => {
      setRemoveError({ key: "couldNotDeleteNameError", values: { name: repo.name, error: String(error) } });
    });
  };

  useEffect(() => {
    const paths = new Set(repos.map((repo) => repo.path));
    const names = new Set(repos.map((repo) => repo.name));
    setOpenRepo((previous) => names.has(previous) ? previous : "");
    setOpenBranch((previous) => previous && names.has(previous.repo) ? previous : null);
    setRepoMenu((previous) => previous && paths.has(previous.repo.path) ? previous : null);
  }, [repos]);

  // Which branch sub-item (Runs / Continue / Resume) is current.
  const sub = route.screen === "newRun" ? route.mode : "runs";

  return (
    <nav data-testid="main-nav" aria-label={t("main")} className="app-sidebar flex w-nav flex-none flex-col overflow-hidden border-r border-line px-4 py-6 text-base">
      <div className="flex shrink-0 items-center gap-3 px-2 pb-7">
        <span className="brand-mark"><Icon name="factory" width="24" height="24" /></span>
        <span className="text-xl font-semibold tracking-[-0.04em]">soft·factory</span>
      </div>
      <div className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto">
        <NavItem tall aria-current={route.screen === "addRepo" ? "page" : undefined} onClick={() => navigate({ screen: "addRepo" })}>
          <Icon name="plus" className="shrink-0" />
          {t("addRepository")}
        </NavItem>

        <span className="flex items-center justify-between px-2 pt-7 pb-3 text-xs font-medium text-muted">
          {t("repositories")}
          <span className="text-faint tabular-nums">{repos.length}</span>
        </span>
        <div className="flex flex-col gap-2">
          {repos.map((saved) => {
            const branches = (branchesByPath[saved.path] ?? []).map((entry) => ({ name: entry.branch, status: "idle" as const }));
            const repo = { name: saved.name, status: "idle" as const, branches };
            const isOpen = openRepo === repo.name;
            return (
              <div key={saved.path} className="flex flex-col">
                <button
                  type="button"
                  aria-haspopup="menu"
                  onContextMenu={(event) => {
                    event.preventDefault();
                    setRemoveError(null);
                    setRepoMenu({ repo: saved, x: event.clientX, y: event.clientY, trigger: event.currentTarget });
                  }}
                  onKeyDown={(event) => {
                    if (event.key !== "ContextMenu" && !(event.shiftKey && event.key === "F10")) return;
                    event.preventDefault();
                    const bounds = event.currentTarget.getBoundingClientRect();
                    setRemoveError(null);
                    setRepoMenu({ repo: saved, x: bounds.left, y: bounds.bottom, trigger: event.currentTarget });
                  }}
                  aria-expanded={isOpen}
                  onClick={() => setOpenRepo(isOpen ? "" : repo.name)}
                  className="repo-nav flex min-h-10 w-full items-center justify-between rounded-md px-2 text-left text-muted hover:bg-hover hover:text-ink aria-expanded:font-medium aria-expanded:text-ink"
                >
                  <span className="flex min-w-0 items-center gap-2">
                    <svg
                      aria-hidden="true"
                      viewBox="0 0 16 16"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="1.5"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      className={cx("size-3.5 shrink-0", isOpen && "rotate-90")}
                    >
                      <path d="m6 4 4 4-4 4" />
                    </svg>
                    <span className="min-w-0 truncate" title={repo.name}>
                      {repo.name}
                    </span>
                  </span>
                  <Dot color={STATUS_COLOR[repo.status]} />
                </button>

                {isOpen && (
                  <div className="ml-3 flex flex-col gap-px border-l border-line pl-3">
                    <NavItem
                      className="text-md"
                      aria-current={route.screen === "config" && route.repo === repo.name ? "page" : undefined}
                      onClick={() => navigate({ screen: "config", repo: repo.name })}
                    >
                      {t("config")}
                    </NavItem>
                    <NavItem
                      className="text-md"
                      aria-current={route.screen === "newRun" && route.mode === RUN_MODE.RUN && route.repo === repo.name ? "page" : undefined}
                      onClick={() => {
                        void onRefreshBranches(saved.path);
                        navigate({ screen: "newRun", repo: repo.name, branch: repo.branches[0]?.name ?? "factory-task", mode: RUN_MODE.RUN });
                      }}
                    >
                      {t("newRun")}
                    </NavItem>
                    {repo.branches.length === 0 && <span className="px-2 py-1 text-sm text-faint">{t("noBranchesYet")}</span>}
                    {repo.branches.map((b) => {
                      const expanded = openBranch?.repo === repo.name && openBranch?.branch === b.name;
                      const isActive = activeRepo === repo.name && activeBranch === b.name;
                      const at = { repo: repo.name, branch: b.name };
                      return (
                        <div key={b.name} className="flex flex-col">
                          <NavItem
                            className="font-mono text-sm"
                            title={b.name}
                            aria-expanded={expanded}
                            onClick={() => {
                              setOpenBranch(expanded ? null : at);
                              if (!expanded) navigate({ screen: "branch", ...at });
                            }}
                          >
                            <svg
                              aria-hidden="true"
                              viewBox="0 0 16 16"
                              fill="none"
                              stroke="currentColor"
                              strokeWidth="1.5"
                              strokeLinecap="round"
                              strokeLinejoin="round"
                              className={cx("size-3.5 shrink-0", expanded && "rotate-90")}
                            >
                              <path d="m6 4 4 4-4 4" />
                            </svg>
                            <Dot color={STATUS_COLOR[b.status]} />
                            <span className="min-w-0 truncate">{b.name}</span>
                          </NavItem>
                          {expanded && (
                            <div className="ml-[11px] flex flex-col gap-px border-l border-line pl-2.5 text-md">
                              <NavItem aria-current={isActive && sub === "runs" ? "page" : undefined} onClick={() => navigate({ screen: "branch", ...at })}>
                                {t("runs")}
                              </NavItem>
                              <NavItem
                                aria-current={isActive && sub === RUN_MODE.CONTINUE ? "page" : undefined}
                                onClick={() => navigate({ screen: "newRun", ...at, mode: RUN_MODE.CONTINUE })}
                              >
                                {t("continue")}
                              </NavItem>
                              <NavItem
                                aria-current={isActive && sub === RUN_MODE.RESUME ? "page" : undefined}
                                onClick={() => navigate({ screen: "newRun", ...at, mode: RUN_MODE.RESUME })}
                              >
                                {t("resume")}
                              </NavItem>
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            );
          })}
        </div>
        {removeError && <p role="alert" className="px-2 pt-3 text-sm text-bad">{messageText(removeError, t)}</p>}
      </div>
      <div className="mt-5 flex shrink-0 flex-col gap-2 border-t border-line pt-3">
        <NavItem tall data-testid="nav-settings"
          aria-current={route.screen === "settings" ? "page" : undefined}
          onClick={() => navigate({ screen: "settings" })}>
          <Icon name="settings" className="shrink-0" />
          {t("settings")}
        </NavItem>
        <p className="px-2 text-left text-xs text-muted">v{version}</p>
      </div>
      {repoMenu && (
        <RepoContextMenu
          key={repoMenu.repo.path}
          label={repoMenu.repo.name}
          x={repoMenu.x}
          y={repoMenu.y}
          trigger={repoMenu.trigger}
          onDelete={deleteFromMenu}
          onClose={closeRepoMenu}
        />
      )}
    </nav>
  );
}
