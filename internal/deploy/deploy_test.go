package deploy

import (
	"path/filepath"
	"testing"

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

func TestBuildAndGitDir(t *testing.T) {
	cache := "/tmp/whatever"
	assert.Equal(t, "/tmp/whatever/build", BuildDir(cache))
	assert.Equal(t, "/tmp/whatever/git", GitDir(cache))
}
