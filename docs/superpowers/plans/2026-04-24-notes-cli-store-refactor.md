# notes-cli Store Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace npub's direct use of `notes-cli` v0.1.66 free functions (`note.Scan`, `note.ParseFrontmatterFields`, `note.StripFrontmatter`) with the v0.3.20 `note.Store` interface. `Build` takes a `note.Store` explicitly; build tests migrate to `note.MemStore`.

**Architecture:** `cmd/npub/main.go` constructs a `note.OSStore` rooted at `cfg.NotesPath` and passes it into `build.Build`. Inside `Build`, one `store.All(note.WithPublic(true))` call replaces the scan+read+parse+filter loop. A new `buildNotePages` helper converts `[]note.Entry` → `[]page.NotePage` + `map[int]string` note index. `render.Render` is retyped to take `string` body and `map[int]string` index.

**Tech Stack:** Go 1.25+, `github.com/dreikanter/notes-cli` v0.3.20, `github.com/yuin/goldmark`, `github.com/spf13/cobra`.

---

## Spec reference

See `docs/superpowers/specs/2026-04-24-notes-cli-store-refactor-design.md` for the full design. This plan implements it.

## File Structure

Files created: none.

Files modified:

- `go.mod`, `go.sum` — bump `notes-cli` to v0.3.20; `go mod tidy` pulls new transitive deps.
- `internal/render/render.go` — signature change: `source []byte` → `source string`, `noteIndex map[string]string` → `noteIndex map[int]string`. Internal `strconv.Atoi` lookup for markdown link targets.
- `internal/render/render_test.go` — update map literal from `"8823"` key to `8823` key.
- `internal/build/build.go` — swap read path to `store.All(note.WithPublic(true))`; extract `buildNotePages`, `chooseSlug`, `titleOrUID`; delete `parseDate`, `trimQuotes`, `trimQuotesList`, `cleanTitle`, `parsedNote`; change `Build` signature to take `note.Store`.
- `internal/build/build_test.go` — migrate three Build tests to `note.MemStore`; drop `writeTestNote` and filesystem fixtures; `testConfig` loses the `notesPath` argument.
- `cmd/npub/main.go` — construct `note.NewOSStore(cfg.NotesPath)` and pass it to `build.Build`.

## Pre-flight

- [ ] **Step 0.1: Verify clean working tree**

```bash
git status
```

Expected: `nothing to commit, working tree clean` (or only the committed spec file).

- [ ] **Step 0.2: Run existing tests — establish green baseline**

```bash
go test ./...
```

Expected: all tests pass. If any fail, stop and investigate before starting — the plan assumes the pre-swap codebase is green.

---

## Task 1: Bump notes-cli to v0.3.20

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1.1: Bump the dependency**

```bash
go get github.com/dreikanter/notes-cli@v0.3.20
```

Expected output includes a line like `go: upgraded github.com/dreikanter/notes-cli v0.1.66 => v0.3.20`.

- [ ] **Step 1.2: Tidy the module**

```bash
go mod tidy
```

Expected: `go.sum` updated; `go.mod` should now list `golang.org/x/sync` as a new transitive dep (used by `OSStore.readConcurrent`).

- [ ] **Step 1.3: Verify the dependency is in place**

```bash
grep notes-cli go.mod
```

Expected: `github.com/dreikanter/notes-cli v0.3.20` on the `require` line.

- [ ] **Step 1.4: Confirm the build currently fails (expected)**

```bash
go build ./...
```

Expected: build errors in `internal/build/build.go` complaining about `note.Scan`, `note.Note`, `note.ParseFrontmatterFields`, `note.StripFrontmatter`, or their types — these are the API calls that have moved under `note.Store` in v0.3.20.

**Do not attempt to fix them in this task** — subsequent tasks replace these call sites. Commit the dependency bump with the known build break; the next tasks restore green.

