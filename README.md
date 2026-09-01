# database-postgres

MuxCore sidecar module providing the `database` / `database.postgres` capability via PostgreSQL ([pgx](https://github.com/jackc/pgx)).

## Configuration

| Variable | Default | Meaning |
|----------|---------|---------|
| `MUXCORE_GRPC_ADDR` | (required) | Core gRPC address |
| `DATABASE_GRPC_ADDR` | `127.0.0.1:9701` | This module's DatabaseService listen address |
| `PGHOST` / `PGPORT` / `PGUSER` / `PGPASSWORD` / `PGDATABASE` | `localhost` / `5432` / `muxcore` / — / `muxcore` | Connection params |
| `PGSCHEMA` | `public` | Schema / search_path; isolates `_migrations` per sidecar |
| `PGSSLMODE` | `disable` | libpq sslmode (dev default; logs a production warning) |
| `PGCONNECT_TIMEOUT` | `10` | Initial Ping timeout in seconds |
| `PGPOOL_MAX_OPEN` / `PGPOOL_MAX_IDLE` | `10` / `5` | Connection pool limits |
| `DATABASE_URL` | — | Optional full URL (preferred for CI; never log) |

Unix socket example (vault/NixOS):

```bash
export PGHOST=/run/postgresql PGUSER=postgres PGDATABASE=muxcore PGSSLMODE=disable
```

## Build / test

```bash
make build
# needs a reachable Postgres (CI/docker uses postgres:16-alpine):
make test
```

## License

GPL-3.0
