# PR verification images

- **Outcome:** Upload all images from `.agent-sandbox-verification/`, including its subfolders, and embed them in the GitHub PR description.
- **User:** Reviewers who need to see verification screenshots.
- **Trigger:** PR creation and PR updates after resumed sessions.
- **History:** Append images on updates. Preserve earlier images and existing description content; do not replace previous images.
- **Constraint:** Never stage or commit verification images. Publish them separately from the Git branch.
- **Out of scope:** Uploading logs, `summary.md`, other non-image files, or files outside the verification folder.

This document records the requested outcome. The upload mechanism and implementation details remain to be determined.
wh

read folder .verification
get files
upload to github