- [ ] **Step 1.5: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: bump notes-cli to v0.3.20 (Store interface)"
```

---

## Task 2: Update render.Render signature (string body + int-keyed note index)

**Files:**
- Modify: `internal/render/render.go:41` (function signature and body)
- Modify: `internal/render/render.go:60` (text.NewReader call)
- Modify: `internal/render/render.go:96` (md.Renderer().Render call)
- Modify: `internal/render/render_test.go:34` (map literal key type)

The `render` package has no dependency on `note`, so this task compiles and tests green on its own. Doing it before `build.go` keeps the compile-red surface small.

- [ ] **Step 2.1: Update render_test.go first (failing test)**

Change `internal/render/render_test.go:33-45`:

```go
func TestRenderNoteLink(t *testing.T) {
	noteIndex := map[int]string{
		8823: "20260106_8823/some-slug",
	}
	md := "See [this note](8823)\n"
	html, err := Render(md, noteIndex, nil)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if !strings.Contains(string(html), `href="/20260106_8823/some-slug"`) {
		t.Errorf("expected resolved link, got: %s", html)
	}
}
```

Also update the remaining `Render(...)` calls in the same file to pass a `string` instead of `[]byte(md)`:

- `internal/render/render_test.go:10` → `Render(md, nil, nil)`
- `internal/render/render_test.go:24` → `Render(md, nil, nil)`
- `internal/render/render_test.go:38` → already updated above
- `internal/render/render_test.go:49` → `Render(md, nil, nil)`
- `internal/render/render_test.go:60` → `Render(md, nil, nil)`
- `internal/render/render_test.go:71` → `Render(md, nil, nil)`
- `internal/render/render_test.go:82` → `Render(md, nil, nil)`
- `internal/render/render_test.go:98` → `Render(md, nil, processImage)`

All eight call sites pass `md` (a `string`) directly, no `[]byte(...)` wrapping.

- [ ] **Step 2.2: Run render tests to confirm they fail to compile**

```bash
go test ./internal/render/ -run TestRenderNoteLink -v
```

Expected: compile error — `Render` still expects `[]byte` and `map[string]string`.

- [ ] **Step 2.3: Update render.go signature**

Replace `internal/render/render.go:38-100` with:

```go
// Render converts Markdown to HTML.
// noteIndex maps note IDs to their public paths (for link resolution).
// processImage is called for external image URLs (may be nil).
func Render(source string, noteIndex map[int]string, processImage ProcessImageFunc) ([]byte, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("monokailight"),
				highlighting.WithGuessLanguage(true),
				highlighting.WithCSSWriter(nil),
				highlighting.WithFormatOptions(
					chromahtml.WithClasses(true),
					chromahtml.WithPreWrapper(chromaPreWrapper{}),
				),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)

	sourceBytes := []byte(source)
	reader := text.NewReader(sourceBytes)
	doc := md.Parser().Parse(reader)

	// Walk AST and transform links and images.
	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch node := n.(type) {
		case *ast.Link:
			dest := string(node.Destination)
			if isNoteID(dest) && noteIndex != nil {
				id, err := strconv.Atoi(dest)
				if err == nil {
					if resolved, ok := noteIndex[id]; ok {
						node.Destination = []byte("/" + resolved)
					}
				}
			}

		case *ast.Image:
			src := string(node.Destination)
			if processImage != nil && isExternalURL(src) {
				localName, err := processImage(src)
				if err != nil {
					return ast.WalkContinue, nil
				}
				node.Destination = []byte(localName)
			}
		}

		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, sourceBytes, doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
