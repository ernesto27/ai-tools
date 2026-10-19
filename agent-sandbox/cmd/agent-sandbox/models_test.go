package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
)

func TestFormatContext(t *testing.T) {
	for _, tt := range []struct {
		name   string
		tokens int
		want   string
	}{
		{name: "below one thousand", tokens: 999, want: "999"},
		{name: "one thousand", tokens: 1000, want: "1K"},
		{name: "fractional thousand", tokens: 1500, want: "1.5K"},
		{name: "binary million", tokens: 1_048_576, want: "1.05M"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatContext(tt.tokens))
		})
	}
}

func TestModelDescription(t *testing.T) {
	for _, tt := range []struct {
		name, raw, want string
	}{
		{
			name: "nested parentheses in link URL",
			raw:  "See [model docs](https://example.com/models(v2)/docs(section)).",
			want: "See model docs.",
		},
		{
			name: "image link",
			raw:  "See ![model diagram](https://example.com/model.png) for details.",
			want: "See model diagram for details.",
		},
		{
			name: "bold text",
			raw:  "A **fast model** for coding.",
			want: "A fast model for coding.",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, modelDescription(tt.raw))
		})
	}
}

func TestModelColumnsLongName(t *testing.T) {
	for _, tt := range []struct {
		name, modelName, wantName string
		width                     int
	}{
		{
			name:      "narrow column",
			modelName: strings.Repeat("x", 80),
			wantName:  strings.Repeat("x", 15) + "…",
			width:     40,
		},
		{
			name:      "wide column",
			modelName: strings.Repeat("x", 80),
			wantName:  strings.Repeat("x", 39) + "…",
			width:     64,
		},
		{
			name:      "wide Unicode characters",
			modelName: strings.Repeat("界", 40),
			wantName:  strings.Repeat("界", 7) + "… ",
			width:     40,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := modelColumns(tt.modelName, "400K", "$2.00", "$10.00", tt.width)
			want := tt.wantName + "    400K   $2.00  $10.00"
			assert.Equal(t, want, got)
			{
				width := ansi.StringWidth(got)
				assert.Equal(t, tt.width, width)
			}
		})
	}
}
