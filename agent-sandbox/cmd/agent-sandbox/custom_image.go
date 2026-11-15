package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCustomImageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "custom-image",
		Short: "Print a prompt for choosing a Docker development image",
		Long: "Print instructions and a reusable prompt to paste into any coding agent\n" +
			"opened in the target project's repository. Prefers an existing Docker Hub\n" +
			"image and requests a custom Dockerfile only when needed. Works from any\n" +
			"directory, ignores configuration, and does not run Docker or an agent.",
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), customImageInstructions+customImagePrompt)
			return err
		},
	}
}

const customImageInstructions = `How to use this prompt:
1. Open any coding agent (Codex, Claude Code, opencode, pi, or another) in the target project's repository.
2. Copy the prompt below and submit it to that agent with repository access and web browsing enabled.
3. Review the recommended image or generated Dockerfile before running the suggested commands.
4. Build the image if needed, then apply the proposed configuration changes yourself.

--- Prompt ---

`

const customImagePrompt = `Inspect this repository and recommend a Docker development image.

First:
- Read the repository instructions, README, dependency manifests, lockfiles, and CI configuration.
- Identify the required language/runtime versions, system libraries, build tools, and test tools.
- Determine the correct working directory and commands for installing dependencies, building, linting, and testing.

Prefer an existing Docker Hub image:
- Search Docker Hub and verify candidate images against their official documentation.
- Prefer official or project-maintained images with explicit version tags.
- Confirm the image provides the required toolchain and supports installing any missing dependencies.
- If an existing image meets the requirements, return its full image reference and Docker Hub link. Explain why it fits. Do not create a Dockerfile.

Create a custom Dockerfile only when no suitable existing image meets the requirements:
- Explain which missing requirements justify customization.
- Preserve existing production Dockerfiles; use a separate development Dockerfile.
- Match the project's declared toolchain versions.
- Treat the repository as mounted at runtime. Avoid baking source code or credentials into the image.
- Use a versioned image tag.

Do not build or pull images or run containers. Leave any Dockerfile for the user to review.

Provide:
1. The recommended image reference and why it fits, with source links.
2. If customization is necessary, the Dockerfile path and contents, plus the exact build command with its image tag and build context.
3. Any configuration changes needed to use the image. Inspect existing configuration and show minimal updates that preserve other settings. Do not apply those changes.
4. An example command for starting the development container with the repository mounted.
5. Instructions for selecting a newer image tag and updating the configuration.
6. Any required external services, Docker daemon access, or compatibility limitations.
7. What was verified through documentation and what still requires the user's build or runtime checks.

Do not commit, push, or publish anything unless explicitly requested.
`
