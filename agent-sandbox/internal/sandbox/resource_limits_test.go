package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ernesto27/ai-tools/agent-sandbox/internal/clierror"
	"github.com/ernesto27/ai-tools/agent-sandbox/internal/docker"
	"github.com/ernesto27/ai-tools/agent-sandbox/internal/git"
)

const (
	testNanoCPUs = 1500000000
	testMemory   = 64 << 20
	testTimeout  = 150 * time.Millisecond
)

func TestResourceLimitsAcrossRunAndResume(t *testing.T) {
	if testing.Short() {
		t.Skip("exercises Git worktrees and the Docker HTTP client")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	repo := setupDeleteTestRepo(t)
	runGit(t, repo, "config", "user.name", "Test User")
	runGit(t, repo, "config", "user.email", "test@example.com")
	base, err := (&git.Repo{Dir: repo}).CurrentBranch(ctx)
	require.NoError(t, err)
	remote := t.TempDir()
	runGit(t, remote, "init", "--bare", "--quiet")
	runGit(t, repo, "push", remote, base)
	runGit(t, repo, "remote", "add", "origin", "ssh://git@github.com/owner/repo.git")

	tools := t.TempDir()
	publishLog := filepath.Join(tools, "publish")
	ssh := filepath.Join(tools, "ssh")
	require.NoError(t, os.WriteFile(ssh, []byte(`#!/bin/sh
for arg do
  case "$arg" in
    git-upload-pack*) exec git-upload-pack "$SANDBOX_TEST_REMOTE" ;;
    git-receive-pack*) echo "git push" >> "$SANDBOX_TEST_PUBLISH"; exit 1 ;;
  esac
done
exit 1
`), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(tools, "gh"), []byte(`#!/bin/sh
case "$1 $2" in
  "auth status"|"repo view") exit 0 ;;
  *) echo "$1 $2" >> "$SANDBOX_TEST_PUBLISH"; exit 1 ;;
esac
`), 0o700))
	t.Setenv("GIT_SSH_COMMAND", ssh)
	t.Setenv("GIT_SSH_VARIANT", "ssh")
	t.Setenv("SANDBOX_TEST_REMOTE", remote)
	t.Setenv("SANDBOX_TEST_PUBLISH", publishLog)
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))

	home := t.TempDir()
	t.Setenv("HOME", home)
	require.NoError(t, os.Mkdir(filepath.Join(home, ".codex"), 0o700))
	first := newLimitTestRun(true, true)
	second := newLimitTestRun(true, false)
	third := newLimitTestRun(false, false)
	startLimitTestDocker(t, first, second, third)

	var phases []string
	runtime := Runtime{Output: io.Discard, ViewOwnsTerminal: true, Observe: func(e Event) {
		phases = append(phases, e.Phase)
	}}
	options := Options{
		AgentName: "codex", APIKey: "test-key", Branch: "feature", Prompt: "test", PR: true,
		NanoCPUs: testNanoCPUs, MemoryBytes: testMemory, Timeout: testTimeout,
	}
	opts, err := NewOptions(options)
	require.NoError(t, err)
	_, runErr := Run(ctx, opts, runtime)
	requireTimeout(t, runErr)
	require.Contains(t, phases, "Timed out")
	require.NotContains(t, phases, "Publishing")
	requireClosed(t, first.removed)
	requireClosed(t, first.waitReturned)

	records, err := recordedWorktrees(repo)
	require.NoError(t, err)
	require.Len(t, records, 1)
	record := records[0]
	require.Equal(t, options.Branch, record.Branch)
	marker := filepath.Join(record.Path, "marker.txt")
	require.FileExists(t, marker)
	var diagnostic *clierror.Error
	require.ErrorAs(t, runErr, &diagnostic)
	require.Contains(t, diagnostic.Summary, testTimeout.String())
	require.Contains(t, diagnostic.Details, clierror.Detail{Label: "Worktree", Value: record.Path})

	options.APIKey, options.PR, options.Push = "", false, true
	opts, err = NewOptions(options)
	require.NoError(t, err)
	_, err = Resume(ctx, opts, runtime)
	requireTimeout(t, err)
	require.NotContains(t, phases, "Publishing")
	require.NoFileExists(t, publishLog)
	requireClosed(t, second.removed)
	requireClosed(t, second.waitReturned)

	opts.Push = false
	status, err := Resume(ctx, opts, runtime)
	require.NoError(t, err)
	require.Zero(t, status)
	require.FileExists(t, marker)
	requireClosed(t, third.removed)
	after, err := recordedWorktrees(repo)
	require.NoError(t, err)
	require.Equal(t, records, after)
}

func TestDockerResourceLimitFailuresAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name                                      string
		discarded, stopFails, cancelled, removing bool
		wantError                                 string
	}{
		{name: "daemon discards memory before start", discarded: true, wantError: "Docker did not apply the requested memory limit"},
		{name: "stop failure retains timeout and forces removal", stopFails: true, wantError: "simulated stop failure"},
		{name: "manual cancellation keeps shutdown grace with a timeout configured", cancelled: true, wantError: "context canceled"},
		{name: "waits when automatic removal is already in progress", removing: true, wantError: "timed out"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := newLimitTestRun(true, false)
			run.discarded, run.stopFails, run.cancelled, run.removing = tc.discarded, tc.stopFails, tc.cancelled, tc.removing
			startLimitTestDocker(t, run)
			client, err := docker.New("test", "", docker.WithStreams(io.Discard, nil))
			require.NoError(t, err)
			defer client.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			opts := docker.RunOptions{
				Entrypoint: "codex", NanoCPUs: testNanoCPUs, MemoryBytes: testMemory, Timeout: testTimeout,
			}
			if tc.cancelled {
				opts.Timeout = time.Minute
				opts.FormatStdout = func(out io.Writer) io.WriteCloser {
					// The output pump starts only after Docker acknowledges start.
					cancel()
					return passthroughTestCloser{out}
				}
			}
			_, err = client.Run(ctx, opts)
			require.ErrorContains(t, err, tc.wantError)
			require.Equal(t, tc.stopFails || tc.removing, IsTimeout(err))
			if tc.cancelled {
				require.ErrorIs(t, err, context.Canceled)
				requireClosed(t, run.waitReturned)
			}
			requireClosed(t, run.removed)
			if tc.discarded {
				select {
				case <-run.waitRegistered:
					t.Fatal("Docker was asked to wait for a container with missing limits")
				default:
				}
			}
		})
	}
}

type passthroughTestCloser struct{ io.Writer }

func (passthroughTestCloser) Close() error { return nil }

func requireTimeout(t *testing.T, err error) {
	t.Helper()
	require.ErrorIs(t, err, docker.ErrRunTimeout)
	var status StatusError
	require.ErrorAs(t, err, &status)
	require.Equal(t, ExitTimeout, status.Status)
}

func requireClosed(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	default:
		t.Fatal("expected Docker request before returning")
	}
}

type limitTestRun struct {
	block, apiKey, discarded, stopFails, cancelled, removing bool
	worktree                                                 chan string
	waitRegistered, waitReturned                             chan struct{}
	exited, removed                                          chan struct{}
	exitOnce                                                 sync.Once
}

func newLimitTestRun(block, apiKey bool) *limitTestRun {
	return &limitTestRun{
		block: block, apiKey: apiKey, worktree: make(chan string, 1),
		waitRegistered: make(chan struct{}), waitReturned: make(chan struct{}),
		exited: make(chan struct{}), removed: make(chan struct{}),
	}
}

