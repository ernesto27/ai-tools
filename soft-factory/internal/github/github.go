// Package github wraps the host GitHub CLI so authentication stays on the host.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// RepositoryFromOrigin resolves the current directory's origin remote to
// host/owner/repository for use with the GitHub CLI.
func RepositoryFromOrigin(ctx context.Context) (string, error) {
	output, err := exec.CommandContext(
		ctx, "git", "remote", "get-url", "origin",
	).Output()
	if err != nil {
		return "", fmt.Errorf("read origin remote: %w", err)
	}

	remote := strings.TrimSpace(string(output))
	if !strings.Contains(remote, "://") {
		userHost, path, ok := strings.Cut(remote, ":")
		_, host, hasUser := strings.Cut(userHost, "@")
		if !ok || !hasUser || host == "" {
			return "", fmt.Errorf("unsupported origin remote")
		}
		remote = "ssh://" + host + "/" + path
	}

	parsed, err := url.Parse(remote)
	if err != nil {
		return "", fmt.Errorf("parse origin remote: %w", err)
	}
	switch parsed.Scheme {
	case "ssh", "https", "http", "git":
	default:
		return "", fmt.Errorf("unsupported origin remote scheme")
	}

	path := strings.TrimSuffix(strings.Trim(parsed.Path, "/"), ".git")
	parts := strings.Split(path, "/")
	if parsed.Hostname() == "" || len(parts) != 2 ||
		parts[0] == "" || parts[1] == "" ||
		parts[0] == "." || parts[0] == ".." ||
		parts[1] == "." || parts[1] == ".." ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("origin must identify a GitHub owner/repository")
	}

	return parsed.Hostname() + "/" + path, nil
}

// AppendBody appends text to an existing PR description without creating files.
// repository identifies the GitHub repository; pr is a number, URL or branch.
func AppendBody(ctx context.Context, repository, pr, text string) error {
	output, err := command(ctx, nil,
		"pr", "view", pr, "--repo", repository, "--json", "body")
	if err != nil {
		return fmt.Errorf("read PR description: %w", err)
	}
	var current struct {
		Body *string `json:"body"`
	}
	if err := json.Unmarshal(output, &current); err != nil {
		return fmt.Errorf("decode PR description: %w", err)
	}
	if current.Body == nil {
		return fmt.Errorf("GitHub CLI returned no PR description")
	}
	if text == "" {
		return nil
	}
	body := *current.Body
	if body != "" {
		body += "\n\n"
	}
	body += text
	if _, err := command(ctx, strings.NewReader(body),
		"pr", "edit", pr, "--repo", repository, "--body-file", "-"); err != nil {
		return fmt.Errorf("append PR description: %w", err)
	}
	return nil
}

func command(ctx context.Context, stdin io.Reader, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Stdin = stdin
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("gh %s: %w: %s",
			strings.Join(args[:2], " "), err, strings.TrimSpace(stderr.String()))
	}
	return output, nil
}
