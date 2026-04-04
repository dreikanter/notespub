# notespub

A static site builder for Markdown notes. Reads notes from a local directory, renders them to HTML with syntax highlighting, and outputs a complete static site with tag pages and an Atom feed.

## Prerequisites

- Go 1.25+
- Node.js (for Tailwind CSS)

## Build

Install dependencies and build:

```sh
npm install
make build
```

`npm install` is only needed once (or when dependencies change). `make build` compiles the Tailwind CSS stylesheet and builds the `notespub` binary.

## Configuration

Create a `notespub.yml` file:

```yaml
notes_path: "~/notes"
build_path: "dist"
site_root_url: "https://example.com"
site_name: "My Notes"
author_name: "Ada Lovelace"
```

All values can be overridden with CLI flags:

| Config option | CLI flag | Default |
|---|---|---|
| `notes_path` | `--notes` | |
| `assets_path` | `--assets` | `<notes_path>/images` |
| `build_path` | `--out` | |
| `static_path` | `--static` | `<notes_path>/static` |
| `site_root_url` | `--url` | |
| `site_name` | `--site-name` | |
| `author_name` | `--author` | |
| `license_name` | `--license-name` | CC BY 4.0 |
| `license_url` | `--license-url` | https://creativecommons.org/licenses/by/4.0/ |

Priority: CLI flags > YAML config.

The config file defaults to `notespub.yml` in the current directory. Override with `--config` or `NOTESPUB_CONFIG` env var.

### Image assets

Downloaded images are cached in an assets directory, organized by note UID. By default this is the `images` subdirectory of `notes_path`. Override with `assets_path` in the config or `--assets` flag.

### Static files

Files in the `static` subdirectory of `notes_path` are copied as-is to the build output root. Use this for `CNAME`, `robots.txt`, `favicon.ico`, etc. Override with `static_path` in the config or `--static` flag.

## Usage

Build the site:

```sh
notespub build
```

Serve locally:

```sh
notespub serve
```

The `serve` command starts a local HTTP server on port 4000 (override with `--port`), serving from `dist` (override with `--dir`).

## Notes format

Notes are Markdown files managed by [notescli](https://github.com/dreikanter/notescli). A note becomes part of the published site when its frontmatter includes `public: true`.
