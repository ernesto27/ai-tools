---
name: update-readme
description: Review repository changes and update README.md with verified user-facing behavior, setup, usage, and compatibility information.
---

# Update README

Keep `README.md` aligned with verified, user-facing behavior.

## Establish the review range

1. Check the working tree and preserve uncommitted README changes.
2. Find the latest committed README change:

   ```sh
   git log -1 --format=%H -- README.md
   ```

3. Review commits after that baseline through `HEAD`.
4. If the README has never been committed, review the relevant repository history or use a user-provided baseline.
5. Honor an explicit user-provided commit range or baseline instead.

## Gather evidence

- Inspect relevant commit diffs and the resulting implementation, tests, documentation, configuration, and user-facing output.
- Consolidate related commits into one documented capability.
- Include only shipped, user-facing behavior. Exclude refactors, test-only work, CI-only changes, and incomplete work.
- Do not infer behavior from commit messages alone.
- State requirements, limitations, and compatibility only when the implementation verifies them.

## Update README

Edit only sections that need correction, preserving the existing style.

- Update installation, setup, configuration, usage, examples, API references, compatibility notes, or feature descriptions when relevant.
- Add copyable examples only when they are supported by the current implementation.
- Remove or correct claims contradicted by the current implementation.

Use safe placeholders in examples; never include credentials or local data.

## Approval and verification

Before editing `README.md`, show the reviewed range, verified changes, target sections, and intentional omissions. Wait for explicit approval unless the user explicitly asked for the README edit itself.

After approval, re-read the updated README for accuracy and run the repository's relevant validation. Report changed sections and verification.
