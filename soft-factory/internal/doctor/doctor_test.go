package doctor

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckAllDependencies(t *testing.T) {
	names := []string{"agent-sandbox", "git", "docker"}
	for mask := 0; mask < 8; mask++ {
		var calls []string
		results := Check(func(name string) (string, error) {
			i := len(calls)
			calls = append(calls, name)
			if mask&(1<<i) != 0 {
				return "untrusted-path", exec.ErrNotFound
			}
			return "/bin/" + name, nil
		})
		require.Equal(t, names, calls)
		for i, result := range results {
			wantPath := "/bin/" + names[i]
			if mask&(1<<i) != 0 {
				wantPath = ""
			}
			assert.Equal(t, names[i], result.Name, "mask %d", mask)
			assert.Equal(t, wantPath, result.Path, "mask %d", mask)
		}
	}
}
