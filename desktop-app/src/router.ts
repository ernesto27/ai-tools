// Minimal in-memory routing: a desktop app has no URL bar, so screens are
// selected by state instead of a router library.

export type NewRunMode = "run" | "continue" | "resume";

export type Route =
  | { screen: "branch"; repo: string; branch: string }
  | { screen: "run"; repo: string; branch: string; runId: string }
  | { screen: "config"; repo: string; branch: string }
  | { screen: "newRun"; repo: string; branch: string; mode: NewRunMode }
  | { screen: "addRepo" };

export type Navigate = (route: Route) => void;
