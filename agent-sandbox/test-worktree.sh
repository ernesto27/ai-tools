#!/usr/bin/env bash
# Runs every agent-sandbox worktree command and its normal execution flags.
# The default path uses claude on golang:1.26-alpine and covers create, resume,
# list, editor, single delete, and bulk delete. It shows that the state file,
# not git, decides what the verbs can see.
# Nothing is asserted: read the output.
# Generated commit messages are displayed after run and resume. Publication
# stays disabled, so commit fallback and post-commit cleanup are not exercised.
#
# Needs Docker and an authenticated claude. Point XDG_CONFIG_HOME at a scratch
# directory to keep the real ~/.config/agent-sandbox out of it.
# TEST_EDITOR=1 opens VS Code for the resumed worktree. TEST_CLAUDE_IMAGES=1
# or TEST_CODEX_IMAGES=1 needs the corresponding authenticated agent and
# verifies actual --image attachment handling.
# JSON PR defaults are overridden with --pr=false on run and resume. No
# GitHub publication is tested, and these sessions leave their changes local.
#
# Usage: ./test-worktree.sh

set -uo pipefail

cd -- "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"

# Package-path invocation also works from the temporary directories below.
sandbox=(go run agent-sandbox/cmd/agent-sandbox)
state="${XDG_CONFIG_HOME:-$HOME/.config}/agent-sandbox/worktrees.jsonl"
agent="${AGENT:-claude}"
model="${MODEL:-opus}"
base_image="${BASE_IMAGE:-golang:1.26-alpine}"
prompt_file="$(mktemp)"
image_file="$(mktemp --suffix=.png)"
clean_dir="$(mktemp -d "$PWD/.test-worktree-clean.XXXXXX")"
config_dir="$(mktemp -d "$PWD/.test-worktree-config.XXXXXX")"

# A real 1x1 PNG exercises the agent's visual attachment path. An empty file
# would only prove that Docker mounted something, not that Claude or Codex can
# decode an image.
base64 --decode >"$image_file" <<'PNG'
iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9Jx60AAAAASUVORK5CYII=
PNG

cleanup() {
  rm -f -- "$prompt_file" "$image_file" "$config_dir/agent-sandbox.json"
  rmdir -- "$clean_dir" "$config_dir"
}
trap cleanup EXIT

execute_from() {
  local directory="$1"
  shift
  printf '\n$ (cd %s && agent-sandbox %s)\n' "$directory" "$*"
  (cd -- "$directory" && "${sandbox[@]}" "$@" </dev/null)
  printf '[exit %d]\n' "$?"
}

execute() {
  execute_from "$clean_dir" "$@"
}

