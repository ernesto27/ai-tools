package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Logs live outside the checkout so diagnosing a run does not add files to
// the caller's working copy. Each invocation gets a private, separate file.
func newLiveLog(mode string) (*os.File, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(config, "agent-sandbox", "logs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating %s log directory: %w", mode, err)
	}
	file, err := os.CreateTemp(dir, mode+"-tui-"+time.Now().UTC().Format("20060102T150405Z")+"-*.log")
	if err != nil {
		return nil, fmt.Errorf("creating %s log: %w", mode, err)
	}
	if _, err := fmt.Fprintf(file, "%s started %s\n%s · %s/%s\n\n",
		mode, time.Now().UTC().Format(time.RFC3339Nano), runtime.Version(), runtime.GOOS, runtime.GOARCH); err != nil {
		file.Close()
		return nil, fmt.Errorf("writing %s log: %w", mode, err)
	}
	return file, nil
}
