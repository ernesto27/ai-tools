import { useEffect, useState } from "react";
import { applyTheme, readThemePreference, THEME_KEY, type Theme } from "../theme";

export function useTheme() {
  const [preference, setPreference] = useState<Theme | null>(readThemePreference);
  const [systemTheme, setSystemTheme] = useState<Theme>(() =>
    window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light",
  );
  const theme = preference ?? systemTheme;

  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const update = () => setSystemTheme(media.matches ? "dark" : "light");
    media.addEventListener("change", update);
    update();
    return () => media.removeEventListener("change", update);
  }, []);

  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  const toggleTheme = () => {
    const next: Theme = theme === "dark" ? "light" : "dark";
    applyTheme(next);
    setPreference(next);
    try {
      window.localStorage.setItem(THEME_KEY, next);
    } catch (error) {
      console.warn("Could not save appearance preference", error);
    }
  };

  return { theme, toggleTheme };
}
