import { useEffect, useRef, useState } from "react";
import { invoke, isTauri } from "@tauri-apps/api/core";

type AvailableUpdate = { version: string };

// Share the promise across React StrictMode effect remounts. Rust also checks once.
let startupCheck: Promise<AvailableUpdate | null> | undefined;

export function useDesktopUpdater() {
  const [update, setUpdate] = useState<AvailableUpdate | null>(null);
  const [installing, setInstalling] = useState(false);
  const installStarted = useRef(false);

  useEffect(() => {
    if (!isTauri()) return;
    let active = true;
    startupCheck ??= invoke<AvailableUpdate | null>("check_desktop_update").catch(() => null);
    void startupCheck.then((candidate) => {
      if (active) setUpdate(candidate);
    });
    return () => { active = false; };
  }, []);

  async function install() {
    if (!update || installStarted.current) return;
    installStarted.current = true;
    setInstalling(true);
    try {
      await invoke("install_desktop_update");
    } catch {
      // Failed updates are quiet; another attempt is made on the next launch.
      setUpdate(null);
    } finally {
      installStarted.current = false;
      setInstalling(false);
    }
  }

  return { update, installing, install, postpone: () => setUpdate(null) };
}

export type DesktopUpdater = ReturnType<typeof useDesktopUpdater>;
