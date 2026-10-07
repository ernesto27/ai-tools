// Package sandbox creates the git worktree an agent works on and runs the
// agent against it inside the sandbox container.
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"agent-sandbox/internal/agent"
	"agent-sandbox/internal/docker"
	"agent-sandbox/internal/git"
)

// imageName is the tag of the sandbox image, shared by every agent.
const imageName = "agent-sandbox"

// workspace is where the worktree is mounted, matching the image's WORKDIR.
const workspace = "/workspace"

// imageAttachmentDir is separate from the worktree so a prompt attachment is
// never mistaken for a repository file or accidentally included in a commit.
const imageAttachmentDir = "/agent-sandbox-images"

// External images run as the invoking host UID/GID, which need not have an
// entry or a writable home in the image's passwd database. Keeping these paths
// under /tmp gives Codex and language tools a disposable writable home without
// changing the host-mounted CODEX_HOME that carries authentication.
const (
	externalHome  = "/tmp/agent-sandbox-home"
	externalCache = "/tmp/agent-sandbox-cache"
)

// semver matches the version inside an agent's --version output, which is
// rarely the bare number.
var semver = regexp.MustCompile(`[0-9]+\.[0-9]+\.[0-9]+`)

// Run creates the worktree, runs the agent on it and optionally publishes the
// result. It returns the agent's own exit code.
func Run(ctx context.Context, opts Options, runtime Runtime) (int, error) {
	runtime = runtime.withDefaults()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	runtime.emit(Event{Phase: "Preparing"})
	executionDir, err := os.Getwd()
	if err != nil {
		return 0, err
	}

	repo, err := git.Open(executionDir)
	if err != nil {
		return 0, err
	}

	if opts.PR {
		remoteExists, err := repo.RemoteBranchExists(opts.Branch)
		if err != nil {
			return 0, err
		}

		if remoteExists {
			return 0, errors.New("remote branch already exists: " + opts.Branch)
		}

	}

	runtime.route(repo)
	baseBranch, err := repo.CurrentBranch(ctx)
	if err != nil {
		return 0, err
	}
	runtime.emit(Event{Repository: repo.Dir, BaseBranch: baseBranch})
	githubClient, err := preparePullRequest(ctx, opts, repo, baseBranch)
	if err != nil {
		return 0, err
	}

	client, err := runtime.prepareImage(ctx, opts)
	if err != nil {
		return 0, err
	}
	defer client.Close()

	config, err := configPaths()
	if err != nil {
		return 0, err
	}
	worktreeDir := filepath.Join(config.WorktreesDir, repoSlug(repo.Dir), opts.Branch)
	if err := os.MkdirAll(filepath.Dir(worktreeDir), 0o755); err != nil {
		return 0, err
	}
	if _, err := os.Stat(worktreeDir); err == nil {
		return 0, fmt.Errorf("worktree path already exists: %s", worktreeDir)
	}
	runtime.emit(Event{Phase: "Creating worktree"})
	newBranch := !repo.BranchExists(opts.Branch)
	if err := repo.AddWorktree(worktreeDir, opts.Branch, newBranch); err != nil {
		return 0, err
	}

	// The worktree is written down before the agent runs, because a worktree
	// missing from the state file is one no worktree verb can see afterwards.
	record := worktreeRecord{
		Repo:       repo.Dir,
		Path:       worktreeDir,
		Branch:     opts.Branch,
		BaseBranch: baseBranch,
		Created:    time.Now().UTC(),
	}
	if err := recordWorktree(record); err != nil {
		// Nothing has run in the worktree yet, so undoing it costs nothing and
		// beats leaving behind one that no worktree verb can reach.
		return 0, errors.Join(err, undoWorktree(repo, worktreeDir, opts.Branch, newBranch))
	}

	runtime.emit(Event{Worktree: worktreeDir})
	return runtime.execute(ctx, opts, record, repo, client, githubClient)
}