```

Add `"strconv"` to the import block in `internal/render/render.go:3-17`. The block becomes:

```go
import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	highlighting "github.com/yuin/goldmark-highlighting/v2"
)
```

- [ ] **Step 2.4: Run render tests to verify green**

```bash
go test ./internal/render/ -v
```

Expected: all seven `TestRender*` tests pass, including the updated `TestRenderNoteLink` and `TestRenderNoteLinkUnresolved`.

- [ ] **Step 2.5: Commit**

```bash
git add internal/render/render.go internal/render/render_test.go
git commit -m "render: accept string source and int-keyed note index"
```

---

## Task 3: Swap Build's read path to note.Store

**Files:**
- Modify: `internal/build/build.go:16` (import)
- Modify: `internal/build/build.go:165-254` (Build function — signature, read path, page construction, render loop)
- Modify: `internal/build/build.go:462-505` (delete obsolete helpers)
- Modify: `cmd/npub/main.go:43` (Build call site)

This is the central task. It changes `Build`'s signature, replaces the scan+ReadFile loop, extracts the `buildNotePages` helper, and updates the render loop to use `int` IDs. The test file is migrated in Task 4.

- [ ] **Step 3.1: Update imports in internal/build/build.go**

Replace `internal/build/build.go:3-21` with:

```go
import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	texttemplate "text/template"

	"github.com/dreikanter/notes-cli/note"
	"github.com/dreikanter/npub/internal/config"
	"github.com/dreikanter/npub/internal/images"
	"github.com/dreikanter/npub/internal/page"
	"github.com/dreikanter/npub/internal/render"
)
```

(`time` stays — used for `PublishedAt`; `strconv` is new — used to stringify int IDs.)

- [ ] **Step 3.2: Replace the Build function body**

Replace `internal/build/build.go:165-396` (the entire `Build` function) with:

```go
// Build generates the static site from notes.
func Build(store note.Store, cfg config.Config, templateFS fs.FS, styleCSS []byte) error {
	// 0. Clean build directory.
	if err := cleanBuildDir(cfg.BuildPath); err != nil {
		return err
	}

	// 0b. Copy static files.
	if cfg.StaticPath != "" {
		if err := copyStaticFiles(cfg.StaticPath, cfg.BuildPath); err != nil {
			return fmt.Errorf("copying static files: %w", err)
		}
	}

	// 1. Read public notes from the store.
	entries, err := store.All(note.WithPublic(true))
	if err != nil {
		return fmt.Errorf("reading notes: %w", err)
	}

	// 2. Build note-page models and the ID → public-path index.
	notePages, noteIndex := buildNotePages(entries, cfg.SiteRootURL)

	// 3. Render Markdown for each note (now that noteIndex is complete).
	imgCache := images.NewCache(cfg.AssetsPath)
	for i, e := range entries {
		uid := notePages[i].UID
		processImage := func(src string) (string, error) {
			entry, err := imgCache.Get(src, uid)
			if err != nil {
				return "", err
			}
			notePages[i].Attachments = append(notePages[i].Attachments, page.Attachment{
				FileName: entry.FileName,
				PageUID:  entry.PageUID,
			})
			return entry.FileName, nil
		}
		rendered, err := render.Render(e.Body, noteIndex, processImage)
		if err != nil {
			return fmt.Errorf("rendering note %d: %w", e.ID, err)
		}
		notePages[i].Body = string(rendered)
	}

	// 4. Sort pages newest first (defensive — store already returns newest-first).
	page.SortNotePages(notePages)

	// 5. Load templates.
	tmpl, err := template.New("").ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return fmt.Errorf("parsing HTML templates: %w", err)
	}

	// Parse feed.xml with text/template (no HTML escaping).
	feedTmpl, err := texttemplate.New("").ParseFS(templateFS, "templates/feed.xml")
	if err != nil {
		return fmt.Errorf("parsing feed template: %w", err)
	}

	cfgData := configData{
		SiteName:     cfg.SiteName,
		SiteDomain:   cfg.SiteDomain(),
		SiteRootURL:  cfg.SiteRootURL,
		SiteRootPath: cfg.SiteRootPath(),
		AuthorName:   cfg.AuthorName,
		FeedURL:      cfg.FeedURL(),
		FeedPath:     cfg.FeedPath(),
		LicenseName:  cfg.LicenseName,
		LicenseURL:   cfg.LicenseURL,
		StyleCSS:     template.CSS(styleCSS),
		HighlightCSS: template.CSS(render.HighlightCSS()),
	}

	// 6. Write note pages and redirects.
	for _, np := range notePages {
		nvd := toNoteViewData(np)

		// Find related notes.
		relatedPages := page.RelatedTo(notePages, np)
		var relatedViews []noteViewData
		for _, rp := range relatedPages {
			relatedViews = append(relatedViews, toNoteViewData(rp))
		}

		inner := noteData{
			Note:    nvd,
			Related: relatedViews,
		}

		pd := pageData{
			Title:           np.Title,
			MetaDescription: np.Description,
			CanonicalPath:   np.CanonicalPath(),
		}

		if err := writeHTMLPage(tmpl, cfg.BuildPath, np.LocalPath(), "note.html", inner, cfgData, pd); err != nil {
			return fmt.Errorf("writing note page %s: %w", np.UID, err)
		}

		// Copy attachments.
		for _, att := range np.Attachments {
			destDir := filepath.Join(cfg.BuildPath, np.UID, np.Slug)
			if err := imgCache.CopyTo(images.Entry{FileName: att.FileName, PageUID: att.PageUID}, destDir); err != nil {
				return fmt.Errorf("copying attachment %s: %w", att.FileName, err)
			}
		}

		// Write redirect page.
		rp := page.RedirectPage{
			UID:        np.UID,
			RedirectTo: "/" + np.PublicPath(),
		}
		if err := writeRedirectPage(tmpl, cfg.BuildPath, rp); err != nil {
			return fmt.Errorf("writing redirect page %s: %w", np.UID, err)
		}
	}

	// 7. Write index page.
	allTags := page.AllTags(notePages)
	var noteViews []noteViewData
	for _, np := range notePages {
		noteViews = append(noteViews, toNoteViewData(np))
	}

	indexInner := indexData{
		Tags:      allTags,
		NotePages: noteViews,
	}
	indexPD := pageData{
		Title:           "",
		MetaDescription: cfg.SiteName,
	}
	if err := writeHTMLPage(tmpl, cfg.BuildPath, "index.html", "index.html", indexInner, cfgData, indexPD); err != nil {
		return fmt.Errorf("writing index page: %w", err)
	}

	// 8. Write tag pages.
	for _, tag := range allTags {
		tagged := page.TaggedPages(notePages, tag)
		var taggedViews []noteViewData
		for _, tp := range tagged {
			taggedViews = append(taggedViews, toNoteViewData(tp))
		}
		tagInner := tagData{
			Tags:        allTags,
			CurrentTag:  tag,
			TaggedPages: taggedViews,
		}
		tp := page.TagPage{Tag: tag}
		tagPD := pageData{
			Title:           tag,
			MetaDescription: fmt.Sprintf("Notes tagged with %s", tag),
			CanonicalPath:   tp.CanonicalPath(),
		}
		if err := writeHTMLPage(tmpl, cfg.BuildPath, tp.LocalPath(), "tag.html", tagInner, cfgData, tagPD); err != nil {
			return fmt.Errorf("writing tag page %s: %w", tag, err)
		}
	}

	// 9. Write feed.
	var feedNotes []feedNoteData
	for _, np := range notePages {
		feedNotes = append(feedNotes, feedNoteData{
			Title: np.Title,
			URL:   np.URL(),
			UID:   np.UID,
			Body:  np.Body,
		})
	}
	fd := feedData{
		Config:    cfgData,
		NotePages: feedNotes,
	}
	if err := writeFile(cfg.BuildPath, "feed.xml", func() ([]byte, error) {
		var buf strings.Builder
		if err := feedTmpl.ExecuteTemplate(&buf, "feed.xml", fd); err != nil {
			return nil, err
		}
		return []byte(buf.String()), nil
	}); err != nil {
		return fmt.Errorf("writing feed: %w", err)
	}

	return nil
}

