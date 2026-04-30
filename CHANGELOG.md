# Changelog

## [0.2.6] - 2026-04-30

### Fixed

- Drop dead `cfg.BuildPath != ""` guard in `serveCmd`'s config-resolution switch. `config.Load` always defaults `cfg.BuildPath` to `./dist` when the YAML omits it, so the guard never failed. ([#66])

[#66]: https://github.com/dreikanter/npub/pull/66

## [0.2.5] - 2026-04-30

### Added

- `npub serve --host` flag (default `localhost`) to control the bind interface. Previously `serve` always bound on all interfaces; the safer default now only listens on loopback. Pass `--host 0.0.0.0` to expose on the LAN.

### Changed

- `npub init`'s positional argument is now `[dir]` (was `[path]`) for clearer naming and to avoid confusion with `--path` (notes path).

## [0.2.4] - 2026-04-30

### Changed

- `npub serve` once again defaults to the configured `build_path` rather than a notes path. The flag is `--dir` (override the directory to serve), and `--config` selects the config file. Falls back to `./dist` when no config is found; surfaces config errors when a config was explicitly requested.

### Removed

- Drop the `NPUB_CONFIG` environment variable. Use `--config` or rely on the standard discovery order (`npub.yml` inside `$NOTES_PATH`, then in the current directory).

## [0.2.3] - 2026-04-30

### Changed

- Rename `--dir`/`--notes` to `--path` on `npub serve` and `npub build`. Both flags now share the help text `notes path (default: NOTES_PATH)` and resolve from `--path` then `$NOTES_PATH`.
- `npub serve` no longer reads the config or accepts `--config`/`--notes`.
- Suppress cobra's usage dump on command errors so error messages stand alone.

### Fixed

- `npub serve` and `npub build` now fail fast with explicit messages when the notes path is unset, missing, inaccessible, or not a directory, instead of failing later with an opaque error.

## [0.2.2] - 2026-04-30

### Fixed

- `npub serve` now defaults to the configured `build_path` instead of always falling back to `./dist`, so it serves the same directory `npub build` writes to. Pass `--dir` to override.

## [0.2.1] - 2026-04-26

### Added

- Add `npub init [path]` to generate a sample `npub.yml` configuration.
- Add GitHub Actions workflows for tests, linting, and vulnerability scanning.
- Add the embedded nview favicon asset to generated sites.

### Changed

- Switch the notes dependency to `github.com/dreikanter/notesctl`.
- Refactor tests to use `testify` assertions and shared helpers.
- Update Go and Tailwind dependencies to the latest stable versions.
- Update the auto-tag workflow to `actions/checkout@v6`.
- Pin the local lint command to `golangci-lint` v2.11.4.

### Fixed

- Bump the Go patch version to `1.25.9` to resolve standard-library vulnerability findings.
- Address lint findings reported by `golangci-lint`.

## [0.2.0] - 2026-04-25

### Changed

- Rename project, module, and CLI from `notespub` / `notes-pub` to `npub`.

## [0.1.14] - 2026-04-24

### Changed

- Bump CHANGELOG heading to `0.1.14` to resync with existing git tags. The prior auto-patch workflow had advanced tags to `v0.1.13` while `CHANGELOG.md` was seeded at `0.1.7`, so the first run of the CHANGELOG-driven workflow skipped with "Tag v0.1.7 already exists". Picking up one past the highest existing tag restores the invariant that `CHANGELOG.md` leads the tag.

## [0.1.7] - 2026-04-24

### Changed

- `.github/workflows/tag.yml` now tags merged PRs using the topmost `## [X.Y.Z]` heading from `CHANGELOG.md` instead of auto-incrementing the patch off the latest git tag. Bump major/minor/patch by writing the desired heading in the PR.
