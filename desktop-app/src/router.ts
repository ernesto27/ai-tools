// Minimal in-memory routing: a desktop app has no URL bar, so screens are
// selected by state instead of a router library.

export const RUN_MODE = {
  RUN: "run",
  CONTINUE: "continue",
  RESUME: "resume",
} as const;

export type NewRunMode = (typeof RUN_MODE)[keyof typeof RUN_MODE];
export type FactoryRunMode = typeof RUN_MODE.RUN | typeof RUN_MODE.CONTINUE;

export type Route =
  | { screen: "home" }
  | { screen: "branch"; repo: string; branch: string }
  | { screen: "run"; repo: string; branch: string; runId: string }
  | { screen: "config"; repo: string }
  | { screen: "newRun"; repo: string; branch: string; mode: NewRunMode }
  | { screen: "addRepo" };

export type Navigate = (route: Route) => void;
