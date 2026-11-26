import type { ComponentProps } from "react";

type IconName = "repository" | "branch" | "plus" | "remove" | "settings" | "chevron" | "sidebar" | "folder" | "folder-open" | "file";

const paths: Record<IconName, string> = {
  sidebar: "M4 4h16v16H4ZM9 4v16",
  repository: "M5 4h13a1 1 0 0 1 1 1v15H6a3 3 0 0 1-3-3V6a2 2 0 0 1 2-2Zm-2 13a3 3 0 0 1 3-3h13M8 8h6",
  branch: "M6 7v10m0-6h7a5 5 0 0 0 5-5M8 5a2 2 0 1 1-4 0 2 2 0 0 1 4 0Zm0 14a2 2 0 1 1-4 0 2 2 0 0 1 4 0ZM20 4a2 2 0 1 1-4 0 2 2 0 0 1 4 0Z",
  remove: "M6 6l12 12M6 18 18 6",
  plus: "M12 5v14M5 12h14",
  settings: "M9 3h6l1 4 4 1v8l-4 1-1 4H9l-1-4-4-1V8l4-1 1-4Zm6 9a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z",
  chevron: "m9 5 7 7-7 7",
  folder: "M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z",
  "folder-open": "M3 17V7a2 2 0 0 1 2-2h4l2 2h7a2 2 0 0 1 2 2v1M5 19h13a2 2 0 0 0 2-1.5l2-6a1 1 0 0 0-1-1.5H8a2 2 0 0 0-2 1.4l-2.7 6A1 1 0 0 0 5 19Z",
  file: "M6 3h8l4 4v14H6ZM14 3v5h4",
};

export function Icon({ name, ...props }: ComponentProps<"svg"> & { name: IconName }) {
  return (
    <svg aria-hidden="true" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" {...props}>
      <path d={paths[name]} />
    </svg>
  );
}
