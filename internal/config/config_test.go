package config

import (
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
build_path: "./dist"
site_root_url: "https://example.com"
site_name: "Test Site"
author_name: "Test Author"
`)

	cfg, err := Load(yamlPath, nil)
	require.NoError(t, err)

	assert.Equal(t, "/tmp/notes", cfg.NotesPath)
	assert.Equal(t, "/tmp/assets", cfg.AssetsPath)
	assert.Equal(t, "./dist", cfg.BuildPath)
	assert.Equal(t, "https://example.com", cfg.SiteRootURL)
	assert.Equal(t, "Test Site", cfg.SiteName)
	assert.Equal(t, "Test Author", cfg.AuthorName)
}

func TestFlagOverridesYAML(t *testing.T) {
	yamlPath := writeConfig(t, `
notes_path: "/tmp/notes"
assets_path: "/tmp/assets"
build_path: "./dist"
site_root_url: "https://example.com"
site_name: "Test Site"
author_name: "Test Author"
`)

	cfg, err := Load(yamlPath, map[string]string{"notes": "/flag/notes"})
	require.NoError(t, err)

	assert.Equal(t, "/flag/notes", cfg.NotesPath)
}

func TestLoadMissingRequiredField(t *testing.T) {
	yamlPath := writeConfig(t, `
notes_path: "/tmp/notes"
`)

	_, err := Load(yamlPath, nil)
	require.Error(t, err)
}

func TestAssetsPathDefaultsToNotesImages(t *testing.T) {
	yamlPath := writeConfig(t, `
notes_path: "/tmp/notes"
build_path: "./dist"
site_root_url: "https://example.com"
site_name: "Test Site"
author_name: "Test Author"
`)

	cfg, err := Load(yamlPath, nil)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join("/tmp/notes", "images"), cfg.AssetsPath)
}

func TestBuildPathDefaultsToDist(t *testing.T) {
	yamlPath := writeConfig(t, `
notes_path: "/tmp/notes"
site_root_url: "https://example.com"
site_name: "Test Site"
author_name: "Test Author"
`)

	cfg, err := Load(yamlPath, nil)
	require.NoError(t, err)

	assert.Equal(t, "./dist", cfg.BuildPath)
}

func TestNotesPathDefaultsToEnvVar(t *testing.T) {
	yamlPath := writeConfig(t, `
build_path: "./dist"
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
build_path: "./dist"
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
