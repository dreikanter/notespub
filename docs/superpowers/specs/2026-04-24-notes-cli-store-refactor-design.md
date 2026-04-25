# npub: adopt notes-cli Store interface

**Date:** 2026-04-24
**Status:** Approved — ready for implementation planning

## Goal

Replace npub's direct use of the `note.Scan` / `note.ParseFrontmatterFields` / `note.StripFrontmatter` free functions from notes-cli v0.1.66 with the new `note.Store` interface (available in notes-cli v0.3.20 and later). The Store is passed into `Build` as an explicit dependency, and build tests migrate to `note.MemStore` so they no longer need filesystem fixtures.

## Non-goals

- No changes to templates, routing, rendered output, image cache, or CLI flags.
- No new features.
- No refactoring of `render`, `page`, `images`, or `config` packages beyond what the Store swap directly requires.
- No new abstraction layer inside npub. The notes-cli `Store` interface is used directly; npub does not define its own wrapper interface.

## Dependency bump

- `go.mod`: `github.com/dreikanter/notes-cli v0.1.66` → `v0.3.20`.
- `go mod tidy` pulls in `golang.org/x/sync` as a new transitive dependency (used by `OSStore.readConcurrent`). `gopkg.in/yaml.v3` is already present.

## Architecture

```
cmd/npub/main.go
   │  store := note.NewOSStore(cfg.NotesPath)
   │  build.Build(store, cfg, templateFS, styleCSS)
   ▼
internal/build/Build(store note.Store, cfg, templateFS, styleCSS)
   │
   ├─ cleanBuildDir / copyStaticFiles            (unchanged)
   │
   ├─ entries := store.All(note.WithPublic(true))
   │
   ├─ pages, noteIndex := buildNotePages(entries, cfg.SiteRootURL)   ← new helper
   │
   ├─ render bodies + attachments into pages     (adapted from step 4)
   │
   └─ sort / write note pages / index / tag pages / feed / redirects (unchanged)
```

The `Store` interface is passed in by the caller. `Build` never touches `cfg.NotesPath` directly anymore; only the caller-constructed store does. Tests substitute `note.MemStore`.

## Read path — the Store seam

**Before** (`internal/build/build.go:180–204`):

```go
notes, err := note.Scan(cfg.NotesPath)
// ...
for _, n := range notes {
    data, err := os.ReadFile(filepath.Join(cfg.NotesPath, n.RelPath))
    // ...
    fm := note.ParseFrontmatterFields(data)
    if !fm.Public { continue }
    body := note.StripFrontmatter(data)
    publicNotes = append(publicNotes, parsedNote{note: n, fm: fm, body: body})
}
```

**After:**

```go
entries, err := store.All(note.WithPublic(true))
if err != nil {
    return fmt.Errorf("reading notes: %w", err)
}
```

The manual `os.ReadFile` loop and the intermediate `parsedNote` struct disappear. Each `note.Entry` carries `ID int`, `Meta` (with `Title`, `Slug`, `Tags`, `Description`, `Public`, `CreatedAt`, `Type`, `Extra`), and `Body string` already populated. `store.All` returns them newest-first by `Meta.CreatedAt`, which matches the existing `page.SortNotePages` step (which can then be reassessed — see "Sort" below).

## UID / date / ID handling

Current `build.go` stringifies dates and IDs from parsed filename parts:

```go
uid := pn.note.Date + "_" + pn.note.ID          // note.Note fields are both strings
// ...
PublishedAt: parseDate(pn.note.Date),           // custom YYYYMMDD parser
```

After the swap:

- `entry.ID` is `int`. `ShortUID` becomes `strconv.Itoa(entry.ID)`.
- `entry.Meta.CreatedAt` is a fully-parsed `time.Time` from the YAML `date` field.
- `UID` becomes `entry.Meta.CreatedAt.Format("20060102") + "_" + strconv.Itoa(entry.ID)`.
- `PublishedAt` is `entry.Meta.CreatedAt` directly.

Deleted from `build.go`:

- `parseDate()` (custom `"20060102"` parser)
- `trimQuotes()` and `trimQuotesList()` (worked around the v0.1.66 hand-rolled YAML parser; v0.3.20 uses `yaml.v3` and returns unquoted strings)
- `cleanTitle()` (replaced by `titleOrUID`; the quote-stripping it did is no longer needed)
- `parsedNote` struct

## Page-model extraction

New unexported helper in `internal/build/build.go`:

```go
func buildNotePages(entries []note.Entry, siteRootURL string) ([]page.NotePage, map[int]string) {
    pages := make([]page.NotePage, 0, len(entries))
    index := make(map[int]string, len(entries))
    for _, e := range entries {
        slug := chooseSlug(e)   // Meta.Slug → slugified Meta.Title → strconv.Itoa(e.ID)
        uid := e.Meta.CreatedAt.Format("20060102") + "_" + strconv.Itoa(e.ID)
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
```

Two small unexported helpers encapsulate fallback chains currently inline in `Build`:

- `chooseSlug(e note.Entry) string` — returns `slugify(e.Meta.Slug)`, falling back to `slugify(e.Meta.Title)`, falling back to `strconv.Itoa(e.ID)`.
- `titleOrUID(title, uid string) string` — returns `title` if non-empty, otherwise `uid`. Replaces the current `cleanTitle` helper (the quote-stripping it did is no longer needed; `yaml.v3` returns unquoted strings).

