# Changelog

## [0.1.1] - 2026-08-10

### Added

- SettingsProvider for connection knobs (`database_url`, host/port/user/password/database/sslmode) with live DB reopen via `ReplaceDatabase`
- Secret masking for URL and password in settings mesh

## [0.1.0] - 2026-08-09

### Added

- Initial `database.postgres` provider (pgx) with DatabaseService gRPC, migrations, rollback
