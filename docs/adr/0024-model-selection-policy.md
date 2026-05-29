# ADR-0024: Model-Selection-Policy

**Status:** Akzeptiert
**Datum:** 2026-05-22
**Autoren:** Matharnica / KI-Assist

## Kontext

Vakt nutzt seit v0.6.x standardmäßig ein lokales Ollama-Modell (`qwen2.5:3b`,
Apache 2.0, ~1,9 GB RAM, CPU-freundlich). Die Konfiguration erfolgte bisher
ausschließlich über ENV-Variablen (`VAKT_AI_MODEL`, `VAKT_AI_BASE_URL`).

Zwei Probleme ergaben sich:

1. **Usability**: Admins mussten Docker Compose editieren und den Stack neu
   starten, um ein anderes Modell auszuprobieren.
2. **Pro-BYOK**: Kunden mit einem OpenAI-kompatiblen Cloud-Anbieter (OpenAI,
   Mistral, Groq) wollen einen eigenen API-Endpunkt konfigurieren, ohne
   Zugriff auf die Serverinfrastruktur.

## Entscheidung

### Tier-Definitionen

| Tier | Modell-Quelle | Base-URL | Konfigurierbar via |
|------|--------------|----------|-------------------|
| CE   | Ollama (lokal) | System-Default aus `VAKT_AI_BASE_URL` | Modell-Name in Org-Settings |
| Pro  | Ollama **oder** BYOK-Endpunkt | Org-eigene Custom-URL | Modell-Name + Base-URL in Org-Settings |

**CE-Einschränkung**: `ai_base_url_override` wird serverseitig ignoriert, wenn
keine `FeatureAIAdvisor`-Lizenz vorliegt. Damit bleibt CE immer auf dem
selbst-gehosteten Stack.

### Speicherung

Zwei NULL-bare Spalten in `organizations`:
- `ai_model_override TEXT NULL` — wenn NULL: System-Default aus ENV
- `ai_base_url_override TEXT NULL` — wenn NULL: System-Default aus ENV (Pro only)

### UI

Org-Einstellungen → Abschnitt "KI-Modell" zeigt:
- Dropdown der verfügbaren Ollama-Modelle (`GET /api/v1/vaktcomply/ai/models`)
- Optionales Freitext-Feld für Custom-Modell-Name (für BYOK-Provider)
- Pro-Badge + Custom-Endpoint-Feld (nur bei `FeatureAIAdvisor`)

### DSGVO / Trust-Badge

- Bei BYOK (Custom-Base-URL != Ollama-Hostname): AI-Trust-Badge zeigt
  "Extern: Eigener Anbieter" — der Kunde trägt die DSGVO-Verantwortung
  für die Datenübertragung an den Cloud-Anbieter (Art. 28 AVV empfohlen).
- Lokales Ollama: Badge zeigt "Lokal · Keine Datenübertragung".

### Modell-Auflösungsreihenfolge

```
1. org.ai_model_override    (wenn gesetzt)
2. VAKT_AI_MODEL            (ENV)
3. "llama3.2:3b"            (Hart-Default)
```

Für Base-URL analog, aber nur wenn Pro-Lizenz für Base-URL-Override vorhanden.

## Alternativen verworfen

- **Nur ENV**: Zu unhandlich für Admins ohne Server-Zugriff.
- **Global einstellbar (nicht per Org)**: In Single-Tenant-Deployments irrelevant;
  für MSP-Szenarien (mehrere Orgs pro Instanz im zukünftigen Angebot) wäre
  per-Org die sicherere Option.
- **Modell-Pool (mehrere Modelle gleichzeitig laden)**: Zu ressourcenintensiv
  für KMU-Hardware; CE-Kunden nutzen oft die kleinste VM.

## Konsequenzen

- **Positiv**: Modell-Wechsel ohne Neustart der Plattform; BYOK für Pro-Kunden.
- **Positiv**: Klare DSGVO-Warnung bei Extern-Nutzung.
- **Risiko**: Custom-Base-URL kann auf beliebige Endpunkte zeigen →
  `ValidateAIBaseURL` aus `config` wird auch für Org-Overrides aufgerufen.
- **Wartung**: Bei Org-Delete müssen keine AI-Settings extra gelöscht werden
  (CASCADE über organizations-FK).

## Referenzen

- Sprint 32 Story S32-3
- [ADR-0020](0020-ai-agent-tool-permissions.md) — AI Agent Tool Permissions
- `backend/internal/config/config.go` — `ValidateAIBaseURL`
- `backend/internal/admin/repository.go` — `GetOrgAISettings`, `SetOrgAISettings`
- `backend/db/migrations/131_org_ai_settings.up.sql`
