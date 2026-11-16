import { useEffect, useSyncExternalStore } from "react";
import { invoke } from "@tauri-apps/api/core";

export type DependencyStatus = "checking" | "missing" | "installing" | "ready" | "error" | "declined";
export type Dependency = {
  id: "agent-sandbox" | "software-factory";
  status: DependencyStatus;
  error?: string;
};

const tools = [
  { id: "agent-sandbox", check: "agent_sandbox_check_bin", install: "agent_sandbox_install_bin", update: "agent_sandbox_check_and_update" },
  { id: "software-factory", check: "software_factory_check_bin", install: "software_factory_install_bin", update: "software_factory_check_and_update" },
] as const;

let snapshot: readonly Dependency[] = tools.map(({ id }) => ({ id, status: "checking" }));
let started = false;
let updatesStarted = false;
const listeners = new Set<() => void>();

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function getSnapshot() {
  return snapshot;
}

function update(id: Dependency["id"], status: DependencyStatus, error?: string) {
  snapshot = snapshot.map((row) => row.id === id ? { id, status, error } : row);
  for (const listener of listeners) listener();

  if (!updatesStarted && snapshot.every((row) => row.status === "ready")) {
    updatesStarted = true;
    for (const tool of tools) void invoke<void>(tool.update).catch(() => undefined);
  }
}

async function checkTool(tool: typeof tools[number]) {
  try {
    const exists = await invoke<boolean>(tool.check);
    update(tool.id, exists ? "ready" : "missing");
  } catch (error) {
    update(tool.id, "error", String(error));
  }
}

async function installTool(tool: typeof tools[number]) {
  update(tool.id, "installing");
  try {
    await invoke<void>(tool.install);
    update(tool.id, "ready");
  } catch (error) {
    update(tool.id, "error", String(error));
  }
}

function canConfirm() {
  return !snapshot.some((row) => row.status === "checking")
    && snapshot.some((row) => row.status === "missing");
}

function acceptInstallation() {
  if (!canConfirm()) return;
  const missing = tools.filter((tool) => snapshot.some((row) => row.id === tool.id && row.status === "missing"));
  for (const tool of missing) void installTool(tool);
}

function declineInstallation() {
  if (!canConfirm()) return;
  for (const row of snapshot) {
    if (row.status === "missing") update(row.id, "declined");
  }
}

function canRecheck() {
  const busy = snapshot.some(
    (row) => row.status === "checking" || row.status === "installing",
  );
  const recoverable = snapshot.some(
    (row) => row.status === "error" || row.status === "declined",
  );

  return recoverable && !busy;
}

function recheck() {
  if (!canRecheck()) return;

  const pending = tools.filter((tool) =>
    snapshot.some((row) => row.id === tool.id && row.status !== "ready"),
  );

  // Mark all pending tools before launching checks so repeated clicks cannot overlap.
  for (const tool of pending) update(tool.id, "checking");
  for (const tool of pending) void checkTool(tool);
}

function start() {
  if (started) return;
  started = true;
  for (const tool of tools) void checkTool(tool);
}

export function useDependencySetup() {
  const dependencies = useSyncExternalStore(subscribe, getSnapshot);
  useEffect(start, []);
  return {
    dependencies,
    needsConfirmation: canConfirm(),
    acceptInstallation,
    declineInstallation,
    canRecheck: canRecheck(),
    recheck,
    ready: dependencies.every((dependency) => dependency.status === "ready"),
  };
}
