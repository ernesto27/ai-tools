// Package docker drives the sandbox image through the Docker Engine API: it
// builds the image, reads the agent versions installed in it and runs an agent
// against a worktree.
package docker

import (
	"context"
	"fmt"
	"io"
	"os"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/client"
)

// Mount is a bind mount from a host path into the container.
type Mount struct {
	Host      string
	Container string
	ReadOnly  bool
}

// Tmpfs is an in-memory mount. Options is the raw tmpfs option string, such as
// "rw,exec,mode=1777".
type Tmpfs struct {
	Path    string
	Options string
}

// RunOptions is the configuration of a single container run.
type RunOptions struct {
	Entrypoint string
	Args       []string
	User       string
	// Interactive attaches stdin to the container.
	Interactive bool
	// TTY allocates a pseudo terminal, which implies Interactive.
	TTY         bool
	Env         []string // "KEY=VALUE" entries
	Mounts      []Mount
	Tmpfs       []Tmpfs
	HostNetwork bool
	// FormatStdout wraps agent output for presentation. Close flushes pending
	// bytes after the attachment is drained without closing the host stream.
	FormatStdout func(io.Writer) io.WriteCloser
}

// Client runs containers of a single image, built from the Dockerfile supplied
// by its caller. The embedded sandbox and generated external-base images share
// the same Docker lifecycle after their definitions have been selected.
type Client struct {
	api        *client.Client
	image      string
	dockerfile string

	stdin            io.Reader
	stdout           io.Writer
	stderr           io.Writer
	viewOwnsTerminal bool
	sizes            <-chan TerminalSize
}

type TerminalSize struct {
	Width, Height int
}

type Option func(*Client)

// WithStreams gives the caller ownership of the host terminal. Docker still
// allocates the agent's requested TTY, but never reads the view's keyboard or
// changes its raw mode. An output-only attachment must remain open: sending
// EOF to attached TTY stdin would also detach Docker's output streams.
func WithStreams(out io.Writer, sizes <-chan TerminalSize) Option {
	return func(c *Client) {
		if out == nil {
			out = io.Discard
		}
		c.stdin = nil
		c.stdout, c.stderr = out, out
		c.viewOwnsTerminal, c.sizes = true, sizes
	}
}

// New connects to the Docker daemon described by the environment and negotiates
// an API version with it. dockerfile is the image definition Build sends to the
// daemon; it may be the embedded default or a generated definition.
func New(image, dockerfile string, options ...Option) (*Client, error) {
	api, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("connecting to Docker: %w", err)
	}

	c := &Client{
		api:        api,
		image:      image,
		dockerfile: dockerfile,
		stdin:      os.Stdin,
		stdout:     os.Stdout,
		stderr:     os.Stderr,
	}
	for _, option := range options {
		option(c)
	}
	return c, nil
}

// Close releases the connection to the daemon.
func (c *Client) Close() error {
	return c.api.Close()
}

// ImageExists reports whether the image is present on the daemon.
func (c *Client) ImageExists(ctx context.Context) (bool, error) {
	if _, err := c.api.ImageInspect(ctx, c.image); err != nil {
		if cerrdefs.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("inspecting image %s: %w", c.image, err)
	}
	return true, nil
}
