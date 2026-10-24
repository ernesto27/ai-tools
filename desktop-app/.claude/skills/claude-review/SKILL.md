---
name: claude-review
description: Review Soft Factory CLI changes for subprocess and error-handling regressions.
---

# Claude Project Review

Use this skill when reviewing changes to Soft Factory. Apply it after the
default code review skill.

- Trace errors from configuration, prompt assembly, and `agent-sandbox`
  execution to the CLI result. Check that a failure stops later stages.
- Check whether generated prompts give the reviewer the intended instructions
  and keep correction work with the fixer.
- Check that filesystem errors identify the attempted path and do not silently
  select a different skill after a non-missing-file error.
- Report concrete findings with a trigger, impact, and location.
