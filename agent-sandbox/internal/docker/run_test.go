package docker

import (
	"reflect"
	"testing"

	"github.com/docker/docker/api/types/mount"
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
	if !reflect.DeepEqual(config.Mounts, want) {
		t.Fatalf("Mounts = %#v, want %#v", config.Mounts, want)
	}
}
