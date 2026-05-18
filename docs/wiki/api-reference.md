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

### Vakt Scan (SecPulse)

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

### Vakt Comply (SecVitals)

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

### Vakt Privacy (SecPrivacy)

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

### Vakt Vault (SecVault)

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
