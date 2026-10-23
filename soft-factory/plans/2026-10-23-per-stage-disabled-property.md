# Plan: Per-stage disabled property

- **Date:** 2026-10-23
- **Domain(s):** Backend / Go CLI configuration, documentation
- **Author:** plan-from-spec (reviewed with repository user)
- **Status:** Draft — scope confirmed; implementation pending

## 1. Summary

Move the stage-disable switch into each `stages.<name>` object beside its agent and model. Replace the top-level `disabledStages` configuration with an ordinary Go `bool` field named `disabled`. Preserve all existing execution, logging, reporting, and resume behavior by keeping `Config.StageDisabled` as the central lookup. This document is a plan only.

## 2. Scope

### In scope

- Support `disabled` for implementation, code review, security review, risk classification, and the final walkthrough.
- Remove the old list, parser, and list-specific validation.
- Ignore the old top-level key, matching `customSkills`; users migrate manually.
- Update configuration and workflow tests, README, and the English documentation site.

### Out of scope / non-goals

- CLI flags, automatic migration, warnings for legacy properties, stage reordering, or changed stage responsibilities.
- Changes to authentication, dependencies, Agent Sandbox configuration types, prompt contents, or sandbox argument transport.
- Editing local agent/model assignments or credentials, committing, releasing, or deploying.
- The separately requested local base-image edit is already complete and is not part of this implementation.

## 3. Resolved decisions

| # | Question | Decision |
|---|----------|----------|
| 1 | Where does the switch belong? | `stages.<name>.disabled`, beside agent and model. |
| 2 | Replace or supplement the old array? | Replace it for all five stages. |
| 3 | What enables or disables a stage? | `true` skips; `false` or omission enables. |
| 4 | How is the old property handled? | Ignore it with no migration, fallback, or warning, regardless of its valid JSON value. |
| 5 | Add custom boolean validation? | No. Use normal `encoding/json` unmarshalling and return its wrapped errors. Consequently `null` leaves a fresh bool false. |
| 6 | Preserve workflow semantics? | Yes: existing skip behavior, settings, and stage ordering remain. |
| 7 | Deliverable now? | Save an implementation plan; do not implement the feature. |
| 8 | Verification and docs scope? | Configuration unit tests, workflow integration tests, README and site updates, Go checks and VitePress build. |
| 9 | Keep tests for the former array? | No. Remove the proposed `TestTopLevelDisabledStagesIgnored`; do not add dedicated legacy-array tests. |

These choices were confirmed before writing. Actors and permissions remain the local CLI user's existing permissions; no services, endpoints, databases, deployment resources, or new performance requirements are introduced.

## 4. Design

Add `Disabled bool` to `StageConfig`. All review configuration types already embed it, including `SecurityReviewConfig`, so the JSON property is flattened naturally into each stage object. Do not add an `UnmarshalJSON` method to the embedded type: that would complicate decoding of its enclosing review fields.

`Config.StageDisabled` switches on the existing `Stage` constants and returns the corresponding embedded bool. Unknown identifiers return false, preserving the previous lookup's behavior. `Stages` remains the authoritative pipeline order. Existing callers in `cmd/factory/main.go` and `internal/sandbox/runner.go` need no production changes.

Keep the existing `agentSandbox` presence check and all unrelated validation. `disabled` is Factory-only and is never forwarded as an Agent Sandbox flag. Keep the upstream pointer semantics untouched.

## 5. Interfaces & contracts

Merge into an existing valid configuration:

```json
{
  "stages": {
    "securityReview": {
      "agent": "claude",
      "model": "claude-opus-5-5",
      "disabled": true
    }
  }
}
```

The model is retained from the user's example; this change does not validate its upstream availability.

- Omitted stage, omitted property, `false`, and `null`: enabled when otherwise eligible to run.
- `true`: disabled, even when agent/model/custom skill or `saveFindings` is configured.
- Strings, numbers, arrays, and objects in `disabled`: return the existing `parse factory configuration: %w` error from `json.Unmarshal`.
- Malformed JSON still fails, including malformed JSON inside the ignored legacy property.
- Any syntactically valid `disabledStages` value is ignored. With only that old property, stages are enabled.
- If both forms exist, only the nested bool has an effect.

## 6. Behavior & states

