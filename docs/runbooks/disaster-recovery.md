# Disaster Recovery Playbook

> Audience: on-call operator of a self-hosted Vakt instance.
> Scope: full or partial loss of the production environment.
> Companion docs: [backup-restore.md](../backup-restore.md) (mechanics) ·
> [encryption-at-rest.md](../encryption-at-rest.md) (key model).

This playbook closes a P2 gap surfaced by the 2026-05-27 AUDITOS audit
(outputs/final_audit.md, Top-3 #3): backup mechanics existed but no
end-to-end Disaster Recovery walkthrough — RTO / RPO targets, Master-Key
rotation during recovery, and failure-scenario decision tree.

---

## Targets

| Metric | Target | How it's met |
|--------|--------|--------------|
| RPO (Recovery Point Objective) | **≤ 24 h** | Daily 02:00 UTC `pg_dump --format=custom` |
| RTO (Recovery Time Objective) | **≤ 4 h** | Full restore on warm hardware: ~30 min DB + 30 min image pull + 30 min DNS + reserve |
| Backup retention | 30 d daily + 12 weekly | Cron + `find -mtime` rotation |
| Restore test cadence | Quarterly | ISO 27001 A.8.13, recorded in Vakt Comply → Interne Audits |

A customer who needs faster RPO/RTO needs PITR (WAL streaming) or a hot
standby — both supported by stock PostgreSQL, neither bundled by Vakt's
default compose. They are documented at the bottom under
[Advanced postures](#advanced-postures).

---

## Verified restore drills (S89-1)

| Date | Scope | Result | Measured RTO |
|------|-------|--------|--------------|
| 2026-06-15 | **DB-level end-to-end drill** (`scripts/restore_drill.sh`): `backup.sh` against a live Postgres → `restore.sh` into a **separate fresh** Postgres → data round-trip verified (2/2 rows) → tampered archive rejected | ✅ pass | **~0.1 s** (DB restore + verify of a small dataset) |
| 2026-06-15 | **Script-level drill** (`scripts/restore_test.sh`): HMAC verification, Master-Key decryption, no key leak to stdout, no `/tmp` key-file residue, tampered-archive rejection | ✅ pass | n/a |
| _<fill in>_ | **Full machine-level drill** — fresh VM incl. image pull + DNS, see procedure below | _<pass/fail>_ | _<minutes from "machine empty" to "/health green">_ |

> **What's verified vs. what's left to the operator:** the DB-level drill above
> is a real `backup.sh` → `restore.sh` → verify cycle against fresh Postgres with
> a measured restore time, and runs reproducibly via `scripts/restore_drill.sh`
> (requires Docker). The **DB-restore RTO scales with dataset size**; the ~0.1 s
> above is for a tiny test set. Before a production go-live, run the **full
> machine-level drill** below on a fresh VM (which additionally includes image
> pull + DNS) with your real data volume and record that end-to-end RTO in the
> third row — that is the number that maps to the ≤ 4 h target.

### Full machine-level drill procedure

1. **Take a backup** on the live (or a representative) host:
   `./scripts/backup.sh /backups/vakt`
2. **Start the clock.** On a fresh VM/container with Docker + Compose but no Vakt
   data, clone the deployment repo and place the backup archive + `.sig`.
3. **Verify + restore:**
   `./scripts/restore.sh /backups/vakt/vakt-backup-<DATE>.tar.gz`
   (passphrase via `VAKT_BACKUP_PASSPHRASE` / `VAKT_BACKUP_PASSPHRASE_FILE` for
   automation, or interactive prompt).
4. **Boot the stack:** `docker compose up -d`.
5. **Stop the clock** when `curl -fsS http://localhost/health` returns `200`.
   That elapsed time is your measured **RTO** — record it above.
6. **Negative test:** corrupt a copy of the archive (append a byte) and confirm
   `restore.sh` **rejects** it on the HMAC check (`signature mismatch`).

### Master-Key handling during restore (hardened, S89-1)

`restore.sh` decrypts the backed-up Master Key only to verify the passphrase and
never echoes it to stdout/logs (not even in `--dry-run`). When the recovered key
differs from the one in `.env` (key rotation), it is handed over via a **`0600`
temp file that is `shred`-deleted on every script exit** (success, error, or
abort) — the operator copies it into `.env` during an interactive pause, after
which it is securely removed. A `umask 077` at the top of the script protects all
temp files it creates. No plaintext key ever lingers in `/tmp`.

---

## Scenario triage

Use this table first. Each row links to a step-by-step section below.

| Symptom | Severity | Section |
|---------|---------|---------|
| App container restart-loops, DB is fine | Low | [1. App-only recovery](#1-app-only-recovery) |
| DB corrupt or accidentally dropped, host intact | Medium | [2. DB-only restore](#2-db-only-restore) |
| Host lost (disk failure, VM gone) | High | [3. Host rebuild](#3-host-rebuild) |
| Master-key compromised | Critical | [4. Key rotation](#4-key-rotation-after-compromise) |
| Backup archive corrupt or missing | Critical | [5. Last-resort](#5-last-resort) |

---

## 1. App-only recovery

The DB still has all data; the app or worker is sick.

```bash
docker compose ps                                # which containers are unhealthy?
docker compose logs --tail 200 vakt-api
docker compose pull vakt-api vakt-worker         # if a bad image was rolled out
docker compose up -d --force-recreate vakt-api vakt-worker
curl -sk https://localhost/health | jq          # 200 + demo:false + version:vX.Y
```

If the issue is a bad migration: roll back the image tag to the previous
release (`vX.Y-1`) in `.env` (`VAKT_IMAGE_TAG`), restart, then file an
issue. Do NOT run `migrate down` blindly — Vakt migrations are forward-only.

---

## 2. DB-only restore

Host is fine, DB is corrupt / dropped / rolled back too far.

```bash
# 1. Stop API + Worker so they don't write during restore.
docker compose stop vakt-api vakt-worker

# 2. Pick the freshest verified backup.
ls -lt /backups/vakt/vakt-backup-*.tar.gz | head -5
./scripts/backup-verify.sh /backups/vakt/vakt-backup-<DATE>.tar.gz

# 3. Restore. Prompts for the passphrase that wraps the Master Key.
./scripts/restore.sh /backups/vakt/vakt-backup-<DATE>.tar.gz

# 4. If the printed VAKT_SECRET_KEY differs from .env, copy it in (.env line
#    VAKT_SECRET_KEY=…) and re-export. If it matches, do nothing.

# 5. Bring the app back.
docker compose up -d
curl -sk https://localhost/health | jq
```

Data loss window = time since the last 02:00 UTC dump (≤ 24 h).
Document the restore as an audit record in Vakt Comply.

---

## 3. Host rebuild

Whole VM lost. The recovery hardware must reach the same DNS name + cert.

```bash
# 1. Provision the new host: same Ubuntu version, same Docker + Compose,
#    same hostname, same TLS cert (LE or your own CA).
# 2. Clone the deployment repo.
git clone git@github.com:norvik-ops/vatk.git /opt/vakt && cd /opt/vakt

# 3. Place the freshest backup on the new host and restore.
scp old-backup-host:/backups/vakt/vakt-backup-<DATE>.tar.gz /backups/vakt/
./scripts/backup-verify.sh /backups/vakt/vakt-backup-<DATE>.tar.gz
./scripts/restore.sh        /backups/vakt/vakt-backup-<DATE>.tar.gz
#    -> writes a .env with the recovered VAKT_SECRET_KEY

# 4. Boot the stack.
docker compose pull
docker compose up -d
curl -sk https://<host>/health
```

DNS / TLS / firewall is half the work — keep an infra-as-code (Ansible /
Terraform) copy of the host configuration. The Vakt repo only ships the
application; the host posture (UFW rules, fail2ban, Caddy / Nginx config,
node_exporter, Zabbix agent) is the operator's responsibility.

Smoke-test after rebuild: `docs/CLAUDE.md` → "Vor jedem Release-Tag" curl
sequence (health, demo/start, login, dashboard).

---

## 4. Key rotation after compromise

Master Key (VAKT_SECRET_KEY) leaked, suspected leaked, or rotated on schedule.

The key encrypts all Vault entries (so_secrets) and a small number of other
columns. Rotation re-wraps every ciphertext under a new key in a single
transaction.

```bash
# 1. Generate a new key.
NEW_KEY=$(openssl rand -hex 32)
echo "NEW VAKT_SECRET_KEY: $NEW_KEY"   # store in your secrets manager NOW

# 2. Take a fresh backup BEFORE rotation (you'll want this restore point if
#    rotation aborts).
./scripts/backup.sh /backups/vakt

# 3. Run the rotation tool. It enumerates every encrypted column, decrypts
#    under the OLD key, re-encrypts under the NEW key, COMMITs only if every
#    row round-trips. Backed by HKDF-derived sub-keys (per service, per
#    project) — see backend/internal/shared/crypto.
docker compose exec vakt-api ./rotate-key \
    --old-key "$OLD_KEY" --new-key "$NEW_KEY"

# 4. Swap the env var.
sed -i "s/^VAKT_SECRET_KEY=.*/VAKT_SECRET_KEY=$NEW_KEY/" .env
docker compose up -d --force-recreate vakt-api vakt-worker

# 5. Verify: open one Vault entry in the UI; it must decrypt.
# 6. Take ANOTHER backup with the new key.
./scripts/backup.sh /backups/vakt
# 7. Securely destroy the old key from any operator-side store
#    (HashiCorp Vault revoke, 1Password archive + version-purge).
```

Regression coverage: `backend/internal/integration_test/rotate_key_real_test.go`
boots a real Postgres, seeds every encrypted column, runs `cmd/rotate-key`
as a child process, and asserts every row decrypts under the new derived
key — and only under that one. The test fails CI if a future refactor
breaks the HKDF chain.

---

## 5. Last-resort

The freshest archive is corrupt and no copy survived.

1. **Check off-site copy** — if you sync to MinIO / S3 / rclone target,
   pull the previous day's archive from there.
2. **Check filesystem snapshots** — ZFS / Btrfs / LVM snapshots on the
   backup host often hold older copies.
3. **No backup at all** — accept partial loss and rebuild from:
   - Source: still in Git (`norvik-ops/vatk` mirror).
   - Org configuration: re-onboard the customer; no shortcut.
   - Compliance evidence: lost permanently if no backup. Inform customer
     and document in their incident register (Vakt Comply → Incidents).
   - Vault secrets: lost — bcrypted hashes cannot be reversed. Re-issue
     all credentials via the rotation page.

Document the incident in `Vakt Comply → Internal Audits` with a copy of
this section + the actual decisions taken, for the next ISMS review.

---

## Advanced postures

For customers with stricter RPO / RTO targets than the defaults.

| Requirement | Solution | Where to start |
|-------------|----------|----------------|
| RPO ≤ 5 min | PostgreSQL WAL archiving (`pg_basebackup` + `archive_command`) to S3 / MinIO. PITR via `recovery_target_time`. | Stock Postgres docs |
| RTO ≤ 1 h | Hot standby (`primary_conninfo` + streaming replication), promote with `pg_ctl promote`. | Stock Postgres docs |
| Geo-redundancy | Two-region active-passive: WAL ship + standby in second region, fail over via DNS. | Customer-specific; out of bundled scope |

These postures need ops work outside the Vakt repo (storage, network,
monitoring). The Vakt app itself is stateless beyond Postgres and the
master key, so any vanilla Postgres HA pattern works.

---

## Quarterly DR exercise

Every 90 days, on a staging instance:

```bash
# 1. Pull yesterday's production backup.
scp prod:/backups/vakt/vakt-backup-<DATE>.tar.gz staging:/tmp/
# 2. Run the full restore path (section 2).
./scripts/restore.sh /tmp/vakt-backup-<DATE>.tar.gz
# 3. Compare counts.
docker compose exec vakt-db psql -U vakt -c \
  "SELECT (SELECT COUNT(*) FROM ck_controls) AS controls,
          (SELECT COUNT(*) FROM ck_evidence) AS evidence,
          (SELECT COUNT(*) FROM so_secrets) AS secrets;"
```

Record the run in Vakt Comply → Interne Audits with the date, who ran it,
and the row counts vs. production. Auditors expect this trail for ISO
27001 A.8.13.
