package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	yamlPath := filepath.Join(t.TempDir(), DefaultConfigFile)
	require.NoError(t, os.WriteFile(yamlPath, []byte(content), 0o644))
	return yamlPath
}

func TestLoadFromYAML(t *testing.T) {
	yamlPath := writeConfig(t, `
notes_path: "/tmp/notes"
assets_path: "/tmp/assets"
site_root_url: "https://example.com"
site_name: "Test Site"
author_name: "Test Author"
`)

	cfg, err := Load(yamlPath, nil)
	require.NoError(t, err)

	assert.Equal(t, "/tmp/notes", cfg.NotesPath)
	assert.Equal(t, "/tmp/assets", cfg.AssetsPath)
	assert.Equal(t, "https://example.com", cfg.SiteRootURL)
	assert.Equal(t, "Test Site", cfg.SiteName)
	assert.Equal(t, "Test Author", cfg.AuthorName)
}

func TestFlagOverridesYAML(t *testing.T) {
	yamlPath := writeConfig(t, `
notes_path: "/tmp/notes"
assets_path: "/tmp/assets"
site_root_url: "https://example.com"
site_name: "Test Site"
author_name: "Test Author"
`)

	cfg, err := Load(yamlPath, map[string]string{"path": "/flag/notes"})
	require.NoError(t, err)

	assert.Equal(t, "/flag/notes", cfg.NotesPath)
}

func TestLoadMissingRequiredField(t *testing.T) {
	yamlPath := writeConfig(t, `
notes_path: "/tmp/notes"
`)

	_, err := Load(yamlPath, nil)
	require.Error(t, err)
	missing, ok := errors.AsType[MissingRequiredError](err)
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"site_root_url", "site_name", "author_name"}, missing.Fields)
}

func TestAssetsPathDefaultsToNotesImages(t *testing.T) {
	yamlPath := writeConfig(t, `
notes_path: "/tmp/notes"
site_root_url: "https://example.com"
site_name: "Test Site"
author_name: "Test Author"
`)

	cfg, err := Load(yamlPath, nil)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join("/tmp/notes", "images"), cfg.AssetsPath)
}

func TestNotesPathDefaultsToEnvVar(t *testing.T) {
	yamlPath := writeConfig(t, `
site_root_url: "https://example.com"
site_name: "Test Site"
author_name: "Test Author"
`)
	t.Setenv("NOTES_PATH", "/env/notes")

	cfg, err := Load(yamlPath, nil)
	require.NoError(t, err)

	assert.Equal(t, "/env/notes", cfg.NotesPath)
}

func TestExpandHomePath(t *testing.T) {
	yamlPath := writeConfig(t, `
notes_path: "~/notes"
assets_path: "~/assets"
site_root_url: "https://example.com"
site_name: "Test Site"
author_name: "Test Author"
`)

	cfg, err := Load(yamlPath, nil)
	require.NoError(t, err)

	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, "notes"), cfg.NotesPath)
}

func TestExpandEnvVarsInYAMLPaths(t *testing.T) {
	yamlPath := writeConfig(t, `
notes_path: "$NPUB_TEST_ROOT/notes"
assets_path: "$NPUB_TEST_ROOT/assets"
static_path: "$NPUB_TEST_ROOT/static"
site_root_url: "https://example.com"
site_name: "Test Site"
author_name: "Test Author"
`)
	t.Setenv("NPUB_TEST_ROOT", "/srv/npub")

	cfg, err := Load(yamlPath, nil)
	require.NoError(t, err)

	assert.Equal(t, "/srv/npub/notes", cfg.NotesPath)
	assert.Equal(t, "/srv/npub/assets", cfg.AssetsPath)
	assert.Equal(t, "/srv/npub/static", cfg.StaticPath)
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	t.Setenv("NPUB_TEST_VAR", "/srv/npub")

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "tilde prefix",
			path: "~/documents",
			want: filepath.Join(home, "documents"),
		},
		{
			name: "tilde only slash",
			path: "~/",
			want: home,
		},
		{
			name: "no tilde",
			path: "/absolute/path",
			want: "/absolute/path",
		},
		{
			name: "tilde in middle is not expanded",
			path: "/some/~/path",
			want: "/some/~/path",
		},
		{
			name: "relative path unchanged",
			path: "relative/path",
			want: "relative/path",
		},
		{
			name: "env var expansion",
			path: "$NPUB_TEST_VAR/notes",
			want: "/srv/npub/notes",
		},
		{
			name: "env var braced expansion",
			path: "${NPUB_TEST_VAR}/notes",
			want: "/srv/npub/notes",
		},
		{
			name: "unset env var collapses to empty",
			path: "$NPUB_UNSET_VAR/notes",
			want: "/notes",
		},
		{
			name: "empty input",
			path: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ExpandPath(tt.path))
		})
	}
}

func TestSiteRootPathAndDomain(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		rootPath string
		domain   string
		feedPath string
	}{
		{
			name:     "domain only",
			url:      "https://example.com",
			rootPath: "/",
			domain:   "example.com",
			feedPath: "/feed.xml",
		},
		{
			name:     "domain with trailing slash",
			url:      "https://example.com/",
			rootPath: "/",
			domain:   "example.com",
			feedPath: "/feed.xml",
		},
		{
			name:     "subpath",
			url:      "https://example.com/blog",
			rootPath: "/blog",
			domain:   "example.com",
			feedPath: "/blog/feed.xml",
		},
		{
			name:     "subpath with trailing slash",
			url:      "https://example.com/blog/",
			rootPath: "/blog",
			domain:   "example.com",
			feedPath: "/blog/feed.xml",
		},
		{
			name:     "nested subpath",
			url:      "https://example.com/a/b",
			rootPath: "/a/b",
			domain:   "example.com",
			feedPath: "/a/b/feed.xml",
		},
		{
			name:     "host with port",
			url:      "http://localhost:4000/blog",
			rootPath: "/blog",
			domain:   "localhost:4000",
			feedPath: "/blog/feed.xml",
		},
		{
			name:     "no scheme falls back to the raw value",
			url:      "example.com/blog",
			rootPath: "/",
			domain:   "example.com/blog",
			feedPath: "/feed.xml",
		},
		{
			name:     "empty",
			url:      "",
			rootPath: "/",
			domain:   "",
			feedPath: "/feed.xml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{SiteRootURL: tt.url}
			assert.Equal(t, tt.rootPath, cfg.SiteRootPath(), "SiteRootPath")
			assert.Equal(t, tt.domain, cfg.SiteDomain(), "SiteDomain")
			assert.Equal(t, tt.feedPath, cfg.FeedPath(), "FeedPath")
		})
	}
}
