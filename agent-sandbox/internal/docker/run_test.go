package docker

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/stretchr/testify/require"
)

func TestHostConfigResources(t *testing.T) {
	for _, tc := range []struct {
		name        string
		cpu, memory int64
	}{
		{name: "unlimited"},
		{name: "cpu only", cpu: 1500000000},
		{name: "memory only", memory: 2 << 30},
		{name: "both", cpu: 1500000000, memory: 2 << 30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := hostConfig(RunOptions{NanoCPUs: tc.cpu, MemoryBytes: tc.memory})
			require.Equal(t, tc.cpu, config.NanoCPUs)
			require.Equal(t, tc.memory, config.Memory)
			require.Zero(t, config.MemorySwap)
			require.True(t, config.AutoRemove)
		})
	}
}

func TestAwaitExitPrefersAvailableExitAtTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeoutCause(t.Context(), -time.Second, ErrRunTimeout)
	defer cancel()
	wait := make(chan container.WaitResponse, 1)
	wait <- container.WaitResponse{StatusCode: 124}
	client := &Client{}
	status, err := client.awaitExit(ctx, "agent", wait, nil)
	require.NoError(t, err)
	require.Equal(t, 124, status)
}

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
