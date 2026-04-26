# Changelog

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
