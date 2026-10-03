package sandbox

import (
	"reflect"
	"testing"

	"agent-sandbox/internal/agent"
	"agent-sandbox/internal/docker"
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
			if err != nil {
				t.Fatal(err)
			}

			runOpts := docker.RunOptions{}
			paths := addImageMounts(&runOpts, Options{Agent: selected, Images: images})
			if !reflect.DeepEqual(paths, tt.wantPaths) {
				t.Fatalf("paths = %q, want %q", paths, tt.wantPaths)
			}
			if !reflect.DeepEqual(runOpts.Mounts, tt.wantMounts) {
				t.Fatalf("mounts = %#v, want %#v", runOpts.Mounts, tt.wantMounts)
			}
		})
	}
}
