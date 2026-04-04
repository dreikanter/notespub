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

All values can be overridden with environment variables or CLI flags:

| YAML key | Env var | CLI flag |
|---|---|---|
| `notes_path` | `NOTES_PATH` | `--notes-path` |
| `assets_path` | `NOTESPUB_ASSETS_PATH` | `--assets-path` |
| `build_path` | `NOTESPUB_BUILD_PATH` | `--out` |
| `site_root_url` | `NOTESPUB_SITE_ROOT_URL` | `--url` |
| `site_name` | `NOTESPUB_SITE_NAME` | `--site-name` |
| `author_name` | `NOTESPUB_AUTHOR_NAME` | `--author` |

Priority: CLI flags > environment variables > YAML file.

The config file path defaults to `notespub.yml` in the current directory. Override it with `--config` or the `NOTESPUB_CONFIG` env var.

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

## Project structure

```
cmd/notespub/     CLI entry point (Cobra commands)
internal/
  build/          Site generator
  config/         YAML + env + flag config loader
  images/         Image cache with redirect support
  page/           Page types (note, tag, redirect)
  render/         Markdown rendering (Goldmark + Chroma)
templates/        HTML templates and Atom feed
stylesheets/      Tailwind CSS source
```
