# Changelog

All notable changes to this project will be documented in this file.

The format is inspired by Keep a Changelog and uses semantic versioning.

## [0.1.0] - 2026-03-31

### Added

- Initial CLI: `run`, `tmux-run`, `status`, `stop`
- Feishu webhook notification with `text` and `post`
- Event lifecycle: `STARTED`, `SUCCEEDED`, `FAILED`, `INTERRUPTED`, `HEARTBEAT`
- SQLite run store and basic status management
- Redaction support and dry-run mode
- Unit and integration tests

### Changed

- Improved FAILED-path visibility in dry-run and notification-failure scenarios