// Resume runs an agent in an existing sandbox worktree. It deliberately does
// not share Run's creation path: a recorded worktree is the authorization to
// reuse a directory, while a missing or stale record must never turn into a
// fresh worktree that only happens to have the same branch name.
func Resume(ctx context.Context, opts Options, runtime Runtime) (int, error) {
	runtime = runtime.withDefaults()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	runtime.emit(Event{Phase: "Finding worktree"})
	found, worktree, err := resumableWorktree(opts.Branch)
	if err != nil {
		return 0, err
	}
	runtime.route(found.repo)
	runtime.emit(Event{Repository: found.repo.Dir, BaseBranch: worktree.BaseBranch, Worktree: worktree.Path})
	githubClient, err := preparePullRequest(ctx, opts, found.repo, worktree.BaseBranch)
	if err != nil {
		return 0, err
	}
	if opts.PR || opts.Push {
		branch, err := found.repo.At(worktree.Path).CurrentBranch(ctx)
		if err != nil {
			return 0, err
		}
		if branch != opts.Branch {
			return 0, fmt.Errorf("recorded PR worktree must be on branch %s, found %q", opts.Branch, branch)
		}
	}

	client, err := runtime.prepareImage(ctx, opts)
	if err != nil {
		return 0, err
	}
	defer client.Close()

	lock, err := lockWorktree(worktree)
	if err != nil {
		return 0, err
	}
	defer lock.Close()

	return runtime.execute(ctx, opts, worktree, found.repo, client, githubClient)
}

// resumableWorktree resolves a recorded target before any image work. The
// state file is the sandbox's source of truth, so the name the user sees in
// worktree-list is enough to recover the worktree path and branch to reuse.
func resumableWorktree(branch string) (repoWorktrees, worktreeRecord, error) {
	found, err := sandboxWorktrees()
	if err != nil {
		return repoWorktrees{}, worktreeRecord{}, err
	}

	worktree, ok := findWorktree(found.worktrees, branch)
	if !ok {
		return repoWorktrees{}, worktreeRecord{}, fmt.Errorf("no sandbox worktree on branch %s; run worktree-list to see the ones there are", branch)
	}

	if _, err := os.Stat(worktree.Path); err != nil {
		if os.IsNotExist(err) {
			return repoWorktrees{}, worktreeRecord{}, staleWorktreeError(worktree)
		}
		return repoWorktrees{}, worktreeRecord{}, err
	}
	return found, worktree, nil
}

func staleWorktreeError(worktree worktreeRecord) error {
	return fmt.Errorf("sandbox worktree %s at %s is stale; run worktree-delete -b %s to remove its record", worktree.Branch, worktree.Path, worktree.Branch)
}

// undoWorktree removes a worktree the run has just created, for the failure
// that happens between creating it and being able to use it. The branch goes
// only if this run is what created it.
func undoWorktree(repo *git.Repo, dir, branch string, newBranch bool) error {
	if err := repo.RemoveWorktree(dir, true); err != nil {
		return err
	}
	if !newBranch {
		return nil
	}
	return repo.DeleteBranch(branch)
}

