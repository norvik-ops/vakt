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
curl -s http://localhost/health
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
| — | `VAKT_CORS_ORIGINS` | New; default `http://localhost,http://localhost:5173` (in Produktion auf die echte Frontend-Domain setzen; `*` bricht im Nicht-Demo-Modus bewusst ab) |
| — | `VAKT_METRICS_DISABLED` | Opt-out flag; metrics are on by default (set to `true` to disable) |

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

## Rollback Strategy

**Rule: the backup taken before the upgrade IS the rollback path.**

Running `DOWN` migrations over many versions is not a supported rollback — Down
migrations over 100+ steps are error-prone and untested. Use the backup instead.

### When to roll back

Roll back if:
- The health check after `update.sh` fails and the service does not recover
- A critical regression is discovered within the first hours after upgrade

### How to roll back

```bash
# 1. Stop the broken version
docker compose down

# 2. Restore the database backup (use restore.sh or psql manually)
./scripts/restore.sh <backup-file>
# or: docker compose exec postgres psql -U vakt vakt < backup-YYYYMMDD.sql

# 3. Start the previous image version
VAKT_TAG=v0.X.Y docker compose up -d
```

Replace `v0.X.Y` with the version that was running before the upgrade.
The previous version tag is printed by `update.sh` at the start of each run.

### If migrations already ran

If `docker compose run --rm migrate` already completed before the failure:

- The DB schema is ahead of the old image — the old image may refuse to start
  if it encounters unknown columns it doesn't expect.
- Restore the DB backup first, then start the old image.
- The backup-before-migrate step in `update.sh` is mandatory for exactly this reason.

### If update.sh backup step failed

If `update.sh` exited at Step 1 (backup failure), no image was pulled and no
migration ran — the current version is still running. Fix the backup issue first.

> **Note:** `down` migrations (`docker compose run --rm migrate down N`) are intended only
> for development environments where schemas are reset frequently. Never use
> `migrate down` as a production rollback path.

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

## Recurring Database Maintenance

### audit_log Partition Maintenance

`audit_log` is a range-partitioned table keyed on `created_at` (migration 151).
Pre-created yearly partitions exist for 2025 – 2028; rows outside that range
fall into `audit_log_default`.

**Before 2029-01-01** an operator must create the 2029 partition:

```sql
CREATE TABLE audit_log_2029 PARTITION OF audit_log
    FOR VALUES FROM ('2029-01-01') TO ('2030-01-01');
```

This can be run against a live database without downtime.  Rows in
`audit_log_default` continue to be served while the partition is being created.

Repeat annually for each subsequent year.  A reminder has been added to
the [2028 milestone](https://github.com/norvik-ops/vakt/milestones) —
create the partition before the Vakt release closest to 2028-12-01.

---

## Support

- Issues: GitHub Issues tracker
- Community: GitHub Discussions
- DACH enterprise support contracts: see `docs/setup.md`