# Help is checked for every command, but successful help pages are too noisy
# for a lifecycle run. Keep the full output visible when a check fails.
check_help() {
  local output status command="agent-sandbox"
  if (($# > 0)); then
    command+=" $*"
  fi
  printf '\n$ %s --help\n' "$command"
  output="$(cd -- "$clean_dir" && "${sandbox[@]}" "$@" --help </dev/null 2>&1)"
  status=$?
  if ((status == 0)); then
    printf '[help OK]\n'
    return
  fi
  printf '%s\n' "$output"
  printf '[exit %d]\n' "$status"
  exit "$status"
}

# git is run directly for the setup a sandbox run would never do.
plain_git() {
  printf '\n$ git %s\n' "$*"
  git "$@"
  printf '[exit %d]\n' "$?"
}

show_state() {
  printf '\n$ cat %s\n' "$state"
  cat -- "$state" 2>/dev/null || printf '(no state file)\n'
}

execute --version
check_help
check_help run
check_help resume
check_help worktree-list
check_help worktree-editor
check_help worktree-delete
check_help worktree-delete-all

# The initial run builds or reuses the image derived from golang:1.26-alpine,
# creates tmp-run-test, and appends one line to the state file. It must show Go
# 1.26 before creating hello.txt. Its flags and prompt come from the JSON in
# the invocation directory; the clean directory used by other commands has no
# JSON and cannot inherit the caller's private agent-sandbox.json.
# The JSON enables PRs so --pr=false must override it before checking gh or
# running the agent. Newly created state records also show the starting branch
# in base_branch, which must survive the resume below.
cat >"$config_dir/agent-sandbox.json" <<'JSON'
{
  "run": {
    "branch": "tmp-run-test",
    "agent": "claude",
    "model": "opus",
    "base-image": "golang:1.26-alpine",
    "pr": true,
    "push": false,
    "query": "run go version, then create a file named hello.txt at the repository root containing the text hello world"
  },
  "resume": {
    "branch": "tmp-run-test",
    "agent": "claude",
    "model": "opus",
    "base-image": "golang:1.26-alpine",
    "pr": true,
    "push": false,
    "query": "this default prompt should be overridden by -q"
  }
}
JSON
execute_from "$config_dir" run --pr=false
show_state

# Resume takes its branch and agent defaults from JSON while -q overrides the
# JSON prompt. It must see the uncommitted hello.txt from the first session,
# add a second file, and leave the state file with its original one line.
# Its PR default is disabled independently, just as on the initial run.
execute_from "$config_dir" resume --pr=false \
  -q "read hello.txt, then create resumed.txt at the repository root containing the text resumed successfully"
show_commit_message tmp-run-test
show_state

# --file-prompt takes the task from a regular host file. This run is kept for
# worktree-delete-all below. Explicit false publication flags also work when
# there is no JSON config in the invocation directory.
printf '%s\n' "create file-prompt.txt at the repository root containing file prompt works" >"$prompt_file"
execute run -b tmp-file-prompt-test -a "$agent" -m "$model" \
  --pr=false --push=false -f "$prompt_file"

# Show that ordinary deletion refuses a dirty worktree, then remove it with
# --force.
execute run -b tmp-delete-test -a "$agent" -m "$model" -i "$base_image" \
  -q "create dirty.txt at the repository root containing pending changes"
execute worktree-delete -b tmp-delete-test
execute worktree-delete -b tmp-delete-test --force

# Opening an editor is intentionally opt-in because it launches a host GUI.
if [[ "${TEST_EDITOR:-0}" == "1" ]]; then
  execute worktree-editor -b tmp-run-test
else
  printf '\n[skipping worktree-editor; set TEST_EDITOR=1 to launch VS Code]\n'
fi

# Claude receives mounted images as visual paths in its initial prompt. This is
# opt-in because the normal worktree lifecycle needs only existing Claude auth.
if [[ "${TEST_CLAUDE_IMAGES:-0}" == "1" ]]; then
  execute run -b tmp-claude-image-test -a claude -m opus -i golang:1.26-alpine \
    --image "$image_file" \
    -q "inspect the attached image as visual input, then create claude-image-flag.txt at the repository root containing image flag works"
else
  printf '\n[skipping Claude --image test; set TEST_CLAUDE_IMAGES=1]\n'
fi

# Codex uses its native image arguments. It stays opt-in because it needs its
# own host authentication in addition to the default Claude lifecycle.
if [[ "${TEST_CODEX_IMAGES:-0}" == "1" ]]; then
  execute run -b tmp-image-test -a codex --image "$image_file" \
    -q "create image-flag.txt at the repository root containing image flag works"
else
  printf '\n[skipping Codex --image test; set TEST_CODEX_IMAGES=1]\n'
fi

# A worktree the sandbox did not create. Git knows it, the state file does not,
# so nothing below may list, delete or even refuse to enter it.
plain_git worktree add -b tmp-hand-test ../tmp-hand-test

execute worktree-list                     # tmp-run-test only
execute worktree-delete -b tmp-hand-test  # refused: no sandbox worktree on that branch

# Resume follows the same state-file boundary: Git knows tmp-hand-test, but
# the sandbox never recorded it, so this must fail before Docker starts.
execute resume -b tmp-hand-test -a claude -m opus -i golang:1.26-alpine \
  -q "this must not start"

# Without --yes the prompt reads EOF from </dev/null, which counts as no.
execute worktree-delete-all
execute worktree-delete-all --yes

execute worktree-list                     # nothing left
show_state                            # and no line left either

plain_git worktree list               # tmp-hand-test survived it all

# Cleaning up the hand-made worktree by hand, which is now the only way.
plain_git worktree remove --force ../tmp-hand-test
plain_git branch -D tmp-hand-test
