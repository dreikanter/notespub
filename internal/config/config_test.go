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

	cfg, err := Load(yamlPath, nil)
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

func TestFlagOverridesYAML(t *testing.T) {
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

	flagOverrides := map[string]string{
		"notes": "/flag/notes",
	}

	cfg, err := Load(yamlPath, flagOverrides)
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

	_, err = Load(yamlPath, nil)
	if err == nil {
		t.Fatal("Load() expected error for missing required fields, got nil")
	}
}

func TestAssetsPathDefaultsToNotesImages(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, DefaultConfigFile)
	err := os.WriteFile(yamlPath, []byte(`
notes_path: "/tmp/notes"
build_path: "./dist"
site_root_url: "https://example.com"
site_name: "Test Site"
author_name: "Test Author"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(yamlPath, nil)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	want := filepath.Join("/tmp/notes", "images")
	if cfg.AssetsPath != want {
		t.Errorf("AssetsPath = %q, want %q", cfg.AssetsPath, want)
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

	cfg, err := Load(yamlPath, nil)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	home, _ := os.UserHomeDir()
	if cfg.NotesPath != filepath.Join(home, "notes") {
		t.Errorf("NotesPath = %q, want %s/notes", cfg.NotesPath, home)
	}
}