// The fake engine responds through the same Docker client used by real runs,
// with an attached connection and a wait request acknowledged before start.
func startLimitTestDocker(t *testing.T, runs ...*limitTestRun) {
	t.Helper()
	mux := http.NewServeMux()
	var created atomic.Int32
	mux.HandleFunc("GET /v1.43/images/{image}/json", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"Config":{"Env":["PATH=/bin"]}}`)
	})
	mux.HandleFunc("POST /v1.43/containers/create", func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			HostConfig container.HostConfig
		}
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&request)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		id := "probe"
		if request.HostConfig.AutoRemove {
			index := int(created.Add(1) - 1)
			if !assert.Less(t, index, len(runs)) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			run := runs[index]
			id = fmt.Sprintf("agent-%d", index)
			assert.EqualValues(t, testNanoCPUs, request.HostConfig.NanoCPUs)
			assert.EqualValues(t, testMemory, request.HostConfig.Memory)
			_, temporaryHome := request.HostConfig.Tmpfs["/codex-home"]
			assert.Equal(t, run.apiKey, temporaryHome)
			path := ""
			for _, mount := range request.HostConfig.Mounts {
				if mount.Target == workspace {
					path = mount.Source
					break
				}
			}
			run.worktree <- path
		} else {
			assert.Zero(t, request.HostConfig.NanoCPUs)
			assert.Zero(t, request.HostConfig.Memory)
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"Id":%q}`, id)
	})
	mux.HandleFunc("POST /v1.43/containers/probe/start", limitTestNoContent)
	mux.HandleFunc("DELETE /v1.43/containers/probe", limitTestNoContent)
	mux.HandleFunc("POST /v1.43/containers/probe/wait", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"StatusCode":0}`)
	})
	mux.HandleFunc("GET /v1.43/containers/probe/logs", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = stdcopy.NewStdWriter(w, stdcopy.Stdout).Write([]byte("1.2.3\n"))
	})
	for i, run := range runs {
		path := fmt.Sprintf("/v1.43/containers/agent-%d", i)
		mux.HandleFunc("GET "+path+"/json", func(w http.ResponseWriter, _ *http.Request) {
			// The successful resume must survive preparation longer than its limit.
			if !run.block {
				time.Sleep(2 * testTimeout)
			}
			memory := testMemory
			if run.discarded {
				memory = 0
			}
			fmt.Fprintf(w, `{"HostConfig":{"NanoCpus":%d,"Memory":%d}}`, testNanoCPUs, memory)
		})
		mux.HandleFunc("POST "+path+"/attach", func(w http.ResponseWriter, _ *http.Request) {
			conn, response, err := w.(http.Hijacker).Hijack()
			if !assert.NoError(t, err) {
				return
			}
			defer conn.Close()
			fmt.Fprint(response, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n")
			if assert.NoError(t, response.Flush()) {
				clientClosed := make(chan struct{})
				go func() {
					defer close(clientClosed)
					_, _ = io.Copy(io.Discard, conn)
				}()
				select {
				case <-run.exited:
					conn.Close()
					<-clientClosed
				case <-clientClosed:
				}
			}
		})
		mux.HandleFunc("POST "+path+"/wait", func(w http.ResponseWriter, r *http.Request) {
			condition := r.URL.Query().Get("condition")
			if condition == "removed" {
				assert.True(t, run.removing)
				close(run.removed)
				fmt.Fprint(w, `{"StatusCode":0}`)
				return
			}
			assert.Equal(t, "next-exit", condition)
			close(run.waitRegistered)
			w.(http.Flusher).Flush()
			select {
			case <-run.exited:
				close(run.waitReturned)
				fmt.Fprint(w, `{"StatusCode":0}`)
			case <-r.Context().Done():
			}
		})
		mux.HandleFunc("POST "+path+"/start", func(w http.ResponseWriter, _ *http.Request) {
			select {
			case <-run.waitRegistered:
			default:
				t.Error("Docker wait was not registered before start")
			}
			if dir := <-run.worktree; dir != "" {
				marker := filepath.Join(dir, "marker.txt")
				if i == 0 {
					assert.NoError(t, os.WriteFile(marker, []byte("agent edit"), 0o600))
				} else {
					data, err := os.ReadFile(marker)
					assert.NoError(t, err)
					assert.Equal(t, "agent edit", string(data))
				}
			}
			w.WriteHeader(http.StatusNoContent)
			if !run.block {
				run.exitOnce.Do(func() { close(run.exited) })
			}
		})
		mux.HandleFunc("POST "+path+"/stop", func(w http.ResponseWriter, r *http.Request) {
			grace := "0"
			if run.cancelled {
				grace = "10"
			}
			assert.Equal(t, grace, r.URL.Query().Get("t"))
			if run.stopFails {
				http.Error(w, `{"message":"simulated stop failure"}`, http.StatusInternalServerError)
				return
			}
			run.exitOnce.Do(func() { close(run.exited) })
			w.WriteHeader(http.StatusNoContent)
		})
		mux.HandleFunc("DELETE "+path, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "1", r.URL.Query().Get("force"))
			run.exitOnce.Do(func() { close(run.exited) })
			if run.removing {
				http.Error(w, `{"message":"removal is already in progress"}`, http.StatusConflict)
				return
			}
			close(run.removed)
			w.WriteHeader(http.StatusNoContent)
		})
	}
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	t.Setenv("DOCKER_HOST", strings.Replace(server.URL, "http://", "tcp://", 1))
	t.Setenv("DOCKER_API_VERSION", "1.43")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")
}

func limitTestNoContent(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
