# Changelog

All notable changes to this project will be documented in this file.

When preparing release `X.Y.Z`, add a section with this format:

`## X.Y.Z - YYYY-MM-DD`

The release scripts read that section and publish it as GitHub release notes.

## Unreleased

## 0.0.2 - 2026-02-19

- Hardened auth/session security and resolved race conditions in profile and secrets flows.
- Added native keychain backend resolution and improved headless password fallback semantics.
- Improved calendar update safety by splitting attendee and reminder Graph patches.
- Expanded contacts support with additional fields and deterministic custom-field handling.
- Added quoted reply support in `mog mail send`.
- Strengthened release verification for Homebrew formula URL variants.
- Isolated test config paths with `XDG_CONFIG_HOME` for more reliable tests.

## 0.0.1 - 2026-02-13

- Initial release.
