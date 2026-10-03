package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"golang.org/x/term"
)

// pump wires the process streams to the attached container and returns a
// channel that yields once the container's output has been fully drained.
func (c *Client) pump(attach types.HijackedResponse, opts RunOptions) <-chan error {
	if opts.attachStdin() {
		go func() {
			defer attach.CloseWrite()
			_, _ = io.Copy(attach.Conn, c.stdin)
		}()
	}

	done := make(chan error, 1)
	go func() {
		var err error
		if opts.TTY {
			// A TTY carries the raw terminal bytes: there are no stdout/stderr
			// frames to demultiplex, and stderr arrives on the same stream.
			_, err = io.Copy(c.stdout, attach.Reader)
		} else {
			_, err = stdcopy.StdCopy(c.stdout, c.stderr, attach.Reader)
		}
		done <- err
	}()

	return done
}

// rawTerminal puts the local terminal in raw mode so the container's TTY sees
// every keystroke — including the interrupts the agent handles itself. It
// returns the function that restores the terminal, which is a no-op when stdin
// is not a terminal.
func (c *Client) rawTerminal() (func(), error) {
	file, ok := c.stdin.(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) {
		return func() {}, nil
	}

	fd := int(file.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("switching the terminal to raw mode: %w", err)
	}

	return func() { _ = term.Restore(fd, state) }, nil
}

// watchResize matches the container's TTY to the local terminal, now and on
// every window change, so full-screen agents lay out correctly.
func (c *Client) watchResize(ctx context.Context, id string) {
	file, ok := c.stdout.(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) {
		return
	}

	fd := int(file.Fd())
	c.resize(ctx, id, fd)

	changed := make(chan os.Signal, 1)
	signal.Notify(changed, syscall.SIGWINCH)

	go func() {
		defer signal.Stop(changed)
		for {
			select {
			case <-ctx.Done():
				return
			case <-changed:
				c.resize(ctx, id, fd)
			}
		}
	}()
}

// resize reports the local terminal size to the container. Failures are
// ignored: the container may already be gone, and a stale size is not worth
// aborting the run over.
func (c *Client) resize(ctx context.Context, id string, fd int) {
	width, height, err := term.GetSize(fd)
	if err != nil {
		return
	}

	_ = c.api.ContainerResize(ctx, id, container.ResizeOptions{
		Height: uint(height),
		Width:  uint(width),
	})
}
