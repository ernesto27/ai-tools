// Package prompts renders stage instructions embedded at build time.
package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed *.txt
var bundled embed.FS

// Parse fixed bundled templates once during package initialization.
var templates = template.Must(template.New("").Option("missingkey=error").ParseFS(bundled, "*.txt"))

// Render uses only bundled text. Data is inserted once without escaping or re-evaluation.
func Render(name string, data any) (string, error) {
	var output bytes.Buffer
	if err := templates.ExecuteTemplate(&output, name, data); err != nil {
		return "", fmt.Errorf("render bundled prompt %q: %w", name, err)
	}
	return output.String(), nil
}
