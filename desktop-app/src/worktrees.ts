export interface BranchWorktree {
  branch: string;
  path: string;
}

export function readDirtyWorktreeError(error: unknown): string | undefined {
  if (typeof error === "object" && error !== null && "dirtyWorktreeError" in error
    && typeof error.dirtyWorktreeError === "string") {
    return error.dirtyWorktreeError;
  }
}
