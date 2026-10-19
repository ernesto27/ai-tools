package openrouter

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestGetModelsByProviderRealAPI(t *testing.T) {
	t.Skip("disabled")

	tests := []struct {
		name      string
		provider  string
		wantEmpty bool
	}{
		{name: "OpenAI models", provider: "openai"},
		{name: "unknown author", provider: "agent-sandbox-nonexistent-author", wantEmpty: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			models, err := NewOpenRouter().GetModelsByProvider(tt.provider)
			if err != nil {
				t.Fatalf("GetModelsByProvider(%q): %v", tt.provider, err)
			}
			if tt.wantEmpty {
				if len(models) != 0 {
					t.Fatalf("unknown author returned %d models, want 0", len(models))
				}
				return
			}
			if len(models) == 0 {
				t.Fatal("expected at least one model")
			}
			t.Logf("received %d models for %s", len(models), tt.provider)

			// The catalog changes, so validate usable fields instead of fixed models or prices.
			for i, model := range models {
				if model.InputPrice == "" || model.OutputPrice == "" {
					t.Errorf("model %q: formatted prices are missing", model.ID)
				}
				author, slug, ok := strings.Cut(model.ID, "/")
				// OpenRouter also returns aliases whose author starts with a tilde.
				if !ok || strings.TrimPrefix(author, "~") != tt.provider || strings.TrimSpace(slug) == "" {
					t.Errorf("model %d: invalid ID %q for author %s", i, model.ID, tt.provider)
				}
				if strings.TrimSpace(model.Name) == "" {
					t.Errorf("model %d: name is empty", i)
				}
				if strings.TrimSpace(model.Description) == "" {
					t.Errorf("model %q: description is empty", model.Name)
				}
				if model.ContextLength <= 0 {
					t.Errorf("model %q: context length = %d, want positive", model.Name, model.ContextLength)
				}
				for label, raw := range map[string]string{"input": model.Pricing.Input, "output": model.Pricing.Output} {
					price, err := strconv.ParseFloat(raw, 64)
					if err != nil || price < 0 || math.IsNaN(price) || math.IsInf(price, 0) {
						t.Errorf("model %q: invalid %s price %q", model.Name, label, raw)
					}
				}
			}
		})
	}
}

func TestPricePerMillion(t *testing.T) {
	tests := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{raw: "0.000002", want: "$2.00 / 1M tokens"},
		{raw: "0.00001", want: "$10.00 / 1M tokens"},
		{raw: "0.00000015", want: "$0.15 / 1M tokens"},
		{raw: "0", want: "$0.00 / 1M tokens"},
		{raw: "", wantErr: true},
		{raw: "invalid", wantErr: true},
		{raw: "-1", wantErr: true},
		{raw: "NaN", wantErr: true},
		{raw: "+Inf", wantErr: true},
		{raw: "1e308", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, err := pricePerMillion(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("pricePerMillion(%q) error = %v, wantErr %t", tt.raw, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("pricePerMillion(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
