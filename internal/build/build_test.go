package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dreikanter/notes-pub/internal/config"
)

func TestCleanBuildDir(t *testing.T) {
	dir := t.TempDir()

	// Create regular files and dirs.
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("hi"), 0o644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0o755)
	os.WriteFile(filepath.Join(dir, "subdir", "page.html"), []byte("hi"), 0o644)

	// Create dotfiles and dotdirs that should survive.
	os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0o755)
	os.WriteFile(filepath.Join(dir, ".nojekyll"), []byte(""), 0o644)

	if err := cleanBuildDir(dir); err != nil {
		t.Fatalf("cleanBuildDir() error: %v", err)
	}

	// Regular files should be gone.
	if _, err := os.Stat(filepath.Join(dir, "index.html")); !os.IsNotExist(err) {
		t.Error("index.html should have been removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "subdir")); !os.IsNotExist(err) {
		t.Error("subdir should have been removed")
	}

	// Dotfiles should remain.
	if _, err := os.Stat(filepath.Join(dir, ".git", "objects")); err != nil {
		t.Error(".git/objects should have been preserved")
	}
	if _, err := os.Stat(filepath.Join(dir, ".nojekyll")); err != nil {
		t.Error(".nojekyll should have been preserved")
	}
}

func TestCleanBuildDirNonExistent(t *testing.T) {
	if err := cleanBuildDir("/tmp/notespub-does-not-exist-" + t.Name()); err != nil {
		t.Fatalf("cleanBuildDir() should not error for non-existent dir, got: %v", err)
	}
}

func TestCleanBuildDirRejectsRoot(t *testing.T) {
	if err := cleanBuildDir("/"); err == nil {
		t.Fatal("cleanBuildDir('/') should return an error")
	}
}

func TestCopyStaticFiles(t *testing.T) {
	staticDir := t.TempDir()
	buildDir := t.TempDir()

	// Create static files.
	os.WriteFile(filepath.Join(staticDir, "CNAME"), []byte("example.com"), 0o644)
	os.WriteFile(filepath.Join(staticDir, "README.md"), []byte("# Hello"), 0o644)
	os.MkdirAll(filepath.Join(staticDir, "sub"), 0o755)
	os.WriteFile(filepath.Join(staticDir, "sub", "file.txt"), []byte("nested"), 0o644)

	if err := copyStaticFiles(staticDir, buildDir); err != nil {
		t.Fatalf("copyStaticFiles() error: %v", err)
	}

	// Verify files were copied.
	data, err := os.ReadFile(filepath.Join(buildDir, "CNAME"))
	if err != nil {
		t.Fatal("CNAME not copied")
	}
	if string(data) != "example.com" {
		t.Errorf("CNAME = %q, want example.com", data)
	}

	data, err = os.ReadFile(filepath.Join(buildDir, "sub", "file.txt"))
	if err != nil {
		t.Fatal("sub/file.txt not copied")
	}
	if string(data) != "nested" {
		t.Errorf("sub/file.txt = %q, want nested", data)
	}
}

func TestCleanBuildDirRejectsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	if err := cleanBuildDir(home); err == nil {
		t.Fatal("cleanBuildDir(home) should return an error")
	}
}

