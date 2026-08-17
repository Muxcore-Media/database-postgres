# Migrating from database-sqlite → database-postgres (laptop)

Single-machine move from the default SQLite store to Postgres. No cloud services required.

## Prerequisites

- Docker or a local Postgres 16+ listening on `localhost:5432`
- MuxCore stack already running with `database-sqlite` (installer / `_mvp`)
- `pg_dump` is **not** used — MuxCore stores app data via the `database` capability, so migration is schema + row copy through the modules (or a one-shot SQL export)

## 1. Start Postgres

```bash
docker run --rm -d --name muxcore-pg \
  -e POSTGRES_USER=muxcore \
  -e POSTGRES_PASSWORD=muxcore \
  -e POSTGRES_DB=muxcore \
  -p 5432:5432 \
  postgres:16-alpine
```

Or reuse CI-style env:

```bash
export PGHOST=localhost PGPORT=5432 PGUSER=muxcore PGPASSWORD=muxcore PGDATABASE=muxcore PGSSLMODE=disable
```

## 2. Snapshot SQLite (optional but recommended)

With `backup-local` registered and `database-sqlite` implementing `Backupable`:

```bash
# CreateBackup including module id database-sqlite (see backup-local README grpcurl examples)
```

Also copy the file as a cold backup:

```bash
cp "$MUXCORE_DATA/sqlite/muxcore.db" "$MUXCORE_DATA/sqlite/muxcore.db.bak"
```

## 3. Swap the module

Stop `database-sqlite`. Start `database-postgres` with the same mesh settings the installer uses for other sidecars, plus:

| Variable | Example |
|----------|---------|
| `MUXCORE_MODULE_ID` | `database-postgres` |
| `PGHOST` / `PGPORT` / `PGUSER` / `PGPASSWORD` / `PGDATABASE` | as above |
| or `DATABASE_URL` | `postgres://muxcore:muxcore@127.0.0.1:5432/muxcore?sslmode=disable` |

Do **not** run both providers at once — they both advertise capability `database`.

## 4. Re-apply schema / data

- Modules that call `Migrate` on startup will recreate their tables against Postgres.
- For existing SQLite rows, use a one-shot dump:

```bash
sqlite3 "$MUXCORE_DATA/sqlite/muxcore.db" .dump > /tmp/muxcore-sqlite.sql
# Review and adapt types (AUTOINCREMENT → SERIAL/IDENTITY, etc.), then:
psql "$DATABASE_URL" -f /tmp/muxcore-adapted.sql
```

Prefer re-seeding from fixture/smoke paths on a laptop demo rather than perfect SQL translation.

## 5. Verify

```bash
# database-postgres module tests (when Postgres is up)
cd database-postgres && go test -count=1 ./...
```

Confirm admin-ui / API still resolve capability `database`, then remove the SQLite process from `up.sh` / installer essentials when you are satisfied.

## Installer note

Default installer profile stays on `database-sqlite`. Optional:

```bash
export MUXCORE_PROFILE=postgres
# or set DATABASE_URL / PG* yourself
./up.sh
```

The installer starts `database-postgres` and, when `DATABASE_URL`/`PGHOST` are unset, a local Docker `postgres:16-alpine` (lab credentials `muxcore`/`muxcore`). Docker is required only when you do not already have Postgres.