// buildNotePages converts note.Entry values into page.NotePage models and
// builds the ID → public-path index used for link resolution.
func buildNotePages(entries []note.Entry, siteRootURL string) ([]page.NotePage, map[int]string) {
	pages := make([]page.NotePage, 0, len(entries))
	index := make(map[int]string, len(entries))
	for _, e := range entries {
		uid := e.Meta.CreatedAt.Format("20060102") + "_" + strconv.Itoa(e.ID)
		slug := chooseSlug(e)
		np := page.NotePage{
			UID:         uid,
			ShortUID:    strconv.Itoa(e.ID),
			Slug:        slug,
			Title:       titleOrUID(e.Meta.Title, uid),
			Description: e.Meta.Description,
			Tags:        e.Meta.Tags,
			SiteRootURL: siteRootURL,
			PublishedAt: e.Meta.CreatedAt,
		}
		index[e.ID] = np.PublicPath()
		pages = append(pages, np)
	}
	return pages, index
}

// chooseSlug returns the slug to use for e, falling back from Meta.Slug to
// slugified Meta.Title to the entry's ID.
func chooseSlug(e note.Entry) string {
	if s := slugify(e.Meta.Slug); s != "" {
		return s
	}
	if s := slugify(e.Meta.Title); s != "" {
		return s
	}
	return strconv.Itoa(e.ID)
}

