# Repository Guidelines

## Project Structure & Module Organization

This repository contains a Go CLI that coordinates implementation, code review, security review, and risk classification through `agent-sandbox`.

- `cmd/factory/main.go`: CLI entry point and workflow sequencing.
- `cmd/factory/commands.go`: Cobra command tree and flags.
- `internal/config/`: JSON configuration loading and validation.
- `internal/taskcontext/`: supporting-document resolution and prompt assembly.
- `internal/sandbox/`: subprocess execution, review prompts, and report storage.
- `skills/`: Markdown instructions for code review, security review, and risk classification.
- `task.txt`: example task input. Generated reports go into ignored `docs/`.

## Build, Test, and Development Commands

Use Go 1.26.0 or later. Run commands from the repository root so relative skill paths resolve.

- `go build -o /tmp/software-factory ./cmd/factory`: build the CLI without adding a repository artifact.
- `go test ./...`: run all package tests.
- `go vet ./...`: check for common Go mistakes.
- `gofmt -w cmd internal`: format Go source.
- `go run ./cmd/factory`: run implementation and all review stages.
- `go run ./cmd/factory review`: run reviews and risk classification on existing changes; reviews can apply corrections.

Workflow commands require `agent-sandbox` on `PATH` and local configuration.

## Coding Style & Naming Conventions

Follow idiomatic Go and use `gofmt` tab indentation. Keep package names lowercase, exported identifiers in PascalCase, and unexported identifiers in camelCase. Place reusable logic under `internal/`; keep CLI orchestration in `cmd/factory`. Return contextual errors and wrap underlying errors with `%w`.

Use unnamed return types and explicit values in return statements. Avoid named return parameters and bare returns.

## Testing Guidelines

Use the existing tests; there is no numeric coverage threshold. Add standard-library `testing` tests beside the relevant source as `*_test.go`, with functions named `TestXxx`. Prefer table-driven cases for configuration validation and document-path handling. Use temporary files and controlled subprocess fixtures when testing filesystem or execution behavior. Run `go test ./...` and `go vet ./...` before submitting code changes.

## Commit & Pull Request Guidelines

Use concise, imperative commit subjects such as `Add configuration validation tests`. Keep changes focused. PR descriptions should explain behavior changes, list verification commands and results, and link relevant issues. Describe prompt or workflow changes explicitly.

## Release Tags

Always use `software-factory-vMAJOR.MINOR.PATCH` for Soft Factory release tags. Select the latest stable tag with this exact prefix and increment its patch version. Do not use unprefixed `vMAJOR.MINOR.PATCH` tags for this CLI. GitHub release workflows run for tags matching `software-factory-v*`.

## Configuration & Reports

Maintain only project-root `software-factory.json`. Embed the full upstream sandbox object under `agentSandbox` (for example `{"documents":[],"agentSandbox":{"agent":"codex","branch":"task","run":{"filePrompt":"task.txt"}}}`). Reuse the imported Agent Sandbox `config.Config`; Factory validates only its own fields. Keep stage overrides under `stages`.

This is a breaking change: manually move the old `agent-sandbox.json` object into `agentSandbox`; never read, merge, or migrate the legacy file automatically. Factory passes sandbox settings from the parsed `config.Config` struct to `agent-sandbox` as CLI flags (`internal/sandbox/args.go`); it never creates, moves, renames, or hides an `agent-sandbox.json` and keeps no runtime configuration directory. `apiKey` and `reviewers` have no CLI flags and are ignored for now. Because `agent-sandbox` still reads `./agent-sandbox.json` from the project root, Factory only warns when that file exists. Ignore both configuration filenames and reports; keep configuration and credentials out of commits, logs, and change evidence. All relative task/sandbox paths use the project root. The task remains `agentSandbox.run.filePrompt` even for resume-based workflows.
