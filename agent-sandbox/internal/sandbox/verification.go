package sandbox

const verificationInstructions = `

Before finishing, attempt verification appropriate to the task and actual changes.
Use project-specific tests, builds, functional checks, infrastructure validation or dry runs, documentation checks, or other relevant checks.
For UI changes, use browser assertions and screenshots where possible. Non-UI tasks do not require screenshots, and a screenshot alone does not prove functional correctness.

Create /workspace/.agent-sandbox-verification/ and keep the final evidence minimal:
- summary.md: overall result (passed, failed, or unavailable), actual checks and commands, exit codes, results, evidence paths, and limitations.
- verification.log: one combined log of relevant check output, with command labels. Omit it if there is no useful output.
- For web UI changes, save final desktop.png (1280x800 viewport) and mobile.png (390x844 viewport). Capture the implemented UI in its requested theme, including dark mode when requested. Keep these screenshots after verification; do not delete them during cleanup. Save additional final screenshots only for distinct important states.
- .gitignore as described below.
Put exit codes directly in summary.md; do not create separate .exit-code files or per-command logs. Do not keep empty logs or intermediate/before-fix screenshots.
Create /workspace/.agent-sandbox-verification/.gitignore containing a single line: *
This keeps evidence, including the ignore file itself, out of normal Git staging without changing the project's tracked ignore rules.
When continuing a task, update the summary, combined log, and screenshots to reflect the latest verification instead of accumulating numbered or dated copies. Preserve useful failure details in the summary or combined log.
Do not invent checks or successful outcomes. Missing tools, credentials, or services mean verification is unavailable, not passed.
If screenshots cannot be captured, explain why in summary.md. Include the saved screenshot paths in your final response.

For browser screenshots and an optional required selector, use the installed tool:
agent-sandbox-browser <url> /workspace/.agent-sandbox-verification/page.png [selector]
For custom browser assertions, use Node.js with require("/opt/agent-sandbox-browser/browser.cjs").launch(); close the browser in a finally block.
Keep temporary verification scripts in the evidence directory while running checks, then remove those scripts and intermediate artifacts created during this execution. Leave existing project files and user-provided files untouched. Use existing project tools and base-image toolchains for other checks.

Never stage, commit, force-add, push, or upload verification evidence. Do not include credentials or secrets in it.
Do not deploy or change external systems solely to collect evidence unless the user's task authorizes it.
Failed or unavailable verification must not prevent completing the task or committing implementation changes when instructed. Do not exit unsuccessfully solely because verification failed or is unavailable.
In your final response, report the verification result and the evidence folder.
`
