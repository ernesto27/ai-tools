package sandbox

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/agent"
	"github.com/ernesto27/ai-tools/agent-sandbox/internal/docker"
)

func TestAddImageMountsUsesAgentImageCapability(t *testing.T) {
	images := []string{"/host/mockup.png", "/host/reference.jpg"}
	wantPaths := []string{
		"/agent-sandbox-images/1.png",
		"/agent-sandbox-images/2.jpg",
	}
	wantMounts := []docker.Mount{
		{Host: images[0], Container: wantPaths[0], ReadOnly: true},
		{Host: images[1], Container: wantPaths[1], ReadOnly: true},
	}

	for _, tt := range []struct {
		name       string
		wantPaths  []string
		wantMounts []docker.Mount
	}{
		{name: "codex", wantPaths: wantPaths, wantMounts: wantMounts},
		{name: "claude", wantPaths: wantPaths, wantMounts: wantMounts},
		{name: "opencode"},
		{name: "pi"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			selected, err := agent.Lookup(tt.name)
			require.NoError(t, err)

			runOpts := docker.RunOptions{}
			paths := addImageMounts(&runOpts, Options{Agent: selected, Images: images})
			require.Equal(t, tt.wantPaths, paths)
			require.Equal(t, tt.wantMounts, runOpts.Mounts)
		})
	}
}
