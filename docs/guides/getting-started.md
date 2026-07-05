# Getting Started with Vakt

This guide takes you from zero to a running Vakt instance with your first compliance control documented. Estimated time: **15 minutes**.

---

## Prerequisites

| Requirement | Version | Notes |
|---|---|---|
| Docker Engine | 24+ | [Install Docker](https://docs.docker.com/engine/install/) |
| Docker Compose | v2 (built-in) | `docker compose version` to verify |
| RAM | 4 GB | 2 GB minimum without AI advisor |
| Disk | 10 GB free | 3 GB additional for AI model (downloaded on first start) |

**No** Go, Node.js, or PostgreSQL installation required — everything runs in containers.

---

## Step 1 — Install and Start

```bash
git clone https://github.com/norvik-ops/vakt
cd vakt
cp .env.example .env

# Generate a secure secret key (required — never leave this empty):
sed -i 's/VAKT_SECRET_KEY=.*/VAKT_SECRET_KEY='"$(openssl rand -hex 32)"'/' .env

docker compose up -d
```

### Verify it started

```bash
docker compose ps
curl http://localhost/health
# Expected: {"status":"ok","version":"..."}
```

Open [http://localhost](http://localhost) in your browser.

> **Note on AI Advisor:** The `ollama-init` container downloads the default AI model (`qwen2.5:7b`, ~4.5 GB, needs 8 GB RAM) on first start. Depending on your bandwidth this takes 3–30 minutes. On VMs with less than 8 GB RAM, set `VAKT_AI_MODEL=qwen2.5:3b` (~1.9 GB). The platform works without it — you can use all compliance features immediately.

---

## Step 2 — Create Your Organisation

On first visit, Vakt shows a setup wizard:

1. Enter organisation name (e.g. "Acme GmbH")
2. Create admin account with email and password (minimum 10 characters)
3. Select your primary compliance framework (NIS2 / ISO 27001 / BSI-Grundschutz)

The setup wizard completes in under 2 minutes.

---

## Step 3 — Document Your First Control in Vakt Comply

Vakt Comply is the compliance hub. Controls are the building blocks of any compliance framework.

1. Navigate to **Vakt Comply** in the left sidebar
2. Select your framework (e.g. "NIS2")
3. Pick a control — start with something you know is already in place (e.g. "Access Control Policy exists")
4. Set status to **Implemented**
5. Add a short description of what you have in place

You now have your first documented control.

---

## Step 4 — Attach Evidence

Controls without evidence are not audit-ready. Evidence can be:
- A policy document (PDF)
- A screenshot of a configuration
- A description of a process
- Automatically collected evidence from Vakt Scan or Vakt Aware

To attach evidence:
1. Open the control you just created
2. Click **Add Evidence**
3. Upload a file or write a text description
4. Save

---

## Step 5 — View Your Compliance Score

1. Navigate to **Vakt Comply → Dashboard**
2. The compliance score shows percentage of controls implemented per framework
3. "Gap Analysis" shows which controls still need attention

That's it — you have a running Vakt instance with audit-ready compliance documentation.

---

## Step 6 — Run Your First Scan (Vakt Scan)

Vakt Scan orchestrates Trivy and Nuclei (both bundled) to find vulnerabilities in your assets.

1. Navigate to **Vakt Scan** in the left sidebar
2. Click **New Asset** → enter a hostname or IP (e.g. `192.168.1.10` or `webserver.internal`)
3. Select asset type (Server, Container, Web Application) and save
4. Open the asset and click **Start Scan** → select **Trivy** (bundled, no setup required)
5. Wait for the scan to complete — findings appear under **Vakt Scan → Findings**
6. Open a finding → review severity and CVE details
7. When a finding is resolved: change status to **Resolved** and save → the resolution is automatically stored as compliance evidence in **Vakt Comply → Auto-Evidence**

> **No scanners showing?** Trivy and Nuclei are bundled with Vakt. If neither shows as available, check that the backend container started correctly: `docker compose logs api | grep trivy`.

---

## Step 7 — Run Your First Phishing Simulation (Vakt Aware)

Vakt Aware sends simulated phishing emails to your team and tracks click-through rates — all anonymised for Betriebsrat compliance.

1. Navigate to **Vakt Aware** in the left sidebar
2. Go to **Templates** → pick a preset (e.g. "IT-Support: Dringende Passwort-Zurücksetzung") or create your own
3. Go to **Zielgruppen** → create a target group and add employee emails
4. Go to **Kampagnen** → click **Neue Kampagne**:
   - Select your template and target group
   - Set a start date (or run immediately)
   - Enable **Betriebsrat-Modus** (anonymises individual results — recommended)
5. After the campaign runs, open it to see the click rate by department
6. Assign a training module to employees who clicked: **Training → Training zuweisen**
7. When employees complete the training, the completion is stored as evidence in **Vakt Comply → Auto-Evidence**

> **SMTP required:** Vakt Aware sends real emails — configure `VAKT_SMTP_HOST` in `.env` before launching a campaign. For testing, [Mailpit](https://github.com/axllent/mailpit) works well locally.

---

## What's Not in This Guide

This is a quickstart, not a full reference. For deeper topics:

- All 6 modules: see [`docs/modules/`](../modules/)
- Configuration reference: [`docs/wiki/configuration.md`](../wiki/configuration.md)
- HTTPS/TLS setup: [`docs/wiki/installation.md`](../wiki/installation.md)
- Backup setup: [`docs/operations/backup-restore.md`](../operations/backup-restore.md)
- Upgrade procedure: [`docs/operations/upgrade.md`](../operations/upgrade.md)

### Aufgabenorientierte ISMS-Workflows (ISB-Alltag)

Schritt-für-Schritt-Anleitungen für die eigentliche ISMS-Arbeit:

- [Schutzbedarfsfeststellung durchführen](schutzbedarfsfeststellung.md)
- [Vom Risiko zur Maßnahme](risiko-zu-massnahme.md)
- [Internes Audit vorbereiten](internes-audit-vorbereiten.md)
- [NIS2-Vorfall melden](nis2-vorfall-melden.md)

---

## Common First-Day Questions

**Q: Where are my data stored?**  
A: All data stays in the PostgreSQL container on your server. Nothing leaves your infrastructure.

**Q: Can I disable the AI advisor?**  
A: Yes. Set `VAKT_AI_PROVIDER=disabled` in `.env` and restart. You can also remove the `ollama` and `ollama-init` services from `docker-compose.yml`.

**Q: How do I add more users?**  
A: Settings → Organisation → Members → Invite by email.

**Q: Is there a demo I can try first?**  
A: Yes — run a local demo with a single command, no sign-up required:
```bash
VAKT_DEMO=true docker compose --profile demo up -d
```
Open [http://localhost](http://localhost) — the login screen shows ready-to-use credentials automatically.
