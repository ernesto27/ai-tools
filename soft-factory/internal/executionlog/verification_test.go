package executionlog

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"soft-factory/internal/config"
)

func TestArchiveVerificationCopiesStageFiles(t *testing.T) {
	prepareLogTestDirectory(t)
	run := &Run{path: t.TempDir()}
	files := map[string][]byte{
		"summary.md":              []byte("Build passed.\n"),
		"verification.log":        []byte("npm run build\nexit: 0\n"),
		".gitignore":              []byte("*\n"),
		"screenshots/desktop.png": {0x89, 'P', 'N', 'G', 0, 0xff},
	}
	for name, content := range files {
		path := filepath.Join(verificationDirectory, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
		require.NoError(t, os.WriteFile(path, content, 0600))
	}
	for _, stage := range []config.Stage{
		config.StageImplementation, config.StageCodeReview, config.StageSecurityReview,
	} {
		require.NoError(t, run.ArchiveVerification(context.Background(), "main", stage))
		for name, want := range files {
			archived, err := os.ReadFile(filepath.Join(run.path, "verification-"+string(stage), name))
			require.NoError(t, err)
			require.Equal(t, want, archived)
			source, err := os.ReadFile(filepath.Join(verificationDirectory, name))
			require.NoError(t, err)
			require.Equal(t, want, source)
		}
	}
}

func TestArchiveVerificationReplacesPreviousAttempt(t *testing.T) {
	prepareLogTestDirectory(t)
	run := &Run{path: t.TempDir()}
	stage := config.StageImplementation
	archive := filepath.Join(run.path, "verification-implementation")
	source := filepath.Join(verificationDirectory, "summary.md")
	require.NoError(t, os.Mkdir(verificationDirectory, 0700))
	require.NoError(t, os.WriteFile(source, []byte("First attempt"), 0600))
	require.NoError(t, run.ArchiveVerification(context.Background(), "main", stage))
	require.NoError(t, os.WriteFile(filepath.Join(archive, "old.png"), []byte("old"), 0600))

	require.NoError(t, os.WriteFile(source, []byte("Latest attempt"), 0600))
	require.NoError(t, run.ArchiveVerification(context.Background(), "main", stage))
	content, err := os.ReadFile(filepath.Join(archive, "summary.md"))
	require.NoError(t, err)
	require.Equal(t, "Latest attempt", string(content))
	_, err = os.Stat(filepath.Join(archive, "old.png"))
	require.ErrorIs(t, err, os.ErrNotExist)

	// Missing evidence clears a stale archive and remains harmless on later calls.
	require.NoError(t, os.Remove(source))
	require.NoError(t, os.Remove(verificationDirectory))
	require.NoError(t, run.ArchiveVerification(context.Background(), "main", stage))
	_, err = os.Stat(archive)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.NoError(t, run.ArchiveVerification(context.Background(), "main", stage))
}

func TestArchiveVerificationSkipsExcludedStagesAndNilLog(t *testing.T) {
	run := &Run{path: t.TempDir()}
	for _, stage := range []config.Stage{config.StageRiskClassification, config.StageReviewChanges} {
		// An invalid branch makes an unexpected worktree lookup fail.
		require.NoError(t, run.ArchiveVerification(context.Background(), "", stage))
	}
	entries, err := os.ReadDir(run.path)
	require.NoError(t, err)
	require.Empty(t, entries)
	var disabled *Run
	require.NoError(t, disabled.ArchiveVerification(context.Background(), "", config.StageImplementation))
}

func TestArchiveVerificationRejectsSymlinkEvidence(t *testing.T) {
	prepareLogTestDirectory(t)
	run := &Run{path: t.TempDir()}
	target := filepath.Join(t.TempDir(), "private.txt")
	require.NoError(t, os.WriteFile(target, []byte("outside evidence"), 0600))
	require.NoError(t, os.Mkdir(verificationDirectory, 0700))
	require.NoError(t, os.Symlink(target, filepath.Join(verificationDirectory, "linked.txt")))

	err := run.ArchiveVerification(context.TODO(), "main", config.StageImplementation)
	require.ErrorContains(t, err, "unsupported verification entry")
	_, err = os.Lstat(filepath.Join(run.path, "verification-implementation", "linked.txt"))
	require.ErrorIs(t, err, os.ErrNotExist)
}