// containerOptions completes the agent's container configuration with the
// pieces that depend on this run: the workspace mount, the invoking user and
// the agent's command line.
func containerOptions(opts Options, worktreeDir string) (docker.RunOptions, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return docker.RunOptions{}, err
	}

	var runOpts docker.RunOptions
	if opts.APIKey != "" {
		keyed, ok := opts.Agent.(agent.APIKeyAgent)
		if !ok {
			return docker.RunOptions{}, usageErrorf("api-key is not supported for agent %s", opts.Agent.Name())
		}
		runOpts = keyed.APIKeyContainer(opts.APIKey)
	} else {
		runOpts, err = opts.Agent.Container(home)
		if err != nil {
			return docker.RunOptions{}, err
		}
	}

	runOpts.Entrypoint = opts.Agent.Binary()
	imagePaths := addImageMounts(&runOpts, opts)
	if opts.APIKey != "" {
		runOpts.Args = opts.Agent.(agent.APIKeyAgent).APIKeyArgs(opts.Model, opts.FullPrompt(), imagePaths)
	} else {
		runOpts.Args = opts.Agent.Args(opts.Model, opts.FullPrompt(), imagePaths)
	}
	runOpts.User = strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid())
	runOpts.Mounts = append(runOpts.Mounts, docker.Mount{Host: worktreeDir, Container: workspace})

	worktree := &git.Repo{Dir: worktreeDir}
	gitDir, commonDir, err := worktree.WorktreeGitDirs()
	if err != nil {
		return docker.RunOptions{}, err
	}
	name, email, err := worktree.CommitIdentity()
	if err != nil {
		return docker.RunOptions{}, err
	}
	// Linked worktrees keep their Git administration under the repository's
	// common directory. Mounting it at its original absolute path keeps the
	// worktree metadata valid without exposing the caller's working files.
	runOpts.Mounts = append(runOpts.Mounts, docker.Mount{Host: commonDir, Container: commonDir})
	runOpts.Env = append(runOpts.Env,
		"GIT_DIR="+gitDir,
		"GIT_COMMON_DIR="+commonDir,
		"GIT_WORK_TREE="+workspace,
		"GIT_AUTHOR_NAME="+name,
		"GIT_AUTHOR_EMAIL="+email,
		"GIT_COMMITTER_NAME="+name,
		"GIT_COMMITTER_EMAIL="+email,
	)

	runOpts.HostNetwork = opts.HostNetwork
	if opts.BaseImage != "" {
		// An arbitrary Alpine base may leave a numeric host UID trying to create
		// /.cache. XDG_CACHE_HOME gives every tool that follows the standard a
		// disposable writable cache without replacing the homes agents configure.
		runOpts.Env = append(runOpts.Env,
			"XDG_CACHE_HOME="+externalCache,
			// Claude Code discovers its command runner from SHELL. Alpine's
			// default /bin/sh is valid for Docker but not enough for that check,
			// so the generated image installs Bash and every agent gets its path.
			"SHELL=/bin/bash",
		)
		if opts.Agent.Name() == "codex" && opts.APIKey == "" {
			// Codex uses CODEX_HOME for its mounted configuration and otherwise
			// needs a normal writable home. Claude, opencode and pi each set HOME
			// themselves, so overriding it here would hide their configuration.
			runOpts.Env = append(runOpts.Env, "HOME="+externalHome)
		}
	}

	return runOpts, nil
}

// addImageMounts exposes supported-agent attachments at stable, generated
// paths. It never forwards host filenames into the container command, and
// read-only binds keep the agent from modifying data outside its disposable
// worktree.
func addImageMounts(runOpts *docker.RunOptions, opts Options) []string {
	if !opts.Agent.SupportsImages() {
		return nil
	}

	paths := make([]string, 0, len(opts.Images))
	for index, image := range opts.Images {
		path := filepath.Join(imageAttachmentDir, imageAttachmentName(index, image))
		runOpts.Mounts = append(runOpts.Mounts, docker.Mount{
			Host: image, Container: path, ReadOnly: true,
		})
		paths = append(paths, path)
	}
	return paths
}

// imageAttachmentName preserves an ordinary extension for programs that use
// one to identify a file format. The generated base name keeps attachment
// paths stable without exposing the host filename to the agent.
func imageAttachmentName(index int, image string) string {
	return strconv.Itoa(index+1) + filepath.Ext(image)
}

// publish pushes an agent-committed branch unless --push was left out.
func publish(ctx context.Context, opts Options, worktreeDir string, repo *git.Repo, out io.Writer) error {
	if !opts.Push {
		fmt.Fprintf(out, "Skipping commit and push. Changes left in %s\n", worktreeDir)
		return nil
	}

	worktree := repo.At(worktreeDir)
	if err := requireCommittedWorktree(worktree); err != nil {
		return err
	}
	return worktree.PushContext(ctx, opts.Branch)
}