func writeTestNote(t *testing.T, root, relPath, content string) {
	t.Helper()
	full := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testConfig(t *testing.T, notesPath, buildPath, assetsPath string) config.Config {
	t.Helper()
	return config.Config{
		NotesPath:   notesPath,
		AssetsPath:  assetsPath,
		BuildPath:   buildPath,
		SiteRootURL: "https://example.com",
		SiteName:    "Test Site",
		AuthorName:  "Test Author",
	}
}

func TestBuildPublicNote(t *testing.T) {
	notesDir := t.TempDir()
	buildDir := t.TempDir()
	assetsDir := t.TempDir()

	writeTestNote(t, notesDir, "2023/01/20230130_3961_my-note.md", `---
title: My Test Note
slug: my-test-note
tags: [golang, testing]
public: true
---

Hello **world**.
`)

	cfg := testConfig(t, notesDir, buildDir, assetsDir)
	templateFS := os.DirFS("../../")
	styleCSS := []byte("/* test */")
	if err := Build(cfg, templateFS, styleCSS); err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Check note page exists.
	notePath := filepath.Join(buildDir, "20230130_3961", "my-test-note", "index.html")
	data, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("note page not found: %v", err)
	}
	html := string(data)
	if !strings.Contains(html, "My Test Note") {
		t.Error("note page missing title")
	}
	if !strings.Contains(html, "<strong>world</strong>") {
		t.Error("note page missing rendered body")
	}

	// Check redirect page exists.
	redirectPath := filepath.Join(buildDir, "20230130_3961", "index.html")
	rdata, err := os.ReadFile(redirectPath)
	if err != nil {
		t.Fatalf("redirect page not found: %v", err)
	}
	if !strings.Contains(string(rdata), "my-test-note") {
		t.Error("redirect page missing slug in redirect URL")
	}

	// Check index page exists.
	indexPath := filepath.Join(buildDir, "index.html")
	idata, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("index page not found: %v", err)
	}
	if !strings.Contains(string(idata), "My Test Note") {
		t.Error("index page missing note title")
	}

	// Check tag page exists.
	tagPath := filepath.Join(buildDir, "tags", "golang", "index.html")
	if _, err := os.Stat(tagPath); err != nil {
		t.Errorf("tag page not found: %v", err)
	}

	// Check feed exists.
	feedPath := filepath.Join(buildDir, "feed.xml")
	fdata, err := os.ReadFile(feedPath)
	if err != nil {
		t.Fatalf("feed not found: %v", err)
	}
	if !strings.Contains(string(fdata), "My Test Note") {
		t.Error("feed missing note title")
	}

	// Check style.css is NOT written to disk (inlined into HTML instead).
	stylePath := filepath.Join(buildDir, "style.css")
	if _, err := os.Stat(stylePath); !os.IsNotExist(err) {
		t.Errorf("style.css should not be written to build dir (CSS is inlined), got err: %v", err)
	}

	// Check CSS is inlined into HTML.
	indexData, err := os.ReadFile(filepath.Join(buildDir, "index.html"))
	if err != nil {
		t.Fatalf("reading index.html: %v", err)
	}
	if !strings.Contains(string(indexData), "/* test */") {
		t.Error("index.html should contain inlined styleCSS")
	}
	if strings.Contains(string(indexData), `href="/style.css"`) {
		t.Error("index.html should not reference external /style.css")
	}
}

func TestBuildSkipsPrivateNote(t *testing.T) {
	notesDir := t.TempDir()
	buildDir := t.TempDir()
	assetsDir := t.TempDir()

	writeTestNote(t, notesDir, "2023/01/20230130_3961_private.md", `---
title: Private Note
tags: [secret]
---

This is private.
`)

	cfg := testConfig(t, notesDir, buildDir, assetsDir)
	templateFS := os.DirFS("../../")
	styleCSS := []byte("/* test */")
	if err := Build(cfg, templateFS, styleCSS); err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	notePath := filepath.Join(buildDir, "20230130_3961", "private", "index.html")
	if _, err := os.Stat(notePath); err == nil {
		t.Error("private note should not be built")
	}

	indexPath := filepath.Join(buildDir, "index.html")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("index page not found: %v", err)
	}
	if strings.Contains(string(data), "Private Note") {
		t.Error("index page should not list private note")
	}
}

func TestBuildNoteLinkResolution(t *testing.T) {
	notesDir := t.TempDir()
	buildDir := t.TempDir()
	assetsDir := t.TempDir()

	writeTestNote(t, notesDir, "2023/01/20230130_3961_first.md", `---
title: First Note
slug: first-note
tags: [test]
public: true
---

See [second note](3962).
`)

	writeTestNote(t, notesDir, "2023/01/20230131_3962_second.md", `---
title: Second Note
slug: second-note
tags: [test]
public: true
---

Hello from second note.
`)

	cfg := testConfig(t, notesDir, buildDir, assetsDir)
	templateFS := os.DirFS("../../")
	styleCSS := []byte("/* test */")
	if err := Build(cfg, templateFS, styleCSS); err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	notePath := filepath.Join(buildDir, "20230130_3961", "first-note", "index.html")
	data, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("note page not found: %v", err)
	}
	if !strings.Contains(string(data), `href="/20230131_3962/second-note"`) {
		t.Errorf("expected resolved note link, got: %s", data)
	}
}
