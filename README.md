# Vakt

**Self-hosted ISMS for SMEs — NIS2, ISO 27001, BSI-Grundschutz**

![License: ELv2](https://img.shields.io/badge/license-Elastic_License_2.0-blue)
![Go](https://img.shields.io/badge/go-1.22%2B-blue)
![Docker](https://img.shields.io/badge/docker-compose%20v2-blue)

---

## What is Vakt?

Vakt is a self-hosted, source-available security and compliance platform built for SMEs in the DACH region. It helps IT teams implement and document NIS2, ISO 27001, and BSI-Grundschutz requirements — without sending any data outside your own infrastructure.

It is a free-to-self-host alternative to commercial tools like Vanta or Drata (~€10,000/year), licensed under the Elastic License 2.0. Deploy it with a single `docker compose up` command in under three minutes.

---

## Modules

| Module | Description |
|---|---|
| 📊 **Vakt Comply** | Compliance hub: control tracking, gap analysis, risk register, incident register, policy templates (10 German templates), auditor portal, audit package export (ZIP), AI-generated reports, NIS2 registration wizard, Trust Center |
| 🔍 **Vakt Scan** | Scanner orchestration: Trivy, Nuclei, OpenVAS. Finding deduplication, SLA tracking, daily BSI CERT-Bund advisory feed, automatic evidence on resolved findings |
| 🔐 **Vakt Vault** | Secrets management: AES-256-GCM storage, Git repo scanning (gitleaks), automatic rotation, CI/CD integration |
| 📧 **Vakt Aware** | Security awareness: internal phishing simulations, micro-trainings, SMTP campaigns, anonymised reporting (Betriebsrats-konform), automatic evidence on training completion |
| 📋 **Vakt Privacy** | GDPR documentation hub: VVT (Art. 30), DPIA (Art. 35), AVV management (Art. 28), DSR workflows, breach notification records (Art. 33/34) |
| 👥 **Vakt HR** | Employee lifecycle management: onboarding and offboarding checklists, checklist runs per employee, employee directory with status tracking. Audit-ready evidence that access provisioning and revocation steps were completed. |

**Shared features across all modules:**

- Webhook alerting — Slack, Teams, generic webhooks with HMAC signing
- In-app notifications and weekly email digest
- Prometheus metrics endpoint
- Global search across all modules
- Data retention policies
- 2FA / TOTP
- Session management
- BSI CERT-Bund advisory feed
- Cross-module evidence automation (e.g. resolved findings and completed trainings flow into Vakt Comply)
- Admin CLI
- Public Trust Pages

---

## Quick Start

```bash
git clone https://github.com/norvik-ops/vatk
cd vatk
cp .env.example .env

# Generate a secure secret key (required):
sed -i 's/VAKT_SECRET_KEY=.*/VAKT_SECRET_KEY='"$(openssl rand -hex 32)"'/' .env

docker compose up -d
```

Open [http://localhost](http://localhost) in your browser.

Demo login (requires `VAKT_DEMO=true`): `admin@vakt.local` / `admin1234`

> **Migrations** run automatically on every `docker compose up -d` — a dedicated `migrate` container applies all pending migrations before the API and worker start.

---

## System Requirements

| | Minimum | Recommended | With AI Advisor (default) |
|---|---|---|---|
| **CPU** | 2 vCPU | 4 vCPU | 4 vCPU — no GPU needed |
| **RAM** | 2 GB | 4 GB | 4 GB (+2 GB for model) |
| **Disk** | 20 GB SSD | 40 GB SSD | 40 GB SSD (+3 GB for model) |
| **Docker Engine** | 24+ | 24+ | 24+ |

The AI advisor runs locally via Ollama on CPU — no GPU, no cloud API key required. Pull the model once after first start: `docker compose --profile ai exec ollama ollama pull llama3.2:3b`. To disable: set `VAKT_AI_PROVIDER=disabled`.

---

## Pro License

Vakt Community is free for self-hosting. Pro unlocks additional features:
- TISAX, DORA, EU AI Act, CRA compliance frameworks
- PDF audit exports and audit packages
- AI-powered compliance advisor
- SSO (OIDC/SAML) integration
- API access tokens
- Advanced scanner features (SBOM, EOL tracking)
- Advanced phishing simulation campaigns

Purchase at [vakt.dev/pricing](https://vakt.dev/pricing). After purchase, enter your
license key in **Settings → License**.

---

## Configuration

The most important environment variables:

| Variable | Default | Description |
|---|---|---|
| `VAKT_DB_URL` | — | PostgreSQL connection string (required) |
| `VAKT_REDIS_URL` | — | Redis connection string (required) |
| `VAKT_SECRET_KEY` | — | 32-byte hex master encryption key (required) |
| `VAKT_MODULES_ENABLED` | all | Comma-separated list of enabled modules |
| `AUTO_MIGRATE` | `false` | Run DB migrations automatically on startup |
| `VAKT_DEMO` | `false` | Seed demo data and enable demo login |
| `VAKT_AI_PROVIDER` | — | AI provider (`openai` for OpenAI-compatible APIs) |
| `VAKT_AI_BASE_URL` | — | Base URL of the AI API |
| `VAKT_AI_API_KEY` | — | API key for the AI provider |
| `VAKT_AI_MODEL` | — | Model name (e.g. `mistral-small-latest`) |
| `VAKT_SMTP_HOST` | — | SMTP host for Vakt Aware campaigns |
| `VAKT_SMTP_PORT` | — | SMTP port |
| `VAKT_SMTP_USER` | — | SMTP username (required for port 587/465) |
| `VAKT_SMTP_PASS` | — | SMTP password (required for port 587/465) |
| `VAKT_SMTP_FROM` | — | From address for campaign emails |

See `docs/configuration.md` for the full reference.

---

## AI Compliance Advisor

Vakt includes a built-in AI advisor that analyses your organisation's real compliance gaps and answers "What should I do this week?" — specific to your data, running entirely on your server.

**Enabled by default** via a local Ollama container (CPU-only, no GPU, no API key). Pull the model once after first start:

```bash
docker compose --profile ai exec ollama ollama pull llama3.2:3b
```

To use a cloud provider instead (e.g. Mistral AI, OpenAI):

```env
VAKT_AI_BASE_URL=https://api.mistral.ai/v1
VAKT_AI_API_KEY=sk-...
VAKT_AI_MODEL=mistral-small-latest
```

To disable entirely: `VAKT_AI_PROVIDER=disabled`

---

## Development

```bash
# Backend — API server
cd backend && go run ./cmd/api

# Backend — background worker
cd backend && go run ./cmd/worker

# Frontend
cd frontend && npm install && npm run dev

# Admin CLI
cd backend && go run ./cmd/admin --help

# Tests and linting
make test
make lint
```

---

## Deployment

See `docs/setup.md` for:

- HTTPS with Let's Encrypt (included Nginx configuration)
- PostgreSQL backup strategies
- Kubernetes deployment via Helm chart (`/helm/vakt`)
- Upgrade procedure between versions

---

## Contributing

Issues and pull requests are welcome.

- Run `make lint` before opening a PR
- Write tests for all service-layer functions (target: 80% coverage for `internal/`)
- Do not commit secrets — use `.env.example` as a template
- Follow the module isolation rules described in `CLAUDE.md`

---

## License

[Elastic License 2.0 (ELv2)](LICENSE) — the source code is publicly available for reading and auditing. Self-hosting for your own organization is free and unrestricted. You may not offer Vakt as a hosted or managed service to third parties. No phone-home, no telemetry, no usage tracking of any kind.
