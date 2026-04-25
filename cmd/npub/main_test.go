package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dreikanter/npub/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitConfigCreatesSampleInNewDirectory(t *testing.T) {
	target := filepath.Join(t.TempDir(), "site")

	cfgPath, err := initConfig(target)
	require.NoError(t, err)

	wantPath := filepath.Join(target, config.DefaultConfigFile)
	require.Equal(t, wantPath, cfgPath)

	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	content := string(data)
	for _, want := range []string{
		`# notes_path: ""`,
		`# build_path: "./dist"`,
		`# license_name: "CC BY 4.0"`,
	} {
		assert.Contains(t, content, want)
	}
}

func TestInitConfigDefaultsToCurrentDirectory(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })
	require.NoError(t, os.Chdir(dir))

	cfgPath, err := initConfig(".")
	require.NoError(t, err)

	assert.Equal(t, config.DefaultConfigFile, cfgPath)
	assert.FileExists(t, filepath.Join(dir, config.DefaultConfigFile))
}

func TestInitConfigDoesNotOverwriteExistingConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, config.DefaultConfigFile)
	require.NoError(t, os.WriteFile(cfgPath, []byte("existing"), 0o644))

	_, err := initConfig(dir)
	require.Error(t, err)

	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "existing", string(data))
}

func TestInitConfigRejectsNonDirectoryPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(path, []byte("file"), 0o644))

	_, err := initConfig(path)
	require.Error(t, err)
}

func TestResolveConfigPath(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	notesDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(notesDir, config.DefaultConfigFile), []byte("---\n"), 0o644))
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

			assert.Equal(t, tt.want, resolveConfigPath(tt.flagValue, tt.envValue, tt.notesPath))
		})
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

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
			assert.Equal(t, tt.want, expandHome(tt.path))
		})
	}
}
