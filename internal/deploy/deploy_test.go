package deploy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dreikanter/npub/internal/build"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoSlug(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"https with .git", "https://github.com/user/repo.git", "repo"},
		{"https without .git", "https://github.com/user/repo", "repo"},
		{"ssh with .git", "git@github.com:user/repo.git", "repo"},
		{"trailing slash", "https://github.com/user/repo/", "repo"},
		{"only basename", "repo.git", "repo"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, RepoSlug(tt.in))
		})
	}
}

func TestDefaultCacheDir(t *testing.T) {
	dir, err := DefaultCacheDir("https://github.com/user/site.git")
	require.NoError(t, err)
	assert.Equal(t, "site", filepath.Base(dir))
	assert.Equal(t, "npub", filepath.Base(filepath.Dir(dir)))
}

func TestBuildGitAndLockDir(t *testing.T) {
	cache := "/tmp/whatever"
	assert.Equal(t, "/tmp/whatever/build", BuildDir(cache))
	assert.Equal(t, "/tmp/whatever/git", GitDir(cache))
	assert.Equal(t, "/tmp/whatever/.npub-cache.lock", LockPath(cache))
}

func TestAcquireLockRejectsConcurrentUse(t *testing.T) {
	cache := t.TempDir()

	lock, err := AcquireLock(cache)
	require.NoError(t, err)
	defer func() { require.NoError(t, lock.Release()) }()

	second, err := AcquireLock(cache)
	require.Error(t, err)
	assert.Nil(t, second)
	assert.Contains(t, err.Error(), "another `npub` command is using this cache")
	assert.Contains(t, err.Error(), LockPath(cache))

	require.NoError(t, lock.Release())
	lock, err = AcquireLock(cache)
	require.NoError(t, err)
}

func TestEnsureGitExcludeAddsPatternOnce(t *testing.T) {
	gitDir := t.TempDir()

	require.NoError(t, ensureGitExclude(gitDir, build.BuildMarkerName))
	require.NoError(t, ensureGitExclude(gitDir, build.BuildMarkerName))

	data, err := os.ReadFile(filepath.Join(gitDir, "info", "exclude"))
	require.NoError(t, err)
	assert.Equal(t, build.BuildMarkerName+"\n", string(data))
}

func TestPrepareRefusesEmptyBuildDir(t *testing.T) {
	root := t.TempDir()
	buildDir := filepath.Join(root, "build")
	gitDir := filepath.Join(root, "git")
	require.NoError(t, os.MkdirAll(buildDir, 0o755))

	err := Prepare("https://example.com/repo.git", gitDir, buildDir, Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is empty")
	assert.Contains(t, err.Error(), "npub build")

	// gitDir was never touched: no destructive clone happened against
	// repoURL because we bailed before touching the network.
	_, statErr := os.Stat(gitDir)
	assert.True(t, os.IsNotExist(statErr), "gitDir should not have been created")
}

func TestPrepareRefusesMissingBuildDir(t *testing.T) {
	root := t.TempDir()
	err := Prepare("https://example.com/repo.git",
		filepath.Join(root, "git"),
		filepath.Join(root, "missing"),
		Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
	assert.Contains(t, err.Error(), "npub build")
}

func TestPrepareTreatsDotfilesAsNonContent(t *testing.T) {
	// A dir holding only dotfiles (e.g. a .DS_Store the user accidentally
	// dropped) is still considered empty for deploy purposes.
	root := t.TempDir()
	buildDir := filepath.Join(root, "build")
	require.NoError(t, os.MkdirAll(buildDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(buildDir, ".DS_Store"), []byte("x"), 0o644))

	err := Prepare("https://example.com/repo.git",
		filepath.Join(root, "git"),
		buildDir,
		Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is empty")
}
