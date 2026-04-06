# Changelog

All notable changes to this project will be documented in this file.

The format is inspired by Keep a Changelog and uses semantic versioning.

## [Unreleased]

### Changed

- Unified public naming across repo/package/CLI/module to `TrainPulse` / `trainpulse`.
- Improved Feishu message rendering with event emojis and more structured `post` content.
- Improved default `text` message readability (multi-line key fields).
- Reworked README with clearer configuration hierarchy and environment variable setup.
- Notification policy changed to alert only on abnormal termination (`FAILED` / `INTERRUPTED`).
- `heartbeat_minutes` is now a silent liveness-check interval (default `30`) used for local state updates only.

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
