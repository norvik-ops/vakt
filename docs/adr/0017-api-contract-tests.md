# ADR-0017: API-Contract-Tests gegen Backend ↔ Frontend Drift

**Status:** Accepted
**Datum:** 2026-05-20
**Entscheider:** Stefan Moseler
**Bezieht sich auf:** [ADR-0009](0009-openapi-single-source-of-truth.md) (OpenAPI als SSoT)

## Kontext

ADR-0009 erklärt OpenAPI 3.0 zur Single Source of Truth — in der Praxis driftete die Spec aber unbemerkt vom tatsächlichen Verhalten weg. Am 2026-05-20 sind beim Demo-Deployment drei zusammenhängende Bugs aufgefallen, die alle dieselbe Ursache haben:

1. **`/health`** lieferte nur `{"status": "ok"}`, aber das Frontend (`useDemoMode`, `Login.tsx`) las `demo`, `sso_enabled`, `version`. Folge: `isDemo` war auf `secdemo.norvikops.de` immer `false`, die Demo-Credentials-UI wurde nie eingeblendet.
2. **`POST /auth/login`** antwortete mit `{access_token, refresh_token, expires_in}`, aber das Frontend rief `setAuth(data.user)` auf — undefined-Zugriff auf `.id` crashte direkt nach erfolgreichem Login.
3. **OpenAPI-Spec** beschrieb das Login-Response mit `token` (statt `access_token`) und User-Schema mit `name`/`role` (statt `display_name`/`roles[]`) — sowohl Backend als auch Frontend abwichen davon in unterschiedliche Richtungen.

Diese Bugs waren wochenlang im Code, weil:

- Backend-Tests prüfen Domain-Logik, nicht JSON-Form (das Login-Test stub-t den Response-Body).
- Frontend-Tests mocken `apiFetch` mit handgeschriebenen Fixtures, die nie gegen den echten Backend-Response validiert wurden.
- OpenAPI wird zwar in CI als Spec-Datei syntaktisch validiert, aber nicht mit den tatsächlichen Response-Bodies abgeglichen.
- Niemand sah die UI im Demo-Modus — die statischen Demo-Credentials „funktionierten" via curl, ohne die Frontend-UI je auszulösen.

Die OpenAPI-SSoT-Entscheidung aus ADR-0009 bleibt richtig, aber sie ist **per Konvention nicht durchgesetzt**. Wir brauchen einen Mechanismus, der Drift erzwingt zu früh aufzufallen.

## Entscheidung

Wir führen **drei sich ergänzende Maßnahmen** ein, um die OpenAPI als SSoT real durchzusetzen:

### 1. OpenAPI-Schemas für alle Frontend-konsumierten Endpoints sind verbindlich

Jeder Endpoint, dessen Response das Frontend liest (also: alles unter `/api/v1/*` und die nicht-API-Health-Endpoints), MUSS ein vollständiges Response-Schema mit `required`-Feldern in `openapi.yaml` führen. Endpoints, die intern sind oder nur Status-Codes liefern, dürfen weiterhin minimal sein.

### 2. Backend-Tests validieren Responses gegen die OpenAPI-Spec

Ein neuer Integration-Test in `backend/cmd/api/openapi_contract_test.go` lädt die embeddete OpenAPI-Spec, ruft eine kuratierte Liste von Endpoints auf (login, health, demo/start, ein paar Listen-Endpoints pro Modul) und prüft das tatsächliche Response gegen das Spec-Schema mit der `kin-openapi`-Bibliothek. Bricht ab, wenn Felder fehlen oder Typen abweichen.

### 3. Frontend-Typen werden aus der OpenAPI-Spec generiert

`openapi-typescript` generiert `frontend/src/api/openapi-types.ts` aus `docs/api/openapi.yaml`. Handgeschriebene Interfaces wie `LoginResponse`, `HealthResponse` werden durch Imports aus den generierten Typen ersetzt. CI hat einen Step, der `npm run openapi:gen` ausführt und scheitert, wenn das Resultat sich vom committeten unterscheidet — damit fällt Drift direkt im PR auf, nicht erst beim Visitor-Klick.

## Alternativen

