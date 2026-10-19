package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"
)

type Model struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Created       int64  `json:"created"`
	ContextLength int    `json:"context_length"`
	InputPrice    string `json:"-"`
	OutputPrice   string `json:"-"`
	Pricing       struct {
		Input  string `json:"prompt"`
		Output string `json:"completion"`
	} `json:"pricing"`
}

type OpenRouter struct {
	ApiURL string
}

func NewOpenRouter() *OpenRouter {
	return &OpenRouter{
		ApiURL: "https://openrouter.ai/api/v1/",
	}
}

func (or *OpenRouter) GetModelsByProvider(provider string) ([]Model, error) {
	var endpoint = or.ApiURL + "models?model_authors=" + provider + "&sort=newest"
	ctx := context.Background()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch models: HTTP %s", resp.Status)
	}

	var result struct {
		Data []Model `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode models: %w", err)
	}

	for i := range result.Data {
		model := &result.Data[i]
		model.InputPrice, err = pricePerMillion(model.Pricing.Input)
		if err != nil {
			return nil, fmt.Errorf("model %s input price: %w", model.ID, err)
		}
		model.OutputPrice, err = pricePerMillion(model.Pricing.Output)
		if err != nil {
			return nil, fmt.Errorf("model %s output price: %w", model.ID, err)
		}
	}

	return result.Data, nil
}

func pricePerMillion(raw string) (string, error) {
	price, err := strconv.ParseFloat(raw, 64)
	if err != nil || price < 0 || math.IsNaN(price) || math.IsInf(price, 0) || math.IsInf(price*1_000_000, 0) {
		return "", fmt.Errorf("invalid price %q", raw)
	}
	return fmt.Sprintf("$%.2f / 1M tokens", price*1_000_000), nil
}
