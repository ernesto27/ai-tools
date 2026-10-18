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

Standard-library tests cover configuration and workflow boundaries; there is no coverage threshold. Add standard-library `testing` tests beside the relevant source as `*_test.go`, with functions named `TestXxx`. Prefer table-driven cases for configuration validation and document-path handling. Use temporary files and controlled subprocess fixtures when testing filesystem or execution behavior. Run `go test ./...` and `go vet ./...` before submitting code changes.

## Commit & Pull Request Guidelines

Use concise, imperative commit subjects such as `Add configuration validation tests`. Keep changes focused. PR descriptions should explain behavior changes, list verification commands and results, and link relevant issues. Describe prompt or workflow changes explicitly.

## Release Tags

Always use `software-factory-vMAJOR.MINOR.PATCH` for Soft Factory release tags. Select the latest stable tag with this exact prefix and increment its patch version. Do not use unprefixed `vMAJOR.MINOR.PATCH` tags for this CLI. GitHub release workflows run for tags matching `software-factory-v*`.

## Configuration & Reports

Create only local `software-factory.json`, for example `{"documents":[],"agentSandbox":{"agent":"codex","branch":"task","run":{"filePrompt":"task.txt"}}}`. Document and sandbox paths retain the project-root context. Breaking change: manually move the old `agent-sandbox.json` object into `agentSandbox`; Factory and its sandbox subprocesses no longer consume that file. Preserve existing stage assignments. The local files and `docs/` remain ignored; keep credentials and sensitive task content out of commits.

The embedded field uses upstream `config.Config`. Load it once per workflow and preserve the original JSON for downstream validation; sandbox semantic checks belong to Agent Sandbox. Invocation arguments (including independent stage agent/model overrides) win over section settings, which win over shared settings. All workflows retain the original local task at `agentSandbox.run.filePrompt`. The installed Agent Sandbox CLI must support explicit `--config` input; the Go schema dependency remains pinned because its exported types are unchanged. Release the updated Agent Sandbox CLI before or alongside Factory. Temporary transport files are private and must be removed after every outcome; never replace the legacy file.
