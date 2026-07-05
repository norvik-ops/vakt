# pgBouncer — Connection Pooling

pgBouncer ships in the Docker Compose stack since v1.0.0. It sits between the
Vakt API/worker and PostgreSQL, multiplexing application connections into a
smaller server-side pool.

## Why it matters

pgxpool opens up to 15 connections per process by default (API + worker = 30
total; tunable via `VAKT_DB_MAX_CONNS`). An MSP running several customer
instances on one Postgres server would otherwise exceed Postgres's default
`max_connections = 100`. pgBouncer caps this at `DEFAULT_POOL_SIZE` (20 by default).

## Configuration (docker-compose.yml)

| Variable | Default | Description |
|---|---|---|
| `POOL_MODE` | `transaction` | Transaction-level pooling — best throughput |
| `DEFAULT_POOL_SIZE` | `20` | Max server connections per user/database pair |
| `MAX_CLIENT_CONN` | `100` | Max simultaneous client connections |
| `IGNORE_STARTUP_PARAMETERS` | `extra_float_digits` | pgx sends this on connect; pgBouncer must ignore it |
| `SERVER_RESET_QUERY` | `DISCARD ALL` | Resets server state between transactions |

## Pool-Sizing Formula

```
DEFAULT_POOL_SIZE = max_postgres_connections / number_of_vakt_instances
```

Example: Postgres `max_connections = 100`, 4 Vakt instances → `DEFAULT_POOL_SIZE = 25`.

Reserve ~10 connections for direct admin/migration access:
`DEFAULT_POOL_SIZE = (100 - 10) / 4 = 22`

## Transaction Mode and Prepared Statements

Vakt is configured to use `QueryExecModeCacheDescribe` (see `internal/shared/db/db.go`),
which is compatible with pgBouncer transaction mode. Do not change this to
`QueryExecModeCacheStatement` — prepared statements do not survive across
transactions in transaction mode.

## Disabling pgBouncer

Remove the `pgbouncer` service from `docker-compose.yml` and restore the
original `VAKT_DB_URL` environment override in the `api` and `worker` services
to point directly to `postgres:5432`.

## Monitoring

Check active connections:
```bash
docker compose exec pgbouncer psql -p 5432 -U vakt pgbouncer -c "SHOW pools;"
```

Expected output shows `cl_active` (busy clients) and `sv_active` (busy server connections).
