# notespub

A static site builder for Markdown notes. Reads notes from a local directory, renders them to HTML with syntax highlighting, and outputs a complete static site with tag pages and an Atom feed.

## Prerequisites

- Go 1.25+
- Node.js (for Tailwind CSS)

## Build

```sh
npm install
make build
```

This compiles the Tailwind CSS stylesheet and builds the `notespub` binary.

## Configuration

Create a `notespub.yml` file:

```yaml
notes_path: "~/Notes"
build_path: "./dist"
assets_path: "~/NotesImages"
site_root_url: "https://example.com"
site_name: "My Notes"
author_name: "Your Name"
```

All values can be overridden with CLI flags:

| YAML key | CLI flag |
|---|---|
| `notes_path` | `--notes-path` |
| `assets_path` | `--assets-path` |
| `build_path` | `--out` |
| `site_root_url` | `--url` |
| `site_name` | `--site-name` |
| `author_name` | `--author` |

Priority: CLI flags > YAML file.

The config file path defaults to `notespub.yml` in the current directory. Override it with `--config` or `NOTESPUB_CONFIG` env var.

## Usage

Build the site:

```sh
./notespub build
```

Serve locally:

```sh
./notespub serve
```

The `serve` command starts a local HTTP server on port 4000 (override with `--port`), serving from `./dist` (override with `--dir`).

## Notes format

Notes are Markdown files managed by [notescli](https://github.com/dreikanter/notescli). A note becomes part of the published site when its frontmatter includes `public: true`.