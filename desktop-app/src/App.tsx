import { useTranslation } from "react-i18next";
import { useCallback, useEffect, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { ask } from "@tauri-apps/plugin-dialog";
import type { Route } from "./router";
import { Sidebar, type SavedRepo } from "./components/Sidebar";
import { BranchRuns } from "./screens/BranchRuns";
import { RunDetail } from "./screens/RunDetail";
import { Config } from "./screens/Config";
import { NewRun } from "./screens/NewRun";
import { Settings } from "./screens/Settings";
import { AddRepo } from "./screens/AddRepo";
import { useDesktopRuns, type DesktopRuns } from "./hooks/useDesktopRuns";
import { useTheme } from "./hooks/useTheme";
import { PrimaryButton, Title } from "./components/ui";

import { Icon } from "./components/Icon";
import type { Theme } from "./theme";

import type { BranchWorktree } from "./worktrees";

const HOME: Route = { screen: "home" };

function App() {
  const { t } = useTranslation();
  const { theme, toggleTheme } = useTheme();
  const [sidebarVisible, setSidebarVisible] = useState(true);
  const [route, setRoute] = useState<Route>(HOME);
  const lastBranchRoute = useRef<Route>(HOME);
  const settingsReturnRoute = useRef<Route>(HOME);
  const mainRef = useRef<HTMLElement>(null);
  const [savedRepos, setSavedRepos] = useState<SavedRepo[]>([]);
  const [branchesByPath, setBranchesByPath] = useState<Record<string, BranchWorktree[]>>({});
  const savedPaths = useRef(new Set<string>());
  const branchRequests = useRef(new Map<string, symbol>());
  savedPaths.current = new Set(savedRepos.map((repo) => repo.path));

  const refreshBranches = useCallback(async (path: string) => {
    const request = Symbol();
    branchRequests.current.set(path, request);
    try {
      const branches = await invoke<BranchWorktree[]>("list_branches", { path });
      if (savedPaths.current.has(path) && branchRequests.current.get(path) === request) {
        setBranchesByPath((previous) => ({ ...previous, [path]: branches }));
      }
    } catch (error) {
      console.error(error);
    }
  }, []);

  function addBranch(path: string, worktree: BranchWorktree) {
    if (!savedPaths.current.has(path)) return;
    branchRequests.current.delete(path);
    setBranchesByPath((previous) => {
      const branches = previous[path] ?? [];
      const exists = branches.some((entry) => entry.branch === worktree.branch);
      return {
        ...previous,
        [path]: exists
          ? branches.map((entry) => entry.branch === worktree.branch ? worktree : entry)
          : [worktree, ...branches],
      };
    });
  }

  useEffect(() => {
    savedPaths.current = new Set(savedRepos.map((repo) => repo.path));
    setBranchesByPath((previous) => Object.fromEntries(
      Object.entries(previous).filter(([path]) => savedPaths.current.has(path)),
    ));
    for (const repo of savedRepos) void refreshBranches(repo.path);
    return () => {
      savedPaths.current.clear();
      branchRequests.current.clear();
    };
  }, [savedRepos, refreshBranches]);

  const desktopRuns = useDesktopRuns(addBranch);
  const removingRepo = useRef(false);

  const removeRepo = async (repo: SavedRepo) => {
    if (removingRepo.current) return;
    removingRepo.current = true;
    try {
      const confirmed = await ask(
        t("areYouSureToRemoveName", { name: repo.name }),
        { title: t("deleteProject"), kind: "warning", okLabel: t("delete"), cancelLabel: t("cancel") },
      );
      if (!confirmed) return;
      await invoke<void>("remove_repo", { path: repo.path });
      setSavedRepos((previous) => previous.filter((saved) => saved.path !== repo.path));
      if ("repo" in lastBranchRoute.current && lastBranchRoute.current.repo === repo.name) {
        lastBranchRoute.current = HOME;
      }
      setRoute((current) => "repo" in current && current.repo === repo.name ? HOME : current);
    } finally {
      removingRepo.current = false;
    }
  };

  const loadRepos = () => {
    invoke<SavedRepo[]>("list_repos").then(setSavedRepos).catch(console.error);
  };

  useEffect(() => {
    loadRepos();
  }, []);

  const navigate = (next: Route) => {
    if (next.screen === "settings" && route.screen !== "settings") settingsReturnRoute.current = route;
    if (route.screen !== "addRepo" && route.screen !== "config" && route.screen !== "settings") lastBranchRoute.current = route;
    setRoute(next);
    mainRef.current?.scrollTo({ top: 0 });
  };

  return (
    <div className="app-shell flex h-dvh overflow-hidden">
      <Sidebar hidden={!sidebarVisible} repos={savedRepos} branchesByPath={branchesByPath} onRefreshBranches={refreshBranches} route={route} navigate={navigate} onRemoveRepo={removeRepo} />
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        <div className="flex shrink-0 px-4 pt-3">
          <button
            type="button"
            aria-controls="main-sidebar"
            aria-expanded={sidebarVisible}
            aria-label={t(sidebarVisible ? "hideSidebar" : "showSidebar")}
            title={t(sidebarVisible ? "hideSidebar" : "showSidebar")}
            className="flex size-9 items-center justify-center rounded-md text-muted hover:bg-hover hover:text-ink"
            onClick={() => setSidebarVisible((visible) => !visible)}
          >
            <Icon name="sidebar" />
          </button>
        </div>
        <main
          ref={mainRef}
          className={`app-main min-h-0 min-w-0 flex-auto ${route.screen === "run" ? "overflow-hidden pb-6" : "overflow-y-auto pb-16"}`}
        >
          <Screen
            theme={theme}
            onToggleTheme={toggleTheme}
            repos={savedRepos}
            desktopRuns={desktopRuns}
            branchesByPath={branchesByPath}
            route={route}
            repoPath={
            "repo" in route
              ? savedRepos.find((repo) => repo.name === route.repo)?.path
              : undefined
            }
            navigate={navigate}
            back={() => navigate(route.screen === "settings" ? settingsReturnRoute.current : lastBranchRoute.current)}
            repoAdded={() => {
              loadRepos();
              navigate(lastBranchRoute.current);
            }}
          />
        </main>
      </div>
    </div>
  );
}

function Screen({
  theme,
  onToggleTheme,
  back,
  repos,
  desktopRuns,
  branchesByPath,
  route,
  navigate,
  repoAdded,
  repoPath,
}: {
  theme: Theme;
  onToggleTheme: () => void;
  repos: SavedRepo[];
  desktopRuns: DesktopRuns;
  branchesByPath: Record<string, BranchWorktree[]>;
  route: Route;
  navigate: (r: Route) => void;
  back: () => void;
  repoAdded: () => void;
  repoPath?: string;
}) {
  const { t } = useTranslation();
  if (route.screen === "settings") return <Settings theme={theme} onToggleTheme={onToggleTheme} onBack={back} />;
  if (route.screen === "home") {
    return (
      <div className="home-workspace flex max-w-[1080px] flex-col gap-8">
        <header className="flex flex-wrap items-start justify-between gap-5">
          <div className="flex max-w-[64ch] flex-col gap-3">
            <Title>{t("selectARepository")}</Title>
            <p className="text-base text-muted">{t("chooseARepositoryFromTheSidebarToViewItsBranchesAndRuns")}</p>
          </div>
          <PrimaryButton onClick={() => navigate({ screen: "addRepo" })}>
            <Icon name="plus" width="18" height="18" />{t("addRepository")}
          </PrimaryButton>
        </header>
        {repos.length > 0 ? (
          <section aria-label={t("repositories")} className="repository-list">
            {repos.map((repo) => {
              const branches = branchesByPath[repo.path] ?? [];
              return (
                <div key={repo.path} className="repository-entry">
                  <div className="flex min-w-0 items-center gap-4">
                    <span className="repository-symbol"><Icon name="repository" width="24" height="24" /></span>
                    <div className="flex min-w-0 flex-col gap-1">
                      <h2 className="text-xl font-semibold tracking-[-0.02em]">{repo.name}</h2>
                      <p className="truncate font-mono text-xs text-muted" title={repo.path}>{repo.path}</p>
                    </div>
                    <button type="button" className="ml-auto min-h-10 shrink-0 rounded-md border border-field-line px-3 text-sm hover:bg-hover" onClick={() => navigate({ screen: "config", repo: repo.name })}>{t("config")}</button>
                  </div>
                  <div className="repository-branches">
                    {branches.map((entry) => (
                      <button key={entry.branch} type="button" className="repository-branch" onClick={() => navigate({ screen: "branch", repo: repo.name, branch: entry.branch })}>
                        <Icon name="branch" width="17" height="17" className="shrink-0 text-muted" />
                        <span className="truncate font-mono text-sm">{entry.branch}</span>
                        <Icon name="chevron" width="14" height="14" className="ml-auto shrink-0 text-muted" />
                      </button>
                    ))}
                    {branches.length === 0 && <p className="py-2 text-sm text-muted">{t("noBranchesYet")}</p>}
                  </div>
                </div>
              );
            })}
          </section>
        ) : (
          <div className="workspace-empty">
            <span className="repository-symbol"><Icon name="repository" width="32" height="32" /></span>
            <h2 className="text-xl font-medium">{t("pointSoftFactoryAtAGitRepositoryOnThisMachine")}</h2>
            <PrimaryButton onClick={() => navigate({ screen: "addRepo" })}>{t("addRepository")}</PrimaryButton>
          </div>
        )}
      </div>
    );
  }
  if (route.screen === "addRepo") return <AddRepo onDone={repoAdded} />;
  if (route.screen === "config") {
    if (!repoPath) return <p>{t("repositoryFolderNotFound")}</p>;
    return <Config key={repoPath} repo={route.repo} repoPath={repoPath} />;
  }

  switch (route.screen) {
    case "branch":
      return repoPath ? (
        <BranchRuns
          key={`${repoPath}:${route.branch}`}
          repo={route.repo}
          repoPath={repoPath}
          branch={route.branch}
          worktreePath={branchesByPath[repoPath]?.find((entry) => entry.branch === route.branch)?.path}
          navigate={navigate}
        />
      ) : (
        <p>{t("repositoryFolderNotFound")}</p>
      );
    case "run":
      return repoPath ? (
        <RunDetail
          desktopRuns={desktopRuns}
          key={`${repoPath}:${route.branch}:${route.runId}`}
          repo={route.repo}
          repoPath={repoPath}
          branch={route.branch}
          runId={route.runId}
          navigate={navigate}
        />
      ) : (
        <p>{t("repositoryFolderNotFound")}</p>
      );
    case "newRun":
      return repoPath ? (
        <NewRun
          desktopRuns={desktopRuns}
          key={`${repoPath}:${route.branch}`}
          repo={route.repo}
          repoPath={repoPath}
          branch={route.branch}
          mode={route.mode}
          navigate={navigate}
        />
      ) : (
        <p>{t("repositoryFolderNotFound")}</p>
      );
  }
}

export default App;
