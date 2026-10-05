# Changelog

## [0.1.5] - 2026-10-05

### Changed
- Built on core v0.6.14 / sdk/go/module v0.6.4: unregisters on shutdown and re-registers after core restarts (ADR-0022).

## [0.1.4] - 2026-10-05


### Changed
- Reported version comes from muxcore.json (ADR-0021); built on core v0.6.12 / sdk/go/module v0.6.3 (mesh enrollment, ADR-0017).

## [0.1.3] - 2026-10-05

### Changed
- CI runs on GitHub-hosted runners from the umbrella template; retired-origin workflows removed.
- Dependencies resolve from published GitHub tags (no filesystem `replace`); requires core v0.6.0.

## [0.1.2] — 2026-08-10

### Added

- Advertise `settings` capability so admin-ui discovers SettingsProvider without ListAll probing.

## [0.1.1] - 2026-08-10

### Added

- SettingsProvider for connection knobs (`database_url`, host/port/user/password/database/sslmode) with live DB reopen via `ReplaceDatabase`
- Secret masking for URL and password in settings mesh

## [0.1.0] - 2026-08-09

### Added

- Initial `database.postgres` provider (pgx) with DatabaseService gRPC, migrations, rollback
