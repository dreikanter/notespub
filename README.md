# npub

A static site builder for Markdown notes. Reads notes from a local directory, renders them to HTML with syntax highlighting, and outputs a complete static site with tag pages and an Atom feed.

## Prerequisites

- Go 1.25+
- Node.js (for Tailwind CSS)

## Install

```sh
go install github.com/dreikanter/npub/cmd/npub@latest
```

## Build

Install dependencies and build:

```sh
npm install
make build
```

`npm install` is only needed once (or when dependencies change). `make build` compiles the Tailwind CSS stylesheet and builds the `npub` binary.

## Configuration

Create a `npub.yml` file:

```yaml
notes_path: "~/notes"
site_root_url: "https://example.com"
site_name: "My Notes"
author_name: "Ada Lovelace"
deploy_repo: "git@github.com:user/site.git"
```

All values can be overridden with CLI flags:

| Config option | CLI flag | Default | Required |
|---|---|---|---|
| `notes_path` | `--path` | `$NOTES_PATH` | |
| `assets_path` | `--assets` | `<notes_path>/images` | |
| `static_path` | `--static` | `<notes_path>/static` | |
| `site_root_url` | `--url` | | Yes |
| `site_name` | `--site-name` | | Yes |
| `author_name` | `--author` | | Yes |
| `license_name` | `--license-name` | CC BY 4.0 | |
| `license_url` | `--license-url` | https://creativecommons.org/licenses/by/4.0/ | |
| `intro` | | | |
| `deploy_repo` | | | For `deploy` |
| `cache_path` | | `~/.cache/npub/<repo>` | |

The build output directory is not a config option. `npub build` writes to
`<cache_path>/build` (where `cache_path` defaults to `~/.cache/npub/<repo>`).
Pass `--out <dir>` to override. Either `deploy_repo` or `--out` must be set;
there is no implicit `./dist`. `build` never talks to the deploy_repo
remote — all git operations happen in `deploy`.

Priority: CLI flags > YAML config.

Config file discovery order:

1. `--config` flag
2. `npub.yml` inside `$NOTES_PATH` (or `--path` value) if it exists
3. `npub.yml` in the current directory

`NOTES_PATH` plays two roles: it is the default source for `notes_path` when
neither `--path` nor the YAML sets it, and it acts as a hint location for
config discovery (step 2).

Run `npub init [dir]` to create a commented `npub.yml` sample in a directory. If `dir` is omitted, the current directory is used.

The optional `intro` field renders as a paragraph above the posts list on the index page. Leave it empty or unset to omit.

### Image assets

Downloaded images are cached in an assets directory, organized by note UID. By default this is the `images` subdirectory of `notes_path`. Override with `assets_path` in the config or `--assets` flag.

### Static files

Files in the `static` subdirectory of `notes_path` are copied as-is to the build output root. Use this for `CNAME`, `robots.txt`, `favicon.ico`, etc. Override with `static_path` in the config or `--static` flag.

## Usage

Create a configuration file:

```sh
npub init
# or create a new project directory
npub init ./my-notes-site
```

Build the site:

```sh
npub build
```

Inspect the resolved configuration:

```sh
npub config
```

The `config` command prints the absolute path of the config file npub will use along with every option after merging YAML, CLI flags, environment variables, and defaults. It accepts the same overrides as `build`, so `npub config --path ~/notes --url https://example.com` previews how a build would see its configuration.

Serve locally:

```sh
npub serve
```

The `serve` command starts a local HTTP server on `localhost:4000` (override with `--host` and `--port`). It serves `<cache_path>/build` (where `cache_path` defaults to `~/.cache/npub/<repo>`). Pass `--dir <path>` to point it at a different directory; either `deploy_repo` or `--dir` must be set.

Deploy to a git remote:

```sh
npub build
npub deploy
```

`npub build` writes the site to `~/.cache/npub/<repo>/build`. It never contacts the remote — everything is offline.

`npub deploy` keeps a bare clone of `deploy_repo` at `~/.cache/npub/<repo>/git` and uses `~/.cache/npub/<repo>/build` as a temporary work-tree (via git's `--git-dir` and `--work-tree` options) when committing. There is no second copy of the site on disk: deploy fetches, points git at the build directory, and runs `add -A` + commit + push. Stale files are removed and changed files updated by the same `add -A` pass. Use `--dry-run` to commit locally without pushing.

## Notes format

Notes are Markdown files managed by [notesctl](https://github.com/dreikanter/notesctl). A note becomes part of the published site when its frontmatter includes `public: true`.

## Versioning

`CHANGELOG.md` is the source of truth for the version. On PR merge, GitHub
Actions (`.github/workflows/tag.yml`) reads the topmost `## [X.Y.Z]` heading
from `CHANGELOG.md` and pushes `vX.Y.Z` as a git tag. Bump major/minor/patch
by writing the desired heading in the PR.
