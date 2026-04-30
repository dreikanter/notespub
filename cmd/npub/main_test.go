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

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{name: "lowest valid", port: 1},
		{name: "common dev port", port: 4000},
		{name: "highest valid", port: 65535},
		{name: "zero rejected", port: 0, wantErr: true},
		{name: "negative rejected", port: -1, wantErr: true},
		{name: "above range rejected", port: 65536, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePort(tt.port)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "must be between 1 and 65535")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestResolveConfigPath(t *testing.T) {
	notesDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(notesDir, config.DefaultConfigFile), []byte("---\n"), 0o644))
	emptyNotesDir := t.TempDir()

	tests := []struct {
		name      string
		flagValue string
		notesPath string
		want      string
	}{
		{
			name:      "flag takes precedence",
			flagValue: "/explicit/config.yml",
			want:      "/explicit/config.yml",
		},
		{
			name:      "flag takes precedence over NOTES_PATH config",
			flagValue: "/explicit/config.yml",
			notesPath: notesDir,
			want:      "/explicit/config.yml",
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
			assert.Equal(t, tt.want, resolveConfigPath(tt.flagValue, tt.notesPath))
		})
	}
}

