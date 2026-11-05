import { invoke, isTauri } from "@tauri-apps/api/core";
import i18n from "./i18n";
import { RUN_STATE } from "./runState";

const RUN_NOTIFICATION_KEY = {
  [RUN_STATE.SUCCEEDED]: "runSucceededNotification",
  [RUN_STATE.FAILED]: "runFailedNotification",
} as const;

interface RunCompletion {
  repo: string;
  branch: string;
  state: keyof typeof RUN_NOTIFICATION_KEY;
}

export async function notifyRunCompletion(run: RunCompletion): Promise<void> {
  if (!isTauri()) return;

  try {
    await invoke<void>("plugin:notification|notify", {
      options: {
        title: "Soft Factory",
        body: i18n.t(
          RUN_NOTIFICATION_KEY[run.state],
          { repo: run.repo, branch: run.branch },
        ),
      },
    });
  } catch (error) {
    console.warn("Could not show run completion notification", error);
  }
}
