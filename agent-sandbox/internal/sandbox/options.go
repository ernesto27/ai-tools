package sandbox

import (
	"agent-sandbox/internal/agent"
	"agent-sandbox/internal/utils"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Options is one invocation of the sandbox.
type Options struct {
	Branch        string
	AgentName     string
	Agent         agent.Agent
	APIKey        string
	Model         string
	BaseImage     string
	Push          bool
	PR            bool
	Prompt        string
	CommitMessage string
	FilePrompt    string
	Images        []string
	HostNetwork   bool
}

// NewOptions turns the values the command line carried into one invocation,
// resolving the agent name and filling in the model the agent defaults to. The
// command line itself is parsed by the caller; what is left here is the part
// that needs to know the agents, which is why an unusable name comes back as a
// UsageError rather than as a plain failure.
func NewOptions(opts Options) (Options, error) {
	if opts.AgentName == "" {
		return Options{}, usageErrorf("--agent is required (%s)", agent.NamesProse())
	}

	selected, err := agent.Lookup(opts.AgentName)
	if err != nil {
		return Options{}, UsageError{err}
	}
	// An empty model is not an omission to report: it means the agent resolves
	// the model itself, and DefaultModel says so by returning an empty string.
	if opts.Model == "" {
		opts.Model = selected.DefaultModel()
	}
	opts.Agent = selected
	if opts.APIKey != "" {
		if _, ok := selected.(agent.APIKeyAgent); !ok {
			return Options{}, usageErrorf("api-key is not supported for agent %s", selected.Name())
		}
	}

	if opts.Agent.SupportsImages() {
		images, err := resolveImagePaths(opts.Images)
		if err != nil {
			return Options{}, err
		}
		opts.Images = images
	}

	if opts.Branch == "" {
		randomBranch, err := generateBranchName()
		if err != nil {
			return Options{}, err
		}
		opts.Branch = randomBranch
	}

	if opts.FilePrompt != "" {
		prompt, err := utils.GetContentFile(opts.FilePrompt)
		if err != nil {
			return Options{}, err
		}
		opts.Prompt = prompt
	}

	return opts, nil
}

// resolveImagePaths turns the paths the shell supplied into absolute Docker
// bind-mount sources. A Docker daemon cannot reliably interpret a relative
// client-side path, and rejecting unusable files before worktree creation
// avoids leaving an otherwise successful sandbox run without its attachments.
func resolveImagePaths(images []string) ([]string, error) {
	resolved := make([]string, 0, len(images))
	for _, image := range images {
		path, err := filepath.Abs(image)
		if err != nil {
			return nil, usageErrorf("resolving --image %q: %v", image, err)
		}

		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, usageErrorf("--image %q must be an existing regular file", image)
		}
		resolved = append(resolved, path)
	}
	return resolved, nil
}

// AgentNames are the agent names the --agent flag accepts, in the order they
// are offered. The command line needs them for its help text and its
// completions, and this keeps it from having to know the agent package.
func AgentNames() []string {
	return agent.Names()
}

// FullPrompt is the prompt handed to the agent: the user's instruction plus the
// house rules for a sandbox run.
func (o Options) FullPrompt() string {
	return fmt.Sprintf(`%s

You decide all, do not ask questions.
Do not stage, commit, or push changes to Git.

After completing the changes, write a commit message to /workspace/%s.
Base it on the actual changes made, not the original request.
Use one short line of plain text, with no prefix, quotes, Markdown, or explanation.
`, o.Prompt, utils.CommitMessageFile)
}

// generateBranchName creates a lowercase-letter name with a six-digit suffix
// without a host dictionary or an additional package. A collision is reported
// by the sandbox rather than being silently retried.
func generateBranchName() (string, error) {
	const (
		letters    = "abcdefghijklmnopqrstuvwxyz"
		wordLength = 8
	)

	var word strings.Builder
	word.Grow(wordLength)
	letterLimit := big.NewInt(int64(len(letters)))
	for range wordLength {
		index, err := rand.Int(rand.Reader, letterLimit)
		if err != nil {
			return "", fmt.Errorf("generate random branch letters: %w", err)
		}
		word.WriteByte(letters[index.Int64()])
	}

	number, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", fmt.Errorf("generate random branch number: %w", err)
	}
	return word.String() + "-" + strconv.FormatInt(number.Int64()+100000, 10), nil
}
