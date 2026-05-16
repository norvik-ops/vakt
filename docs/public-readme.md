# Vakt

**Self-hosted ISMS for SMEs — NIS2, ISO 27001, BSI-Grundschutz**

![License: ELv2](https://img.shields.io/badge/license-Elastic_License_2.0-blue)
![Docker](https://img.shields.io/badge/docker-compose%20v2-blue)

> **Live Demo:** [secdemo.norvikops.de](https://secdemo.norvikops.de) — Login: `admin@vakt.local` / `admin1234`

---

## What is Vakt?

Vakt is a self-hosted, source-available security and compliance platform built for SMEs in the DACH region. It helps IT teams implement and document NIS2, ISO 27001, and BSI-Grundschutz requirements — without sending any data outside your own infrastructure.

Free alternative to Vanta or Drata (~€10,000–24,000/year). Deploy in under 3 minutes with Docker Compose.

---

## Modules

| Module | Description |
|---|---|
| 📊 **Vakt Comply** | Compliance hub: control tracking, gap analysis, risk register, incident register, policy templates, auditor portal, audit export, AI compliance advisor, NIS2 registration wizard, Trust Center |
| 🔍 **Vakt Scan** | Scanner orchestration: Trivy, Nuclei, OpenVAS. Finding deduplication, SLA tracking, BSI CERT-Bund advisory feed, automatic evidence on resolved findings |
| 🔐 **Vakt Vault** | Secrets management: AES-256-GCM storage, Git repo scanning, automatic rotation, CI/CD integration |
| 📧 **Vakt Aware** | Security awareness: internal phishing simulations, micro-trainings, anonymised reporting (Betriebsrats-konform), automatic evidence on training completion |
| 📋 **Vakt Privacy** | GDPR documentation hub: VVT (Art. 30), DPIA (Art. 35), AVV management (Art. 28), DSR workflows, breach notification records (Art. 33/34) |

---

## Quick Start

```bash
git clone https://github.com/norvik-ops/vatk
cd vatk
cp .env.example .env

# Set your secret key:
sed -i 's/VAKT_SECRET_KEY=.*/VAKT_SECRET_KEY='"$(openssl rand -hex 32)"'/' .env

docker compose up -d
```

Open [http://localhost](http://localhost) in your browser.

> Migrations run automatically on startup.

---

## System Requirements

| | Minimum | Recommended | With AI Advisor |
|---|---|---|---|
| **CPU** | 2 vCPU | 4 vCPU | 4 vCPU |
| **RAM** | 2 GB | 4 GB | 4 GB (+2 GB for model) |
| **Disk** | 20 GB SSD | 40 GB SSD | 40 GB SSD (+3 GB for model) |
| **Docker Engine** | 24+ | 24+ | 24+ |

---

## Configuration

Key environment variables:

| Variable | Description |
|---|---|
| `VAKT_DB_URL` | PostgreSQL connection string (required) |
| `VAKT_REDIS_URL` | Redis connection string (required) |
| `VAKT_SECRET_KEY` | 32-byte hex master encryption key (required) |
| `VAKT_MODULES_ENABLED` | Comma-separated list of enabled modules (default: all) |
| `VAKT_AI_PROVIDER` | AI provider (`openai` for OpenAI-compatible APIs) |
| `VAKT_AI_BASE_URL` | Base URL of the AI API |
| `VAKT_AI_API_KEY` | API key for the AI provider |
| `VAKT_AI_MODEL` | Model name (e.g. `mistral-small-latest`) |
| `VAKT_SMTP_HOST` | SMTP host for Vakt Aware campaigns |
| `VAKT_SMTP_FROM` | From address for campaign emails |

Full reference: [docs/wiki/configuration.md](docs/wiki/configuration.md)

---

## AI Compliance Advisor

Built-in AI advisor runs locally via Ollama (CPU-only, no GPU, no API key required):

```bash
docker compose exec ollama ollama pull llama3.2:3b
```

To use a cloud provider (e.g. Mistral AI):

```env
VAKT_AI_PROVIDER=openai
VAKT_AI_BASE_URL=https://api.mistral.ai/v1
VAKT_AI_API_KEY=sk-...
VAKT_AI_MODEL=mistral-small-latest
```

To disable: `VAKT_AI_PROVIDER=disabled`

---

## Documentation

- [Installation](docs/wiki/installation.md)
- [Configuration reference](docs/wiki/configuration.md)
- [Vakt Comply](docs/wiki/modules/comply.md)
- [Vakt Scan](docs/wiki/modules/scan.md)
- [Vakt Vault](docs/wiki/modules/vault.md)
- [Vakt Aware](docs/wiki/modules/aware.md)
- [Vakt Privacy](docs/wiki/modules/privacy.md)
- [FAQ](docs/wiki/faq.md)

---

## License

[Elastic License 2.0 (ELv2)](LICENSE) — source-available, free to self-host for your own organization. You may not offer Vakt as a hosted or managed service to third parties. No phone-home, no telemetry, no usage tracking.

© 2026 NorvikOps
