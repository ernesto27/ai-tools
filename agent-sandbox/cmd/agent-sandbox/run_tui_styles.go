package main

import (
	"image/color"
	"strings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type livePalette struct {
	text, muted, accent, border, focus, track, thumb, warning, failure color.Color
}

func liveColors(dark bool) livePalette {
	choose := lipgloss.LightDark(dark)
	return livePalette{
		text:    choose(lipgloss.Color("#26313D"), lipgloss.Color("#D5DCE3")),
		muted:   choose(lipgloss.Color("#65717D"), lipgloss.Color("#8996A3")),
		accent:  choose(lipgloss.Color("#317386"), lipgloss.Color("#88B7C7")),
		border:  choose(lipgloss.Color("#CDD4DB"), lipgloss.Color("#303A43")),
		focus:   choose(lipgloss.Color("#7E99A4"), lipgloss.Color("#526975")),
		track:   choose(lipgloss.Color("#E0E4E8"), lipgloss.Color("#29323B")),
		thumb:   choose(lipgloss.Color("#7B8791"), lipgloss.Color("#58636E")),
		warning: choose(lipgloss.Color("#94651C"), lipgloss.Color("#D3B17B")),
		failure: choose(lipgloss.Color("#B34444"), lipgloss.Color("#D58B8B")),
	}
}

func liveOutput(lines []string, width int, palette livePalette) string {
	styled := make([]string, len(lines))
	for i, line := range lines {
		style := lipgloss.NewStyle().Foreground(palette.text)
		switch {
		case strings.HasPrefix(line, "ERROR:") || strings.Contains(line, " ERROR "):
			style = style.Foreground(palette.failure)
		case strings.HasPrefix(line, "warning:"):
			style = style.Foreground(palette.warning)
		case strings.HasPrefix(line, "$ "):
			style = style.Foreground(palette.accent).Bold(true)
		case line == "user", line == "thinking", line == "exec", line == "codex", line == "environment", line == "environment summary":
			style = style.Foreground(palette.muted).Bold(true)
		case strings.HasPrefix(line, "OpenAI Codex"):
			style = style.Foreground(palette.muted)
		}
		styled[i] = style.Render(ansi.Wrap(line, max(1, width), "/"))
	}
	return strings.Join(styled, "\n")
}

func livePanel(title string, v viewport.Model, focused bool, palette livePalette) string {
	border, marker, markerColor := palette.border, "○", palette.muted
	if focused {
		border, marker, markerColor = palette.focus, "●", palette.accent
	}
	gap := max(0, v.Width()+2-lipgloss.Width(marker+" "+title))
	header := lipgloss.NewStyle().Foreground(markerColor).Render(marker) + " " +
		lipgloss.NewStyle().Bold(true).Foreground(palette.text).Render(title) +
		strings.Repeat(" ", gap)
	rows := strings.Split(liveScrollbar(v), "\n")
	for i, row := range rows {
		barColor := palette.track
		if row == "▐" {
			barColor = palette.thumb
		}
		rows[i] = lipgloss.NewStyle().Foreground(barColor).Render(row)
	}
	content := lipgloss.JoinHorizontal(lipgloss.Top, v.View(),
		lipgloss.NewStyle().PaddingLeft(1).Render(strings.Join(rows, "\n")))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).
		Padding(0, 1).Render(header + "\n\n" + content)
}

func liveScrollbar(v viewport.Model) string {
	height, total := v.Height(), v.TotalLineCount()
	if height <= 0 {
		return ""
	}
	rows := make([]string, height)
	if total <= height {
		for i := range rows {
			rows[i] = " "
		}
		return strings.Join(rows, "\n")
	}
	// TotalLineCount includes soft-wrapped rows, keeping the thumb proportional
	// to the text actually visible after a terminal resize.
	thumb := max(1, height*height/total)
	start := int(v.ScrollPercent()*float64(height-thumb) + 0.5)
	for i := range rows {
		rows[i] = "│"
		if i >= start && i < start+thumb {
			rows[i] = "▐"
		}
	}
	return strings.Join(rows, "\n")
}
