# Redis HA — Sentinel Setup

For deployments requiring Redis high-availability (e.g., MSP environments
with uptime SLAs), Vakt supports Redis Sentinel out of the box.
No code changes are needed — go-redis v9 and Asynq both support Sentinel natively.

## When You Need This

Single-node Redis is fine for most self-hosted KMU deployments.
Consider Sentinel when:
- Uptime SLA > 99.9% (Sentinel detects failure within ~30s and promotes a replica)
- You run on-premise with manual failover being unacceptable
- Compliance framework requires no single point of failure for the job queue

## Architecture

```
                    ┌─────────────┐
                    │  Sentinel 1  │
                    └──────┬──────┘
                           │ monitor
┌─────────┐   replication  ▼
│ Primary │ ──────────────► Replica 1
│  :6379  │
└─────────┘ ──────────────► Replica 2
                           │
                    ┌──────┴──────┐
                    │  Sentinel 2  │
                    └─────────────┘
```

Minimum setup: 1 primary + 1 replica + 3 sentinels (quorum = 2).

## Docker Compose Snippet

```yaml
services:
  redis-primary:
    image: redis:7-alpine
    command: redis-server --requirepass "${REDIS_PASSWORD}" --masterauth "${REDIS_PASSWORD}"
    volumes:
      - redis_primary:/data

  redis-replica:
    image: redis:7-alpine
    command: >
      redis-server
      --replicaof redis-primary 6379
      --requirepass "${REDIS_PASSWORD}"
      --masterauth "${REDIS_PASSWORD}"
    depends_on:
      - redis-primary

  redis-sentinel:
    image: redis:7-alpine
    command: >
      sh -c "echo 'sentinel monitor vakt redis-primary 6379 2
      sentinel auth-pass vakt ${REDIS_PASSWORD}
      sentinel down-after-milliseconds vakt 5000
      sentinel failover-timeout vakt 60000
      sentinel parallel-syncs vakt 1' > /tmp/sentinel.conf
      && redis-sentinel /tmp/sentinel.conf"
    depends_on:
      - redis-primary
    deploy:
      replicas: 3
```

## VAKT_REDIS_URL for Sentinel

```
VAKT_REDIS_URL=redis+sentinel://:${REDIS_PASSWORD}@redis-sentinel:26379/vakt
```

The `vakt` at the end is the sentinel master name (matches `sentinel monitor vakt ...`).

## Verification

```bash
redis-cli -h redis-sentinel -p 26379 sentinel masters
# Should show vakt master with status=ok
```
