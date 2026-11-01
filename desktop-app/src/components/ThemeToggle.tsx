import type { Theme } from "../theme";
import { cx } from "./ui";

interface Props {
  theme: Theme;
  onToggle: () => void;
}

export function ThemeToggle({ theme, onToggle }: Props) {
  const dark = theme === "dark";
  return (
    <button
      type="button"
      role="switch"
      aria-label="Dark mode"
      aria-checked={dark}
      onClick={onToggle}
      className="flex min-h-9 w-full items-center justify-between gap-2 rounded-md px-2 text-sm hover:bg-hover"
    >
      <span className={dark ? "text-muted" : "text-ink"}>Light</span>
      <span aria-hidden="true" className="flex h-5 w-9 shrink-0 items-center rounded-full border border-field-line bg-selected p-0.5">
        <span className={cx("size-3 rounded-full bg-ink", dark && "translate-x-4")} />
      </span>
      <span className={dark ? "text-ink" : "text-muted"}>Dark</span>
    </button>
  );
}