Preserve all existing checks around stage eligibility, command selection, starting stage, cancellation, and branch selection. A disabled stage starts no agent and loads no custom-skill file. Existing custom-skill name validation still applies. Skip output and summary logging remain unchanged. Disabled reviews remain missing evidence for risk classification, and disabling risk classification retains report cleanup. Resume planning uses the current nested switches through `plannedStages`.

Setting every stage disabled does not bypass existing environment, branch, document, Jira, or installation checks. No new auth, IO, or network behavior is introduced by the switch itself.

## 7. Implementation tasks

### Task 1 — Replace the configuration source

- **Why:** Make the nested bool the single source of stage-disable decisions.
- **Files & changes:** `internal/config/config.go`.
- **Depends on:** —

In the imports, remove `slices` only after removing the old validation (its only uses in this file are the old list lookup and validation). Retain `strings`, which validates custom-skill names elsewhere.

In `Config`, remove the `DisabledStages` comment and field. Keep `Stages StagesConfig` with its existing JSON tag.

```diff
 type StageConfig struct {
     Agent string `json:"agent,omitempty"`
     Model string `json:"model,omitempty"`
+    Disabled bool `json:"disabled,omitempty"`
 }
```

Replace `StageDisabled` completely:

```go
// StageDisabled reports whether the stage's configuration disables execution.
func (c Config) StageDisabled(stage Stage) bool {
    switch stage {
    case StageImplementation:
        return c.Stages.Implementation.Disabled
    case StageCodeReview:
        return c.Stages.CodeReview.Disabled
    case StageSecurityReview:
        return c.Stages.SecurityReview.Disabled
    case StageRiskClassification:
        return c.Stages.RiskClassification.Disabled
    case StageReviewChanges:
        return c.Stages.ReviewChanges.Disabled
    default:
        return false
    }
}
```

In `Load`'s `raw` anonymous struct remove `DisabledStages json.RawMessage`. After the `agentSandbox` presence check, remove the entire `cfg.DisabledStages, err = parseDisabledStages(...)` assignment and its error branch. Keep both existing JSON decodes and the call to `cfg.Validate()`.

Delete the entire `parseDisabledStages` function. Delete the initial `for i, stage := range c.DisabledStages` loop from `Validate`; keep the `skillNames` declaration and all subsequent validation unchanged. Do not add replacement validation.

### Task 2 — Replace configuration tests and migrate fixtures

- **Why:** Verify nested JSON decoding and preserve unrelated settings.
- **Files & changes:** `internal/config/config_test.go`, `internal/config/security_findings_test.go`.
- **Depends on:** Task 1.

In `config_test.go`, remove the now-unused `slices`, `strings`, and `github.com/stretchr/testify/assert` imports and add `errors` and `path/filepath`. Replace `TestLoadDisabledStages` with:

