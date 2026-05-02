package deploy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const deployLockFile = ".deploy.lock"

var errLockHeld = errors.New("deploy lock already held")

// LockPath returns the sentinel file used to serialize npub deploy runs for a
// cache directory.
func LockPath(cacheDir string) string {
	return filepath.Join(cacheDir, deployLockFile)
}

// Lock is an exclusive deploy cache lock. Call Release when the deploy run is
// finished.
type Lock struct {
	path string
	file *os.File
}

// AcquireLock takes an exclusive, non-blocking lock for cacheDir. It fails fast
// when another npub deploy process already holds the same cache lock.
func AcquireLock(cacheDir string) (*Lock, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating cache directory %s: %w", cacheDir, err)
	}

	path := LockPath(cacheDir)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("opening deploy lock %s: %w", path, err)
	}

	if err := lockFile(file); err != nil {
		_ = file.Close()
		if errors.Is(err, errLockHeld) {
			return nil, fmt.Errorf("another `npub deploy` is in progress (lock at `%s`); rerun once it finishes", path)
		}
		return nil, fmt.Errorf("locking deploy cache %s: %w", path, err)
	}

	return &Lock{path: path, file: file}, nil
}

// Release unlocks and closes the lock file.
func (l *Lock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	if err := unlockFile(l.file); err != nil {
		_ = l.file.Close()
		return fmt.Errorf("unlocking deploy cache %s: %w", l.path, err)
	}
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("closing deploy lock %s: %w", l.path, err)
	}
	l.file = nil
	return nil
}
