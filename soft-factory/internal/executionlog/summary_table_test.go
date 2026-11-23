package executionlog

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSummaryTableWrapsLongCells(t *testing.T) {
	path := "/" + strings.Repeat("p", 70)
	row := formatSummaryRow(summaryRow{
		"1. IMPLEMENTATION", "SUCCEEDED", "codex\ngpt-6.1-sol",
		"1 min 6 sec", "107K", "$0.0319", "Created café.txt.\r\n\r\n" + path,
	})

	want := []string{"Created café.txt.", "", path[:45], path[45:]}
	lines := strings.Split(strings.TrimSuffix(row, "\n"), "\n")
	require.Len(t, lines, len(want))
	for i, line := range lines {
		require.Len(t, []rune(line), len(summaryRule())-1)
		cells := strings.Split(line, "|")
		require.Len(t, cells, 9)
		require.Equal(t, want[i], strings.TrimSpace(cells[7]))
	}
	require.Contains(t, lines[0], "codex")
	require.Contains(t, lines[1], "gpt-6.1-sol")
}
