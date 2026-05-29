# internal/services/

Cross-Module-Services mit eigener Geschäftslogik. Im Unterschied zu
`internal/shared/` (Bibliotheks-Code, Cross-Cutting-Concerns wie Audit,
Crypto, Middleware) leben hier **Services**, die selbst Domain-Logik
besitzen und über Interfaces von Modulen konsumiert werden.

Konsolidierung in Sprint 15 (S15-10, „Welle 2") aus `internal/shared/`
entstanden, weil sich diese Pakete als Service-Layer-Code entlarvt
hatten — und der Bericht den Sprawl von `shared/` zu Recht kritisiert
hat.

| Service | Vorher | Verantwortlich für |
|---|---|---|
| `ai/` | `shared/ai/` | OpenAI-kompatibler Client, Streaming, UsageTracker, Rate-Limit, Cache, Cost-Tracking |
| `alerting/` | `shared/alerting/` | Outgoing-Webhook-Channels (Slack/Teams/Email), Delivery-Pipeline mit Retry + HMAC-Signature |
| `evidence_auto/` | `shared/evidence_auto/` | Automatische Evidence-Sammlung aus SecPulse, GitHub-Action, SecReflex in die ck_evidence-Inbox |
| `crossevidence/` | `shared/crossevidence/` | Worker-Job, der Module-übergreifende Evidence-Events persistiert |

## Regeln

- Services dürfen `internal/shared/*` importieren (Audit, Crypto, DB-Helper).
- Services dürfen sich **gegenseitig** importieren, wenn die Domain das
  rechtfertigt (z.B. `evidence_auto → ai` für AI-gestützte Klassifikation).
- Module (`internal/modules/*`) dürfen Services importieren.
- Services dürfen Module **nicht** importieren — sonst entsteht ein Zyklus.
  Cross-Module-Hooks laufen über Interfaces (siehe `vaktprivacy → vaktcomply`
  via Asynq-Job-Bridge).

## Welche Pakete bleiben in `internal/shared/`?

Echte Cross-Cutting-Concerns ohne eigene Domain-Logik:
`audit`, `account`, `apidocs`, `apikeys`, `auditor`, `auditreport`,
`auditexport`, `bsi`, `comments`, `controltests`, `crypto`, `dashboard`,
`dataexport`, `db`, `demo`, `demoseed`, `emaildigest`, `errorbudget`,
`feedback`, `integrations`, `ldap`, `metrics`, `middleware`,
`notifications`, `notify`, `onboarding`, `pagination`, `retention`,
`safego`, `scheduledreports`, `search`, `setup`, `telemetry`,
`trustcenter`, `updatecheck`, `usermgmt`, `webhooks`.

Weitere Welle-3-Kandidaten (für eine spätere Iteration):
`scheduledreports`, `emaildigest`, `notifications` — alle drei haben
auch Service-Charakter. Wurden in Sprint 15 belassen, weil ihre
Aufrufer-Kette breiter ist und der Migrations-Aufwand höher.
