//go:build unix

package sandbox

import (
	"syscall"
	"testing"
)

func createLegacyFIFO(t *testing.T) {
	t.Helper()
	if err := syscall.Mkfifo("agent-sandbox.json", 0600); err != nil {
		t.Fatal(err)
	}
}
