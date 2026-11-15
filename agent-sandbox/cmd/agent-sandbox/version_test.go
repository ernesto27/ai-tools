package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    [3]uint64
		wantErr bool
	}{
		{name: "plain", input: "1.2.3", want: [3]uint64{1, 2, 3}},
		{name: "tag", input: "agent-sandbox-v1.2.3", want: [3]uint64{1, 2, 3}},
		{name: "zero", input: "0.0.0", want: [3]uint64{}},
		{name: "empty", input: "", wantErr: true},
		{name: "development", input: "dev", wantErr: true},
		{name: "missing patch", input: "1.2", wantErr: true},
		{name: "bare v prefix", input: "v1.2.3", wantErr: true},
		{name: "other project", input: "other-v1.2.3", wantErr: true},
		{name: "prerelease", input: "1.2.3-rc.1", wantErr: true},
		{name: "metadata", input: "1.2.3+build", wantErr: true},
		{name: "leading zero", input: "01.2.3", wantErr: true},
		{name: "whitespace", input: " 1.2.3 ", wantErr: true},
		{name: "overflow", input: "18446744073709551616.0.0", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVersion(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name string
		a, b [3]uint64
		want bool
	}{
		{"equal", [3]uint64{1, 2, 3}, [3]uint64{1, 2, 3}, false},
		{"newer patch", [3]uint64{1, 2, 4}, [3]uint64{1, 2, 3}, true},
		{"older patch", [3]uint64{1, 2, 2}, [3]uint64{1, 2, 3}, false},
		{"newer minor", [3]uint64{1, 3, 0}, [3]uint64{1, 2, 9}, true},
		{"older minor", [3]uint64{1, 1, 9}, [3]uint64{1, 2, 0}, false},
		{"newer major", [3]uint64{2, 0, 0}, [3]uint64{1, 9, 9}, true},
		{"older major", [3]uint64{1, 9, 9}, [3]uint64{2, 0, 0}, false},
		{"numeric comparison", [3]uint64{0, 0, 10}, [3]uint64{0, 0, 9}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isNewer(tt.a, tt.b))
		})
	}
}

type releaseTransport struct {
	status  int
	body    string
	err     error
	request *http.Request
	cancel  context.CancelFunc
}

func (rt *releaseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.request = req
	if rt.cancel != nil {
		rt.cancel()
		if err := req.Context().Err(); err != nil {
			return nil, err
		}
	}
	if rt.err != nil {
		return nil, rt.err
	}
	return &http.Response{
		StatusCode: rt.status,
		Status:     fmt.Sprintf("%d %s", rt.status, http.StatusText(rt.status)),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(rt.body)),
		Request:    req,
	}, nil
}

type versionFailingWriter struct {
	err error
}

func (w versionFailingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestVersionCheckCommand(t *testing.T) {
	writeErr := errors.New("output unavailable")
	tests := []struct {
		name    string
		cancel  bool
		writer  io.Writer
		wantErr error
	}{
		{name: "JSON output"},
		{name: "command cancellation", cancel: true, wantErr: context.Canceled},
		{name: "output error", writer: versionFailingWriter{err: writeErr}, wantErr: writeErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The command constructs its own client, so replace the default transport
			// to keep these tests offline. These tests must not run in parallel.
			originalTransport, originalVersion := http.DefaultTransport, version
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
				version = originalVersion
			})
			version = "agent-sandbox-v0.0.55"
			transport := &releaseTransport{
				status: http.StatusOK,
				body:   `[{"tag_name":"agent-sandbox-v0.0.56"}]`,
			}
			http.DefaultTransport = transport

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.cancel {
				transport.cancel = cancel
			}

			var output bytes.Buffer
			root, _ := newRootCmd()
			root.SetArgs([]string{"version-check"})
			root.SetOut(&output)
			if tt.writer != nil {
				root.SetOut(tt.writer)
			}

			err := root.ExecuteContext(ctx)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, output.String())
			} else {
				require.NoError(t, err)
				assert.JSONEq(t, `{"latest":"agent-sandbox-v0.0.56","newer":true}`, output.String())
			}
			require.NotNil(t, transport.request)
		})
	}
}

func TestCheckAgentSandboxReleaseFiltersTags(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		current string
		want    string
	}{
		{name: "valid project tag", tag: "agent-sandbox-v1.2.4", current: "1.2.3", want: "agent-sandbox-v1.2.4"},
		{name: "plain version", tag: "9.0.0", current: "1.2.3", want: "agent-sandbox-v1.2.3"},
		{name: "other project", tag: "other-v9.0.0", current: "1.2.3", want: "agent-sandbox-v1.2.3"},
		{name: "leading zero", tag: "agent-sandbox-v09.0.0", current: "1.2.3", want: "agent-sandbox-v1.2.3"},
		{name: "prerelease suffix", tag: "agent-sandbox-v9.0.0-rc.1", current: "1.2.3", want: "agent-sandbox-v1.2.3"},
		{name: "prefixed comparison input", tag: "agent-sandbox-v1.2.4", current: "agent-sandbox-v1.2.3", want: "agent-sandbox-v1.2.4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the default transport to exercise release selection offline.
			// These tests must not run in parallel because the transport is global.
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			http.DefaultTransport = &releaseTransport{
				status: http.StatusOK,
				body:   fmt.Sprintf(`[{"tag_name":"agent-sandbox-v1.2.3"},{"tag_name":%q}]`, tt.tag),
			}

			latest, newer, err := CheckAgentSandboxRelease(context.Background(), tt.current)
			require.NoError(t, err)
			assert.Equal(t, tt.want, latest)
			assert.Equal(t, tt.want == "agent-sandbox-v1.2.4", newer)
		})
	}
}

func TestFetchReleases(t *testing.T) {
	networkErr := errors.New("network unavailable")
	tests := []struct {
		name    string
		status  int
		body    string
		err     error
		want    []release
		wantErr bool
	}{
		{
			name: "success", status: http.StatusOK,
			body: `[{"tag_name":"agent-sandbox-v1.2.3","draft":false,"prerelease":false},
				{"tag_name":"agent-sandbox-v2.0.0","draft":true,"prerelease":true}]`,
			want: []release{
				{Tag: "agent-sandbox-v1.2.3"},
				{Tag: "agent-sandbox-v2.0.0", Draft: true, Prerelease: true},
			},
		},
		{name: "empty", status: http.StatusOK, body: `[]`, want: []release{}},
		{name: "invalid JSON", status: http.StatusOK, body: `{`, wantErr: true},
		{name: "rate limited", status: http.StatusForbidden, body: `{"message":"API rate limit exceeded"}`, wantErr: true},
		{name: "server error", status: http.StatusInternalServerError, wantErr: true},
		{name: "network error", err: networkErr, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := &releaseTransport{status: tt.status, body: tt.body, err: tt.err}
			client := &http.Client{Transport: transport}
			got, err := fetchReleases(context.Background(), client, 2)
			if tt.wantErr {
				require.Error(t, err)
				if tt.err != nil {
					assert.ErrorIs(t, err, tt.err)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}

			req := transport.request
			require.NotNil(t, req)
			assert.Equal(t, "https://api.github.com/repos/ernesto27/ai-tools/releases?per_page=100&page=2", req.URL.String())
			assert.Equal(t, http.MethodGet, req.Method)
			assert.Equal(t, "application/vnd.github+json", req.Header.Get("Accept"))
			assert.Equal(t, "agent-sandbox", req.Header.Get("User-Agent"))
		})
	}
}
