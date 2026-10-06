package sandbox

import (
	"context"
	"fmt"
	"io"
	"os"

	"agent-sandbox/internal/docker"
	"agent-sandbox/internal/git"
	"agent-sandbox/internal/github"
)

// Event reports known execution facts without exposing credentials or process
// environments. A nil observer leaves ordinary terminal execution unchanged.
type Event struct {
	Phase, Repository, BaseBranch, Worktree, Image, Version string
}

// Runtime supplies presentation choices, not a second execution workflow.
// ViewOwnsTerminal routes all output through the view and leaves keyboard and
// raw-mode ownership there; otherwise Docker retains its direct host streams.
type Runtime struct {
	Output           io.Writer
	Sizes            <-chan docker.TerminalSize
	Observe          func(Event)
	ViewOwnsTerminal bool
}

func (r Runtime) withDefaults() Runtime {
	if r.Output == nil {
		r.Output = os.Stdout
	}
	return r
}

func (r Runtime) emit(event Event) {
	if r.Observe != nil {
		r.Observe(event)
	}
}

func (r Runtime) route(repo *git.Repo) {
	if r.ViewOwnsTerminal {
		repo.Stdout, repo.Stderr = r.Output, r.Output
	}
}

func (r Runtime) prepareImage(ctx context.Context, opts Options) (*docker.Client, error) {
	var options []docker.Option
	if r.ViewOwnsTerminal {
		options = append(options, docker.WithStreams(r.Output, r.Sizes))
	}
	client, err := imageClient(opts, options...)
	if err != nil {
		return nil, err
	}
	image := imageName
	if opts.BaseImage != "" {
		image = externalImageName(opts.BaseImage)
	}
	r.emit(Event{Phase: "Preparing image", Image: image})
	if err := ensureImage(ctx, client, opts, r); err != nil {
		client.Close()
		return nil, err
	}
	if err := ensureCommitGit(ctx, client, opts, r.Output); err != nil {
		client.Close()
		return nil, err
	}
	return client, nil
}

// Creation and resume share execution only after resolving their worktree.
// Keeping that boundary preserves registration and locking while sharing the
// container, publication, exit-status, and event behavior across all commands.
func (r Runtime) execute(ctx context.Context, opts Options, record worktreeRecord, repo *git.Repo, client *docker.Client, githubClient *github.Client) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if opts.PR {
		worktree := repo.At(record.Path)
		if err := checkPRContentPath(ctx, worktree); err != nil {
			return 0, err
		}
		base, err := worktree.FetchBase(ctx, record.BaseBranch)
		if err != nil {
			return 0, fmt.Errorf("fetching PR base before agent execution: %w", err)
		}
		opts.prBase = base
	}
	runOpts, err := containerOptions(opts, record.Path)
	if err != nil {
		return 0, err
	}
	if r.ViewOwnsTerminal && opts.Agent.Name() == "claude" {
		runOpts.Args = append([]string{"--verbose"}, runOpts.Args...)
	}
	r.emit(Event{Phase: "Running agent"})
	status, err := client.Run(ctx, runOpts)
	if err != nil {
		return 0, err
	}
	if opts.PR || opts.Push {
		r.emit(Event{Phase: "Publishing"})
	}
	if err := publishResult(ctx, opts, record, repo, githubClient, status, r.Output); err != nil {
		return 0, err
	}
	phase := "Completed"
	if status != 0 {
		phase = "Failed"
	}
	r.emit(Event{Phase: phase})
	return status, nil
}