- **Status quo (Konvention)** — verworfen, weil OpenAPI seit ADR-0009 (~9 Monate) als SSoT galt und Drift trotzdem wochenlang unbemerkt blieb. Konvention reicht nicht ohne Werkzeug.
- **Strict Backend-Generation aus OpenAPI** (z.B. `oapi-codegen` für Go-Handler) — verworfen, weil das Echo-Routing-Pattern und sqlc-Integration tief im Backend stecken. Generierte Handler-Stubs würden mehr Reibung als Nutzen bringen. Wir prüfen Drift, statt sie zu eliminieren.
- **Schema-First-only (kein Code)** — verworfen, weil das gegen die Eigenständigkeit der einzelnen Module verstößt (ADR-0004: Modul-Isolation). Jedes Modul hält seine Handler in Go-Code, der OpenAPI-Snippets werden manuell zusammengeführt.
- **Vollständige Pact/Contract-Test-Suite** — overkill für eine Single-Frontend / Single-Backend-Architektur. Pact glänzt bei Multi-Consumer/Multi-Provider; bei uns reicht der direkte Response-Vergleich.

## Konsequenzen

### Positive

- **Drift fällt zur CI-Zeit auf**, nicht beim Demo-Visitor. Der konkrete Bug von heute (`user`-Feld fehlt im Response) wäre der Test-Suite sofort begegnet.
- **Frontend-Refactors gegen die Backend-Realität** — Type-Errors statt Runtime-Crashes. „can't access property `id` of undefined" wird zu einem Compile-Fehler.
- **OpenAPI als echte Doku** — wer das Public Repo klont, kann den OpenAPI-Spec lesen und weiß, was rauskommt. Das war bisher nur halb wahr.
- **SDK-Generierung möglich** — externe SDKs können aus der Spec generiert werden, ohne dass wir manuell mitziehen müssen.

### Negative

- **Initialer Aufwand**: OpenAPI auf realistischen Stand bringen, openapi-typescript einbauen, Backend-Contract-Test schreiben. Schätzung: 4–6 Stunden für die initiale Welle, danach inkrementell pro neuem Endpoint.
- **CI-Step mehr**: zwei zusätzliche Checks (Backend Contract-Test, Frontend `openapi-types`-Diff). Beide schnell, aber müssen grün bleiben.
- **Coverage-Aufwand**: Nicht jedes Backend-Endpoint hat heute ein vollständiges Spec-Schema. Initiale Welle deckt nur die kritischen ab (auth, health, demo, dashboard, vaktcomply-listing); der Rest kommt nach Bedarf.

### Neutrale

- Die Implementierung selbst kommt in einem nachfolgenden Sprint — diese ADR dokumentiert die Entscheidung, die konkreten Tools werden bei Umsetzung gewählt (kin-openapi für Go-Validation, openapi-typescript für TS-Generation).
- Bestehende handgeschriebene TS-Interfaces können schrittweise migriert werden (per Endpoint), nicht im Big-Bang.

## Sofort-Maßnahmen (heute, 2026-05-20)

Die ADR selbst legt die Strategie fest. Davon unabhängig wurden die heutigen Bugs als Spot-Fix bereinigt und die OpenAPI-Spec auf realen Stand gebracht (siehe Commit-Reihe vom 2026-05-20):

- Backend `/health` liefert `demo`, `sso_enabled`, `version`
- Backend `/auth/login` liefert `user`-Objekt mit id/email/display_name/roles
- OpenAPI-Schemas `LoginResponse`, `User`, `/health` angepasst

Die Sofort-Fixes sind reaktiv. Diese ADR ist die proaktive Antwort, damit dieselbe Klasse von Bugs nicht in einem anderen Endpoint wieder auftritt.

## Referenzen

- ADR-0009: OpenAPI Single Source of Truth
- OpenAPI-Spec: `backend/internal/shared/apidocs/openapi.yaml` (embedded → `/api/openapi`)
- Frontend-Konsumenten: `frontend/src/pages/Login.tsx`, `frontend/src/shared/hooks/useDemoMode.ts`
- Tools (für Implementierung): `github.com/getkin/kin-openapi`, `openapi-typescript`
