package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dreikanter/notespub/internal/config"
)

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

	// Check style.css is copied.
	stylePath := filepath.Join(buildDir, "style.css")
	if _, err := os.Stat(stylePath); err != nil {
		t.Errorf("style.css not found: %v", err)
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
