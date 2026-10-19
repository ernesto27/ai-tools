package docker

import (
	"testing"

	"github.com/docker/docker/api/types/mount"
	"github.com/stretchr/testify/require"
)

func TestHostConfigKeepsExistingMountsWritableAndMakesAttachmentsReadOnly(t *testing.T) {
	config := hostConfig(RunOptions{Mounts: []Mount{
		{Host: "/host/worktree", Container: "/workspace"},
		{Host: "/host/Screenshot at 10:30:00.png", Container: "/agent-sandbox-images/1.png", ReadOnly: true},
	}})
	want := []mount.Mount{
		{Type: mount.TypeBind, Source: "/host/worktree", Target: "/workspace"},
		{Type: mount.TypeBind, Source: "/host/Screenshot at 10:30:00.png", Target: "/agent-sandbox-images/1.png", ReadOnly: true},
	}
	require.Equal(t, want, config.Mounts)
}
