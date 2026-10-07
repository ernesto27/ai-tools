package googledrive

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"

	"google.golang.org/api/drive/v3"
)

type Document struct {
	ID         string
	Name       string
	FolderName string
	Text       string
}

// DocumentIDFromURL extracts a file ID from a Google Docs document URL.
// Query parameters and fragments do not affect the extracted ID.
func DocumentIDFromURL(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("parse Google Docs URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Host != "docs.google.com" || parsed.User != nil {
		return "", fmt.Errorf("Google Docs URL must use https://docs.google.com/document/d/FILE_ID")
	}
	parts := strings.Split(parsed.Path, "/")
	if len(parts) < 4 || parts[1] != "document" || parts[2] != "d" || parts[3] == "" {
		return "", fmt.Errorf("Google Docs URL must contain /document/d/FILE_ID")
	}
	id := parts[3]
	for _, char := range id {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' || char == '-' || char == '_') {
			return "", fmt.Errorf("Google Docs URL contains an invalid file ID")
		}
	}
	return id, nil
}

// ReadDocuments exports direct Google Docs from folders matched by exact name.
// Subfolders, shortcuts, and other file types are excluded.
func (c *Client) ReadDocuments(ctx context.Context, folderNames []string) ([]Document, error) {
	var documents []Document
	seenFolders := make(map[string]bool)
	seenDocuments := make(map[string]bool)

	for _, name := range folderNames {
		query := "trashed = false" +
			" and mimeType = 'application/vnd.google-apps.folder'" +
			" and name = " + queryString(name)

		folders, err := c.listFiles(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("find Drive folder %q: %w", name, err)
		}
		if len(folders) == 0 {
			return nil, fmt.Errorf("Drive folder %q was not found or is inaccessible", name)
		}
		if len(folders) > 1 {
			return nil, fmt.Errorf(
				"multiple Drive folders match %q; give the intended folder a unique name",
				name,
			)
		}

		folder := folders[0]
		if seenFolders[folder.Id] {
			continue
		}
		seenFolders[folder.Id] = true

		query = "trashed = false" +
			" and mimeType = 'application/vnd.google-apps.document'" +
			" and " + queryString(folder.Id) + " in parents"

		files, err := c.listFiles(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("list documents in Drive folder %q: %w", name, err)
		}

		sort.Slice(files, func(i, j int) bool {
			if files[i].Name == files[j].Name {
				return files[i].Id < files[j].Id
			}
			return files[i].Name < files[j].Name
		})

		for _, file := range files {
			if seenDocuments[file.Id] {
				continue
			}

			text, err := c.ExportText(ctx, file.Id)
			if err != nil {
				return nil, fmt.Errorf(
					"read Google Doc %q (%s) in folder %q: %w",
					file.Name, file.Id, name, err,
				)
			}

			seenDocuments[file.Id] = true
			documents = append(documents, Document{
				ID:         file.Id,
				Name:       file.Name,
				FolderName: folder.Name,
				Text:       text,
			})
		}
	}

	return documents, nil
}

func (c *Client) listFiles(ctx context.Context, query string) ([]*drive.File, error) {
	var files []*drive.File
	pageToken := ""

	for {
		page, err := c.service.Files.List().
			Q(query).
			Spaces("drive").
			PageSize(1000).
			Fields("nextPageToken,files(id,name)").
			PageToken(pageToken).
			Context(ctx).
			Do()
		if err != nil {
			return nil, err
		}

		files = append(files, page.Files...)
		pageToken = page.NextPageToken
		if pageToken == "" {
			return files, nil
		}
	}
}

// ExportText exports a Google Doc as plain text using its file ID.
func (c *Client) ExportText(ctx context.Context, id string) (string, error) {
	response, err := c.service.Files.Export(id, "text/plain").
		Context(ctx).
		Download()
	if err != nil {
		return "", fmt.Errorf("export document: %w", err)
	}
	defer response.Body.Close()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("read exported text: %w", err)
	}
	return string(data), nil
}

func queryString(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "'", "\\'")
	return "'" + value + "'"
}
