// Package googledrive connects to Google Drive using service-account credentials.
package googledrive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

type Client struct {
	service *drive.Service
}

// New loads a service-account JSON key and authenticates automatically.
// It requests read-only access to file metadata and contents.
func New(ctx context.Context, credentialsPath string) (*Client, error) {
	data, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("read Google Drive credentials: %w", err)
	}
	var credentials struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &credentials); err != nil {
		return nil, fmt.Errorf("parse Google Drive credentials: %w", err)
	}
	if credentials.Type != "service_account" {
		return nil, fmt.Errorf("Google Drive requires a service-account JSON key with type service_account; OAuth client credentials are not supported")
	}
	config, err := google.JWTConfigFromJSON(data, drive.DriveReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("configure Google Drive service account: %w", err)
	}
	if config.Email == "" || len(config.PrivateKey) == 0 {
		return nil, fmt.Errorf("Google Drive service-account credentials require client_email and private_key")
	}
	tokenContext := context.WithValue(
		ctx,
		oauth2.HTTPClient,
		&http.Client{Timeout: 30 * time.Second},
	)
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &oauth2.Transport{
			Source: config.TokenSource(tokenContext),
		},
	}
	service, err := drive.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("create Google Drive service: %w", err)
	}
	return &Client{service: service}, nil
}

// PrintFileNames prints one name per line for every accessible, non-trashed
// file or folder. It follows pagination and requests names only.
func (c *Client) PrintFileNames(ctx context.Context, output io.Writer) error {
	pageToken := ""
	for {
		page, err := c.service.Files.List().Q("trashed = false").
			Spaces("drive").PageSize(1000).OrderBy("name").
			Fields("nextPageToken,files(name)").PageToken(pageToken).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("list Google Drive files: %w", err)
		}
		for _, file := range page.Files {
			if _, err := fmt.Fprintln(output, file.Name); err != nil {
				return fmt.Errorf("print Google Drive file name: %w", err)
			}
		}
		pageToken = page.NextPageToken
		if pageToken == "" {
			return nil
		}
	}
}
