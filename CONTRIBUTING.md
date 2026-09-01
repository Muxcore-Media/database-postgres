# Contributing to database-postgres

## Development Setup

### Prerequisites

- Go 1.26.x
- PostgreSQL 16+ (local, Docker, or vault lab socket)
- golangci-lint (optional but recommended)

### Clone and build

```bash
git clone https://github.com/Muxcore-Media/database-postgres.git
cd database-postgres
make build
```

### Run against a local muxcored

```bash
# Terminal 1: start core in dev mode
cd ../core
MUXCORE_INSECURE_DISABLE_TLS=true ./muxcored

# Terminal 2: start module
export PGHOST=localhost PGUSER=muxcore PGPASSWORD=muxcore PGDATABASE=muxcore
make build && ./database-postgres --muxcore-mesh-addr localhost:9090
```

## Running Tests

```bash
docker run --rm -d --name muxcore-pg \
  -e POSTGRES_USER=muxcore -e POSTGRES_PASSWORD=muxcore -e POSTGRES_DB=muxcore \
  -p 5432:5432 postgres:16-alpine
make test
```

Tests must not depend on a running muxcored instance.

## Linting

```bash
make lint
```

## Code Conventions

- No comments explaining what the code does — name things well instead.
- Comments only for non-obvious WHY — hidden invariants, workarounds.
- No `os.Exit` from library code — only `main` exits.
- Structured logging via `log/slog` — no `fmt.Println` in non-test code.
- Context propagation — every function that does I/O takes `ctx context.Context` as its first argument.
- Error wrapping — use `fmt.Errorf("operation %q: %w", name, err)`.

## Branch Naming

```
feat/<short-description>
fix/<short-description>
docs/<short-description>
refactor/<short-description>
```

## Pull Request Process

1. Branch from `master`.
2. Make your changes with tests.
3. Run `make ci` locally — it must pass.
4. Open a PR against `master`.
5. Squash-merge preferred.

## Security Vulnerabilities

Do **not** open a public issue. See [SECURITY.md](SECURITY.md) for the private reporting process.

## License

By contributing, you agree that your contributions will be licensed under the GPL-3.0 license.
