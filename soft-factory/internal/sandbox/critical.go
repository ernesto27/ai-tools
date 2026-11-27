package sandbox

import (
	"context"
	"fmt"
	"strings"

	"soft-factory/internal/config"
	"soft-factory/internal/executionlog"
)

func CriticalChanges(ctx context.Context, branch string, critical config.Critical) ([]string, error) {
	worktree, err := executionlog.WorktreeForBranch(ctx, branch)
	if err != nil {
		return nil, err
	}
	prefix, err := gitOutput(ctx, ".", "rev-parse", "--show-prefix")
	if err != nil {
		return nil, fmt.Errorf("resolve project path for critical files: %w", err)
	}
	// The worktree is rooted at the repository, while config paths are project-relative.
	relative := "--relative=" + strings.TrimSuffix(string(prefix), "\n")
	base, _ := comparisonBase(ctx, worktree, branch)
	out, err := gitOutput(ctx, worktree, "diff", relative, "--name-only", "--no-renames", "-z", base+"..HEAD", "--")
	if err != nil {
		return nil, err
	}
	changed := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	fmt.Println(changed)

	resp := []string{}

	for _, file := range changed {
		fmt.Println(file)
		for _, criticalFile := range critical.Files {
			if file == criticalFile {
				resp = append(resp, file)
			}
		}

		for _, folder := range critical.Folders {
			folder = strings.TrimSuffix(folder, "/")
			if strings.HasPrefix(file, folder+"/") {
				resp = append(resp, file)
				break
			}
		}
	}

	return resp, nil
}

// CriticalSection formats changed critical files as a Markdown PR section.
// It returns "" when no files matched.
func CriticalSection(files []string) string {
	if len(files) == 0 {
		return ""
	}
	var section strings.Builder
	section.WriteString("## ⚠️ Critical files changed\n")
	section.WriteString("The following files are marked as critical in software-factory.json:\n")
	for _, file := range files {
		fmt.Fprintf(&section, "- `%s`\n", file)
	}
	return section.String()
}
