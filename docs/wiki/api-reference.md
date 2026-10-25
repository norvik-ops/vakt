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
  "token": "v4.local.xxxx",
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
(`vaktcomply:ro`) form. `vaktcomply` is the only module since the others were
archived (2026-10-23).

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
- Login (`/auth/login`): request limit **5 req/min per (IP, email)** plus **30 req/min per IP** across all emails — colleagues behind one office NAT do not block each other (R1-KW-4). Requests without a readable JSON email count at 5 req/min per IP. Answer: `429 {"code":"AUTH_RATE_LIMIT"}`
- Register and password reset (`/auth/register`, `/auth/password-reset/*`): **5 req/min per IP**
- Login (`/auth/login`), additionally: failure-based lockout — **10 failed attempts per (IP, email)** → 15 min block (only that account, NAT-safe); secondary **50 failed attempts per IP** total (`VAKT_RATELIMIT_IP_MAX`)

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

### Vakt Comply (`vaktcomply`)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/vaktcomply/frameworks` | List installed frameworks |
| `POST` | `/vaktcomply/frameworks/:name/enable` | Enable framework (NIS2, ISO27001, BSI, EU AI Act, …) |
| `GET` | `/vaktcomply/frameworks/:id` | Get framework with score |
| `DELETE` | `/vaktcomply/frameworks/:id` | Remove framework |
| `GET` | `/vaktcomply/frameworks/:id/controls` | List controls (paginated) |
| `GET` | `/vaktcomply/frameworks/:id/report` | Readiness report |
| `GET` | `/vaktcomply/frameworks/:id/export-pdf` | PDF export (Pro) |
| `GET` | `/vaktcomply/frameworks/:id/gaps` | Gap analysis |
| `GET` | `/vaktcomply/controls/:id` | Get control |
| `GET` | `/vaktcomply/controls/export/xlsx` | Export controls as XLSX |
| `GET` | `/vaktcomply/assets` | List assets (paginated; old path `/vaktscan/assets` stays as an alias) |
| `POST` | `/vaktcomply/assets` | Create asset |
| `GET` | `/vaktcomply/assets/:id` | Get asset |
| `PUT` | `/vaktcomply/assets/:id` | Update asset |
| `DELETE` | `/vaktcomply/assets/:id` | Delete asset |
| `POST` | `/vaktcomply/assets/import/csv` | Bulk import via CSV |
| `GET` | `/vaktcomply/risks` | List risks |
| `POST` | `/vaktcomply/risks` | Create risk |
| `GET` | `/vaktcomply/risks/export/xlsx` | Export risks as XLSX |
| `GET` | `/vaktcomply/incidents` | List incidents |
| `POST` | `/vaktcomply/incidents` | Create incident |
| `GET` | `/vaktcomply/incidents/:id` | Get incident |
| `PUT` | `/vaktcomply/incidents/:id` | Update incident |

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

## Example: create an asset

```bash
TOKEN="v4.local.xxx"

curl -X POST https://vakt.example.com/api/v1/vaktcomply/assets \
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
