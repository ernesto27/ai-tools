package sandbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	agentsandbox "github.com/ernesto27/ai-tools/agent-sandbox"
	"github.com/ernesto27/ai-tools/agent-sandbox/internal/docker"
)

const externalImagePrefix = "agent-sandbox-base-"

// imageClient uses the same embedded Dockerfile for default and derived
// images. A derived image gets a tag made from the complete base reference
// and the shared definition. Docker tag syntax therefore cannot
// be affected by registry slashes, ports, or arbitrary valid image-reference
// punctuation, and a changed bootstrap definition cannot reuse an older image
// that lacks its new runtime requirements.
func imageClient(opts Options, options ...docker.Option) (*docker.Client, error) {
	name := imageName
	if opts.BaseImage != "" {
		name = externalImageName(opts.BaseImage)
	}
	return docker.New(name, agentsandbox.Dockerfile, options...)
}

func externalImageName(baseImage string) string {
	digest := sha256.Sum256([]byte(baseImage + "\x00" + agentsandbox.Dockerfile))
	return externalImagePrefix + hex.EncodeToString(digest[:])
}

// ensureImage checks the selected agent on both image paths. Reusing a derived
// image must not leave its agent indefinitely pinned to the first build, even
// though the user-selected base reference and cache tag remain the same.
func ensureImage(ctx context.Context, client *docker.Client, opts Options, runtime Runtime) error {
	if opts.BaseImage == "" {
		return ensureImageLatest(ctx, client, opts.Agent, runtime)
	}
	if err := ensureBaseSandboxImage(ctx, client, opts.BaseImage, runtime.Output); err != nil {
		return err
	}
	return refreshAgentVersion(ctx, client, opts.Agent,
		externalImageName(opts.BaseImage),
		map[string]string{"BASE_IMAGE": opts.BaseImage}, runtime)
}

func ensureBaseSandboxImage(ctx context.Context, client *docker.Client, baseImage string, out io.Writer) error {
	exists, err := client.ImageExists(ctx)
	if err != nil {
		return err
	}
	if exists {
		fmt.Fprintf(out, "Using cached image %s built from %s.\n", externalImageName(baseImage), baseImage)
		return nil
	}

	name := externalImageName(baseImage)
	fmt.Fprintf(out, "Image %s not found. Building it from %s.\n", name, baseImage)
	return client.Build(ctx, map[string]string{
		"BASE_IMAGE":          baseImage,
		"CODEX_VERSION":       "latest",
		"CLAUDE_CODE_VERSION": "latest",
		"OPENCODE_VERSION":    "latest",
		"PI_VERSION":          "latest",
	})
}