// titleOrUID returns title when non-empty, otherwise uid as a fallback.
func titleOrUID(title, uid string) string {
	if title == "" {
		return uid
	}
	return title
}
```

**What's gone vs the old version:**

- Lines 186–204 (the `parsedNote` struct, the `notes, err := note.Scan(...)` call, the per-note `os.ReadFile` + `ParseFrontmatterFields` + `StripFrontmatter` loop) — replaced by `store.All(note.WithPublic(true))` and `buildNotePages`.
- Lines 206–233 (inline page construction with `trimQuotes`/`cleanTitle`/`parseDate`) — moved into `buildNotePages`.
- The old `for i, pn := range publicNotes` render loop now reads `entries[i].Body` directly (a `string`), matching render.Render's new signature. `e.ID` (an int) is used for error messages and the `noteIndex` key.

- [ ] **Step 3.3: Delete the obsolete helpers at the bottom of build.go**

Delete these helper functions from `internal/build/build.go` (they are no longer called from anywhere):

- `cleanTitle(title, fallback string) string` (around lines 472–479)
- `trimQuotes(s string) string` (around lines 481–485)
- `trimQuotesList(vals []string) []string` (around lines 487–494)
- `parseDate(dateStr string) time.Time` (around lines 496–505)

**Keep:** `slugify`, `toNoteViewData`, `writeHTMLPage`, `writeRedirectPage`, `writeFile`, `cleanBuildDir`, `copyStaticFiles`, the `nonAlphanumeric` regex — all still in use.

After deletion, `time` is still imported (used by `PublishedAt` in `noteViewData`).

- [ ] **Step 3.4: Update the call site in cmd/npub/main.go**

Replace `cmd/npub/main.go:43` (inside `buildCmd.RunE`) with:

```go
			log.Printf("building site from %s to %s", cfg.NotesPath, cfg.BuildPath)
			store := note.NewOSStore(cfg.NotesPath)
			if err := build.Build(store, cfg, npub.TemplateFS, npub.StyleCSS); err != nil {
				return fmt.Errorf("build failed: %w", err)
			}
