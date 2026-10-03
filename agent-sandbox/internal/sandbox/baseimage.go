package sandbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	agentsandbox "agent-sandbox"
	"agent-sandbox/internal/docker"
)

// baseSandboxDockerfile derives a runnable image for every registered agent
// from a user-selected base without assuming anything about its name. The
// package-manager check is deliberately inside the build: OCI image metadata
// has no portable package-manager field, while the command that will install
// the tools is the source of truth.
const baseSandboxDockerfile = `ARG BASE_IMAGE
FROM ${BASE_IMAGE}

USER 0
RUN set -eu; \
    if command -v apk >/dev/null 2>&1; then \
        apk add --no-cache bash nodejs npm ripgrep ca-certificates curl git; \
    elif command -v apt-get >/dev/null 2>&1; then \
        apt-get update; \
        apt-get install -y --no-install-recommends \
            bash ca-certificates curl git gnupg ripgrep; \
        mkdir -p /etc/apt/keyrings; \
        curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key \
            | gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg; \
        echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_22.x nodistro main" \
            > /etc/apt/sources.list.d/nodesource.list; \
        apt-get update; \
        apt-get install -y --no-install-recommends nodejs; \
        rm -rf /var/lib/apt/lists/*; \
    elif command -v dnf >/dev/null 2>&1 \
        || command -v microdnf >/dev/null 2>&1; then \
        if command -v dnf >/dev/null 2>&1; then rpm_pm=dnf; \
        else rpm_pm=microdnf; fi; \
        "$rpm_pm" install -y bash ca-certificates curl git ripgrep; \
        rpm_arch="$(uname -m)"; \
        case "$rpm_arch" in \
            x86_64|aarch64) ;; \
            *) echo "unsupported RPM architecture: $rpm_arch" >&2; exit 1 ;; \
        esac; \
        mkdir -p /etc/yum.repos.d; \
        printf '%s\n' \
            '[nodesource-nodejs]' \
            "name=Node.js Packages for Linux RPM based distros - $rpm_arch" \
            "baseurl=https://rpm.nodesource.com/pub_22.x/nodistro/nodejs/$rpm_arch" \
            'priority=9' \
            'enabled=1' \
            'gpgcheck=1' \
            'gpgkey=https://rpm.nodesource.com/gpgkey/ns-operations-public.key' \
            'module_hotfixes=1' \
            > /etc/yum.repos.d/nodesource-nodejs.repo; \
        "$rpm_pm" install -y nodejs; \
        "$rpm_pm" clean all; \
    else \
        echo "unsupported base image: apk, apt-get, dnf, or microdnf is required" >&2; exit 1; \
    fi
RUN set -eu; \
    printf '\n# Keep the base image toolchain available to login shells.\nPATH=%s\nexport PATH\n' "$PATH" >> /etc/profile

ARG CODEX_VERSION=latest
RUN npm install --global "@openai/codex@${CODEX_VERSION}"

ARG CLAUDE_CODE_VERSION=latest
RUN npm install --global "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}"

ARG OPENCODE_VERSION=latest
RUN npm install --global "opencode-ai@${OPENCODE_VERSION}"

ARG PI_VERSION=latest
RUN npm install --global --ignore-scripts "@earendil-works/pi-coding-agent@${PI_VERSION}"

RUN mkdir -p /agent-sandbox-images

WORKDIR /workspace
CMD ["bash"]
`

const externalImagePrefix = "agent-sandbox-base-"

// imageClient selects the embedded image for normal runs and a generated image
// for an external base. The generated image gets a tag made from the complete
// reference and the generated definition. Docker tag syntax therefore cannot
// be affected by registry slashes, ports, or arbitrary valid image-reference
// punctuation, and a changed bootstrap definition cannot reuse an older image
// that lacks its new runtime requirements.
func imageClient(opts Options) (*docker.Client, error) {
	if opts.BaseImage == "" {
		return docker.New(imageName, agentsandbox.Dockerfile)
	}
	return docker.New(externalImageName(opts.BaseImage), baseSandboxDockerfile)
}

func externalImageName(baseImage string) string {
	digest := sha256.Sum256([]byte(baseImage + "\x00" + baseSandboxDockerfile))
	return externalImagePrefix + hex.EncodeToString(digest[:])
}

// ensureImage keeps the established npm freshness check for the image the tool
// owns. An external base is explicitly caller-owned: once derived, it is reused
// until the user removes its cache image, so this path neither pulls a mutable
// base tag nor checks npm for a newer Codex release.
func ensureImage(ctx context.Context, client *docker.Client, opts Options, out io.Writer) error {
	if opts.BaseImage == "" {
		return ensureImageLatest(ctx, client, opts.Agent, out)
	}
	return ensureBaseSandboxImage(ctx, client, opts.BaseImage, out)
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
