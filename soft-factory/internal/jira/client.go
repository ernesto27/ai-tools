// Package jira reads Jira Cloud issues for use as factory tasks.
package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	sitePattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*\.atlassian\.net$`)
	issuePattern = regexp.MustCompile(`^/browse/([A-Za-z][A-Za-z0-9_]*-[1-9][0-9]*)/?$`)
)

type Client struct {
	email string
	token string
	http  *http.Client
}

type Issue struct {
	Key         string
	URL         string
	Summary     string
	Description string
}

// New authenticates with a direct-site API token. It copies the supplied HTTP
// client so enforcing a timeout and rejecting redirects does not modify it.
func New(email, token string, client *http.Client) (*Client, error) {
	if strings.TrimSpace(email) == "" || strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("Jira requires JIRA_EMAIL and JIRA_API_TOKEN")
	}
	var transport http.Client
	if client != nil {
		transport = *client
	}
	if transport.Timeout == 0 {
		transport.Timeout = 30 * time.Second
	}
	transport.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &Client{email: email, token: token, http: &transport}, nil
}

func parseIssueURL(raw string) (*url.URL, string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, "", fmt.Errorf("invalid Jira issue URL")
	}
	host := strings.ToLower(u.Host)
	if u.Scheme != "https" || u.User != nil || !sitePattern.MatchString(host) {
		return nil, "", fmt.Errorf("Jira issue URL must use HTTPS and a <site>.atlassian.net host without credentials or a custom port")
	}
	match := issuePattern.FindStringSubmatch(u.Path)
	if match == nil {
		return nil, "", fmt.Errorf("Jira issue URL must have the path /browse/PROJECT-123")
	}
	key := strings.ToUpper(match[1])
	return &url.URL{Scheme: "https", Host: host, Path: "/browse/" + key}, key, nil
}

// GetIssue converts a browser link into a REST request and reads its task text.
func (c *Client) GetIssue(ctx context.Context, issueURL string) (Issue, error) {
	source, key, err := parseIssueURL(issueURL)
	if err != nil {
		return Issue{}, err
	}
	endpoint := *source
	endpoint.Path = "/rest/api/3/issue/" + key
	query := url.Values{"fields": {"summary,description"}}
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return Issue{}, fmt.Errorf("create Jira request: %w", err)
	}
	req.SetBasicAuth(c.email, c.token)
	req.Header.Set("Accept", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return Issue{}, fmt.Errorf("fetch Jira issue %s: %w", key, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		// Do not echo response bodies: they can contain sensitive data or HTML.
		return Issue{}, fmt.Errorf("fetch Jira issue %s: HTTP %d (check the issue URL, permissions, and direct-site API token)", key, response.StatusCode)
	}
	const maxResponseSize = 2 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize+1))
	if err != nil {
		return Issue{}, fmt.Errorf("read Jira issue %s: %w", key, err)
	}
	if len(body) > maxResponseSize {
		return Issue{}, fmt.Errorf("Jira issue %s response exceeds 2 MiB", key)
	}
	var payload struct {
		Key    string `json:"key"`
		Fields *struct {
			Summary     *string         `json:"summary"`
			Description json.RawMessage `json:"description"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return Issue{}, fmt.Errorf("decode Jira issue %s: invalid JSON response", key)
	}
	if payload.Key == "" || payload.Fields == nil || payload.Fields.Summary == nil || strings.TrimSpace(*payload.Fields.Summary) == "" {
		return Issue{}, fmt.Errorf("decode Jira issue %s: missing issue key or summary", key)
	}
	if !issuePattern.MatchString("/browse/" + payload.Key) {
		return Issue{}, fmt.Errorf("decode Jira issue %s: invalid issue key", key)
	}
	description, err := descriptionText(payload.Fields.Description)
	if err != nil {
		return Issue{}, fmt.Errorf("decode Jira issue %s description: %w", key, err)
	}
	// Jira can return a different key when an issue has moved.
	source.Path = "/browse/" + payload.Key
	return Issue{Key: payload.Key, URL: source.String(), Summary: *payload.Fields.Summary, Description: description}, nil
}

// Task formats an issue as a task, without credentials or API response metadata.
func (i Issue) Task() string {
	description := i.Description
	if description == "" {
		description = "No description provided."
	}
	return fmt.Sprintf("## Jira issue: %s\n\nSource: %s\n\n## Summary\n\n%s\n\n## Description\n\n%s", i.Key, i.URL, i.Summary, description)
}
