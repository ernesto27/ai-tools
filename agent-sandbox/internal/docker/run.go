package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/pkg/stdcopy"
)

// stopGrace is how long a cancelled agent gets to shut down on its own before
// the daemon kills it.
const stopGrace = 10 * time.Second

// stopOverhead is the slack added to the stop request's own deadline, so the
// kill that follows the grace period still has time to be reported.
const stopOverhead = 5 * time.Second

// attachStdin reports whether the container reads from our stdin. A TTY always
// implies it.
func (o RunOptions) attachStdin() bool {
	return o.Interactive || o.TTY
}

// Output runs a throwaway container and returns its standard output. It is the
// probe used to read the versions installed in the image.
func (c *Client) Output(ctx context.Context, entrypoint string, args ...string) (string, error) {
	config := &container.Config{
		Image:        c.image,
		Entrypoint:   strslice.StrSlice{entrypoint},
		Cmd:          args,
		AttachStdout: true,
		AttachStderr: true,
	}

	created, err := c.api.ContainerCreate(ctx, config, &container.HostConfig{}, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("creating container for %s: %w", entrypoint, err)
	}
	// AutoRemove stays off on purpose: the logs have to outlive the exit so they
	// can be read below. Removal happens here instead, even on a cancelled run.
	defer c.remove(context.WithoutCancel(ctx), created.ID)

	if err := c.api.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("starting container for %s: %w", entrypoint, err)
	}

	status, err := c.wait(ctx, created.ID)
	if err != nil {
		return "", err
	}

	stdout, stderr, err := c.logs(ctx, created.ID)
	if err != nil {
		return "", err
	}

	if status != 0 {
		command := strings.TrimSpace(entrypoint + " " + strings.Join(args, " "))
		return "", fmt.Errorf("%s exited with status %d: %s", command, status, strings.TrimSpace(stderr))
	}
	return stdout, nil
}

// Run runs the agent in the sandbox, wiring the process streams to it, and
// returns the agent's own exit code.
func (c *Client) Run(ctx context.Context, opts RunOptions) (int, error) {
	env, err := c.runtimeEnv(ctx, opts.Env)
	if err != nil {
		return 0, err
	}
	opts.Env = env

	created, err := c.api.ContainerCreate(ctx, containerConfig(c.image, opts), hostConfig(opts), nil, nil, "")
	if err != nil {
		return 0, fmt.Errorf("creating container: %w", err)
	}

	attach, err := c.api.ContainerAttach(ctx, created.ID, container.AttachOptions{
		Stream: true,
		Stdin:  opts.attachStdin(),
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return 0, fmt.Errorf("attaching to container: %w", err)
	}
	defer attach.Close()

	// AutoRemove only fires when a container exits, so until this one is
	// started nothing will ever clean it up. Give up before then and it has to
	// be removed here.
	started := false
	defer func() {
		if !started {
			c.remove(context.WithoutCancel(ctx), created.ID)
		}
	}()

	if opts.TTY {
		restore, err := c.rawTerminal()
		if err != nil {
			return 0, err
		}
		defer restore()
	}

	// The wait is registered before the start: AutoRemove can delete the
	// container the moment it exits, before a wait registered afterwards
	// would ever see it.
	waitCh, waitErrCh := c.api.ContainerWait(ctx, created.ID, container.WaitConditionNotRunning)

	if err := c.api.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return 0, fmt.Errorf("starting container: %w", err)
	}
	started = true

	outputDone := c.pump(attach, opts)
	if opts.TTY {
		c.watchResize(ctx, created.ID)
	}

	status, err := c.awaitExit(ctx, created.ID, waitCh, waitErrCh)
	if err != nil {
		return 0, err
	}

	// Only the output side is drained: the stdin copy can still be blocked on a
	// read that will never return.
	if err := <-outputDone; err != nil {
		return 0, fmt.Errorf("streaming container output: %w", err)
	}
	return status, nil
}

// runtimeEnv layers the sandbox's explicit environment over the image's own
// environment. Supplying Config.Env to the Engine replaces the image values,
// rather than adding to them, so forwarding only agent settings would discard
// a language image's PATH (for example, /usr/local/go/bin in golang:alpine).
func (c *Client) runtimeEnv(ctx context.Context, overrides []string) ([]string, error) {
	image, err := c.api.ImageInspect(ctx, c.image)
	if err != nil {
		return nil, fmt.Errorf("inspecting image environment for %s: %w", c.image, err)
	}
	return mergeEnv(image.Config.Env, overrides), nil
}

