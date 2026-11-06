import type { ComponentProps } from "react";

type IconName = "repository" | "branch" | "plus" | "settings" | "chevron" | "factory" | "sidebar";

const paths: Record<IconName, string> = {
  sidebar: "M4 4h16v16H4ZM9 4v16",
  repository: "M5 4h13a1 1 0 0 1 1 1v15H6a3 3 0 0 1-3-3V6a2 2 0 0 1 2-2Zm-2 13a3 3 0 0 1 3-3h13M8 8h6",
  branch: "M6 7v10m0-6h7a5 5 0 0 0 5-5M8 5a2 2 0 1 1-4 0 2 2 0 0 1 4 0Zm0 14a2 2 0 1 1-4 0 2 2 0 0 1 4 0ZM20 4a2 2 0 1 1-4 0 2 2 0 0 1 4 0Z",
  plus: "M12 5v14M5 12h14",
  settings: "M9 3h6l1 4 4 1v8l-4 1-1 4H9l-1-4-4-1V8l4-1 1-4Zm6 9a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z",
  chevron: "m9 5 7 7-7 7",
  factory: "M4 19V9l8 4V5l8 4v10H4ZM8 16h1m6 0h1",
};

export function Icon({ name, ...props }: ComponentProps<"svg"> & { name: IconName }) {
  return (
    <svg aria-hidden="true" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" {...props}>
      <path d={paths[name]} />
    </svg>
  );
}
