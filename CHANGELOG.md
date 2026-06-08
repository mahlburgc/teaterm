# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.2] - 2026-06-08

### Changed

- fixed command history scroll up if command history is closed

## [0.3.1] - 2026-06-06

### Added

- vhs tape (create gifs) to showcase teaterm features

### Changed

- mocked port now emulates a battery monitoring IoT device

## [0.3.0] - 2026-06-06

### Added

- background bar for selected cmd in cmd history
- open cmd history automatically selects the last (most recent)
  command
- Regression tests simulating the bubbletea event loop
  (`internal/tui_test.go`)
- changelog added

### Changed

- cmd history pop moves msg log up instead of overlaying it
- input value stays untouched while scrolling through the command history
- Accepting auto-completion suggestions now updates the command history filter

## [0.2.0] - 2026-04-29

### Added

- fzf-like command history: fuzzy filtering while typing, with match
  highlighting in the popup.

## [0.1.0] - 2026-04-29

### Added

- Initial release: serial terminal TUI with separated input, message log and
  command history, automatic reconnect, command history persisted over
  sessions, mouse support, message log filtering, timestamps, log files,
  auto-completion suggestions and external editor support.

[0.3.0]: https://github.com/mahlburgc/teaterm/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/mahlburgc/teaterm/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/mahlburgc/teaterm/releases/tag/v0.1.0