```go
func TestLoadStageDisabled(t *testing.T) {
    for _, stage := range Stages {
        for _, tc := range []struct {
            name, value string
            want, invalid bool
        }{
            {"omitted", "", false, false},
            {"false", "false", false, false},
            {"true", "true", true, false},
            {"null", "null", false, false},
            {"string", `"true"`, false, true},
            {"number", "1", false, true},
            {"array", "[]", false, true},
            {"object", "{}", false, true},
        } {
            t.Run(string(stage)+"/"+tc.name, func(t *testing.T) {
                property := ""
                if tc.value != "" {
                    property = `,"disabled":` + tc.value
                }
                data := `{"agentSandbox":{},"stages":{"` + string(stage) +
                    `":{"agent":"codex","model":"stage-model"` + property + `}}}`
                path := filepath.Join(t.TempDir(), "software-factory.json")
                require.NoError(t, os.WriteFile(path, []byte(data), 0600))
                cfg, err := Load(path)
                if tc.invalid {
                    require.ErrorContains(t, err, "parse factory configuration")
                    var typeErr *json.UnmarshalTypeError
                    require.True(t, errors.As(err, &typeErr))
                    return
                }
                require.NoError(t, err)
                for _, other := range Stages {
                    require.Equal(t, other == stage && tc.want, cfg.StageDisabled(other))
                }
                require.False(t, cfg.StageDisabled(Stage("unknown")))
            })
        }
    }
}
```

Keep `TestLoadDisabledStagesKeepsOtherValidation` but rename it `TestLoadStageDisabledKeepsOtherValidation` and replace its `data` value with:

```go
data := `{"agentSandbox":{},"stages":{"codeReview":{"disabled":true,"customSkill":"../escape"}}}`
```

In `TestUnifiedSandboxConfiguration`'s shared/section fixture remove the old top-level list and add `"reviewChanges":{"disabled":true}` to `stages`. In that case's assertions add:

```go
require.True(t, cfg.StageDisabled(StageReviewChanges))
```

In `security_findings_test.go`, change the format string used to build `data` to:

```go
data := fmt.Sprintf(`{"agentSandbox":{"agent":"shared"},"stages":{"securityReview":{"disabled":true,"agent":"codex","model":"security-model","customSkill":"project"%s},"codeReview":{"model":"code-model"}}}`, property)
```

Retain all assertions for `saveFindings`, custom skill, agent, model, and the disabled state. Existing malformed-JSON cases in `TestUnifiedSandboxConfiguration` continue to protect syntax-error handling.

### Task 3 — Exercise the workflow boundary

- **Why:** Prove loaded JSON controls planning, actual execution guards, and skill eligibility.
- **Files & changes:** `cmd/factory/main_test.go`.
- **Depends on:** Tasks 1–2.

For `TestWorkflowSkipsIneligibleCustomSkills`, change the disabled case's fragment from `,"disabledStages":["codeReview"]` to `,"disabled":true`. Move interpolation of `tc.disabled` inside the `codeReview` object:

```go
data := `{"agentSandbox":{},"stages":{"codeReview":{"customSkill":"skip-missing"` + tc.disabled + `},"securityReview":{"customSkill":"eligible-missing"}}}`
```

Add `context`, `encoding/json`, `path/filepath`, and `soft-factory/internal/sandbox` imports and this integration test:

```go
func TestLoadedDisabledStageControlsWorkflow(t *testing.T) {
    for _, stage := range config.Stages {
        for _, disabled := range []bool{false, true} {
            name := string(stage) + "/enabled"
            if disabled {
                name = string(stage) + "/disabled"
            }
            t.Run(name, func(t *testing.T) {
                data, err := json.Marshal(map[string]any{
                    "agentSandbox": map[string]any{},
                    "stages": map[string]any{
                        string(stage): map[string]any{"disabled": disabled},
                    },
                })
                require.NoError(t, err)
                path := filepath.Join(t.TempDir(), "software-factory.json")
                require.NoError(t, os.WriteFile(path, data, 0600))
                cfg, err := config.Load(path)
                require.NoError(t, err)
                planned := plannedStages(cfg)
                if disabled {
                    require.NotContains(t, planned, stage)
                } else {
                    require.Equal(t, config.Stages, planned)
                }
                called := false
                input := sandbox.TaskContext{Context: context.Background(), Config: cfg}
                require.NoError(t, runStage(input, cfg, stage, "test stage", func() error {
                    called = true
                    return nil
                }))
                require.Equal(t, !disabled, called)
            })
        }
    }
}
```

This wires real file loading, stage lookup, the plan used for execution logs/resume, and the actual callback guard without Docker, agent binaries, network, or credentials. The existing `runWorkflow` skill-eligibility test exercises preflight wiring. Existing sandbox risk-evidence and execution-log tests remain regression checks; no changes to their contracts are needed.

### Task 4 — Document the new contract

- **Why:** Users must move legacy list entries manually before running the new version.
- **Files & changes:** `README.md`, `site-docs/guide/configuration.md`, `site-docs/guide/workflow.md`.
- **Depends on:** Tasks 1–3.

In README and the configuration guide, replace the `disabledStages` option table row with:

```markdown
| `stages.<name>.disabled` | Optional boolean for any of the five stages. Set `true` to skip the stage; omission or `false` enables it. See [Disabling stages](#disabling-stages). |
```

In both files, replace the text from the start of the `Disabling stages` section through the paragraph preceding `Disabling a stage never disables later stages` with:

````markdown
Set `disabled: true` inside a stage to skip it without changing the order of later stages. Merge the stage settings into your existing `software-factory.json`:

```json
{
  "stages": {
    "securityReview": {"disabled": true},
    "riskClassification": {"disabled": true}
  }
}
```

All five stage keys support this property: `implementation`, `codeReview`, `securityReview`, `riskClassification`, and `reviewChanges`. Omission or `false` enables a stage when it is eligible for the selected workflow. Normal JSON decoding also treats `null` as false; incompatible value types return a configuration parsing error.

The former top-level `disabledStages` property is ignored. Move each listed stage to `stages.<name>.disabled: true` manually; there is no automatic migration or fallback. Preserve its existing agent, model, custom skill, and other settings.

Each skipped stage prints `Skipping disabled stage: <name>` and adds that line to `summary.log`. Skipped stages start no agent, create no stage log or report, and do not load their configured custom skill. Skill names are still validated.
````

Keep the following existing paragraphs about later stages, missing evidence, branches and preflight checks unchanged. In the configuration guide's stage agents/models section replace:

```diff
-`disabledStages` still skips a stage even when it has settings here.
+`disabled: true` still skips a stage even when it has agent or model settings here.
```

In `site-docs/guide/workflow.md` replace:

```diff
-returns without running stages. Current `disabledStages` settings still apply.
+returns without running stages. Current `stages.<name>.disabled` settings still apply.
```

Follow the repository's `update-site-docs` skill when implementing these documentation edits. Preserve the existing design and anchors.

### Task 5 — Format, verify, and review

- **Why:** Validate the complete change and catch unintended transport/workflow effects.
- **Files & changes:** No new production files. Formatting may modify the Go files listed above.
- **Depends on:** Tasks 1–4.

Run from the module root:

```bash
gofmt -w internal/config/config.go internal/config/config_test.go internal/config/security_findings_test.go cmd/factory/main_test.go
go test ./...
go vet ./...
go build -o /tmp/software-factory ./cmd/factory
```

From `site-docs`, run `npm ci` only if dependencies are missing, then `npm run docs:build`. Review the final diff using `code-review-and-quality`. Do not merge or publish. Inspect remaining legacy references with `rg -n 'disabledStages|DisabledStages|parseDisabledStages' README.md site-docs cmd internal`: only intentional migration documentation should remain outside this plan.

## 8. Testing

- **Unit tests:** Every stage with omitted/false/true/null; incompatible JSON values preserve wrapped `json.UnmarshalTypeError`; unrelated custom-skill validation remains; shared/section settings and security `saveFindings` survive decoding. Existing tests cover malformed JSON and missing required `agentSandbox`.
- **Integration tests:** Actual temporary configuration file to `config.Load` to `plannedStages` and `runStage`, checking each stage enabled/disabled and callback execution. `runWorkflow` preflight confirms disabled missing skills are skipped while eligible missing skills still fail. Existing risk evidence, transport, resume, and execution-log suites run unchanged to protect their contracts.
- **Setup/teardown:** `t.TempDir`, scoped environment setup already present in the workflow test, no real agents, no Docker or authentication. No numeric coverage target; every case above must pass.
- **Documentation:** VitePress production build and manual comparison of examples with the final Go struct and tests.

## 9. Acceptance criteria

1. Each of the five nested `disabled` switches independently controls stage lookup, planning, and execution guards (unit and integration tests).
2. Ordinary Go JSON semantics apply, including accepted null and wrapped decoding errors; there is no custom disabled validation (unit tests and diff review).
3. Legacy `disabledStages` has no runtime effect because its field, parser, and validation are removed (diff review; no dedicated legacy-array tests).
4. Existing custom skills, agent/model selections, `saveFindings`, skip reporting, stage order, and sandbox transport remain intact (updated fixtures and full regression suite).
5. README and site describe manual migration and the new switch; VitePress builds successfully.
6. Go tests, vet, and build pass; no feature implementation is considered complete until those checks are recorded.

## 10. Risks & open items

- Ignoring the old key enables previously disabled stages unless users migrate. This is the explicitly selected compatibility policy; document it clearly without adding runtime warnings or fallback.
- Native bool decoding accepts null as false. This is intentional, not an unresolved validation decision.
- No open product decisions remain. The plan does not alter the ignored local configuration's stage switches or existing private values.
- Verification commands above are implementation acceptance requirements; they have not been run as part of writing this plan.
