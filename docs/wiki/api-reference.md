# API Reference

Vakt exposes a REST API at `/api/v1`. The full machine-readable spec lives in [`docs/api/openapi.yaml`](../../docs/api/openapi.yaml) (OpenAPI 3.0.3).

## Base URL

```
https://<your-vakt-host>/api/v1
```

## Authentication

All endpoints (except `POST /auth/login`, `POST /auth/register`, and health checks) require a **Paseto v2** bearer token:

```http
Authorization: Bearer <token>
```

Obtain a token:

```bash
curl -X POST https://vakt.example.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"yourpassword"}'
```

Response:
```json
{
  "token": "v2.local.xxxx",
  "user": { "id": "...", "email": "admin@example.com", "role": "admin" }
}
```

For programmatic access, create a long-lived **API Key** in Settings → API Keys (Pro). API keys use the same `Authorization: Bearer` header.

## Pagination

List endpoints accept `?page=1&limit=25` and return:

```json
{
  "data": [...],
  "pagination": { "page": 1, "limit": 25, "total": 120, "total_pages": 5 }
}
```

## Error format

```json
{ "error": "human-readable message", "code": "ERROR_CODE", "details": {} }
```

Common HTTP status codes: `400` (validation), `401` (unauthenticated), `403` (forbidden), `404` (not found), `429` (rate limited).

## Rate limiting

- Authenticated requests: **300 req/min per organisation** (Redis token bucket)
- Auth endpoints (`/auth/*`): **10 req/min per IP**

Rate-limit headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`.

---

## Endpoints by module

### Auth

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/auth/login` | Obtain Paseto token |
| `POST` | `/auth/register` | Register new account |
| `POST` | `/auth/password-reset/request` | Send reset e-mail |
| `POST` | `/auth/password-reset/confirm` | Set new password |

### Vakt Scan (`vaktscan`)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/vaktscan/assets` | List assets (paginated) |
| `POST` | `/vaktscan/assets` | Create asset |
| `GET` | `/vaktscan/assets/:id` | Get asset |
| `PUT` | `/vaktscan/assets/:id` | Update asset |
| `DELETE` | `/vaktscan/assets/:id` | Delete asset |
| `POST` | `/vaktscan/assets/import/csv` | Bulk import via CSV |
| `GET` | `/vaktscan/findings` | List findings (filterable by asset, severity, status) |
| `GET` | `/vaktscan/findings/:id` | Get finding |
| `PUT` | `/vaktscan/findings/:id` | Update finding status/severity |
| `GET` | `/vaktscan/scans` | List scans |
| `POST` | `/vaktscan/scans` | Trigger scan |

### Vakt Comply (`vaktcomply`)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/vaktcomply/frameworks` | List installed frameworks |
| `POST` | `/vaktcomply/frameworks/:name/enable` | Enable framework (NIS2, ISO27001, DORA, TISAX, …) |
| `GET` | `/vaktcomply/frameworks/:id` | Get framework with score |
| `DELETE` | `/vaktcomply/frameworks/:id` | Remove framework |
| `GET` | `/vaktcomply/frameworks/:id/controls` | List controls (paginated) |
| `GET` | `/vaktcomply/frameworks/:id/report` | Readiness report |
| `GET` | `/vaktcomply/frameworks/:id/export-pdf` | PDF export (Pro) |
| `GET` | `/vaktcomply/frameworks/:id/gaps` | Gap analysis |
| `GET` | `/vaktcomply/controls/:id` | Get control |
| `GET` | `/vaktcomply/controls/export/xlsx` | Export controls as XLSX |
| `GET` | `/vaktcomply/risks` | List risks |
| `POST` | `/vaktcomply/risks` | Create risk |
| `GET` | `/vaktcomply/risks/export/xlsx` | Export risks as XLSX |
| `GET` | `/vaktcomply/incidents` | List incidents |
| `POST` | `/vaktcomply/incidents` | Create incident |
| `GET` | `/vaktcomply/incidents/:id` | Get incident |
| `PUT` | `/vaktcomply/incidents/:id` | Update incident |

### Vakt Privacy (`vaktprivacy`)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/vaktprivacy/vvt` | List processing activities (VVT, Art. 30) |
| `POST` | `/vaktprivacy/vvt` | Create VVT entry |
| `GET/PUT/DELETE` | `/vaktprivacy/vvt/:id` | CRUD VVT entry |
| `GET` | `/vaktprivacy/dpias` | List DPIAs (Art. 35) |
| `POST` | `/vaktprivacy/dpias` | Create DPIA |
| `GET/PUT/DELETE` | `/vaktprivacy/dpias/:id` | CRUD DPIA |
| `GET` | `/vaktprivacy/avvs` | List processor agreements (AVV, Art. 28) |
| `POST` | `/vaktprivacy/avvs` | Create AVV |
| `GET/PUT/DELETE` | `/vaktprivacy/avvs/:id` | CRUD AVV |
| `GET` | `/vaktprivacy/breaches` | List breach records (Art. 33/34) |
| `POST` | `/vaktprivacy/breaches` | Create breach record |
| `POST` | `/vaktprivacy/breaches/:id/notify-authority` | Mark authority notified |
| `GET` | `/vaktprivacy/dsr` | List data subject requests |
| `POST` | `/vaktprivacy/dsr` | Create DSR |

