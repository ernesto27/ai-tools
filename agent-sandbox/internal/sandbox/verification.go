package sandbox

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const verificationDir = ".agent-sandbox-verification"

func clearVerificationFiles(worktreePath string) error {
	dir := filepath.Join(worktreePath, verificationDir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		// Preserve the ignore rule so future evidence stays out of Git staging.
		if entry.Name() == ".gitignore" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

const verificationInstructions = `

At the start of the current session, note the existing project state, including any uncommitted and untracked files, so you can distinguish your changes from earlier work.
Before committing or finishing, determine whether this session made net changes to project files (additions, edits, or deletions) compared with that starting state. Earlier commits, pre-existing uncommitted changes, staging or committing earlier work, verification artifacts, and routine generated build output do not count. Edits fully reverted during this session do not count.
If this session made no net project changes, skip automatic verification and do not create or update ` + verificationDir + `/ or any logs, reports, or screenshots. Leave evidence from earlier sessions untouched and report that verification was skipped because this session made no changes.
Apply the remaining verification instructions only when this session made net project changes.

Before finishing, attempt verification appropriate to this session's changes.
Use project-specific tests, builds, functional checks, infrastructure validation or dry runs, documentation checks, or other relevant checks.
For UI changes, use browser assertions, screenshots, and video recordings where possible. Non-UI tasks do not require screenshots or videos, and visual evidence alone does not prove functional correctness.

Create /workspace/` + verificationDir + `/ and keep the final evidence minimal:
- summary.md: overall result (passed, failed, or unavailable), actual checks and commands, exit codes, results, evidence paths, and limitations.
- verification.log: one combined log of relevant check output, with command labels. Omit it if there is no useful output.
- For web UI changes, save final desktop.png (1280x800 viewport) and mobile.png (390x844 viewport). Capture the implemented UI in its requested theme, including dark mode when requested. Keep these screenshots after verification; do not delete them during cleanup. Save additional final screenshots only for distinct important states.
- For web UI changes, automatically record a short demo.webm in this evidence directory. Demonstrate the changed functionality through relevant clicks, typing, and navigation, showing the final UI in its requested theme. Stop and finalize the recording before closing the browser. Keep the final video alongside the screenshots after verification; do not delete it during cleanup. Describe what the video demonstrates in summary.md. Remove incomplete recordings created during this execution.
- .gitignore as described below.
Put exit codes directly in summary.md; do not create separate .exit-code files or per-command logs. Do not keep empty logs or intermediate/before-fix screenshots.
Create /workspace/` + verificationDir + `/.gitignore containing a single line: *
This keeps evidence, including the ignore file itself, out of normal Git staging without changing the project's tracked ignore rules.
When continuing a task, update the summary, combined log, screenshots, and video to reflect the latest verification instead of accumulating numbered or dated copies. Preserve useful failure details in the summary or combined log.
Do not invent checks or successful outcomes. Missing tools, credentials, or services mean verification is unavailable, not passed.
If screenshots or video cannot be captured, explain why in summary.md. Include the saved screenshot and video paths in your final response.

For browser screenshots and an optional required selector, use the installed tool:
agent-sandbox-browser <url> /workspace/` + verificationDir + `/page.png [selector]
For custom browser assertions and video recording, use Node.js with require("/opt/agent-sandbox-browser/browser.cjs").launch() and a recording API supported by the installed Puppeteer and Chromium versions. FFmpeg is available in newly built images. Finalize any active recording and close the browser in a finally block.
Keep temporary verification scripts in the evidence directory while running checks, then remove those scripts and intermediate artifacts created during this execution. Leave existing project files and user-provided files untouched. Use existing project tools and base-image toolchains for other checks.

Never stage, commit, force-add, push, or upload verification evidence. Do not include credentials or secrets in it.
Do not deploy or change external systems solely to collect evidence unless the user's task authorizes it.
Failed or unavailable verification must not prevent completing the task or committing implementation changes when instructed. Do not exit unsuccessfully solely because verification failed or is unavailable.
In your final response, report the verification result and the evidence folder.
`
