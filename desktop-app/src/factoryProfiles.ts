import { invoke } from "@tauri-apps/api/core";
import { useEffect, useState } from "react";
import { parseFactoryConfig } from "./config";
import { toUiMessage, type UiMessage } from "./messages";

export interface FactoryProfile {
  id: string;
  name: string;
}

export async function loadFactoryConfiguration(path: string, id: string) {
  if (id) return parseFactoryConfig(await invoke<unknown>("load_factory_profile", { path, id }));
  const value = await invoke<unknown>("load_factory_config", { path });
  return value === null ? null : parseFactoryConfig(value);
}

export function useFactoryProfiles(path: string) {
  const [profiles, setProfiles] = useState<FactoryProfile[]>([]);
  const [error, setError] = useState<UiMessage | null>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    setError(null);
    invoke<FactoryProfile[]>("list_factory_profiles", { path })
      .then((rows) => { if (active) setProfiles(rows); })
      .catch((failure) => { if (active) setError(toUiMessage(failure)); });
    return () => { active = false; };
  }, [path, attempt]);

  return { profiles, error, reload: () => setAttempt((value) => value + 1) };
}
