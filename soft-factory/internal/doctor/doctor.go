// Package doctor checks machine dependencies without executing them.
package doctor

// Result records whether a dependency could be resolved on PATH.
type Result struct {
	Name string
	Path string
}

// Check resolves every required dependency before returning its results.
// Lookup errors indicate that the executable is unavailable.
func Check(lookPath func(string) (string, error)) []Result {
	results := make([]Result, 0, 3)
	for _, name := range []string{"agent-sandbox", "git", "docker"} {
		path, err := lookPath(name)
		if err != nil {
			path = ""
		}
		results = append(results, Result{Name: name, Path: path})
	}
	return results
}
