import { useEffect, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import type { Route } from "./router";
import { Sidebar, type SavedRepo } from "./components/Sidebar";
import { BranchRuns } from "./screens/BranchRuns";
import { RunDetail } from "./screens/RunDetail";
import { Config } from "./screens/Config";
import { NewRun } from "./screens/NewRun";
import { AddRepo } from "./screens/AddRepo";

const HOME: Route = { screen: "branch", repo: "soft-factory", branch: "feature-logs" };

function App() {
  const [route, setRoute] = useState<Route>(HOME);
  const lastBranchRoute = useRef<Route>(HOME);
  const mainRef = useRef<HTMLElement>(null);
  const [savedRepos, setSavedRepos] = useState<SavedRepo[]>([]);

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
      <Sidebar repos={savedRepos} route={route} navigate={navigate} />
      <main ref={mainRef} className="min-w-0 flex-auto overflow-y-auto px-14 pt-10 pb-16">
         <Screen
          route={route}
          repoPath={
          route.screen !== "addRepo"
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
  route,
  navigate,
  repoAdded,
  repoPath,
}: {
  route: Route;
  navigate: (r: Route) => void;
  back: () => void;
  repoAdded: () => void;
  repoPath?: string;
}) {
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
      return <RunDetail key={route.runId} repo={route.repo} branch={route.branch} navigate={navigate} />;
    case "newRun":
      return repoPath ? (
        <NewRun
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
