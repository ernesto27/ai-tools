#!/usr/bin/env bash
# Prints a change set for review: a per-file line count, then every file's
# unified diff. Untracked files are included as all-added (working tree only).
#
# Usage: changes.sh [--staged | <commit> | <range>] [-- <paths>...]
#   (nothing)       working tree and index against HEAD, untracked included
#   --staged        the index against HEAD
#   HEAD~1          that commit to the working tree
#   main..feature   a range
set -euo pipefail

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "NOT_A_GIT_REPO"
  exit 0
fi

target=""
if [[ $# -gt 0 && $1 != "--" ]]; then
  target=$1
  shift
fi
[[ ${1:-} == "--" ]] && shift
paths=("$@")

# A repository with no commits yet diffs against the empty tree.
if git rev-parse --verify -q HEAD >/dev/null; then
  base=HEAD
else
  base=$(git hash-object -t tree /dev/null)
fi

case $target in
  "") args=("$base") ;;
  --staged) args=(--cached "$base") ;;
  *) args=("$target") ;;
esac

untracked=()
if [[ -z $target ]]; then
  while IFS= read -r -d '' file; do
    untracked+=("$file")
  done < <(git ls-files --others --exclude-standard -z -- "${paths[@]}")
fi

echo "== FILES (added removed path) =="
git diff --numstat "${args[@]}" -- "${paths[@]}"
for file in "${untracked[@]}"; do
  if grep -Iq . "$file" 2>/dev/null; then
    echo "$(wc -l <"$file" | tr -d ' ')	0	$file (untracked)"
  else
    echo "-	-	$file (untracked, binary or empty)"
  fi
done

echo
echo "== DIFF =="
git diff --no-color "${args[@]}" -- "${paths[@]}"
for file in "${untracked[@]}"; do
  # --no-index exits 1 when the files differ, which they always do here.
  git diff --no-color --no-index -- /dev/null "$file" || true
done
