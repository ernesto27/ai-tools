//go:build !unix

package sandbox

import "testing"

func createLegacyFIFO(t *testing.T) { t.Skip("FIFO requires Unix") }