Rendering (`render.Render` loop) stays in `Build` because it needs `imgCache` and mutates `pages[i].Body` / `pages[i].Attachments` — pulling it out adds no clarity for this refactor.

## Render.Render signature change

`internal/render/render.go`:

- `body []byte` → `body string` (matches `Entry.Body`).
- `noteIndex map[string]string` → `noteIndex map[int]string` (IDs are `int` now).

Inside `render.Render`, markdown link targets like `[see](3962)` are parsed strings; convert with `strconv.Atoi` at lookup. Non-numeric targets fall through to unchanged behavior. Existing `render_test.go` cases need their `noteIndex` map keys updated from `"3962"` → `3962`.

Image processing callback stays in npub — it is a publishing concern, not a storage concern, and does not belong upstream in notes-cli.

## Sort order

`store.All` returns entries newest-first (for OSStore: by filename `date DESC, id DESC`; for MemStore: by `Meta.CreatedAt` DESC). npub's current `page.SortNotePages(notePages)` after rendering sorts by `PublishedAt` DESC — same order, so the explicit sort becomes redundant. **Keep the explicit sort** anyway: it is cheap, defensive, and makes the order guarantee local to `Build` rather than relying on the Store implementation's ordering.

## CLI wiring

`cmd/npub/main.go`: construct the store before calling `Build`:

```go
store := note.NewOSStore(cfg.NotesPath)
if err := build.Build(store, cfg, templateFS, styleCSS); err != nil {
    // ...
}
```

`cfg.NotesPath` remains in the config struct (loaded from YAML and CLI flags) — only its direct use inside `Build` goes away.

## Tests

### `internal/build/build_test.go`

Migrate the three note-building tests to `note.MemStore`:

- `TestBuildPublicNote`
- `TestBuildSkipsPrivateNote`
- `TestBuildNoteLinkResolution`

Each test:

```go
store := note.NewMemStore()
_, err := store.Put(note.Entry{
    ID: 3961,
    Meta: note.Meta{
        Title:     "My Test Note",
        Slug:      "my-test-note",
        Tags:      []string{"golang", "testing"},
        Public:    true,
        CreatedAt: time.Date(2023, 1, 30, 0, 0, 0, 0, time.UTC),
    },
    Body: "Hello **world**.\n",
})
// ...
Build(store, cfg, templateFS, styleCSS)
```

`writeTestNote` helper, `notesDir`, and all markdown-with-YAML literal strings are removed from these tests. `testConfig` no longer needs a `notesPath` argument.

`TestCleanBuildDir*` and `TestCopyStaticFiles` stay unchanged — they test filesystem helpers, not the Store seam.

### Other tests

`internal/config/`, `internal/page/`, `internal/render/`, `internal/images/` tests: only `render_test.go` changes (see "Render.Render signature change" above). The rest are untouched.

## Error handling

No new error paths. The existing wrapper `fmt.Errorf("scanning notes: %w", err)` is renamed to `"reading notes: %w"` to match the new semantics (it's a Store call, not a filesystem scan).

`store.All` errors surface via `%w`. `store.Put` / `store.Get` / `store.Delete` are not used by npub — it is read-only.

## Migration order (single PR)

1. Bump `go.mod` to `notes-cli v0.3.20`; run `go mod tidy`.
2. Update `render.Render` signature (`[]byte` → `string`, `map[string]string` → `map[int]string`); update `render_test.go` map keys.
3. In `internal/build/build.go`:
   - Replace the scan-and-parse loop with `store.All(note.WithPublic(true))`.
   - Extract `buildNotePages` + `chooseSlug` + `titleOrUID` helpers.
   - Delete `parseDate`, `trimQuotes`, `trimQuotesList`, `parsedNote`.
4. Change `Build` signature to `Build(store note.Store, cfg config.Config, templateFS fs.FS, styleCSS []byte) error`.
5. Update `cmd/npub/main.go` to construct `note.NewOSStore(cfg.NotesPath)` and pass it in.
6. Migrate the three Build tests to `MemStore`.
7. `go test ./...` + manual `npub build` against a real notes directory as a smoke test.

## Risks

**Frontmatter parity.** notes-cli v0.1.66 used a hand-rolled parser; v0.3.20's `OSStore` uses `yaml.v3`. Any edge case the hand-rolled parser tolerated (unusual whitespace, missing delimiters, etc.) may now error or parse differently. Mitigation: manual build against a real notes directory after the swap, diff the output tree against a pre-swap build.

**`Entry.Body` is `string`, render path currently takes `[]byte`.** Trivial cast at the `render.Render` boundary; no real risk.

**Hashtag tag merging.** `OSStore` merges frontmatter `tags` with body `#hashtag` tokens into `Meta.Tags`. npub's current behavior only reads frontmatter tags. This is a behavior change for any note that uses body hashtags. Mitigation: note in PR description; verify on a real build whether existing notes depend on this being absent. If it is a problem, add an explicit filter in `buildNotePages` that keeps only frontmatter-source tags — but this requires a notes-cli API to distinguish them, which does not currently exist, so accept the behavior change for now and reconsider if it causes real-world issues.

## Out of scope — revisit later

- Further decomposition of `Build` (separate rendering and writing units).
- A npub-local interface wrapping `note.Store` for finer-grained testability.
- Parallel rendering (OSStore is already parallel on reads; rendering remains sequential).
- Moving image processing into notes-cli (rejected: publishing concern, not storage concern).
