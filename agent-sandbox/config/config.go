// Package config defines the JSON configuration types used by agent-sandbox.
package config

// Config represents agent-sandbox.json, with root values shared by run and resume.
type Config struct {
	Run         *Section `json:"run"`
	Resume      *Section `json:"resume"`
	Model       string   `json:"model"`
	Agent       string   `json:"agent"`
	BaseImage   string   `json:"baseImage"`
	Push        bool     `json:"push"`
	PR          bool     `json:"pr"`
	HostNetwork bool     `json:"hn"`
}

// Section contains command-specific values that override shared root defaults.
// Pointers preserve the difference between an omitted field and an explicit
// zero value, which matters when JSON defaults are merged with CLI flags.
type Section struct {
	Branch        *string   `json:"branch"`
	Agent         *string   `json:"agent"`
	APIKey        *string   `json:"apiKey"`
	Model         *string   `json:"model"`
	BaseImage     *string   `json:"baseImage"`
	Query         *string   `json:"query"`
	Push          *bool     `json:"push"`
	PR            *bool     `json:"pr"`
	HostNetwork   *bool     `json:"hn"`
	CommitMessage *string   `json:"commitMessage"`
	FilePrompt    *string   `json:"filePrompt"`
	Image         *[]string `json:"image"`
	Reviewers     []string  `json:"reviewers"`
}
