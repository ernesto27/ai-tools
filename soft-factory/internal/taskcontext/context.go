package taskcontext

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"soft-factory/internal/config"
	"soft-factory/internal/googledrive"
)

func Build(ctx context.Context, cfg config.Config, baseDir string) (string, error) {
	var result strings.Builder

	for _, path := range cfg.Documents {
		document, err := readFile(baseDir, path)
		if err != nil {
			return "", fmt.Errorf("read supporting document: %w", err)
		}

		fmt.Fprintf(&result, "# Document: %s\n\n", path)
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
				fmt.Fprintf(
					&result,
					"# Google Doc: %s\n\nFolder: %s\nSource: https://docs.google.com/document/d/%s/edit\n\n",
					document.Name,
					document.FolderName,
					document.ID,
				)
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

				fmt.Fprintf(
					&result,
					"# Google Doc: \nSource: https://docs.google.com/document/d/%s/edit\n\n",
					documentID,
				)

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
