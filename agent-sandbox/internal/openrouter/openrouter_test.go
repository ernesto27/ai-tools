package openrouter

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			require.NoError(t, err)
			if tt.wantEmpty {
				require.Empty(t, models)
				return
			}
			require.NotEmpty(t, models)
			t.Logf("received %d models for %s", len(models), tt.provider)

			// The catalog changes, so validate usable fields instead of fixed models or prices.
			for i, model := range models {
				assert.NotEmpty(t, model.InputPrice, "model %q", model.ID)
				assert.NotEmpty(t, model.OutputPrice, "model %q", model.ID)
				author, slug, ok := strings.Cut(model.ID, "/")
				// OpenRouter also returns aliases whose author starts with a tilde.
				assert.True(t, ok, "model %d: invalid ID %q", i, model.ID)
				assert.Equal(t, tt.provider, strings.TrimPrefix(author, "~"))
				assert.NotEmpty(t, strings.TrimSpace(slug), "model %d: invalid ID %q", i, model.ID)
				assert.NotEqual(t, "", strings.TrimSpace(model.Name))
				assert.NotEqual(t, "", strings.TrimSpace(model.Description))
				assert.Greater(t, model.ContextLength, 0)
				for label, raw := range map[string]string{"input": model.Pricing.Input, "output": model.Pricing.Output} {
					price, err := strconv.ParseFloat(raw, 64)
					if assert.NoError(t, err, "model %q: invalid %s price %q", model.Name, label, raw) {
						assert.GreaterOrEqual(t, price, float64(0))
						assert.False(t, math.IsNaN(price), "model %q: %s price", model.Name, label)
						assert.False(t, math.IsInf(price, 0), "model %q: %s price", model.Name, label)
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
			require.Equal(t, tt.wantErr, (err != nil))
			assert.Equal(t, tt.want, got)
		})
	}
}
