// Package agentoutput defines shared types for agent-sandbox's structured output.
package agentoutput

// TokenUsage contains the token counts emitted in a JSON usage event.
type TokenUsage struct {
	InputTokens       int64 `json:"input_tokens"`
	CachedInputTokens int64 `json:"cached_input_tokens"`
	OutputTokens      int64 `json:"output_tokens"`
	TotalTokens       int64 `json:"total_tokens"`
}
