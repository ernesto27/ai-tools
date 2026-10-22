package tokencost

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ernesto27/ai-tools/agent-sandbox/agentoutput"
)

//go:embed model-pricing.json
var pricingJSON []byte

type Model struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Price    struct {
		Input     *float64 `json:"input"`
		Output    *float64 `json:"output"`
		CacheRead *float64 `json:"cache_read"`
	} `json:"price"`
}

type TokenCost struct {
	models []Model
}

func NewTokenCost() (*TokenCost, error) {
	var catalog struct {
		Models []Model `json:"models"`
	}

	if err := json.Unmarshal(pricingJSON, &catalog); err != nil {
		return nil, fmt.Errorf("decode embedded model pricing: %w", err)
	}

	return &TokenCost{
		models: catalog.Models,
	}, nil
}

func (tc *TokenCost) GetCostUsage(agent string, model string, tokenUsage agentoutput.TokenUsage) (float64, error) {
	if tokenUsage.TotalCostUSD != nil {
		return *tokenUsage.TotalCostUSD, nil
	}

	provider := ""
	switch agent {
	case "codex":
		provider = "openai"
	case "claude":
		provider = "anthropic"
	}

	if provider == "" {
		return 0, errors.New("no provider found")
	}

	currentModel := provider + "/" + model

	for _, m := range tc.models {
		if m.Model == currentModel {
			uncachedInput := tokenUsage.InputTokens - tokenUsage.CachedInputTokens

			cost := float64(uncachedInput)*(*m.Price.Input) +
				float64(tokenUsage.OutputTokens)*(*m.Price.Output)
			if tokenUsage.CachedInputTokens > 0 {
				if m.Price.CacheRead == nil {
					return 0, errors.New("cache-read price unavailable")
				}
				cost += float64(tokenUsage.CachedInputTokens) * (*m.Price.CacheRead)
			}
			return cost / 1_000_000, nil
		}
	}

	return 0, errors.New("no model found")
}
