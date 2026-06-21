# Incident Response Playbook

> Audience: on-call operator of a self-hosted Vakt instance (Norvik demo server or
> customer deployment).
> Scope: security incidents, service degradation, and personal data breaches.
> Companion docs: [disaster-recovery.md](disaster-recovery.md) (data loss / host failure) ·
> [backup-restore.md](../backup-restore.md) (backup mechanics).

This playbook closes finding OPS-BB-001 from the 2026-05-29 audit: a platform
that advises customers on NIS2 Art.21 incident handling must itself practice
NIS2 Art.21.

---

## 1. Severity Classification

| Level | Definition | Examples | Response time |
|-------|-----------|---------|---------------|
| **P0** | Platform down or active security breach | Auth bypass discovered, confirmed data breach, all containers down, master key compromised | Immediate |
| **P1** | Significant degradation or security risk | ISMS down > 15 min, backup failure, TLS cert expiry < 48 h, suspected brute-force campaign | < 1 hour |
| **P2** | Non-critical service degraded | Slow response, single worker failure, non-critical feature broken, Asynq queue growing | < 4 hours |
| **P3** | Minor issue | Cosmetic bug, log noise, non-urgent config drift | Next sprint |

P0 and P1 always produce an incident record in Vakt Comply → Vorfall-Register
(see [Section 5](#5-post-incident-process)).

---

## 2. First Response Checklist (P0 / P1)

Run these in order. Stop at the step that reveals the problem.

```bash
# 1. Which Telegram alert fired? (Zabbix is the single alert source)
#    Read the alert text — it names the trigger and the host.

# 2. Health check — is the app responding at all?
curl -sf https://isms.norvikops.de/health | jq '.version'
# Expected: version string. Any curl error or non-200 → go to S1.

# 3. Container state
ssh norvikserver 'docker compose -f /opt/vakt/docker-compose.yml ps'
# Look for "Restarting" or "Exit N" → go to S1.

# 4. Structured logs (last 30 min)
#    Grafana → Explore → Loki → {host="norvikserver"} |= "error" | last 30m
#    Or directly from the host:
ssh norvikserver 'docker compose -f /opt/vakt/docker-compose.yml logs --tail 100 api'

# 5. Distributed traces (if response is slow but container is up)
#    Grafana → Explore → Tempo → {service.name="vakt-api"} → sort by duration desc

# 6. Redis lockout state (if users report login failures)
ssh norvikserver 'docker compose -f /opt/vakt/docker-compose.yml exec redis \
  redis-cli KEYS "login_fail_ip:*"'

# 7. Asynq queue depth (if background jobs are stalling)
ssh norvikserver 'docker compose -f /opt/vakt/docker-compose.yml exec redis \
  redis-cli LLEN "asynq:{default}:pending"'
```

---

## 3. Failure Scenario Runbooks

Each scenario follows: **Detection → Diagnosis → Fix → Verify → Document**.

---

### S1: API container in restart loop

**Detection:** Telegram alert (Zabbix container-down trigger or OBS-005 smoke
test failure). `docker compose ps` shows `Restarting` for `vakt-api`.

**Diagnosis:**

```bash
docker compose logs api --tail 100
```

Common causes and tell-tale log lines:

| Log pattern | Root cause | Fix path |
|-------------|-----------|---------|
| `migration: dirty state` or `SQLSTATE` | Failed DB migration | Roll back image tag (see below) |
| `connect: connection refused` on DB | Postgres not yet ready or crashed | Check `docker compose ps vakt-db`; restart DB first |
| `VAKT_SECRET_KEY` / `required env` | Missing environment variable | Check `.env`; compare against `.env.example` |
| `address already in use :8080` | Port conflict (duplicate container) | `docker compose down && docker compose up -d` |

**Fix — bad image / failed migration:**

```bash
# Roll back to the previous known-good tag
# Edit .env: VAKT_IMAGE_TAG=vX.Y-1 (the tag before the broken release)
docker compose pull api worker
docker compose up -d --force-recreate api worker
```

Do NOT run `migrate down` — Vakt migrations are forward-only. File an issue
instead and wait for a patch release.

**Fix — environment variable missing:**

```bash
diff .env .env.example   # spot the missing key
# Add the variable, then:
docker compose up -d --force-recreate api worker
```

**Verify:**

```bash
curl -sf https://isms.norvikops.de/health | jq
# Must return 200 with version and demo fields populated.
```

**Document:** Incident record in Vakt Comply → Vorfall-Register. Include the
log excerpt that identified the cause and the fix applied.

---

### S2: Credential stuffing / brute-force detected

**Detection:** Zabbix auth-failure spike trigger, or users report "account
locked" / "IP locked" errors.

**Understand the lockout mechanism:**

- After 10 failed login attempts from one IP → that IP is blocked for 15
  minutes for all accounts (`ipLockoutFailMax = 10`, `ipLockoutTTL = 15min`).
- Defined in `backend/internal/auth/service.go`.
- Redis key pattern: `login_fail_ip:<IP-ADDRESS>`.

**Diagnosis:**

```bash
# See all currently blocked IPs
ssh norvikserver 'docker compose -f /opt/vakt/docker-compose.yml exec redis \
  redis-cli KEYS "login_fail_ip:*"'

# Check remaining TTL for a specific IP
ssh norvikserver 'docker compose -f /opt/vakt/docker-compose.yml exec redis \
  redis-cli TTL "login_fail_ip:1.2.3.4"'
```

**Response — automatic lockout, no legitimate user affected:**
No action needed. The IP is released automatically after 15 minutes. This is
the intended anti-abuse behavior.

**Response — legitimate user or your own IP accidentally locked out:**

```bash
ssh norvikserver 'docker compose -f /opt/vakt/docker-compose.yml exec redis \
  redis-cli DEL "login_fail_ip:<IP-ADDRESS>"'
```

**Response — sustained attack from many IPs (P1):**

```bash
# Block at the firewall level — Vakt has no built-in geo-block.
# On Ubuntu + UFW:
ufw deny from <CIDR> to any port 443
```

Also check Nginx access logs for the full IP list:

```bash
ssh norvikserver 'docker compose -f /opt/vakt/docker-compose.yml logs nginx \
  --tail 500 | grep "POST /api/v1/auth/login" | awk "{print \$1}" | sort | uniq -c | sort -rn'
```

**Verify:** Confirm the targeted IP is no longer generating lock events in
Loki: `{host="norvikserver"} |= "IP_LOCKED"`.

**Document:** If this reaches P1 (sustained campaign, multiple users affected),
create an incident record in Vakt Comply → Vorfall-Register.

---

### S4: Suspected data breach

**This is always P0.** Do not delay diagnosis trying to confirm — start the
procedure immediately upon suspicion.

**Immediate actions:**

1. **Preserve evidence first — do NOT restart containers.** Logs are lost on
   restart; capture them now:
   ```bash
   ssh norvikserver 'docker compose -f /opt/vakt/docker-compose.yml logs \
     --timestamps --no-color > /tmp/incident-$(date +%Y%m%d-%H%M%S).log'
   scp norvikserver:/tmp/incident-*.log ./
   ```

2. **Isolate if possible** — if you can confirm active exfiltration, take the
   API offline:
   ```bash
   ssh norvikserver 'docker compose -f /opt/vakt/docker-compose.yml stop api'
   ```
   Weigh this against the cost of service disruption. An isolated demo server
   should be taken offline; a customer production instance is the customer's
   call.

3. **Continue to the GDPR Art.33 procedure** in [Section 4](#4-gdpr-art33--personal-data-breach-procedure).

**Diagnosis checklist:**

- Were Paseto tokens forged or stolen? Check for unusual login patterns in
  Loki: `{host="norvikserver"} |= "auth" | json | line_format "{{.user_id}} {{.ip}}"`
- Was there unauthorized access to the secrets store (`so_secrets`)? Check
  audit_log table: `SELECT * FROM ck_audit_log WHERE resource_type = 'secret' ORDER BY created_at DESC LIMIT 100;`
- Was the master key (`VAKT_SECRET_KEY`) exposed? If yes, follow
  [disaster-recovery.md Section 4](disaster-recovery.md#4-key-rotation-after-compromise)
  immediately in parallel.
- Was it a dependency vulnerability? Check Trivy results and GitHub Dependabot
  alerts.

---

### S5: Worker queue stall

**Detection:** Zabbix Asynq queue-depth item exceeds threshold, or users
report that background jobs (scan results, cleanup, report generation) are not
completing.

**Diagnosis:**

```bash
# Queue depth
ssh norvikserver 'docker compose -f /opt/vakt/docker-compose.yml exec redis \
  redis-cli LLEN "asynq:{default}:pending"'

# Worker logs
ssh norvikserver 'docker compose -f /opt/vakt/docker-compose.yml logs worker --tail 100'
```

Common causes:

| Symptom | Cause | Fix |
|---------|-------|-----|
| Worker in restart loop | Same as S1 for the worker container | Follow S1 for `worker` |
| `context deadline exceeded` on a specific task | External dependency (scanner, SMTP) unreachable | Check connectivity; re-queue after fix |
| Queue grows but worker runs | Panic in a handler leaving tasks in-progress | Check logs for `panic`; restart worker |

**Fix:**

```bash
docker compose restart worker
# Wait 30 seconds, then check queue depth again.
```

**Verify:**

```bash
ssh norvikserver 'docker compose -f /opt/vakt/docker-compose.yml exec redis \
  redis-cli LLEN "asynq:{default}:pending"'
# Should be decreasing or zero within 2 minutes of restart.
```

**Document:** P2 — document only if queue was stalled for > 1 hour or if
compliance evidence generation was affected.

---

### S6: TLS certificate expiry (P1)

**Detection:** Zabbix SSL expiry trigger (fires at < 14 days remaining; alert
at < 48 h).

**Renewal — Let's Encrypt via Certbot:**

```bash
ssh norvikserver 'certbot renew --dry-run'   # test first
ssh norvikserver 'certbot renew'
ssh norvikserver 'docker compose -f /opt/vakt/docker-compose.yml restart nginx'
```

**Verify:**

```bash
echo | openssl s_client -connect isms.norvikops.de:443 2>/dev/null \
  | openssl x509 -noout -dates
# notAfter should be > 60 days from now after successful renewal.
```

---

## 4. GDPR Art.33 — Personal Data Breach Procedure

This section makes Vakt compliant with the advice it gives customers. Apply
whenever a suspected breach involves personal data (user accounts, email
addresses, compliance evidence, or data in the Vakt Privacy / HR modules).

### Assessment (within 1 hour of discovery)

Answer these questions before deciding whether to notify:

1. Was personal data accessed, altered, destroyed, or disclosed without
   authorization?
2. Which data subjects are affected? Estimated count?
3. What data categories?
   - Standard: email addresses, names, job titles
   - Special category (higher urgency, GDPR Art.9): health data, ethnic
     origin, biometric data — unlikely in a typical Vakt deployment but
     possible in Vakt Privacy breach records
4. What is the likely risk to individuals? (identity theft, discrimination,
   financial loss, reputational harm)

**Decision rule:**

- Risk to individuals likely → **notify the Supervisory Authority within
  72 hours** of discovery (not of confirmation — if in doubt, notify).
- No risk to individuals (e.g., encrypted data with no key exposure, internal
  test data only) → document the assessment and the reasoning; no external
  notification required, but internal record is still mandatory under
  Art.33(5).

### 72-hour Notification — Supervisory Authority

For DACH deployments:

| Country | Authority | Contact |
|---------|-----------|---------|
| Germany (federal) | BfDI (Bundesbeauftragter für den Datenschutz) | https://www.bfdi.bund.de/meldung |
| Germany (state) | Relevant Landesbehörde (varies by Bundesland) | See list at bfdi.bund.de |
| Austria | Datenschutzbehörde | https://www.dsb.gv.at |
| Switzerland | EDÖB | https://www.edoeb.admin.ch |

**Notification template (Art.33 required elements):**

```
To: [Supervisory Authority — see table above]
Subject: Notification of Personal Data Breach pursuant to Art. 33 DSGVO

Controller: Norvik / Vakt
Responsible person: [Name, contact email, phone]

Date and time of breach (if known): [YYYY-MM-DD HH:MM UTC]
Date and time of discovery: [YYYY-MM-DD HH:MM UTC]

Nature of the breach:
  [ ] Unauthorized access (confidentiality breach)
  [ ] Unauthorized alteration (integrity breach)
  [ ] Data destruction or loss (availability breach)

Approximate number of data subjects affected: [...]
Approximate number of records affected: [...]

Categories of personal data involved:
  [e.g., email addresses, names, compliance evidence, authentication credentials]

Likely consequences for data subjects:
  [...]

Measures taken or proposed to address the breach and mitigate its effects:
  [...]

If notification is delayed beyond 72 hours: reason for delay:
  [...]

Data Protection Officer (if applicable): [Name / N/A]
```

### Art.34 — Notification to data subjects

If the breach is likely to result in **high risk** to individuals (e.g., leaked
credentials that could enable identity theft), you must also notify affected
data subjects directly, without undue delay.

High-risk indicators: passwords or tokens exposed in plaintext; special
category data exposed; financial data exposed.

Not required if: data was encrypted and the key was not compromised; data is
not re-identifiable.

### Internal documentation (Art.33(5) — mandatory regardless of notification)

**Immediately** upon discovery (even before completing the assessment):

```
Vakt Comply → Vorfall-Register → Neuer Vorfall
  Titel: Personal Data Breach [date]
  Kategorie: Datenpanne (Art. 33 DSGVO)
  Entdeckt: [timestamp]
  Status: In Bearbeitung
```

Add the full Art.33 assessment as an evidence attachment once complete.
This record must be retained for at least 3 years (standard GDPR audit
expectation).

---

## 5. Post-Incident Process

All P0 and P1 incidents require:

### 5.1 Incident record in Vakt Comply (mandatory)

```
Vakt Comply → Vorfall-Register → Neuer Vorfall
  Titel: [Brief description — e.g., "API restart loop 2026-06-05 14:30"]
  Schweregrad: P0 / P1 / P2
  Entdeckt: [timestamp of first alert or report]
  Behoben: [timestamp of confirmed resolution]
  Ursache: [one-paragraph root cause]
  Massnahmen: [what was done to fix it]
```

This is "eating our own dog food" — the same evidence trail we ask customers
to maintain for NIS2 Art.21 and ISO 27001 A.16.

### 5.2 Post-mortem (within 48 hours of resolution)

Write a brief post-mortem covering:

1. **What happened** — timeline with timestamps
2. **Why it happened** — root cause, not symptoms
3. **What we changed** — concrete fixes, not "be more careful"
4. **Detection gap** — did Zabbix alert in time? If not, what trigger is missing?
5. **Runbook gap** — was this scenario covered? Update this document if not.

Store the post-mortem as a Vakt Comply → Interne Audits record or as a file
in `docs/postmortems/YYYY-MM-DD-<slug>.md`.

### 5.3 Zabbix trigger review

After any P0 or P1:

- Did the Zabbix alert fire before users noticed? If not, lower the threshold
  or add a new trigger.
- Was there a trigger that should have fired but did not? Add it.
- Reference: CLAUDE.md → Observability section for the full trigger topology.

### 5.4 Runbook update

If you encountered a failure mode not covered in this document:

1. Add it as a new scenario in [Section 3](#3-failure-scenario-runbooks).
2. Commit the update before closing the incident record.

---

## Quick-Reference Card

| Question | Answer |
|----------|--------|
| Alert source | Zabbix → Telegram (only) |
| Logs | Grafana → Loki → `{host="norvikserver"}` |
| Traces | Grafana → Tempo → `{service.name="vakt-api"}` |
| Redis lockout key | `login_fail_ip:<IP>` |
| Unlock IP | `redis-cli DEL "login_fail_ip:<IP>"` |
| Demo rate limit | 10/min; reset after 5 min; no action needed |
| DR playbook | [disaster-recovery.md](disaster-recovery.md) |
| GDPR notification deadline | 72 h from discovery |
| Incident record location | Vakt Comply → Vorfall-Register |
