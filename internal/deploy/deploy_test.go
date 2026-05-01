package deploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
