package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newVersionCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "check-version",
		Aliases: []string{"version-check"},
		Short:   "Report whether a newer software-factory release exists",
		Long: "Compare the running version with the latest stable software-factory release.\n" +
			"Print JSON containing latest (release tag) and newer (boolean).\n" +
			"Makes unauthenticated GitHub API calls and requires network access.\n" +
			"Checks availability without installing an update. Development builds\n" +
			"report an error when their version is dev.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			latest, newer, err := checkSoftwareFactoryRelease(ctx, version)
			if err != nil {
				return err
			}

			result := struct {
				Latest string `json:"latest"`
				Newer  bool   `json:"newer"`
			}{
				Latest: latest,
				Newer:  newer,
			}

			data, err := json.Marshal(result)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return err
		},
	}

	return cmd
}

const tagPrefix = "software-factory-v"

var versionPattern = regexp.MustCompile(
	`^(?:` + regexp.QuoteMeta(tagPrefix) +
		`)?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`,
)

func parseVersion(value string) ([3]uint64, error) {
	var version [3]uint64
	matches := versionPattern.FindStringSubmatch(value)
	if matches == nil {
		return version, fmt.Errorf("invalid version %q", value)
	}

	for i := range version {
		n, err := strconv.ParseUint(matches[i+1], 10, 64)
		if err != nil {
			return version, fmt.Errorf("invalid version %q: %w", value, err)
		}
		version[i] = n
	}

	return version, nil
}

func isNewer(a, b [3]uint64) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

type release struct {
	Tag        string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

func fetchReleases(
	ctx context.Context,
	client *http.Client,
	page int,
) ([]release, error) {
	url := fmt.Sprintf(
		"https://api.github.com/repos/ernesto27/ai-tools/releases?per_page=100&page=%d",
		page,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "software-factory")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("checking releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checking releases: %s", resp.Status)
	}

	var releases []release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decoding releases: %w", err)
	}
	return releases, nil
}

func checkSoftwareFactoryRelease(
	ctx context.Context,
	current string,
) (string, bool, error) {
	currentVersion, err := parseVersion(current)
	if err != nil {
		return "", false, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var latest string
	var latestVersion [3]uint64

	for page := 1; ; page++ {
		releases, err := fetchReleases(ctx, client, page)
		if err != nil {
			return "", false, err
		}

		for _, release := range releases {
			if release.Draft || release.Prerelease ||
				!strings.HasPrefix(release.Tag, tagPrefix) {
				continue
			}

			version, err := parseVersion(release.Tag)
			if err != nil {
				continue
			}

			if latest == "" || isNewer(version, latestVersion) {
				latest = release.Tag
				latestVersion = version
			}
		}

		if len(releases) < 100 {
			break
		}
	}

	if latest == "" {
		return "", false, fmt.Errorf("no stable software-factory release found")
	}

	return latest, isNewer(latestVersion, currentVersion), nil
}
