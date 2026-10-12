package taskcontext

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"soft-factory/internal/config"
	"soft-factory/internal/googledrive"
	"soft-factory/internal/prompts"
)

func Build(ctx context.Context, cfg config.Config, baseDir string) (string, error) {
	var result strings.Builder

	for _, path := range cfg.Documents {
		document, err := readFile(baseDir, path)
		if err != nil {
			return "", fmt.Errorf("read supporting document: %w", err)
		}

		header, err := prompts.Render("document.txt", map[string]any{"Path": path})
		if err != nil {
			return "", err
		}
		result.WriteString(header)
		result.WriteString(document)
		result.WriteString("\n\n")
	}
	if cfg.GoogleDrive != nil {
		client, err := googledrive.New(ctx, "service_account.json")
		if err != nil {
			return "", fmt.Errorf("initialize Google Drive context: %w", err)
		}

		if len(cfg.GoogleDrive.Folders) > 0 {
			documents, err := client.ReadDocuments(ctx, cfg.GoogleDrive.Folders)
			if err != nil {
				return "", fmt.Errorf("load Google Drive context: %w", err)
			}

			for _, document := range documents {
				header, err := prompts.Render("google-document.txt", map[string]any{
					"Name": document.Name, "FolderName": document.FolderName, "ID": document.ID,
				})
				if err != nil {
					return "", err
				}
				result.WriteString(header)
				result.WriteString(document.Text)
				result.WriteString("\n\n")
			}
		}

		if len(cfg.GoogleDrive.Files) > 0 {
			for _, f := range cfg.GoogleDrive.Files {
				documentID, err := googledrive.DocumentIDFromURL(f)
				if err != nil {
					return "", err
				}

				header, err := prompts.Render("google-document-url.txt", map[string]any{"ID": documentID})
				if err != nil {
					return "", err
				}
				result.WriteString(header)

				documentContent, err := client.ExportText(ctx, documentID)
				if err != nil {
					return "", err
				}

				result.WriteString(documentContent)
				result.WriteString("\n\n")
			}
		}
	}

	return result.String(), nil
}

func readFile(baseDir, path string) (string, error) {
	resolved := path
	if !filepath.IsAbs(path) {
		resolved = filepath.Join(baseDir, path)
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", fmt.Errorf("%q: %w", resolved, err)
	}

	return string(data), nil
}
