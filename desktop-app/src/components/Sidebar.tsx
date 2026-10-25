import { useEffect, useState, type ComponentProps } from "react";
import { invoke } from "@tauri-apps/api/core";
import { STATUS_COLOR } from "../data";
import type { Navigate, Route } from "../router";
import { Dot } from "./Dot";
import { cx } from "./ui";

interface Props {
  repos: SavedRepo[];
  route: Route;
  navigate: Navigate;
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
        "flex w-full items-center gap-2 rounded-md px-2 text-left text-muted hover:text-ink",
        tall ? "min-h-9" : "min-h-8",
        "aria-expanded:text-ink aria-[current=page]:bg-selected aria-[current=page]:font-medium aria-[current=page]:text-ink",
        className,
      )}
      {...props}
    />
  );
}

export function Sidebar({ repos, route, navigate }: Props) {
  const activeRepo = route.screen === "addRepo" ? null : route.repo;
  const activeBranch = route.screen === "addRepo" ? null : route.branch;
  const [openRepo, setOpenRepo] = useState(activeRepo ?? repos[0]?.name ?? "");
  const [branchesByPath, setBranchesByPath] = useState<Record<string, string[]>>({});

  useEffect(() => {
    for (const repo of repos) {
      invoke<string[]>("list_branches", { path: repo.path })
        .then((branches) => setBranchesByPath((prev) => ({ ...prev, [repo.path]: branches })))
        .catch(console.error);
    }
  }, [repos]);

  // Which branch sub-item (Runs / Continue / Config) is current.
  const sub =
    route.screen === "config" ? "config" : route.screen === "newRun" && route.mode !== "run" ? "continue" : route.screen === "newRun" ? null : "runs";

  return (
    <nav aria-label="Main" className="flex w-nav flex-none flex-col gap-0.5 overflow-y-auto border-r border-line px-4 py-7 text-base">
      <span className="px-2 pb-7 font-mono text-md font-medium">soft·factory</span>
      <NavItem tall aria-current={route.screen === "addRepo" ? "page" : undefined} onClick={() => navigate({ screen: "addRepo" })}>
        Add repository
      </NavItem>

      <span className="px-2 pt-7 pb-2 text-2xs tracking-[0.08em] text-muted uppercase">Repositories</span>
      <div className="flex flex-col gap-2">
        {repos.map((saved) => {
          const branches = (branchesByPath[saved.path] ?? []).map((name) => ({ name, status: "idle" as const }));
          const repo = { name: saved.name, status: "idle" as const, branches };
          const isOpen = openRepo === repo.name;
          return (
            <div key={saved.path} className="flex flex-col">
              <button
                type="button"
                aria-expanded={isOpen}
                onClick={() => setOpenRepo(isOpen ? "" : repo.name)}
                className="flex min-h-8.5 w-full items-center justify-between rounded-md px-2 text-left text-muted hover:text-ink aria-expanded:font-medium aria-expanded:text-ink"
              >
                <span className="min-w-0 truncate" title={repo.name}>
                  {repo.name}
                </span>
                <Dot color={STATUS_COLOR[repo.status]} />
              </button>

              {isOpen && (
                <div className="flex flex-col gap-px pl-3">
                  {repo.branches.length === 0 && <span className="px-2 py-1 text-sm text-faint">No branches yet</span>}
                  {repo.branches.map((b) => {
                    const expanded = activeRepo === repo.name && activeBranch === b.name;
                    const at = { repo: repo.name, branch: b.name };
                    return (
                      <div key={b.name} className="flex flex-col">
                        <NavItem
                          className="font-mono text-sm"
                          title={b.name}
                          aria-expanded={expanded}
                          onClick={() => navigate({ screen: "branch", ...at })}
                        >
                          <Dot color={STATUS_COLOR[b.status]} />
                          <span className="min-w-0 truncate">{b.name}</span>
                        </NavItem>
                        {expanded && (
                          <div className="ml-[11px] flex flex-col gap-px border-l border-line pl-2.5 text-md">
                            <NavItem aria-current={sub === "runs" ? "page" : undefined} onClick={() => navigate({ screen: "branch", ...at })}>
                              Runs
                            </NavItem>
                            <NavItem
                              aria-current={sub === "continue" ? "page" : undefined}
                              onClick={() => navigate({ screen: "newRun", ...at, mode: "continue" })}
                            >
                              Continue
                            </NavItem>
                            <NavItem aria-current={sub === "config" ? "page" : undefined} onClick={() => navigate({ screen: "config", ...at })}>
                              Config
                            </NavItem>
                          </div>
                        )}
                      </div>
                    );
                  })}
                  <NavItem
                    className="text-md"
                    aria-current={route.screen === "newRun" && route.mode === "run" && route.repo === repo.name ? "page" : undefined}
                    onClick={() =>
                      navigate({ screen: "newRun", repo: repo.name, branch: repo.branches[0]?.name ?? "factory-task", mode: "run" })
                    }
                  >
                    + New run
                  </NavItem>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </nav>
  );
}
