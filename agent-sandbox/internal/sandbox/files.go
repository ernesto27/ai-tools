package sandbox

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/docker"
)

// Numbered directories preserve basenames without colliding when attachments
// from different host directories have the same name.
func fileAttachmentPath(index int, file string) string {
	return filepath.Join("/agent-sandbox-files", strconv.Itoa(index+1), filepath.Base(file))
}

func addFileMounts(opts *docker.RunOptions, files []string) {
	for index, file := range files {
		opts.Mounts = append(opts.Mounts, docker.Mount{
			Host: file, Container: fileAttachmentPath(index, file), ReadOnly: true,
		})
	}
}

func fileAttachmentManifest(files []string) string {
	if len(files) == 0 {
		return ""
	}
	var manifest strings.Builder
	manifest.WriteString("\n\nReference files mounted read-only (use the container paths when reading them):\n")
	for index, file := range files {
		// Quoting prevents newlines in filenames from impersonating manifest rows.
		fmt.Fprintf(&manifest, "%s: %s\n", strconv.Quote(filepath.Base(file)), strconv.Quote(fileAttachmentPath(index, file)))
	}
	return manifest.String()
}
