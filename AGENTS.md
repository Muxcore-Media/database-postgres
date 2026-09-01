# AGENTS.md — database-postgres

MuxCore sidecar module (`database-postgres`). Workspace deploy and SSH: [`../AGENTS.md`](../AGENTS.md). Default ports: [`_mvp/PORTS.md`](../_mvp/PORTS.md).

## Module identity

| Field | Value |
|-------|-------|
| Directory | `database-postgres` |
| Capabilities | see muxcore.json |
| Contracts | DatabaseProvider, DatabaseService, Backupable |

## Agent rules

- Modules run as gRPC sidecars; capabilities are the security boundary.
- TLS required in production (`MUXCORE_INSECURE_DISABLE_TLS` is dev-only).
- Match existing Go patterns; run `gofmt` and package tests before finishing.
- Cross-module events: prefer `github.com/Muxcore-Media/contracts-media/events` over deprecated `core/pkg/contracts` aliases.
- Do not edit polluted workspace dumps (see `MASTER-ROADMAP.md` Appendix H).

## Build

```bash
cd database-postgres
nix-shell -p go postgresql --run 'go test ./...'
```

Vault unix socket: `PGHOST=/run/postgresql PGUSER=postgres`.
