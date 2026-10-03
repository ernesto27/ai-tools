package agent

import (
	"embed"
	"encoding/json"
)

//go:embed providers.json
var modelFiles embed.FS

type provider struct {
	DefaultModel string `json:"defaultModel"`
}

func readProvider() (map[string]provider, error) {
	data, err := modelFiles.ReadFile("providers.json")
	if err != nil {
		return nil, err
	}

	var providers map[string]provider
	if err := json.Unmarshal(data, &providers); err != nil {
		return nil, err
	}
	return providers, nil
}
