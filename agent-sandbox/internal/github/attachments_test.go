package github

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareAttachments(t *testing.T) {
	const original = "Existing description\n"
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".mp4", ".mov", ".webm", ".PNG"} {
		t.Run("media/"+ext, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "capture"+ext)
			body, args, err := prepareAttachments(original, []string{path})
			require.NoError(t, err)
			require.Equal(t, original, body)
			require.Equal(t, []string{"--attach", path}, args)
		})
	}
	for _, ext := range []string{".txt", ".log", ".md", ".json", ".LOG"} {
		t.Run("text/"+ext, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "report&"+ext)
			require.NoError(t, os.WriteFile(path, []byte("<script>alert(\"x\")</script>\nA & B"), 0o600))
			body, args, err := prepareAttachments(original, []string{path})
			require.NoError(t, err)
			require.Empty(t, args)
			require.Equal(t, original+"\n\n<details>\n<summary>report&amp;"+ext+
				"</summary>\n\n<pre>&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;\nA &amp; B</pre>\n</details>", body)
		})
	}
	t.Run("mixed attachments preserve body and media order", func(t *testing.T) {
		dir := t.TempDir()
		text := filepath.Join(dir, "summary.md")
		require.NoError(t, os.WriteFile(text, []byte("Checks passed"), 0o600))
		image := filepath.Join(dir, "desktop.png")
		video := filepath.Join(dir, "demo.webm")
		body, args, err := prepareAttachments(original, []string{"check.cjs", image, "page.html", text, video, "report.xml"})
		require.NoError(t, err)
		require.Equal(t, []string{"--attach", image, "--attach", video}, args)
		require.Equal(t, original+"\n\n<details>\n<summary>summary.md</summary>\n\n<pre>Checks passed</pre>\n</details>", body)
	})
	for _, tc := range []struct {
		name  string
		paths []string
		want  string
	}{
		{name: "no attachments"},
		{name: "missing text", paths: []string{filepath.Join(t.TempDir(), "missing.log")}, want: "reading attachment"},
		{name: "unsupported files skipped", paths: []string{"archive.zip", "check.cjs", "page.html", "report.xml"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, args, err := prepareAttachments(original, tc.paths)
			if tc.want != "" {
				require.ErrorContains(t, err, tc.want)
				require.Empty(t, body)
				require.Empty(t, args)
				return
			}
			require.NoError(t, err)
			require.Equal(t, original, body)
			require.Empty(t, args)
		})
	}
}
