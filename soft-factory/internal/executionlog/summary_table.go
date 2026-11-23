package executionlog

import "strings"

type summaryRow [7]string

var summaryWidths = [len(summaryRow{})]int{23, 11, 16, 12, 11, 11, 45}

var summaryWhitespace = strings.NewReplacer("\r\n", "\n", "\r", "\n", "\t", "    ")

func summaryRule() string {
	var rule strings.Builder
	rule.WriteByte('+')
	for _, width := range summaryWidths {
		rule.WriteString(strings.Repeat("-", width+2))
		rule.WriteByte('+')
	}
	rule.WriteByte('\n')
	return rule.String()
}

func summaryTableHeader() string {
	rule := summaryRule()
	return rule + formatSummaryRow(summaryRow{
		"Stage", "Status", "Agent / Model", "Time", "Tokens", "Cost (USD)", "Result",
	}) + rule
}

func formatSummaryRow(row summaryRow) string {
	var cells [len(summaryWidths)][][]rune
	height := 0
	for column, value := range row {
		cells[column] = wrapSummaryCell(value, summaryWidths[column])
		height = max(height, len(cells[column]))
	}

	var out strings.Builder
	for line := range height {
		out.WriteByte('|')
		for column, width := range summaryWidths {
			var value []rune
			if line < len(cells[column]) {
				value = cells[column][line]
			}
			out.WriteByte(' ')
			out.WriteString(string(value))
			out.WriteString(strings.Repeat(" ", width-len(value)))
			out.WriteString(" |")
		}
		out.WriteByte('\n')
	}
	return out.String()
}

func wrapSummaryCell(value string, width int) [][]rune {
	var lines [][]rune
	for _, paragraph := range strings.Split(summaryWhitespace.Replace(value), "\n") {
		text := []rune(paragraph)
		for len(text) > width {
			end := width
			for end > 0 && text[end] != ' ' {
				end--
			}
			if end == 0 {
				// A word, path, or URL longer than the column must wrap too.
				lines = append(lines, text[:width])
				text = text[width:]
				continue
			}
			lines = append(lines, text[:end])
			text = text[end+1:]
		}
		lines = append(lines, text)
	}
	return lines
}