### Vakt Vault (`vaktvault`)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/vaktvault/projects` | List projects |
| `POST` | `/vaktvault/projects` | Create project |
| `DELETE` | `/vaktvault/projects/:id` | Delete project |
| `GET` | `/vaktvault/projects/:id/envs` | List environments |
| `GET` | `/vaktvault/projects/:pid/envs/:eid/secrets` | List secret keys |
| `GET` | `/vaktvault/projects/:pid/envs/:eid/secrets/:key` | Get secret (decrypted) |
| `PUT` | `/vaktvault/projects/:pid/envs/:eid/secrets/:key` | Set secret |
| `DELETE` | `/vaktvault/projects/:pid/envs/:eid/secrets/:key` | Delete secret |
| `POST` | `/vaktvault/git-scans` | Trigger Git repo scan for leaked secrets |
| `GET` | `/vaktvault/git-scans/:id/results` | Get scan results |

### Webhooks

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/webhooks` | List webhooks |
| `POST` | `/webhooks` | Create webhook |
| `PUT` | `/webhooks/:id` | Update webhook |
| `DELETE` | `/webhooks/:id` | Delete webhook |
| `POST` | `/webhooks/:id/test` | Send test ping |

Available events: `finding.created`, `finding.severity_changed`, `incident.created`, `incident.status_changed`, `control.status_changed`.

Payloads are HMAC-SHA256 signed with the `X-Vakt-Signature` header when a secret is set.

### Notifications

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/notifications/preferences` | Get current user's preferences |
| `PUT` | `/notifications/preferences` | Update preferences |

### Vakt HR (`hr`)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/hr/employees` | List employees (paginated) |
| `POST` | `/hr/employees` | Create employee |
| `GET` | `/hr/employees/:id` | Get employee |
| `PUT` | `/hr/employees/:id` | Update employee (status, role, end date) |
| `DELETE` | `/hr/employees/:id` | Delete employee |
| `GET` | `/hr/employees/:id/checklist-runs` | List checklist runs for employee |
| `GET` | `/hr/checklists` | List checklist templates |
| `POST` | `/hr/checklists` | Create checklist template |
| `DELETE` | `/hr/checklists/:id` | Delete checklist template |
| `POST` | `/hr/checklist-runs` | Start checklist run |
| `GET` | `/hr/checklist-runs/:id` | Get checklist run |
| `PUT` | `/hr/checklist-runs/:id` | Update run progress / mark completed |

### Vakt Aware (`vaktaware`) — Pro

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/vaktaware/templates` | List phishing email templates |
| `GET` | `/vaktaware/templates/presets` | List built-in preset templates |
| `POST` | `/vaktaware/templates` | Create template |
| `GET` | `/vaktaware/groups` | List target groups |
| `POST` | `/vaktaware/groups` | Create target group |
| `GET` | `/vaktaware/groups/:id/targets` | List targets in group |
| `POST` | `/vaktaware/groups/:id/targets/import` | Import targets from CSV |
| `GET` | `/vaktaware/landing-pages` | List landing pages |
| `POST` | `/vaktaware/landing-pages` | Create landing page |
| `GET` | `/vaktaware/campaigns` | List campaigns |
| `POST` | `/vaktaware/campaigns` | Create campaign |
| `GET` | `/vaktaware/campaigns/:id` | Get campaign |
| `POST` | `/vaktaware/campaigns/:id/launch` | Launch campaign (sends emails) |
| `POST` | `/vaktaware/campaigns/:id/abort` | Abort running campaign |
| `GET` | `/vaktaware/campaigns/:id/stats` | Campaign statistics |
| `GET` | `/vaktaware/campaigns/:id/report` | Export PDF report |
| `GET` | `/vaktaware/training-modules` | List training modules |
| `POST` | `/vaktaware/training-modules` | Create training module |
| `GET` | `/vaktaware/assignments` | List assignments for current user |
| `POST` | `/vaktaware/assignments/:id/complete` | Mark assignment completed |
| `GET` | `/vaktaware/phish-reports` | List phish reports |
| `GET` | `/vaktaware/phish-reports/stats` | Aggregate phish-report stats |
| `POST` | `/vaktaware/phish-report-token/regenerate` | Regenerate webhook token |

---

## Example: create an asset

```bash
TOKEN="v2.local.xxx"

curl -X POST https://vakt.example.com/api/v1/vaktscan/assets \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production API",
    "type": "web_app",
    "target": "https://api.example.com",
    "criticality": "critical",
    "tags": ["prod", "internet-facing"]
  }'
```

## Example: update a control status

```bash
curl -X PUT https://vakt.example.com/api/v1/vaktcomply/controls/$CONTROL_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status": "implemented", "responsible": "alice@example.com"}'
```