// ensureCommitGit upgrades a cached default image built before Git was installed.
// Custom-base images already include Git, so a failure there is a broken image
// rather than a reason to rebuild an unrelated caller-owned base.
func ensureCommitGit(ctx context.Context, client *docker.Client, opts Options, out io.Writer) error {
	if !opts.PR && !opts.Push {
		return nil
	}
	if _, err := client.Output(ctx, "git", "--version"); err == nil {
		return nil
	} else if opts.BaseImage != "" {
		return fmt.Errorf("--push and --pr require Git in the sandbox image: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	fmt.Fprintf(out, "Rebuilding %s with Git for publication.\n", imageName)
	if err := client.Build(ctx, nil); err != nil {
		return err
	}
	if _, err := client.Output(ctx, "git", "--version"); err != nil {
		return fmt.Errorf("Git is unavailable in rebuilt sandbox image: %w", err)
	}
	return nil
}

// ensureImageLatest builds the image when it is missing and rebuilds it when
// npm has a newer release of the agent than the one baked in.
func ensureImageLatest(ctx context.Context, client *docker.Client, target agent.Agent, runtime Runtime) error {
	out := runtime.Output
	exists, err := client.ImageExists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		fmt.Fprintf(out, "Image %s not found. Building it.\n", imageName)
		if err := client.Build(ctx, nil); err != nil {
			return err
		}
	}

	return refreshAgentVersion(ctx, client, target, imageName, nil, runtime)
}

// refreshAgentVersion shares the freshness policy across embedded and derived
// images. Rebuild arguments retain the external base while pinning the selected
// agent, so Docker invalidates its npm installation layer even when the image
// definition has not changed.
func refreshAgentVersion(ctx context.Context, client *docker.Client, target agent.Agent, name string, baseArgs map[string]string, runtime Runtime) error {
	out := runtime.Output
	installed, err := installedVersion(ctx, client, target)
	if err != nil {
		return err
	}
	runtime.emit(Event{Version: installed})
	latest, err := latestVersion(ctx, client, target)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "%s in image: %s\n", target.Name(), installed)
	fmt.Fprintf(out, "%s latest:   %s\n", target.Name(), latest)

	if installed == latest {
		fmt.Fprintf(out, "%s is up to date.\n", target.Name())
		return nil
	}

	fmt.Fprintf(out, "Update available. Rebuilding %s with %s %s.\n", name, target.Name(), latest)
	buildArgs := make(map[string]string, len(baseArgs)+1)
	for key, value := range baseArgs {
		buildArgs[key] = value
	}
	buildArgs[target.BuildArg()] = latest
	if err := client.Build(ctx, buildArgs); err != nil {
		return err
	}

	rebuilt, err := installedVersion(ctx, client, target)
	if err != nil {
		return err
	}
	if rebuilt != latest {
		return fmt.Errorf("rebuilt %s is %s; expected %s", target.Name(), rebuilt, latest)
	}
	runtime.emit(Event{Version: rebuilt})

	fmt.Fprintf(out, "%s rebuilt at version %s.\n", target.Name(), rebuilt)
	return nil
}

// installedVersion is the version the agent reports from inside the image.
func installedVersion(ctx context.Context, client *docker.Client, target agent.Agent) (string, error) {
	out, err := client.Output(ctx, target.Binary(), "--version")
	if err != nil {
		return "", err
	}

	version := semver.FindString(out)
	if version == "" {
		return "", fmt.Errorf("no version found in %s --version output: %q", target.Binary(), strings.TrimSpace(out))
	}
	return version, nil
}

// latestVersion is the version npm publishes for the agent's package.
func latestVersion(ctx context.Context, client *docker.Client, target agent.Agent) (string, error) {
	out, err := client.Output(ctx, "npm", "view", target.Package(), "version")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
