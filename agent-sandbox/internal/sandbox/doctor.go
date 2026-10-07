package sandbox

import "os/exec"

// Dependency describes a host tool's installation and the feature that uses it.
type Dependency struct {
	Name      string
	Optional  string
	Installed bool
}

// Dependencies checks PATH without running tools or contacting services.
func Dependencies() []Dependency {
	dependencies := []Dependency{
		{Name: "git"},
		{Name: "docker"},
		{Name: "gh", Optional: "PRs"},
		{Name: "code", Optional: "worktree-editor"},
	}
	// Docker uses the Engine API during sandbox execution. Here, its CLI is
	// only an installation indicator, not proof that the engine is available.
	for i := range dependencies {
		_, err := exec.LookPath(dependencies[i].Name)
		dependencies[i].Installed = err == nil
	}
	return dependencies
}
