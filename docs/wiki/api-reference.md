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

### Vakt Scan (`secpulse`)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/secpulse/assets` | List assets (paginated) |
| `POST` | `/secpulse/assets` | Create asset |
| `GET` | `/secpulse/assets/:id` | Get asset |
| `PUT` | `/secpulse/assets/:id` | Update asset |
| `DELETE` | `/secpulse/assets/:id` | Delete asset |
| `POST` | `/secpulse/assets/import/csv` | Bulk import via CSV |
| `GET` | `/secpulse/findings` | List findings (filterable by asset, severity, status) |
| `GET` | `/secpulse/findings/:id` | Get finding |
| `PUT` | `/secpulse/findings/:id` | Update finding status/severity |
| `GET` | `/secpulse/scans` | List scans |
| `POST` | `/secpulse/scans` | Trigger scan |

### Vakt Comply (`secvitals`)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/secvitals/frameworks` | List installed frameworks |
| `POST` | `/secvitals/frameworks/:name/enable` | Enable framework (NIS2, ISO27001, DORA, TISAX, …) |
| `GET` | `/secvitals/frameworks/:id` | Get framework with score |
| `DELETE` | `/secvitals/frameworks/:id` | Remove framework |
| `GET` | `/secvitals/frameworks/:id/controls` | List controls (paginated) |
| `GET` | `/secvitals/frameworks/:id/report` | Readiness report |
| `GET` | `/secvitals/frameworks/:id/export-pdf` | PDF export (Pro) |
| `GET` | `/secvitals/frameworks/:id/gaps` | Gap analysis |
| `GET` | `/secvitals/controls/:id` | Get control |
| `GET` | `/secvitals/controls/export/xlsx` | Export controls as XLSX |
| `GET` | `/secvitals/risks` | List risks |
| `POST` | `/secvitals/risks` | Create risk |
| `GET` | `/secvitals/risks/export/xlsx` | Export risks as XLSX |
| `GET` | `/secvitals/incidents` | List incidents |
| `POST` | `/secvitals/incidents` | Create incident |
| `GET` | `/secvitals/incidents/:id` | Get incident |
| `PUT` | `/secvitals/incidents/:id` | Update incident |

### Vakt Privacy (`secprivacy`)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/secprivacy/vvt` | List processing activities (VVT, Art. 30) |
| `POST` | `/secprivacy/vvt` | Create VVT entry |
| `GET/PUT/DELETE` | `/secprivacy/vvt/:id` | CRUD VVT entry |
| `GET` | `/secprivacy/dpias` | List DPIAs (Art. 35) |
| `POST` | `/secprivacy/dpias` | Create DPIA |
| `GET/PUT/DELETE` | `/secprivacy/dpias/:id` | CRUD DPIA |
| `GET` | `/secprivacy/avvs` | List processor agreements (AVV, Art. 28) |
| `POST` | `/secprivacy/avvs` | Create AVV |
| `GET/PUT/DELETE` | `/secprivacy/avvs/:id` | CRUD AVV |
| `GET` | `/secprivacy/breaches` | List breach records (Art. 33/34) |
| `POST` | `/secprivacy/breaches` | Create breach record |
| `POST` | `/secprivacy/breaches/:id/notify-authority` | Mark authority notified |
| `GET` | `/secprivacy/dsr` | List data subject requests |
| `POST` | `/secprivacy/dsr` | Create DSR |

### Vakt Vault (`secvault`)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/secvault/projects` | List projects |
| `POST` | `/secvault/projects` | Create project |
| `DELETE` | `/secvault/projects/:id` | Delete project |
| `GET` | `/secvault/projects/:id/envs` | List environments |
| `GET` | `/secvault/projects/:pid/envs/:eid/secrets` | List secret keys |
| `GET` | `/secvault/projects/:pid/envs/:eid/secrets/:key` | Get secret (decrypted) |
| `PUT` | `/secvault/projects/:pid/envs/:eid/secrets/:key` | Set secret |
| `DELETE` | `/secvault/projects/:pid/envs/:eid/secrets/:key` | Delete secret |
| `POST` | `/secvault/git-scans` | Trigger Git repo scan for leaked secrets |
| `GET` | `/secvault/git-scans/:id/results` | Get scan results |

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

### Vakt Aware (`secreflex`) — Pro

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/secreflex/templates` | List phishing email templates |
| `GET` | `/secreflex/templates/presets` | List built-in preset templates |
| `POST` | `/secreflex/templates` | Create template |
| `GET` | `/secreflex/groups` | List target groups |
| `POST` | `/secreflex/groups` | Create target group |
| `GET` | `/secreflex/groups/:id/targets` | List targets in group |
| `POST` | `/secreflex/groups/:id/targets/import` | Import targets from CSV |
| `GET` | `/secreflex/landing-pages` | List landing pages |
| `POST` | `/secreflex/landing-pages` | Create landing page |
| `GET` | `/secreflex/campaigns` | List campaigns |
| `POST` | `/secreflex/campaigns` | Create campaign |
| `GET` | `/secreflex/campaigns/:id` | Get campaign |
| `POST` | `/secreflex/campaigns/:id/launch` | Launch campaign (sends emails) |
| `POST` | `/secreflex/campaigns/:id/abort` | Abort running campaign |
| `GET` | `/secreflex/campaigns/:id/stats` | Campaign statistics |
| `GET` | `/secreflex/campaigns/:id/report` | Export PDF report |
| `GET` | `/secreflex/training-modules` | List training modules |
| `POST` | `/secreflex/training-modules` | Create training module |
| `GET` | `/secreflex/assignments` | List assignments for current user |
| `POST` | `/secreflex/assignments/:id/complete` | Mark assignment completed |
| `GET` | `/secreflex/phish-reports` | List phish reports |
| `GET` | `/secreflex/phish-reports/stats` | Aggregate phish-report stats |
| `POST` | `/secreflex/phish-report-token/regenerate` | Regenerate webhook token |

---

## Example: create an asset

```bash
TOKEN="v2.local.xxx"

curl -X POST https://vakt.example.com/api/v1/secpulse/assets \
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
curl -X PUT https://vakt.example.com/api/v1/secvitals/controls/$CONTROL_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status": "implemented", "responsible": "alice@example.com"}'
```
