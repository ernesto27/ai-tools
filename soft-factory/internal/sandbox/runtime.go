package sandbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const runtimeDirectory = ".soft-factory-sandbox-runtime"
const runtimeFilename = "agent-sandbox.json"

// Runtime owns an exclusive, project-local configuration transport. The directory
// is deliberately retained on interruption or unsafe cleanup, including its
// owner record and original file, so a later invocation cannot destroy evidence.
type Runtime struct {
	directory string
	target    string
	backup    string
	original  os.FileInfo
	generated os.FileInfo
	closed    bool
}

// PrepareRuntime publishes the original embedded JSON without re-marshalling the
// upstream struct. An existing runtime directory means active or interrupted work;
// fail rather than guessing whether its children are still using the file.
func PrepareRuntime(data []byte) (*Runtime, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("locate sandbox working directory: %w", err)
	}
	r := &Runtime{directory: filepath.Join(root, runtimeDirectory), target: filepath.Join(root, runtimeFilename)}
	r.backup = filepath.Join(r.directory, "original.json")
	if err := os.Mkdir(r.directory, 0700); err != nil {
		return nil, fmt.Errorf("acquire sandbox workflow lock: %w; another workflow may be active or interrupted. Stop Factory and its sandbox children before recovery; inspect %s/owner and preserve %s before manually restoring agent-sandbox.json and removing %s", err, r.directory, r.backup, r.directory)
	}
	// Register cleanup before setup mutates the transport or original file.
	var cleanupErr error
	err = func() error {
		ready := false
		defer func() {
			if !ready {
				cleanupErr = r.Close()
			}
		}()
		if err := r.prepare(data); err != nil {
			return err
		}
		ready = true
		return nil
	}()
	if err != nil {
		return nil, errors.Join(err, cleanupErr)
	}
	return r, nil
}

func (r *Runtime) prepare(data []byte) error {
	owner := fmt.Sprintf("Factory PID: %d\nWorking directory: %s\nOriginal backup: %s\nStop Factory and sandbox children before manual recovery. Preserve the backup; restore it only after inspecting the generated path.\n", os.Getpid(), filepath.Dir(r.target), r.backup)
	if err := os.WriteFile(filepath.Join(r.directory, "owner"), []byte(owner), 0600); err != nil {
		return fmt.Errorf("write sandbox recovery ownership: %w", err)
	}
	info, err := os.Lstat(r.target)
	if err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("sandbox runtime path %q must be a regular file; symlinks are refused", r.target)
		}
		if err := os.Rename(r.target, r.backup); err != nil {
			return fmt.Errorf("back up sandbox runtime path: %w", err)
		}
		r.original = info
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect sandbox runtime path: %w", err)
	}
	temp := filepath.Join(r.directory, "generated.json")
	f, err := os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("create generated sandbox configuration: %w", err)
	}
	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return fmt.Errorf("write generated sandbox configuration: %w", err)
	}
	info, err = os.Lstat(temp)
	if err != nil {
		return fmt.Errorf("inspect generated sandbox configuration: %w", err)
	}
	// Linking is atomic and refuses an unexpected file created after the backup.
	if err := os.Link(temp, r.target); err != nil {
		return fmt.Errorf("publish generated sandbox configuration: %w", err)
	}
	r.generated = info
	return nil
}

// Close must be called after every child has stopped and been waited for. Never
// overwrite an unexpected replacement, or discard a backup on restoration error.
func (r *Runtime) Close() error {
	if r.closed {
		return nil
	}
	if r.generated != nil {
		info, err := os.Lstat(r.target)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return r.recoveryError(err)
		}
		if err == nil {
			if !os.SameFile(info, r.generated) {
				return r.recoveryError(fmt.Errorf("sandbox runtime path was replaced by another process"))
			}
			if err := os.Remove(r.target); err != nil {
				return r.recoveryError(err)
			}
		}
		r.generated = nil
	}
	if r.original != nil {
		info, err := os.Lstat(r.backup)
		if err != nil {
			return r.recoveryError(err)
		}
		if !os.SameFile(info, r.original) {
			return r.recoveryError(fmt.Errorf("original sandbox backup was replaced"))
		}
		// Restore with no replacement, preserving the inode, bytes and permissions.
		if err := os.Link(r.backup, r.target); err != nil {
			return r.recoveryError(err)
		}
		if err := os.Remove(r.backup); err != nil {
			return r.recoveryError(err)
		}
		r.original = nil
	}
	for _, name := range []string{"generated.json", "owner"} {
		if err := os.Remove(filepath.Join(r.directory, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return r.recoveryError(err)
		}
	}
	if err := os.Remove(r.directory); err != nil {
		return r.recoveryError(err)
	}
	r.closed = true
	return nil
}

func (r *Runtime) recoveryError(err error) error {
	return fmt.Errorf("clean up sandbox configuration: %w; recovery artifacts retained in %s. Stop Factory and sandbox children, preserve original.json, inspect agent-sandbox.json, then restore the original manually before removing the recovery directory", err, r.directory)
}
