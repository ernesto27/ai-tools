import { useRef, useState } from "react";
import { repos } from "./data";
import type { Route } from "./router";
import { Sidebar } from "./components/Sidebar";
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

  const navigate = (next: Route) => {
    if (route.screen !== "addRepo") lastBranchRoute.current = route;
    setRoute(next);
    mainRef.current?.scrollTo({ top: 0 });
  };

  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar route={route} navigate={navigate} />
      <main ref={mainRef} className="min-w-0 flex-auto overflow-y-auto px-14 pt-10 pb-16">
        <Screen route={route} navigate={navigate} back={() => navigate(lastBranchRoute.current)} />
      </main>
    </div>
  );
}

function Screen({ route, navigate, back }: { route: Route; navigate: (r: Route) => void; back: () => void }) {
  if (route.screen === "addRepo") return <AddRepo onDone={back} />;

  const branch = repos.find((r) => r.name === route.repo)?.branches.find((b) => b.name === route.branch);

  switch (route.screen) {
    case "branch":
      return branch ? <BranchRuns key={branch.name} repo={route.repo} branch={branch} navigate={navigate} /> : null;
    case "run":
      return <RunDetail key={route.runId} repo={route.repo} branch={route.branch} navigate={navigate} />;
    case "config":
      return <Config key={route.branch} repo={route.repo} branch={route.branch} />;
    case "newRun":
      return <NewRun key={route.branch} repo={route.repo} branch={route.branch} mode={route.mode} navigate={navigate} />;
  }
}

export default App;
