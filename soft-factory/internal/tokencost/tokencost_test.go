package tokencost

import (
	"testing"

	"github.com/ernesto27/ai-tools/agent-sandbox/agentoutput"
	"github.com/stretchr/testify/require"
)

func TestGetCostUsage(t *testing.T) {
	calculator, err := NewTokenCost()
	require.NoError(t, err)
	// Expected costs use the rates in the bundled model-pricing.json snapshot.

	for _, tt := range []struct {
		name    string
		agent   string
		model   string
		usage   agentoutput.TokenUsage
		want    float64
		wantErr string
	}{
		{
			name: "Codex input and output in USD", agent: "codex", model: "gpt-6.1-sol",
			usage: agentoutput.TokenUsage{InputTokens: 100_000, OutputTokens: 10_000, TotalTokens: 110_000},
			want:  0.30,
		},
		{
			name: "Claude selects Anthropic prices", agent: "claude", model: "claude-opus-5.5",
			usage: agentoutput.TokenUsage{InputTokens: 100_000, OutputTokens: 10_000, TotalTokens: 110_000},
			want:  0.60,
		},
		{
			name: "Sol 6.1 one million mixed tokens", agent: "codex", model: "gpt-6.1-sol",
			usage: agentoutput.TokenUsage{InputTokens: 900_000, OutputTokens: 100_000, TotalTokens: 1_000_000},
			want:  2.80,
		},
		{
			name: "Sol 6.1 multiple million tokens with cache", agent: "codex", model: "gpt-6.1-sol",
			usage: agentoutput.TokenUsage{InputTokens: 4_000_000, CachedInputTokens: 3_000_000, OutputTokens: 1_000_000, TotalTokens: 5_000_000},
			want:  12.30,
		},
		{
			name: "Opus 5.5 one million mixed tokens", agent: "claude", model: "claude-opus-5.5",
			usage: agentoutput.TokenUsage{InputTokens: 900_000, OutputTokens: 100_000, TotalTokens: 1_000_000},
			want:  5.60,
		},
		{
			name: "Opus 5.5 multiple million tokens with cache", agent: "claude", model: "claude-opus-5.5",
			usage: agentoutput.TokenUsage{InputTokens: 4_000_000, CachedInputTokens: 3_000_000, OutputTokens: 1_000_000, TotalTokens: 5_000_000},
			want:  24.60,
		},
		{
			name: "cached input charged separately without double counting", agent: "codex", model: "gpt-6.1-sol",
			usage: agentoutput.TokenUsage{InputTokens: 400_000, CachedInputTokens: 300_000, OutputTokens: 10_000, TotalTokens: 410_000},
			want:  0.33,
		},
		{
			name: "input only", agent: "codex", model: "gpt-6.1-sol",
			usage: agentoutput.TokenUsage{InputTokens: 1_000_000, TotalTokens: 1_000_000},
			want:  2,
		},
		{
			name: "cache price unnecessary without cached usage", agent: "codex", model: "gpt-3.5-turbo",
			usage: agentoutput.TokenUsage{InputTokens: 100_000, OutputTokens: 10_000, TotalTokens: 110_000},
			want:  0.065,
		},
		{
			name: "cache price required for cached usage", agent: "codex", model: "gpt-3.5-turbo",
			usage:   agentoutput.TokenUsage{InputTokens: 400_000, CachedInputTokens: 300_000, OutputTokens: 10_000, TotalTokens: 410_000},
			wantErr: "cache-read price unavailable",
		},
		{
			name: "output only", agent: "codex", model: "gpt-6.1-sol",
			usage: agentoutput.TokenUsage{OutputTokens: 1_000_000, TotalTokens: 1_000_000},
			want:  10,
		},
		{name: "zero usage", agent: "codex", model: "gpt-6.1-sol"},
		{
			name: "Claude cached input", agent: "claude", model: "claude-opus-5.5",
			usage: agentoutput.TokenUsage{InputTokens: 400_000, CachedInputTokens: 300_000, OutputTokens: 10_000, TotalTokens: 410_000},
			want:  0.66,
		},
		{name: "unknown agent", agent: "other", model: "gpt-6.1-sol", wantErr: "no provider found"},
		{name: "empty agent", model: "gpt-6.1-sol", wantErr: "no provider found"},
		{name: "unknown model", agent: "codex", model: "missing", wantErr: "no model found"},
		{name: "empty model", agent: "codex", wantErr: "no model found"},
		{name: "alias unsupported", agent: "claude", model: "opus", wantErr: "no model found"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calculator.GetCostUsage(tt.agent, tt.model, tt.usage)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.Zero(t, got)
				return
			}
			require.NoError(t, err)
			require.InDelta(t, tt.want, got, 1e-12)
		})
	}
}
