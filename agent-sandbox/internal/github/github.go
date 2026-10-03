// Package github wraps the host GitHub CLI. It has no access to the agent or
// Docker configuration, so GitHub authentication stays on the host.
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
	"regexp"
	"strings"
)

type Client struct {
	Dir        string
	Repository string
	host       string
	owner      string
	name       string
}

var repositoryPart = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// New resolves origin without consulting gh's configurable default repository.
// Both URL and ordinary git@host:owner/repo spellings address the same target.
func New(dir, remote string) (*Client, error) {
	if !strings.Contains(remote, "://") {
		userHost, path, ok := strings.Cut(remote, ":")
		_, host, hasUser := strings.Cut(userHost, "@")
		if !ok || !hasUser || host == "" {
			return nil, fmt.Errorf("origin must be a GitHub SSH or HTTPS repository URL")
		}
		remote = "ssh://git@" + host + "/" + path
	}
	parsed, err := url.Parse(remote)
	if err != nil || parsed.Hostname() == "" {
		return nil, fmt.Errorf("origin must be a GitHub SSH or HTTPS repository URL")
	}
	switch parsed.Scheme {
	case "ssh", "https", "http", "git":
	default:
		return nil, fmt.Errorf("origin must be a GitHub SSH or HTTPS repository URL")
	}
	path := strings.TrimSuffix(strings.Trim(parsed.Path, "/"), ".git")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || !repositoryPart.MatchString(parts[0]) || !repositoryPart.MatchString(parts[1]) ||
		parts[0] == "." || parts[0] == ".." || parts[1] == "." || parts[1] == ".." ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("origin must identify a GitHub owner/repository")
	}
	host := strings.ToLower(parsed.Hostname())
	return &Client{Dir: dir, Repository: host + "/" + path, host: host, owner: parts[0], name: parts[1]}, nil
}

func (c *Client) MatchesRemote(remote string) bool {
	other, err := New(c.Dir, remote)
	return err == nil && strings.EqualFold(c.Repository, other.Repository)
}

func (c *Client) Validate(ctx context.Context) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("--pr requires GitHub CLI (gh) on the host: %w", err)
	}
	if _, err := c.command(ctx, nil, "auth", "status", "--active", "--hostname", c.host); err != nil {
		return fmt.Errorf("checking GitHub authentication; run gh auth login --hostname %s: %w", c.host, err)
	}
	if _, err := c.command(ctx, nil, "repo", "view", c.Repository, "--json", "nameWithOwner"); err != nil {
		return fmt.Errorf("checking origin GitHub repository: %w", err)
	}
	return nil
}

type PullRequest struct {
	URL            string `json:"url"`
	IsDraft        bool   `json:"isDraft"`
	HeadRefName    string `json:"headRefName"`
	BaseRefName    string `json:"baseRefName"`
	HeadRepository struct {
		Name string `json:"name"`
	} `json:"headRepository"`
	HeadRepositoryOwner struct {
		Login string `json:"login"`
	} `json:"headRepositoryOwner"`
}

// FindOpen verifies the head repository as well as its branch. A PR from a
// fork with the same branch name must not be mistaken for this publication.
func (c *Client) FindOpen(ctx context.Context, head, base string) (*PullRequest, error) {
	output, err := c.command(ctx, nil, "pr", "list", "--repo", c.Repository,
		"--state", "open", "--head", head, "--base", base, "--limit", "1000",
		"--json", "url,isDraft,headRefName,baseRefName,headRepositoryOwner,headRepository")
	if err != nil {
		return nil, err
	}
	var prs []PullRequest
	if err := json.Unmarshal(output, &prs); err != nil {
		return nil, fmt.Errorf("reading open pull requests: %w", err)
	}
	for _, pr := range prs {
		if pr.HeadRefName == head && pr.BaseRefName == base &&
			strings.EqualFold(pr.HeadRepositoryOwner.Login, c.owner) &&
			strings.EqualFold(pr.HeadRepository.Name, c.name) {
			if strings.TrimSpace(pr.URL) == "" {
				return nil, fmt.Errorf("matching open pull request has no URL")
			}
			return &pr, nil
		}
	}
	return nil, nil
}

// Create supplies every interactive input explicitly and omits --draft. The
// description is stdin data, so quotes, newlines and shell syntax stay literal.
func (c *Client) Create(ctx context.Context, head, base, title, body string) (string, error) {
	output, err := c.command(ctx, strings.NewReader(body), "pr", "create",
		"--repo", c.Repository, "--head", head, "--base", base,
		"--title", title, "--body-file", "-")
	if err != nil {
		return "", err
	}
	prURL := strings.TrimSpace(string(output))
	if prURL == "" {
		return "", fmt.Errorf("GitHub CLI returned no pull request URL")
	}
	return prURL, nil
}

func (c *Client) command(ctx context.Context, stdin io.Reader, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = c.Dir
	cmd.Stdin = stdin
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("gh %s: %w: %s", strings.Join(args[:2], " "), err, strings.TrimSpace(stderr.String()))
	}
	return output, nil
}
