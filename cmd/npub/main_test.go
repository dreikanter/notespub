package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/dreikanter/npub/internal/build"
	"github.com/dreikanter/npub/internal/config"
	"github.com/dreikanter/npub/internal/deploy"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetFlags(cmd *cobra.Command) {
	visit := func(f *pflag.Flag) {
		f.Changed = false
		_ = f.Value.Set(f.DefValue)
	}
	cmd.PersistentFlags().VisitAll(visit)
	for _, sub := range cmd.Commands() {
		sub.Flags().VisitAll(visit)
	}
}

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
		`# license_name: "CC BY 4.0"`,
		`# deploy_repo: ""`,
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

func TestConfigCommandPrintsResolvedConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, config.DefaultConfigFile)
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
notes_path: "/tmp/notes"
site_root_url: "https://example.com"
site_name: "Test Site"
author_name: "Test Author"
`), 0o644))

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"config", "--config", cfgPath})
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		resetFlags(rootCmd)
	})

	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	absPath, err := filepath.Abs(cfgPath)
	require.NoError(t, err)
	assert.Contains(t, out, "config: "+absPath)
	assert.Contains(t, out, "notes_path: /tmp/notes")
	assert.Contains(t, out, "assets_path: /tmp/notes/images")
	assert.Contains(t, out, "static_path: /tmp/notes/static")
	assert.Contains(t, out, "site_root_url: https://example.com")
	assert.Contains(t, out, "site_name: Test Site")
	assert.Contains(t, out, "author_name: Test Author")
	assert.Contains(t, out, "license_name: CC BY 4.0")
	assert.Contains(t, out, "license_url: https://creativecommons.org/licenses/by/4.0/")
	assert.Contains(t, out, "deploy_repo: \"\"")
	assert.NotContains(t, out, "build_path:")
}

func TestConfigCommandAppliesFlagOverrides(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, config.DefaultConfigFile)
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
notes_path: "/tmp/notes"
site_root_url: "https://example.com"
site_name: "Test Site"
author_name: "Test Author"
`), 0o644))

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"config", "--config", cfgPath, "--url", "https://override.example.org", "--site-name", "Override"})
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		resetFlags(rootCmd)
	})

	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "site_root_url: https://override.example.org")
	assert.Contains(t, out, "site_name: Override")
}

func TestConfigCommandPrintsPartialConfigOnMissingFields(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, config.DefaultConfigFile)
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
notes_path: "/tmp/notes"
`), 0o644))

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"config", "--config", cfgPath})
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		resetFlags(rootCmd)
	})

	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required fields")

	out := buf.String()
	assert.Contains(t, out, "notes_path: /tmp/notes")
}

func TestClearCommandDryRunUsesManagedBuildDir(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	buildDir := deploy.BuildDir(cacheDir)
	require.NoError(t, build.WriteBuildMarker(buildDir))
	cfgPath := writeClearTestConfig(t, dir, cacheDir)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"clear", "--config", cfgPath, "--dry-run"})
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		resetFlags(rootCmd)
	})

	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "would clear "+buildDir)
	assert.DirExists(t, buildDir)
}

func TestClearCommandDryRunChecksMarker(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	buildDir := deploy.BuildDir(cacheDir)
	require.NoError(t, os.MkdirAll(buildDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(buildDir, "index.html"), []byte("site"), 0o644))
	cfgPath := writeClearTestConfig(t, dir, cacheDir)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"clear", "--config", cfgPath, "--dry-run"})
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		resetFlags(rootCmd)
	})

	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not marked as an npub build directory")
	assert.FileExists(t, filepath.Join(buildDir, "index.html"))
}

func writeClearTestConfig(t *testing.T, dir, cacheDir string) string {
	t.Helper()
	cfgPath := filepath.Join(dir, config.DefaultConfigFile)
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
notes_path: "/tmp/notes"
site_root_url: "https://example.com"
site_name: "Test Site"
author_name: "Test Author"
cache_path: "`+cacheDir+`"
`), 0o644))
	return cfgPath
}

func TestClearCommandRejectsPositionalPathAndBuildHasNoOutFlag(t *testing.T) {
	assert.Nil(t, clearCmd.Flags().Lookup("out"))
	assert.Nil(t, buildCmd.Flags().Lookup("out"))

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"clear", "/tmp/site"})
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		resetFlags(rootCmd)
	})

	require.Error(t, rootCmd.Execute())
}

func TestClearBuildDirRequiresMarkerForNonEmptyDir(t *testing.T) {
	buildDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(buildDir, "index.html"), []byte("site"), 0o644))

	cleared, err := clearBuildDir(buildDir)
	require.Error(t, err)
	assert.False(t, cleared)
	assert.Contains(t, err.Error(), "not marked as an npub build directory")
	assert.FileExists(t, filepath.Join(buildDir, "index.html"))
}

func TestClearBuildDirRemovesMarkedDir(t *testing.T) {
	buildDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(buildDir, "index.html"), []byte("site"), 0o644))
	require.NoError(t, build.WriteBuildMarker(buildDir))

	cleared, err := clearBuildDir(buildDir)
	require.NoError(t, err)
	assert.True(t, cleared)
	assert.NoDirExists(t, buildDir)
}

func TestClearBuildDirRemovesEmptyUnmarkedDir(t *testing.T) {
	buildDir := t.TempDir()

	cleared, err := clearBuildDir(buildDir)
	require.NoError(t, err)
	assert.True(t, cleared)
	assert.NoDirExists(t, buildDir)
}

func TestClearBuildDirReturnsFalseForMissingDir(t *testing.T) {
	buildDir := filepath.Join(t.TempDir(), "missing")

	cleared, err := clearBuildDir(buildDir)
	require.NoError(t, err)
	assert.False(t, cleared)
}

func TestValidateClearTargetRejectsImportantPaths(t *testing.T) {
	cacheDir := t.TempDir()
	cfg := config.Config{NotesPath: deploy.BuildDir(cacheDir)}

	err := validateClearTarget(deploy.BuildDir(cacheDir), cacheDir, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "notes_path")
}

func TestValidateClearTargetRejectsSymlink(t *testing.T) {
	cacheDir := t.TempDir()
	realBuild := filepath.Join(cacheDir, "real-build")
	require.NoError(t, os.MkdirAll(realBuild, 0o755))
	buildDir := deploy.BuildDir(cacheDir)
	require.NoError(t, os.Symlink(realBuild, buildDir))

	err := validateClearTarget(buildDir, cacheDir, config.Config{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlinked")
}

func TestValidateClearTargetRejectsSymlinkedCachePath(t *testing.T) {
	root := t.TempDir()
	realCache := filepath.Join(root, "real-cache")
	require.NoError(t, os.MkdirAll(realCache, 0o755))
	cacheDir := filepath.Join(root, "cache-link")
	require.NoError(t, os.Symlink(realCache, cacheDir))

	err := validateClearTarget(deploy.BuildDir(cacheDir), cacheDir, config.Config{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlinked")
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
