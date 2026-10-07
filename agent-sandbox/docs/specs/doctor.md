# Spec: Doctor Command

Status: Implementation authorized through the implement skill invocation.

## Problem Statement

Users need a quick way to see which host tools are installed for
agent-sandbox. Discovering missing tools through a sandbox run is expensive:
execution can require credentials, contact npm, and build a Docker image.
Users want a simple installation list, without operational diagnostics or
automatic fixes.

## Solution

Add `agent-sandbox doctor`, which lists `git`, `docker`, `gh`, and `code` in
that order. Each tool receives green `installed` or red `not installed`
text according to whether its executable is discoverable through `PATH`.
Label `gh` as optional for PRs and `code` as optional for worktree-editor.

The command works from any directory, ignores agent-sandbox configuration,
and exits successfully even when dependencies are missing. It only prints
the list and statuses; it does not execute the tools or change the system.

## User Stories

1. As an agent-sandbox user, I want a dedicated doctor command, so that I can inspect installed host tools without starting a sandbox session.
2. As an agent-sandbox user, I want Git listed, so that I can see whether the tool used for repository and worktree operations is installed.
3. As an agent-sandbox user, I want Docker listed, so that I can see whether its command-line executable is installed.
4. As a user publishing PRs, I want GitHub CLI listed, so that I can see whether the host tool for PR publication is installed.
5. As a user opening worktrees in VS Code, I want the code executable listed, so that I can see whether the editor launcher is installed.
6. As an agent-sandbox user, I want optional tools labeled with their features, so that I understand which operations need them.
7. As an agent-sandbox user, I want every listed tool to have an installed or not installed status, so that the report is easy to scan.
8. As an agent-sandbox user, I want installed statuses in green, so that available tools are easy to recognize.
9. As an agent-sandbox user, I want missing statuses in red, so that missing tools stand out.
10. As an agent-sandbox user, I want installation checks to use PATH, so that the report reflects executable discovery in my current environment.
11. As an agent-sandbox user, I want the complete list even when tools are missing, so that I can see all installation gaps at once.
12. As an agent-sandbox user, I want doctor to work outside a Git repository, so that I can inspect my installation before choosing a project.
13. As an agent-sandbox user, I want doctor to ignore project configuration, so that prompt, agent, and API-key settings do not affect the report.
14. As an agent-sandbox user, I want missing tools to leave the command successful, so that installation statuses remain informational.
15. As an agent-sandbox user, I want checks without executing dependency programs, so that the report does not launch tools or depend on their behavior.
16. As an agent-sandbox user, I want doctor to leave my system unchanged, so that inspection does not install tools or alter configuration.
17. As an agent-sandbox user, I want doctor to avoid service and authentication checks, so that it reports installation without requiring working services or credentials.
18. As an agent-sandbox user, I want host checks limited to the agreed tools, so that container-provided runtimes and agents are not reported as missing host dependencies.

## Implementation Decisions

- Add a no-argument doctor subcommand to the existing Cobra CLI. Preserve existing command dispatch and usage-error conventions.
- Keep installation checking in the sandbox package and CLI presentation in the command package, preserving the existing dependency direction.
- Discover executable availability through Go's standard PATH lookup. Do not invoke version commands, probe services, or perform network requests.
- Always list git, docker, gh, and code, in that order, independently of installed state or project configuration.
- Label gh as optional for PRs and code as optional for worktree-editor.
- Present the list under a Dependency checks heading with aligned Dependency, Status, and Usage columns. Mark core tools as Required and optional tools with their feature names. Do not add versions, paths, remediation instructions, or a diagnostic summary.
- Emit green/red ANSI status colors unconditionally, including when redirected or NO_COLOR is set. Reset the color after each status.
- Missing dependencies do not cause a nonzero exit status. Preserve normal error handling for invalid invocations or output failures.
- Do not load project configuration, resolve a Git repository, construct a Docker client, inspect images, or access agent credentials.
- Document that the Docker CLI is only an installation indicator. Sandbox execution uses the Engine API, so a missing CLI does not establish that the engine is unavailable, and a present CLI does not establish that it is usable.
- No new third-party dependency, persistence format, or configuration field is needed.

## Testing Decisions

Testing seam: exercise the doctor command through the
existing root-command execution boundary, capture output and command status,
and control PATH using temporary directories. This keeps tests at one public
command seam rather than introducing separate mocks for each tool.

- Test externally visible behavior, rather than helper names, internal data structures, or the number of lookup calls.
- Use table-driven cases for all tools present, all absent, and mixed availability. Include a non-executable file, which must not count as installed on Linux.
- Provide harmless executable fixtures that would record invocation if run; verify doctor never runs them.
- Check all four rows, their order, optional labels, exact status text, ANSI colors, and color resets.
- Check successful completion when required or optional tools are missing.
- Execute from a temporary directory outside a Git repository containing invalid project configuration; the report must still succeed.
- Verify colors remain present with captured output and NO_COLOR set.
- Verify positional arguments and unsupported flags follow the existing usage-error convention, and doctor is discoverable in command help.
- Prior art is the existing root-command and configuration tests, which construct Cobra commands, execute or validate them, and use temporary filesystem fixtures.
- Do not use a manual sandbox run as verification. Implementation validation should use the standard Go build, vet, and test commands.

## Out of Scope

- Installing or upgrading dependencies, starting services, or changing configuration.
- Docker daemon connectivity, permissions, contexts, or health checks.
- Credential presence, authentication validity, Git identity, repository state, or remote validation.
- Version checks, minimum supported versions, or npm/network access.
- Host checks for Node.js, npm, Go, or agent executables.
- Container creation, image inspection or builds, and worktree changes.
- Machine-readable output, verbosity options, remediation messages, or color-control flags.
- Implementation work during this specification task.

## Further Notes

The installation-only behavior intentionally does not certify that a sandbox
run will succeed. Node.js, npm, and agent executables run in containers;
Go is needed to build from source, rather than to use a release binary.

Implementation proceeds locally through the implement skill; issue-tracker
publication is outside this implementation task.
