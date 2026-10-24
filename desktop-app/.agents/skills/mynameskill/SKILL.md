---
name: mynameskill
description: Review Go CLI changes in this project for configuration and workflow correctness.
---

# Project Code Review

Use this skill when reviewing changes to Soft Factory. Apply it after the
default code review skill.

- Trace each new configuration value from JSON loading and validation to the
  stage that consumes it. Check absent, malformed, and missing-file behavior.
- Check project-relative paths against the directory of `config.json`,
  including a run started from another working directory.
- Verify that code review additions do not alter security review, risk
  classification, or the reviewer/fixer delegation rules.
- Report a finding only when it identifies a concrete failure or maintainability
  issue in the changed code. Include the triggering case and file location.
