package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dreikanter/npub/internal/config"
)

func TestInitConfigCreatesSampleInNewDirectory(t *testing.T) {
	target := filepath.Join(t.TempDir(), "site")

	cfgPath, err := initConfig(target)
	if err != nil {
		t.Fatalf("initConfig() error = %v", err)
	}

	wantPath := filepath.Join(target, config.DefaultConfigFile)
	if cfgPath != wantPath {
		t.Fatalf("initConfig() path = %q, want %q", cfgPath, wantPath)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{
		`# notes_path: ""`,
		`# build_path: "./dist"`,
		`# license_name: "CC BY 4.0"`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("generated config missing %q", want)
		}
	}
}

func TestInitConfigDefaultsToCurrentDirectory(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	cfgPath, err := initConfig(".")
	if err != nil {
		t.Fatalf("initConfig() error = %v", err)
	}
	if cfgPath != config.DefaultConfigFile {
		t.Fatalf("initConfig() path = %q", cfgPath)
	}
	if _, err := os.Stat(filepath.Join(dir, config.DefaultConfigFile)); err != nil {
		t.Fatal(err)
	}
}

func TestInitConfigDoesNotOverwriteExistingConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, config.DefaultConfigFile)
	if err := os.WriteFile(cfgPath, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := initConfig(dir); err == nil {
		t.Fatal("initConfig() error = nil, want error")
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "existing" {
		t.Fatalf("existing config overwritten: %q", data)
	}
}

func TestInitConfigRejectsNonDirectoryPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(path, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := initConfig(path); err == nil {
		t.Fatal("initConfig() error = nil, want error")
	}
}

func TestResolveConfigPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	notesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(notesDir, config.DefaultConfigFile), []byte("---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	emptyNotesDir := t.TempDir()

	tests := []struct {
		name      string
		flagValue string
		envValue  string
		notesPath string
		env       map[string]string
		want      string
	}{
		{
			name:      "flag takes precedence",
			flagValue: "/explicit/config.yml",
			want:      "/explicit/config.yml",
		},
		{
			name:      "flag takes precedence over NPUB_CONFIG",
			flagValue: "/explicit/config.yml",
			envValue:  "/some/dir/npub.yml",
			want:      "/explicit/config.yml",
		},
		{
			name:      "flag takes precedence over NOTES_PATH config",
			flagValue: "/explicit/config.yml",
			notesPath: notesDir,
			want:      "/explicit/config.yml",
		},
		{
			name:      "NPUB_CONFIG takes precedence over NOTES_PATH config",
			envValue:  "/some/dir/npub.yml",
			notesPath: notesDir,
			want:      "/some/dir/npub.yml",
		},
		{
			name:     "NPUB_CONFIG basic",
			envValue: "/some/dir/npub.yml",
			want:     "/some/dir/npub.yml",
		},
		{
			name:     "NPUB_CONFIG with tilde",
			envValue: "~/notes/npub.yml",
			want:     filepath.Join(home, "notes", "npub.yml"),
		},
		{
			name:     "NPUB_CONFIG with env var",
			envValue: "$TEST_NPUB_HOME/npub.yml",
			env:      map[string]string{"TEST_NPUB_HOME": "/expanded"},
			want:     "/expanded/npub.yml",
		},
		{
			name:      "NOTES_PATH config when present",
			notesPath: notesDir,
			want:      filepath.Join(notesDir, config.DefaultConfigFile),
		},
		{
			name:      "NOTES_PATH with no config file falls back to cwd",
			notesPath: emptyNotesDir,
			want:      config.DefaultConfigFile,
		},
		{
			name: "falls back to cwd",
			want: config.DefaultConfigFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			got := resolveConfigPath(tt.flagValue, tt.envValue, tt.notesPath)
			if got != tt.want {
				t.Errorf("resolveConfigPath(%q, %q, %q) = %q, want %q",
					tt.flagValue, tt.envValue, tt.notesPath, got, tt.want)
			}
		})
	}
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandHome(tt.path)
			if got != tt.want {
				t.Errorf("expandHome(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
