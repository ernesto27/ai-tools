package executionlog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"soft-factory/internal/config"
)

const verificationDirectory = ".agent-sandbox-verification"

// ArchiveVerification copies stage evidence directly from the worktree into the
// run logs, replacing the previous archive.
func (r *Run) ArchiveVerification(ctx context.Context, branch string, stage config.Stage) error {
	if r == nil {
		return nil
	}
	switch stage {
	case config.StageImplementation, config.StageCodeReview, config.StageSecurityReview:
	default:
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	sourcePath, err := StageReportPath(ctx, branch, verificationDirectory)
	if err != nil {
		return fmt.Errorf("locate verification evidence: %w", err)
	}
	parent, err := os.OpenRoot(filepath.Dir(sourcePath))
	if err != nil {
		return fmt.Errorf("open verification parent: %w", err)
	}
	defer parent.Close()
	info, err := parent.Lstat(verificationDirectory)
	missing := errors.Is(err, os.ErrNotExist)
	if err != nil && !missing {
		return fmt.Errorf("inspect verification evidence: %w", err)
	}
	if !missing && !info.IsDir() {
		return fmt.Errorf("verification evidence must be a directory, not a symlink or file")
	}

	destination, err := os.OpenRoot(r.path)
	if err != nil {
		return fmt.Errorf("open verification log directory: %w", err)
	}
	defer destination.Close()
	name := "verification-" + string(stage)
	previous, err := destination.Lstat(name)
	hasPrevious := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect previous verification archive: %w", err)
	}
	if hasPrevious && !previous.IsDir() {
		return fmt.Errorf("previous verification archive must be a directory, not a symlink or file")
	}
	if missing {
		// An absent source must not leave an earlier attempt looking current.
		if hasPrevious {
			return destination.RemoveAll(name)
		}
		return nil
	}

	source, err := parent.OpenRoot(verificationDirectory)
	if err != nil {
		return fmt.Errorf("open verification evidence: %w", err)
	}
	defer source.Close()
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("copy verification evidence: %w", err)
	}
	if hasPrevious {
		if err := destination.RemoveAll(name); err != nil {
			return fmt.Errorf("remove previous verification archive: %w", err)
		}
	}
	if err := destination.Mkdir(name, 0700); err != nil {
		return fmt.Errorf("create verification archive: %w", err)
	}
	archive, err := destination.OpenRoot(name)
	if err != nil {
		return fmt.Errorf("open verification archive: %w", err)
	}
	defer archive.Close()
	if err := copyVerification(ctx, source, archive); err != nil {
		return fmt.Errorf("copy verification evidence (archive may be incomplete): %w", err)
	}
	return nil
}

func copyVerification(ctx context.Context, source, destination *os.Root) error {
	return fs.WalkDir(source.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		info, err := source.Lstat(name)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return destination.Mkdir(name, 0700)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported verification entry %q: %s", name, info.Mode())
		}
		return copyVerificationFile(ctx, source, destination, name)
	})
}

func copyVerificationFile(ctx context.Context, source, destination *os.Root, name string) error {
	input, err := source.Open(name)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := destination.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, verificationReader{ctx: ctx, reader: input})
	return errors.Join(copyErr, output.Close())
}

type verificationReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r verificationReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}
