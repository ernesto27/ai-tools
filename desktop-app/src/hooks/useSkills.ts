import { useEffect, useState } from "react";
import { invoke } from "@tauri-apps/api/core";

interface Skill {
  name: string;
}

interface SkillState {
  path: string;
  names: string[];
  loading: boolean;
  error: string | null;
}

export function useSkills(repoPath: string) {
  const [revision, setRevision] = useState(0);
  const [state, setState] = useState<SkillState>({
    path: repoPath, names: [], loading: true, error: null,
  });

  useEffect(() => {
    let active = true;
    setState({ path: repoPath, names: [], loading: true, error: null });
    invoke<Skill[]>("list_skills", { path: repoPath })
      .then((skills) => {
        if (!active) return;
        setState({
          path: repoPath,
          names: skills.map((skill) => skill.name).sort(),
          loading: false,
          error: null,
        });
      })
      .catch((error: unknown) => {
        if (active) {
          setState({ path: repoPath, names: [], loading: false, error: String(error) });
        }
      });
    return () => { active = false; };
  }, [repoPath, revision]);

  const current = state.path === repoPath
    ? state
    : { names: [], loading: true, error: null };
  return {
    names: current.names,
    loading: current.loading,
    error: current.error,
    retry: () => {
      setState({ path: repoPath, names: [], loading: true, error: null });
      setRevision((value) => value + 1);
    },
  };
}
