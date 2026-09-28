---
name: risk-classification
description: Classify the risk of proposed changes using their scope, impact, and review evidence. Use when a change-risk assessment is requested.
---

# Risk Classification

Assess the final changes using the task requirements, supporting context,
code-review report, and security-review report.

## Establish scope

- Identify the changes in the current working directory and task branch.
- Use review reports supplied for this execution.
- Inspect relevant files when needed to confirm the reports' conclusions.
- Report missing evidence or an unclear change boundary.

## Classify risk

Choose one level:

- **LOW:** Small, well-understood changes with limited impact and sufficient verification.
- **MEDIUM:** Changes affect behavior or interactions, but their impact is bounded and sufficiently understood.
- **HIGH:** Changes can cause serious disruption, data loss, unauthorized access, or broad impact.
- **UNKNOWN:** Missing evidence prevents a defensible classification.

Consider affected functionality, potential impact, reversibility,
unresolved findings, and verification gaps.

Evaluate the final implementation after corrections. Do not assume that
an absence of findings proves low risk. Distinguish known high risk from
uncertainty caused by incomplete verification.

## Report

Provide:

- Risk level: LOW, MEDIUM, HIGH, or UNKNOWN.
- A concise explanation supported by evidence.
- Affected functionality and potential impact.
- Relevant unresolved findings.
- Verification gaps.
- Conditions required before delivery.

Do not modify implementation files, apply corrections, publish changes,
or start another review cycle.
