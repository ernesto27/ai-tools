package doctor

import (
	"os/exec"
	"reflect"
	"testing"
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
		if !reflect.DeepEqual(calls, names) {
			t.Fatalf("mask %d: calls = %v", mask, calls)
		}
		for i, result := range results {
			wantPath := "/bin/" + names[i]
			if mask&(1<<i) != 0 {
				wantPath = ""
			}
			if result.Name != names[i] || result.Path != wantPath {
				t.Errorf("mask %d: result = %+v, want %s %q", mask, result, names[i], wantPath)
			}
		}
	}
}
