package sandbox

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/docker"
)

var attachmentTestKinds = []struct {
	name   string
	kind   attachmentKind
	root   string
	header string
}{
	{
		name:   "files",
		kind:   fileAttachments,
		root:   "/agent-sandbox-files",
		header: "Reference files mounted read-only (use the container paths when reading them):",
	},
	{
		name:   "folders",
		kind:   folderAttachments,
		root:   "/agent-sandbox-folders",
		header: "Reference folders mounted read-only, read content of all files",
	},
}

func TestAttachmentPath(t *testing.T) {
	for _, kind := range attachmentTestKinds {
		t.Run(kind.name, func(t *testing.T) {
			for _, tt := range []struct {
				name  string
				index int
				host  string
				want  string
			}{
				{"first", 0, "/host/docs", "/1/docs"},
				{"second", 1, "/other/docs", "/2/docs"},
				{"spaces", 0, "/host/my docs", "/1/my docs"},
				{"trailing slash", 0, "/host/docs/", "/1/docs"},
			} {
				t.Run(tt.name, func(t *testing.T) {
					want := kind.root + tt.want
					require.Equal(t, want, kind.kind.path(tt.index, tt.host))
				})
			}
		})
	}
}

func TestAddAttachmentMounts(t *testing.T) {
	existing := docker.Mount{Host: "/host/worktree", Container: "/workspace"}
	for _, kind := range attachmentTestKinds {
		t.Run(kind.name, func(t *testing.T) {
			for _, tt := range []struct {
				name  string
				hosts []string
				want  []docker.Mount
			}{
				{
					name: "no attachments preserves existing mounts",
					want: []docker.Mount{existing},
				},
				{
					name:  "duplicate basenames have separate read-only mounts",
					hosts: []string{"/host/docs", "/other/docs"},
					want: []docker.Mount{
						existing,
						{Host: "/host/docs", Container: kind.root + "/1/docs", ReadOnly: true},
						{Host: "/other/docs", Container: kind.root + "/2/docs", ReadOnly: true},
					},
				},
			} {
				t.Run(tt.name, func(t *testing.T) {
					opts := docker.RunOptions{Mounts: []docker.Mount{existing}}
					kind.kind.addMounts(&opts, tt.hosts)
					require.Equal(t, tt.want, opts.Mounts)
				})
			}
		})
	}
}

func TestAttachmentManifest(t *testing.T) {
	for _, kind := range attachmentTestKinds {
		t.Run(kind.name, func(t *testing.T) {
			header := "\n\n" + kind.header + "\n"
			for _, tt := range []struct {
				name  string
				hosts []string
				want  string
			}{
				{
					name: "no attachments",
					want: "",
				},
				{
					name:  "duplicate basenames",
					hosts: []string{"/host/docs", "/other/docs"},
					want: header +
						"\"docs\": \"" + kind.root + "/1/docs\"\n" +
						"\"docs\": \"" + kind.root + "/2/docs\"\n",
				},
				{
					name:  "spaces",
					hosts: []string{"/host/my docs"},
					want:  header + "\"my docs\": \"" + kind.root + "/1/my docs\"\n",
				},
				{
					name:  "quotes and newlines are escaped",
					hosts: []string{"/host/a\"b\nc"},
					want:  header + "\"a\\\"b\\nc\": \"" + kind.root + "/1/a\\\"b\\nc\"\n",
				},
			} {
				t.Run(tt.name, func(t *testing.T) {
					got := kind.kind.manifest(tt.hosts)
					require.Equal(t, tt.want, got)
				})
			}
		})
	}
}
