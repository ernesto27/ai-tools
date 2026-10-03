package sandbox

import (
	"fmt"
	"os"
	"syscall"
)

// worktreeLock holds the advisory lock that gives one resumed agent exclusive
// access to a worktree. Its file is deliberately a sibling of the worktree,
// rather than a file inside it, so publish's git add -A can never stage it.
type worktreeLock struct {
	file *os.File
}

// lockWorktree takes the non-blocking lock for one recorded worktree. The file
// remains after Close: deleting it would let a waiter retain a descriptor to
// the old inode while another process created and locked a new one. A leftover
// file is harmless because the OS releases flock locks whenever the owning
// process exits, including on a crash.
func lockWorktree(worktree worktreeRecord) (*worktreeLock, error) {
	path := worktree.Path + ".agent-sandbox.lock"
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return nil, fmt.Errorf("sandbox worktree %s is already in use by another session", worktree.Branch)
		}
		return nil, fmt.Errorf("locking sandbox worktree %s: %w", worktree.Path, err)
	}
	return &worktreeLock{file: file}, nil
}

// Close releases the advisory lock. The lock file stays beside the worktree,
// but an unlocked file never prevents a later resume from taking the lock.
func (lock *worktreeLock) Close() error {
	if err := syscall.Flock(int(lock.file.Fd()), syscall.LOCK_UN); err != nil {
		return err
	}
	return lock.file.Close()
}
