package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dreikanter/notes-pub/internal/config"
)

func TestResolveConfigPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name      string
		flagValue string
		envValue  string
		env       map[string]string
		want      string
	}{
		{
			name:      "flag takes precedence",
			flagValue: "/explicit/config.yml",
			want:      "/explicit/config.yml",
		},
		{
			name:      "flag takes precedence over NOTESPUB_CONFIG",
			flagValue: "/explicit/config.yml",
			envValue:  "/some/dir/notespub.yml",
			want:      "/explicit/config.yml",
		},
		{
			name:     "NOTESPUB_CONFIG basic",
			envValue: "/some/dir/notespub.yml",
			want:     "/some/dir/notespub.yml",
		},
		{
			name:     "NOTESPUB_CONFIG with tilde",
			envValue: "~/notes/notespub.yml",
			want:     filepath.Join(home, "notes", "notespub.yml"),
		},
		{
			name:     "NOTESPUB_CONFIG with env var",
			envValue: "$TEST_NOTESPUB_HOME/notespub.yml",
			env:      map[string]string{"TEST_NOTESPUB_HOME": "/expanded"},
			want:     "/expanded/notespub.yml",
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
			got := resolveConfigPath(tt.flagValue, tt.envValue)
			if got != tt.want {
				t.Errorf("resolveConfigPath(%q, %q) = %q, want %q",
					tt.flagValue, tt.envValue, got, tt.want)
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