// mergeEnv preserves the image order and replaces a value only when the
// sandbox explicitly owns the same variable. That lets agent-specific settings
// such as CODEX_HOME win without losing unrelated image configuration.
func mergeEnv(imageEnv, overrides []string) []string {
	merged := append([]string(nil), imageEnv...)
	positions := make(map[string]int, len(merged))
	for i, entry := range merged {
		positions[envName(entry)] = i
	}

	for _, override := range overrides {
		name := envName(override)
		if i, ok := positions[name]; ok {
			merged[i] = override
			continue
		}
		positions[name] = len(merged)
		merged = append(merged, override)
	}
	return merged
}

func envName(entry string) string {
	name, _, _ := strings.Cut(entry, "=")
	return name
}

// containerConfig translates the run options into the container's own settings.
func containerConfig(image string, opts RunOptions) *container.Config {
	stdin := opts.attachStdin()

	config := &container.Config{
		Image:        image,
		User:         opts.User,
		Env:          opts.Env,
		Cmd:          opts.Args,
		Tty:          opts.TTY,
		OpenStdin:    stdin,
		StdinOnce:    stdin,
		AttachStdin:  stdin,
		AttachStdout: true,
		AttachStderr: true,
	}
	if opts.Entrypoint != "" {
		config.Entrypoint = strslice.StrSlice{opts.Entrypoint}
	}
	return config
}

// hostConfig translates the run options into the host-side settings. The
// container is always disposable: it is removed as soon as the agent exits.
func hostConfig(opts RunOptions) *container.HostConfig {
	config := &container.HostConfig{AutoRemove: true}

	for _, bind := range opts.Mounts {
		config.Mounts = append(config.Mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   bind.Host,
			Target:   bind.Container,
			ReadOnly: bind.ReadOnly,
		})
	}
	if len(opts.Tmpfs) > 0 {
		config.Tmpfs = make(map[string]string, len(opts.Tmpfs))
		for _, tmpfs := range opts.Tmpfs {
			config.Tmpfs[tmpfs.Path] = tmpfs.Options
		}
	}

	if opts.HostNetwork {
		config.NetworkMode = "host"
	}

	return config
}

// wait blocks until the container stops and returns its exit status.
func (c *Client) wait(ctx context.Context, id string) (int, error) {
	waitCh, errCh := c.api.ContainerWait(ctx, id, container.WaitConditionNotRunning)
	return c.awaitExit(ctx, id, waitCh, errCh)
}

// awaitExit resolves the pair of channels ContainerWait hands back. A cancelled
// context stops the container rather than abandoning it: the agent holds the
// worktree open, so walking away would leave it editing files unattended.
func (c *Client) awaitExit(ctx context.Context, id string, waitCh <-chan container.WaitResponse, errCh <-chan error) (int, error) {
	select {
	case err := <-errCh:
		return 0, fmt.Errorf("waiting for container: %w", err)
	case result := <-waitCh:
		if result.Error != nil {
			return 0, errors.New(result.Error.Message)
		}
		return int(result.StatusCode), nil
	case <-ctx.Done():
		return 0, c.stop(ctx, id, waitCh, errCh)
	}
}

// stop shuts down a container whose run was cancelled and waits for the exit,
// which is what gives AutoRemove its chance to clean up. It returns the
// cancellation that caused it, so the caller still reports why the run ended.
func (c *Client) stop(ctx context.Context, id string, waitCh <-chan container.WaitResponse, errCh <-chan error) error {
	cause := ctx.Err()

	// The stop cannot run on the cancelled context: that would abort the very
	// request meant to clean it up. It gets a deadline of its own, longer than
	// the grace period so a SIGKILL still has time to land and be reported.
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stopGrace+stopOverhead)
	defer cancel()

	grace := int(stopGrace.Seconds())
	err := c.api.ContainerStop(stopCtx, id, container.StopOptions{Timeout: &grace})
	if err != nil && !cerrdefs.IsNotFound(err) {
		return fmt.Errorf("stopping container after %v: %w", cause, err)
	}

	// The wait registered before the start is still live, and the stop is what
	// makes it fire.
	select {
	case <-waitCh:
	case <-errCh:
	case <-stopCtx.Done():
	}

	return cause
}

// remove deletes a container, forcing it if it is somehow still running.
// Failures are not worth reporting over whatever brought us here.
func (c *Client) remove(ctx context.Context, id string) {
	_ = c.api.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
}

// logs returns the container's buffered stdout and stderr.
func (c *Client) logs(ctx context.Context, id string) (stdout, stderr string, err error) {
	reader, err := c.api.ContainerLogs(ctx, id, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return "", "", fmt.Errorf("reading container logs: %w", err)
	}
	defer reader.Close()

	var out, errOut bytes.Buffer
	if _, err := stdcopy.StdCopy(&out, &errOut, reader); err != nil {
		return "", "", fmt.Errorf("reading container logs: %w", err)
	}
	return out.String(), errOut.String(), nil
}
