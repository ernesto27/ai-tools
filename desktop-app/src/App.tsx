import { useCallback, useEffect, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { ask } from "@tauri-apps/plugin-dialog";
import type { Route } from "./router";
import { Sidebar, type SavedRepo } from "./components/Sidebar";
import { BranchRuns } from "./screens/BranchRuns";
import { RunDetail } from "./screens/RunDetail";
import { Config } from "./screens/Config";
import { NewRun } from "./screens/NewRun";
import { AddRepo } from "./screens/AddRepo";
import { useDesktopRuns, type DesktopRuns } from "./hooks/useDesktopRuns";
import { Title } from "./components/ui";

const HOME: Route = { screen: "home" };

function App() {
  const [route, setRoute] = useState<Route>(HOME);
  const lastBranchRoute = useRef<Route>(HOME);
  const mainRef = useRef<HTMLElement>(null);
  const [savedRepos, setSavedRepos] = useState<SavedRepo[]>([]);
  const [branchesByPath, setBranchesByPath] = useState<Record<string, string[]>>({});
  const savedPaths = useRef(new Set<string>());
  const branchRequests = useRef(new Map<string, symbol>());
  savedPaths.current = new Set(savedRepos.map((repo) => repo.path));

  const refreshBranches = useCallback(async (path: string) => {
    const request = Symbol();
    branchRequests.current.set(path, request);
    try {
      const branches = await invoke<string[]>("list_branches", { path });
      if (savedPaths.current.has(path) && branchRequests.current.get(path) === request) {
        setBranchesByPath((previous) => ({ ...previous, [path]: branches }));
      }
    } catch (error) {
      console.error(error);
    }
  }, []);

  function addBranch(path: string, branch: string) {
    if (!savedPaths.current.has(path)) return;
    branchRequests.current.delete(path);
    setBranchesByPath((previous) => {
      const branches = previous[path] ?? [];
      return branches.includes(branch) ? previous : { ...previous, [path]: [branch, ...branches] };
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
        `Are you sure to remove "${repo.name}"?`,
        { title: "Delete project", kind: "warning", okLabel: "Delete", cancelLabel: "Cancel" },
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
    if (route.screen !== "addRepo" && route.screen !== "config") lastBranchRoute.current = route;
    setRoute(next);
    mainRef.current?.scrollTo({ top: 0 });
  };

  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar repos={savedRepos} branchesByPath={branchesByPath} onRefreshBranches={refreshBranches} route={route} navigate={navigate} onRemoveRepo={removeRepo} />
      <main
        ref={mainRef}
        className={`min-w-0 flex-auto px-14 pt-10 ${route.screen === "run" ? "overflow-hidden pb-6" : "overflow-y-auto pb-16"}`}
      >
         <Screen
          desktopRuns={desktopRuns}
          route={route}
          repoPath={
          "repo" in route
            ? savedRepos.find((repo) => repo.name === route.repo)?.path
            : undefined
          }
          navigate={navigate}
          back={() => navigate(lastBranchRoute.current)}
          repoAdded={() => {
            loadRepos();
            navigate(lastBranchRoute.current);
          }}
        />
      </main>
    </div>
  );
}

function Screen({
  desktopRuns,
  route,
  navigate,
  repoAdded,
  repoPath,
}: {
  desktopRuns: DesktopRuns;
  route: Route;
  navigate: (r: Route) => void;
  back: () => void;
  repoAdded: () => void;
  repoPath?: string;
}) {
  if (route.screen === "home") {
    return (
      <div className="flex flex-col gap-3">
        <Title>Select a repository</Title>
        <p className="text-muted">Choose a repository from the sidebar to view its branches and runs.</p>
      </div>
    );
  }
  if (route.screen === "addRepo") return <AddRepo onDone={repoAdded} />;
  if (route.screen === "config") {
    if (!repoPath) return <p>Repository folder not found.</p>;
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
          navigate={navigate}
        />
      ) : (
        <p>Repository folder not found.</p>
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
        <p>Repository folder not found.</p>
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
        <p>Repository folder not found.</p>
      );
  }
}

export default App;
