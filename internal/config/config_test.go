package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromYAML(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, DefaultConfigFile)
	err := os.WriteFile(yamlPath, []byte(`
notes_path: "/tmp/notes"
assets_path: "/tmp/assets"
build_path: "./dist"
site_root_url: "https://example.com"
site_name: "Test Site"
author_name: "Test Author"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(yamlPath, nil, nil)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.NotesPath != "/tmp/notes" {
		t.Errorf("NotesPath = %q, want /tmp/notes", cfg.NotesPath)
	}
	if cfg.AssetsPath != "/tmp/assets" {
		t.Errorf("AssetsPath = %q, want /tmp/assets", cfg.AssetsPath)
	}
	if cfg.BuildPath != "./dist" {
		t.Errorf("BuildPath = %q, want ./dist", cfg.BuildPath)
	}
	if cfg.SiteRootURL != "https://example.com" {
		t.Errorf("SiteRootURL = %q, want https://example.com", cfg.SiteRootURL)
	}
	if cfg.SiteName != "Test Site" {
		t.Errorf("SiteName = %q, want Test Site", cfg.SiteName)
	}
	if cfg.AuthorName != "Test Author" {
		t.Errorf("AuthorName = %q, want Test Author", cfg.AuthorName)
	}
}

func TestEnvOverridesYAML(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, DefaultConfigFile)
	err := os.WriteFile(yamlPath, []byte(`
notes_path: "/tmp/notes"
assets_path: "/tmp/assets"
build_path: "./dist"
site_root_url: "https://example.com"
site_name: "Test Site"
author_name: "Test Author"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	envOverrides := map[string]string{
		"NOTES_PATH":             "/env/notes",
		"NOTESPUB_BUILD_PATH":    "/env/dist",
		"NOTESPUB_SITE_ROOT_URL": "https://env.example.com",
		"NOTESPUB_SITE_NAME":     "Env Site",
		"NOTESPUB_AUTHOR_NAME":   "Env Author",
		"NOTESPUB_ASSETS_PATH":   "/env/assets",
	}

	cfg, err := Load(yamlPath, envOverrides, nil)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.NotesPath != "/env/notes" {
		t.Errorf("NotesPath = %q, want /env/notes", cfg.NotesPath)
	}
	if cfg.BuildPath != "/env/dist" {
		t.Errorf("BuildPath = %q, want /env/dist", cfg.BuildPath)
	}
	if cfg.SiteRootURL != "https://env.example.com" {
		t.Errorf("SiteRootURL = %q, want https://env.example.com", cfg.SiteRootURL)
	}
}

func TestFlagOverridesEnv(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, DefaultConfigFile)
	err := os.WriteFile(yamlPath, []byte(`
notes_path: "/tmp/notes"
assets_path: "/tmp/assets"
build_path: "./dist"
site_root_url: "https://example.com"
site_name: "Test Site"
author_name: "Test Author"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	envOverrides := map[string]string{
		"NOTES_PATH": "/env/notes",
	}

	flagOverrides := map[string]string{
		"notes-path": "/flag/notes",
	}

	cfg, err := Load(yamlPath, envOverrides, flagOverrides)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.NotesPath != "/flag/notes" {
		t.Errorf("NotesPath = %q, want /flag/notes", cfg.NotesPath)
	}
}

func TestLoadMissingRequiredField(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, DefaultConfigFile)
	err := os.WriteFile(yamlPath, []byte(`
notes_path: "/tmp/notes"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Load(yamlPath, nil, nil)
	if err == nil {
		t.Fatal("Load() expected error for missing required fields, got nil")
	}
}

func TestExpandHomePath(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, DefaultConfigFile)
	err := os.WriteFile(yamlPath, []byte(`
notes_path: "~/notes"
assets_path: "~/assets"
build_path: "./dist"
site_root_url: "https://example.com"
site_name: "Test Site"
author_name: "Test Author"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(yamlPath, nil, nil)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	home, _ := os.UserHomeDir()
	if cfg.NotesPath != filepath.Join(home, "notes") {
		t.Errorf("NotesPath = %q, want %s/notes", cfg.NotesPath, home)
	}
}
