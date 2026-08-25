package deploy

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	assert.True(t, errors.Is(statErr, fs.ErrNotExist), "gitDir should not have been created")
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

// quietOptions discards git's streamed output so test logs stay readable.
func quietOptions() Options { return Options{Stdout: io.Discard, Stderr: io.Discard} }

// initBareRemote creates an empty bare repository usable as a deploy_repo and
// returns its path.
func initBareRemote(t *testing.T) string {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "remote.git")
	require.NoError(t, exec.Command("git", "init", "--bare", remote).Run())
	return remote
}

// gitRemote runs git against a bare repository directory and returns trimmed
// stdout.
func gitRemote(t *testing.T, gitDir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"--git-dir=" + gitDir}, args...)...).Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(out))
}

// deployOnce writes files into a fresh build dir and runs the full
// Prepare/Commit/Push cycle, returning whether a commit was created.
func deployOnce(t *testing.T, repoURL, cacheDir, message string, force bool, files map[string]string) bool {
	t.Helper()
	buildDir := BuildDir(cacheDir)
	gitDir := GitDir(cacheDir)
	// Start from a clean build dir so each call mirrors a fresh `npub build`.
	require.NoError(t, os.RemoveAll(buildDir))
	require.NoError(t, os.MkdirAll(buildDir, 0o755))
	for name, content := range files {
		path := filepath.Join(buildDir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	opt := quietOptions()
	opt.Force = force
	require.NoError(t, Prepare(repoURL, gitDir, buildDir, opt))
	committed, err := Commit(gitDir, buildDir, message, opt)
	require.NoError(t, err)
	if committed {
		require.NoError(t, Push(gitDir, opt))
	}
	return committed
}

func TestDeployFirstPushCreatesBranch(t *testing.T) {
	remote := initBareRemote(t)
	cache := filepath.Join(t.TempDir(), "cache")

	committed := deployOnce(t, remote, cache, "Deploy 1", false, map[string]string{
		"index.html": "v1",
	})
	require.True(t, committed)

	branch := "main"
	assert.Equal(t, "1", gitRemote(t, remote, "rev-list", "--count", branch))
	assert.Equal(t, "v1", gitRemote(t, remote, "show", branch+":index.html"))
}

func TestDeployAppendsCommitByDefault(t *testing.T) {
	remote := initBareRemote(t)
	cache := filepath.Join(t.TempDir(), "cache")

	require.True(t, deployOnce(t, remote, cache, "Deploy 1", false, map[string]string{
		"index.html": "v1",
	}))
	require.True(t, deployOnce(t, remote, cache, "Deploy 2", false, map[string]string{
		"index.html": "v2",
	}))

	branch := "main"
	assert.Equal(t, "2", gitRemote(t, remote, "rev-list", "--count", branch),
		"a non-destructive deploy should append to remote history")
	assert.NotEmpty(t, gitRemote(t, remote, "log", "--format=%P", "-1", branch),
		"the appended commit should have the previous tip as its parent")
	assert.Equal(t, "v2", gitRemote(t, remote, "show", branch+":index.html"))
}

func TestDeployForceReplacesHistoryWithRootCommit(t *testing.T) {
	remote := initBareRemote(t)
	cache := filepath.Join(t.TempDir(), "cache")

	require.True(t, deployOnce(t, remote, cache, "Deploy 1", true, map[string]string{
		"index.html": "v1",
		"old.html":   "stale",
	}))
	require.True(t, deployOnce(t, remote, cache, "Deploy 2", true, map[string]string{
		"index.html": "v2",
	}))

	branch := "main"
	assert.Equal(t, "1", gitRemote(t, remote, "rev-list", "--count", branch),
		"force deploy must collapse remote history to a single root commit")
	assert.Empty(t, gitRemote(t, remote, "log", "--format=%P", "-1", branch),
		"the published commit should be a root commit with no parent")
	assert.Equal(t, "v2", gitRemote(t, remote, "show", branch+":index.html"))
	// The file dropped from the second build must be gone from the tip tree.
	assert.NotContains(t, gitRemote(t, remote, "ls-tree", "--name-only", branch), "old.html")
}

func TestDeploySkipsPushWhenUnchanged(t *testing.T) {
	remote := initBareRemote(t)
	cache := filepath.Join(t.TempDir(), "cache")

	require.True(t, deployOnce(t, remote, cache, "Deploy 1", false, map[string]string{
		"index.html": "v1",
	}))
	// Re-running with identical content yields no new commit.
	committed := deployOnce(t, remote, cache, "Deploy 2", false, map[string]string{
		"index.html": "v1",
	})
	assert.False(t, committed, "unchanged build should not create a commit")
}
