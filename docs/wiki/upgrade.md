# Upgrade Guide

This document describes how to upgrade between minor Vakt versions (e.g. 0.5 → 0.6).
Patch releases (0.5.2 → 0.5.3) follow the same procedure but rarely require migration steps.

---

## General Upgrade Procedure

```bash
# 1. Pull the new image
docker compose pull

# 2. Stop the stack (a few seconds of downtime)
docker compose down

# 3. Run database migrations
docker compose run --rm migrate

# 4. Start the new stack
docker compose up -d

# 5. Verify health
curl -s http://localhost:8080/api/v1/health
# → {"status":"ok"}
```

For zero-downtime upgrades in Kubernetes see the Helm section below.

---

## Breaking Changes Format

Each release's `CHANGELOG.md` marks breaking changes with **[BREAKING]**.
They always include:

- What changed (endpoint, env var, DB column, config key)
- What to do before upgrading (migration, env-var rename, data backfill)
- What happens if you skip the step (hard error, silent regression)

Example:

```
### [BREAKING] VAKT_OLLAMA_* renamed to VAKT_AI_*
Before upgrading: rename env vars in your .env / docker-compose.override.yml.
If skipped: AI report generation silently disabled.
```

---

## Version-specific Notes

### 0.5.x → 0.6.x

No breaking schema changes planned. The upgrade follows the general procedure above.

Key env-var changes introduced in 0.5.x (apply if upgrading from < 0.5):

| Old | New | Notes |
|-----|-----|-------|
| `VAKT_OLLAMA_URL` | `VAKT_AI_BASE_URL` | Rename required |
| `VAKT_OLLAMA_MODEL` | `VAKT_AI_MODEL` | Rename required |
| — | `VAKT_AI_PROVIDER` | New; default `ollama` |
| — | `VAKT_CORS_ORIGINS` | New; default `*` |
| — | `VAKT_METRICS_ENABLED` | New; default `false` |

---

### 0.4.x → 0.5.x

**[BREAKING]** The Jira integration (`VAKT_JIRA_*` env vars) was removed in 0.5.2.
Remove any Jira env vars from your configuration before upgrading — they are ignored but will produce a startup warning.

**[BREAKING]** SMTP authentication is now required on ports 587 and 465.
If you use port 587 or 465, set `VAKT_SMTP_USER` and `VAKT_SMTP_PASS`.

**[BREAKING]** Password minimum length increased from 8 to 10 characters.
Existing passwords are not affected. New passwords (and the setup wizard) now enforce 10 characters + complexity.

---

## Migration Failures

If `docker compose run --rm migrate` fails:

1. Check the migration log — the last applied migration number is printed.
2. Never run `DOWN` migrations in production unless instructed in the changelog.
3. Restore from backup, investigate the failing migration SQL, and re-run.

Backup before every upgrade:

```bash
docker compose exec postgres pg_dump -U vakt vakt > backup-$(date +%Y%m%d).sql
```

---

## Rollback Procedure

Rollback is only safe if no `UP` migration was applied. If migrations ran:

1. Restore the database backup taken before the upgrade.
2. Pull the previous image tag (e.g. `ghcr.io/norvik-gmbh/vakt:0.5.3`).
3. Start with the restored DB + old image.

```bash
docker compose down
# Restore backup (see pg_restore or psql)
VAKT_VERSION=0.5.3 docker compose up -d
```

---

## Helm / Kubernetes Upgrade

```bash
# Pull updated chart values
helm repo update

# Dry-run to preview changes
helm upgrade vakt norvik/vakt --dry-run --diff -f values.yaml

# Apply
helm upgrade vakt norvik/vakt -f values.yaml --wait --timeout 5m
```

Rolling updates: the Deployment uses `RollingUpdate` strategy.
Ensure `VAKT_DB_URL` points to a database that has already been migrated
(run the migrate job before triggering the rollout):

```bash
helm upgrade vakt norvik/vakt -f values.yaml \
  --set migrate.runOnUpgrade=true \
  --wait
```

---

## Support

- Issues: GitHub Issues tracker
- Community: GitHub Discussions
- DACH enterprise support contracts: see `docs/setup.md`