```

Add `"github.com/dreikanter/notes-cli/note"` to the import block at `cmd/npub/main.go:3-16`:

```go
import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/dreikanter/notes-cli/note"
	npub "github.com/dreikanter/npub"
	"github.com/dreikanter/npub/internal/build"
	"github.com/dreikanter/npub/internal/config"
	"github.com/spf13/cobra"
)
```

- [ ] **Step 3.5: Verify build compiles**

```bash
go build ./...
```

Expected: no errors. If there are errors, they almost certainly concern `internal/build/build_test.go` (next task) — that test file has not been migrated yet and will fail to compile against the new `Build` signature.

If `go build ./...` succeeds (tests are compiled by `go test`, not `go build`), move on. If `go test ./...` is run here it will fail at `internal/build/build_test.go` — expected, fixed in Task 4.

- [ ] **Step 3.6: Run the render and config and page tests to make sure we didn't break anything adjacent**

```bash
go test ./internal/render/ ./internal/config/ ./internal/page/ ./internal/images/ -v
```

Expected: all pass. Build tests are not run here because they are in the middle of migration.

- [ ] **Step 3.7: Commit**

```bash
git add internal/build/build.go cmd/npub/main.go
git commit -m "build: read notes via note.Store; extract buildNotePages helper"
```

---

## Task 4: Migrate build tests to note.MemStore

**Files:**
- Modify: `internal/build/build_test.go` (entire `TestBuildPublicNote`, `TestBuildSkipsPrivateNote`, `TestBuildNoteLinkResolution`; helper `testConfig`; remove `writeTestNote`)

The three tests currently write markdown files to `t.TempDir()` and pass the directory to `Build`. After migration they construct an in-memory `note.MemStore`, call `store.Put(entry)` for each note, and pass the store to `Build`.

- [ ] **Step 4.1: Update imports and helpers**

Replace `internal/build/build_test.go:1-10` with:

```go
package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dreikanter/notes-cli/note"
	"github.com/dreikanter/npub/internal/config"
)
```

Delete the `writeTestNote` helper at `internal/build/build_test.go:99-108`. It is no longer needed.

Change `testConfig` at `internal/build/build_test.go:110-120` to drop the `notesPath` parameter:

```go
func testConfig(t *testing.T, buildPath, assetsPath string) config.Config {
	t.Helper()
	return config.Config{
		AssetsPath:  assetsPath,
		BuildPath:   buildPath,
		SiteRootURL: "https://example.com",
		SiteName:    "Test Site",
		AuthorName:  "Test Author",
	}
}
```

(`NotesPath` is gone from the config literal — `Build` no longer reads it directly.)

- [ ] **Step 4.2: Migrate TestBuildPublicNote**

Replace `internal/build/build_test.go:122-211` with:

```go
func TestBuildPublicNote(t *testing.T) {
	buildDir := t.TempDir()
	assetsDir := t.TempDir()

	store := note.NewMemStore()
	if _, err := store.Put(note.Entry{
		ID: 3961,
		Meta: note.Meta{
			Title:     "My Test Note",
			Slug:      "my-test-note",
			Tags:      []string{"golang", "testing"},
			Public:    true,
			CreatedAt: time.Date(2023, 1, 30, 0, 0, 0, 0, time.UTC),
		},
		Body: "Hello **world**.\n",
	}); err != nil {
		t.Fatalf("store.Put: %v", err)
	}

	cfg := testConfig(t, buildDir, assetsDir)
	templateFS := os.DirFS("../../")
	styleCSS := []byte("/* test */")
	if err := Build(store, cfg, templateFS, styleCSS); err != nil {
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
```

- [ ] **Step 4.3: Migrate TestBuildSkipsPrivateNote**

Replace `internal/build/build_test.go:213-246` with:

```go
func TestBuildSkipsPrivateNote(t *testing.T) {
	buildDir := t.TempDir()
	assetsDir := t.TempDir()

	store := note.NewMemStore()
	if _, err := store.Put(note.Entry{
		ID: 3961,
		Meta: note.Meta{
			Title:     "Private Note",
			Tags:      []string{"secret"},
			Public:    false,
			CreatedAt: time.Date(2023, 1, 30, 0, 0, 0, 0, time.UTC),
		},
		Body: "This is private.\n",
	}); err != nil {
		t.Fatalf("store.Put: %v", err)
	}

	cfg := testConfig(t, buildDir, assetsDir)
	templateFS := os.DirFS("../../")
	styleCSS := []byte("/* test */")
	if err := Build(store, cfg, templateFS, styleCSS); err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	notePath := filepath.Join(buildDir, "20230130_3961", "private-note", "index.html")
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
```

- [ ] **Step 4.4: Migrate TestBuildNoteLinkResolution**

Replace `internal/build/build_test.go:248-288` with:

```go
func TestBuildNoteLinkResolution(t *testing.T) {
	buildDir := t.TempDir()
	assetsDir := t.TempDir()

	store := note.NewMemStore()
	if _, err := store.Put(note.Entry{
		ID: 3961,
		Meta: note.Meta{
			Title:     "First Note",
			Slug:      "first-note",
			Tags:      []string{"test"},
			Public:    true,
			CreatedAt: time.Date(2023, 1, 30, 0, 0, 0, 0, time.UTC),
		},
		Body: "See [second note](3962).\n",
	}); err != nil {
		t.Fatalf("store.Put (first): %v", err)
	}
	if _, err := store.Put(note.Entry{
		ID: 3962,
		Meta: note.Meta{
			Title:     "Second Note",
			Slug:      "second-note",
			Tags:      []string{"test"},
			Public:    true,
			CreatedAt: time.Date(2023, 1, 31, 0, 0, 0, 0, time.UTC),
		},
		Body: "Hello from second note.\n",
	}); err != nil {
		t.Fatalf("store.Put (second): %v", err)
	}

	cfg := testConfig(t, buildDir, assetsDir)
	templateFS := os.DirFS("../../")
	styleCSS := []byte("/* test */")
	if err := Build(store, cfg, templateFS, styleCSS); err != nil {
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
```

- [ ] **Step 4.5: Run the build tests**

```bash
go test ./internal/build/ -v
```

Expected: all five tests pass — `TestCleanBuildDir`, `TestCleanBuildDirNonExistent`, `TestCleanBuildDirRejectsRoot`, `TestCopyStaticFiles`, `TestCleanBuildDirRejectsHome`, `TestBuildPublicNote`, `TestBuildSkipsPrivateNote`, `TestBuildNoteLinkResolution`.

**Watch for:** `TestBuildSkipsPrivateNote` expects a path containing `"private-note"` (slugified from `"Private Note"`). The old test used `"private"` because the filename slug was `"private"`. Adjust if your implementation produces a different slug — the slug comes from `chooseSlug` → `slugify(Meta.Title)` → `"private-note"`.

- [ ] **Step 4.6: Run the full suite**

```bash
go test ./...
```

Expected: everything green.

- [ ] **Step 4.7: Commit**

```bash
git add internal/build/build_test.go
git commit -m "build: migrate tests to note.MemStore"
```

---

## Task 5: Verification

**Files:** none modified; manual and programmatic checks only.

- [ ] **Step 5.1: Full build**

```bash
go build ./...
go test ./...
```

Expected: both succeed with no output on build; all tests pass.

- [ ] **Step 5.2: Vet**

```bash
go vet ./...
```

Expected: no output. Any findings must be fixed before continuing.

- [ ] **Step 5.3: Confirm obsolete helpers are fully gone**

```bash
grep -n -E "parseDate|trimQuotes|trimQuotesList|cleanTitle|parsedNote|note\.Scan|note\.ParseFrontmatterFields|note\.StripFrontmatter" internal/build/build.go internal/build/build_test.go cmd/npub/main.go
```

Expected: no matches. Any match means a leftover reference — remove it before finishing.

- [ ] **Step 5.4: Confirm the Store-path replacements are in place**

```bash
grep -n -E "store\.All|note\.NewOSStore|note\.NewMemStore|note\.WithPublic|note\.Entry|note\.Store" internal/build/build.go internal/build/build_test.go cmd/npub/main.go
```

Expected at least:

- `build.go`: one `store.All(note.WithPublic(true))`, function signature mentions `note.Store`, `note.Entry` appears in `buildNotePages`.
- `build_test.go`: three `note.NewMemStore()` calls, several `note.Entry{...}` literals.
- `main.go`: one `note.NewOSStore(cfg.NotesPath)`.

- [ ] **Step 5.5: Manual smoke test against a real notes directory (if available)**

If the operator has a populated notes directory locally, run:

```bash
go run ./cmd/npub build --notes ~/notes --out /tmp/npub-smoke --url https://example.com --site-name "Smoke" --author "Smoke"
```

Expected: `build complete` logged, `/tmp/npub-smoke/` contains `index.html`, `feed.xml`, at least one `YYYYMMDD_ID/slug/index.html`, and `tags/…` directories.

Compare the diff of a pre-swap build (from `main`) against the post-swap build of the same notes tree — there should be no meaningful content diff except possibly:

- Tags may now include body `#hashtag` tokens (OSStore merges them into `Meta.Tags`). This is the known behavior change called out in the spec — document it in the PR description if any notes rely on it.

If the operator does not have a populated notes directory, skip this step; the MemStore tests already exercise the full render pipeline.

- [ ] **Step 5.6: Commit anything left (there shouldn't be)**

```bash
git status
```

Expected: `nothing to commit, working tree clean`. If anything is uncommitted, it is a missed edit from a prior task — stage and commit it with a descriptive message.

---

## Self-review summary

**Spec coverage:**

| Spec section | Plan task |
|---|---|
| Dependency bump to v0.3.20 | Task 1 |
| Read path via `store.All(note.WithPublic(true))` | Task 3 |
| `buildNotePages` + `chooseSlug` + `titleOrUID` extraction | Task 3 |
| Delete `parseDate`, `trimQuotes`, `trimQuotesList`, `cleanTitle`, `parsedNote` | Task 3 + Task 5.3 verification |
| `Build(store note.Store, cfg, ...)` signature | Task 3 |
| CLI wiring via `note.NewOSStore` | Task 3 |
| `render.Render` signature: `string` body + `map[int]string` index | Task 2 |
| Build tests migrate to `note.MemStore` | Task 4 |
| Sort order still explicit in `Build` (defensive) | Task 3 (step 3.2, "4. Sort pages newest first") |
| Error wrapper message changed to `"reading notes: %w"` | Task 3 (step 3.2) |
| Hashtag tag-merging behavior change documented | Task 5.5 |

All spec items map to tasks. No placeholders, all code blocks complete, all file paths absolute or repo-relative.
