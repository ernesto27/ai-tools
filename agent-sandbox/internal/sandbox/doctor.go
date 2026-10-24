package sandbox

import (
	"encoding/json"
	"io"
	"os/exec"
)

// Dependency describes a host tool's installation and the feature that uses it.
type Dependency struct {
	Name      string
	Optional  string
	Installed bool
}

// WriteDependenciesJSON writes only the installation fields consumed by CLI clients.
func WriteDependenciesJSON(w io.Writer, dependencies []Dependency) error {
	type dependencyStatus struct {
		Name      string `json:"name"`
		Installed bool   `json:"installed"`
	}
	statuses := make([]dependencyStatus, len(dependencies))
	for i, dependency := range dependencies {
		statuses[i] = dependencyStatus{Name: dependency.Name, Installed: dependency.Installed}
	}
	return json.NewEncoder(w).Encode(statuses)
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
