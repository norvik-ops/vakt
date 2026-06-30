# API Reference

Vakt exposes a REST API at `/api/v1`. The full machine-readable spec lives in [`backend/internal/shared/apidocs/openapi.yaml`](../../backend/internal/shared/apidocs/openapi.yaml) (OpenAPI 3.0.3).

## Base URL

```
https://<your-vakt-host>/api/v1
```

## Authentication

All endpoints (except `POST /auth/login`, `POST /auth/register`, and health checks) require a **Paseto v4** bearer token:

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

### API-Key scopes

A key carries one or more scopes that gate which module paths it may call:

| Scope | Grants |
|-------|--------|
| _(empty, personal `vakt_…` key)_ | Full user-level access |
| `admin` | Full access to all modules (role `Admin`) |
| `vaktcomply` (or wildcard `vaktcomply.*`) | Read-write access to the module (role `SecurityAnalyst`) |
| `vaktcomply:ro` | **Read-only** access — `GET`/`HEAD` only (role `Viewer`) |

Read-only keys (`:ro` suffix) are rejected with `403 AUTH_READONLY_KEY` on any
write method (`POST`/`PUT`/`PATCH`/`DELETE`). Use them for dashboards, monitoring
or auditor-export jobs that must never mutate state. The same module name works
in bare (`vaktcomply`), wildcard (`vaktcomply.*`) and read-only
(`vaktcomply:ro`) form. Replace `vaktcomply` with any module
(`vaktscan`, `vaktvault`, `vaktaware`, `vaktprivacy`, `vakthr`).

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

### Vakt HR (`vakthr`)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/vakthr/employees` | List employees (paginated) |
| `POST` | `/vakthr/employees` | Create employee |
| `GET` | `/vakthr/employees/:id` | Get employee |
| `PUT` | `/vakthr/employees/:id` | Update employee (status, role, end date) |
| `DELETE` | `/vakthr/employees/:id` | Delete employee |
| `GET` | `/vakthr/employees/:id/checklist-runs` | List checklist runs for employee |
| `GET` | `/vakthr/checklists` | List checklist templates |
| `POST` | `/vakthr/checklists` | Create checklist template |
| `DELETE` | `/vakthr/checklists/:id` | Delete checklist template |
| `POST` | `/vakthr/checklist-runs` | Start checklist run |
| `GET` | `/vakthr/checklist-runs/:id` | Get checklist run |
| `PUT` | `/vakthr/checklist-runs/:id` | Update run progress / mark completed |

### Vakt Aware (`vaktaware`)

Training assignment and completion tracking are available in Community. The phishing-campaign, template and target-group endpoints below require **Pro** (`FeatureVaktAware`).

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

---

## API Deprecation Policy

Vakt follows a deliberate deprecation cycle so that integrations built against the API
have time to adapt before fields or endpoints are removed.

### Fields (response / request body)

1. **Mark:** The field is marked `deprecated: true` in the embedded `openapi.yaml`
   for at least **2 minor releases** before removal.
2. **Log:** The handler logs a `warn`-level message on first use of a deprecated
   field to assist operators monitoring their instances.
3. **Remove:** Fields are removed only in a `major` version or a dedicated
   `/v2` endpoint group — never silently in a minor release.
4. **Changelog:** Every deprecation and removal is listed under
   `### Deprecated` / `### Removed` in `CHANGELOG.md`.

### Endpoints (paths)

- Endpoints are deprecated and removed on the same schedule as fields.
- Deprecated endpoints return an `X-Vakt-Deprecated: true` response header.
- The replacement endpoint (if any) is referenced in the OpenAPI `description`
  and in the changelog.

### Versioning

The API is currently at `/api/v1`. A `/v2` endpoint group will be introduced
when breaking changes accumulate. `/v1` will remain supported for at least
**6 months** after `/v2` launch before being sunset.

### How to check

```bash
# Inspect deprecated fields in the embedded spec:
curl -s https://vakt.example.com/api/v1/openapi.json | jq '.components.schemas | .. | .deprecated? // empty'
```
