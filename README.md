# database-postgres

MuxCore sidecar module providing the `database` / `database.postgres` capability via PostgreSQL ([pgx](https://github.com/jackc/pgx)).

## Configuration

| Variable | Default | Meaning |
|----------|---------|---------|
| `MUXCORE_GRPC_ADDR` | (required) | Core gRPC address |
| `DATABASE_GRPC_ADDR` | `:9701` | This module's DatabaseService listen address |
| `PGHOST` / `PGPORT` / `PGUSER` / `PGPASSWORD` / `PGDATABASE` | `localhost` / `5432` / `muxcore` / — / `muxcore` | Connection params |
| `PGSSLMODE` | `disable` | libpq sslmode (dev default) |
| `DATABASE_URL` | — | Optional full URL (preferred for CI; never log) |

## Build / test

```bash
make build
# needs a reachable Postgres (CI uses a service container):
make test
```

## License

GPL-3.0
