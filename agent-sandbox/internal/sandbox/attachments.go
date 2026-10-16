package sandbox

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/docker"
)

type attachmentKind struct {
	root   string
	header string
}

var (
	fileAttachments = attachmentKind{
		root:   "/agent-sandbox-files",
		header: "Reference files mounted read-only (use the container paths when reading them):",
	}
	folderAttachments = attachmentKind{
		root:   "/agent-sandbox-folders",
		header: "Reference folders mounted read-only, read content of all files",
	}
)

// Numbered directories preserve basenames without colliding when attachments
// from different host directories have the same name.
func (k attachmentKind) path(index int, host string) string {
	return filepath.Join(k.root, strconv.Itoa(index+1), filepath.Base(host))
}

func (k attachmentKind) addMounts(opts *docker.RunOptions, hosts []string) {
	for index, host := range hosts {
		opts.Mounts = append(opts.Mounts, docker.Mount{
			Host: host, Container: k.path(index, host), ReadOnly: true,
		})
	}
}

func (k attachmentKind) manifest(hosts []string) string {
	if len(hosts) == 0 {
		return ""
	}
	var manifest strings.Builder
	fmt.Fprintf(&manifest, "\n\n%s\n", k.header)
	for index, host := range hosts {
		// Quoting prevents newlines in names from impersonating manifest rows.
		fmt.Fprintf(&manifest, "%s: %s\n", strconv.Quote(filepath.Base(host)), strconv.Quote(k.path(index, host)))
	}
	return manifest.String()
}
