# npub Design

Date: 2026-03-29

## Overview

`npub` is a standalone Go binary that builds a static site from a local notes store and serves it locally for preview. It replaces the existing Ruby-based site builder with a single portable binary installable via `go install`.

```
go install github.com/dreikanter/npub@latest
```

## Goals

- Single self-contained binary — no Ruby, Node, or Caddy required at runtime
- Survives a year without use on any machine with Go installed
- Easy to run on CI (GitHub Actions)
- Prefer building locally and pushing HTML to GitHub Pages
- No knowledge duplication with `notescli`

## Prerequisite

Requires `notescli` frontmatter extension (see `notescli` spec `2026-03-29-frontmatter-extension-design.md`) to be implemented and released first. `npub` imports `github.com/dreikanter/notescli/note` and uses `note.ParseFrontmatterFields` for all frontmatter access — all required fields (`public`, `slug`, `title`, `tags`, `description`) are covered by `FrontmatterFields`.

## Commands

```
npub build [--config npub.yml] [--out ./dist]
npub serve [--dir ./dist] [--port 4000]
```

### `build`

1. Load configuration
2. Scan notes store via `note.Scan`
3. For each public note: parse frontmatter, render Markdown, process images
4. Generate all pages (index, notes, redirects, tags, feed)
5. Render templates and write to `build_path`

### `serve`

Serves an already-built `build_path` over `localhost` using `net/http`. Does not rebuild — run `build` first. No Caddy required.

## Configuration

Three-layer precedence: **flags > env vars > `npub.yml`**

| Key | Flag | Env var | Default |
|---|---|---|---|
| `notes_path` | `--notes-path` | `NOTES_PATH` | `~/notes` |
| `assets_path` | `--assets-path` | `NPUB_ASSETS_PATH` | `~/npub-assets` |
| `build_path` | `--out` | `NPUB_BUILD_PATH` | `./dist` |
| `site_root_url` | `--url` | `NPUB_SITE_ROOT_URL` | — (required) |
| `site_name` | `--site-name` | `NPUB_SITE_NAME` | — (required) |
| `author_name` | `--author` | `NPUB_AUTHOR_NAME` | — (required) |

`NOTES_PATH` is shared with `notescli` — set once, works for both tools.

Config file location: `./npub.yml` by default, overridable via `--config` or `NPUB_CONFIG`.

## Project Structure

```
npub/
  cmd/npub/
    main.go
  internal/
    build/        — site builder: orchestrates pages, renders, writes dist/
    serve/        — local HTTP server (net/http)
    render/       — Markdown rendering (goldmark + extensions)
    images/       — external image downloading + disk cache
    config/       — config loading (flags > env > YAML)
    page/         — page types: note, index, tag, redirect, feed
  templates/      — html/template files (go:embed)
  stylesheets/    — Tailwind CSS source
  style.css       — compiled CSS output (go:embed)
  Makefile
  go.mod
```

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/dreikanter/notescli/note` | Note scanning, filename parsing, frontmatter via `ParseFrontmatterFields` |
| `github.com/spf13/cobra` | CLI (consistent with notescli) |
| `github.com/yuin/goldmark` | Markdown rendering |
| `github.com/alecthomas/chroma` | Syntax highlighting |
| `gopkg.in/yaml.v3` | Config file parsing |

## Embedded Assets

Both `style.css` and all `templates/*.html` are embedded into the binary via `go:embed`. The binary is fully self-contained — no supporting files needed at runtime.

## Data Flow

```
NOTES_PATH/**/*.md
       ↓
  notescli/note       scan store, parse filenames, parse frontmatter
       ↓
  internal/render     goldmark: parse markdown → AST transformers:
                        - image nodes: download external URLs via internal/images, rewrite src
                        - link nodes: resolve numeric note IDs → relative paths
                      → render HTML
       ↓
  internal/page       construct page objects (note, index, tag, redirect, feed)
       ↓
  internal/build      html/template rendering → write to dist/
```

## Pages Generated

| Page | Output path | Notes |
|---|---|---|
| Index | `dist/index.html` | All public notes, sorted by date desc |
| RSS feed | `dist/feed.xml` | |
| Note | `dist/{date}_{id}/{slug}/index.html` | |
| UID redirect | `dist/{date}_{id}/index.html` | HTML redirect to slug URL |
| Tag | `dist/tags/{tag}/index.html` | |

Only notes with `public: true` frontmatter are included.

## Image Processing

During `build`, the `images` package:

1. Scans rendered Markdown for external image URLs (`![alt](https://...)`)
2. Checks disk cache (`assets_path/index.json`, keyed by URL)
3. If not cached: resolves URL (CleanShot URLs require following `url+` redirect), downloads, saves to `assets_path/`, updates index
4. Rewrites URL to local relative path in HTML output
5. Copies local image file to `dist/` alongside note HTML

Cache persists between builds — subsequent builds are fast and work offline.

## Note ID Link Resolution

A Goldmark AST transformer resolves short note references:

```markdown
See [this note](8823)
```

Transformer checks if link destination is a numeric note ID, looks it up in the site's note index, rewrites to the note's relative path. Unresolved IDs log a warning and are left as-is.

## Templates

Six `html/template` files, embedded in binary:

- `layout.html` — base layout (nav, footer)
- `note.html` — note body, tags, related notes
- `index.html` — note listing
- `tag.html` — notes filtered by tag
- `redirect.html` — UID redirect page
- `feed.xml` — RSS feed

Template variables mirror the Ruby originals: `Config`, `Site`, `Page`.

## CSS

Tailwind CSS source lives in `stylesheets/`. Compiled `style.css` is committed to the repo and embedded in the binary.

`make build` runs: Tailwind compilation → `go build` (embeds fresh CSS).

End users running `go install` get the pre-compiled CSS baked in — no Node required.

## Makefile

```makefile
build:      ## Compile CSS then build binary
    npx tailwindcss -i stylesheets/main.css -o style.css --minify
    go build ./cmd/npub

dev:        ## Watch mode: recompile on changes
    npx tailwindcss -i stylesheets/main.css -o style.css --watch &
    go build ./cmd/npub

test:
    go test ./...

lint:
    go tool golangci-lint run
```

## Out of Scope

- CI/GitHub Actions workflow (user prefers building locally and pushing HTML)
- Image format conversion
- Draft/scheduled notes
- Search
