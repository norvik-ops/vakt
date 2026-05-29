# ADR-0034: LocalLLMBadge `providerHost` — Trust-Cue ehrlich machen

**Status:** Akzeptiert
**Datum:** 2026-05-27
**Entscheider:** Stefan Moseler
**Sprint:** 56 (Marktreife-Welle 1)
**Related:** Audit-Befund F2 (`vakt-app-analyse_9/ai_contextops_report.md`), [[ADR-0032]] (AI-Integrity)

## Kontext

`LocalLLMBadge` zeigt unter jeder AI-Ausgabe ein Trust-Cue an: ein grünes „Lokal · Keine Datenübertragung"-Badge wenn ein lokales LLM (Ollama, LM-Studio, llm-proxy, lm-studio) konfiguriert ist, sonst ein gelbes „Cloud"-Badge.

Der Audit-Bericht hat zwei Drift-Probleme aufgedeckt:

1. **`SecVitalsOverviewPage.tsx:405`** rendert `<AIAdvisor aiAvailable={…} />` ohne `providerHost`-Prop. AIAdvisor reicht den Wert lediglich durch — wenn er nicht ankommt, fällt `LocalLLMBadge.tsx:29` (`if (!host) return true`) auf „Lokal" zurück.
2. **Der Backend-`/ai/status`-Endpoint** lieferte `provider_host` gar nicht aus. Das Frontend hätte den Wert also auch aus eigener Kraft nicht senden können.

Effekt: Das Badge zeigt „Lokal" selbst wenn der Admin `VAKT_AI_PROVIDER=openai` mit `VAKT_AI_BASE_URL=https://api.openai.com/v1` konfiguriert hat. Compliance-Selbstwiderspruch.

## Entscheidung

1. **Backend** liefert `provider_host` in `GET /api/v1/vaktcomply/ai/status`. Die Information wird aus `client.baseURL` extrahiert (`url.Parse(...).Hostname()`). Bei leerem oder ungültigem URL ist `provider_host` ein Leer-String — das Frontend interpretiert das als „unbekannt" und zeigt zur Sicherheit das Cloud-Badge.
2. **Frontend** erweitert `AIStatus` um `provider_host: string` und reicht den Wert von `useAIStatus()` in `SecVitalsOverviewPage` an `<AIAdvisor>` weiter, das ihn unverändert an `<LocalLLMBadge>` durchreicht.
3. **LocalLLMBadge** bleibt unverändert in seiner Logik: positiv-listet die bekannten Local-Container-Namen (`ollama`, `ai-llm`, `llm-proxy`, `lm-studio`) und schaltet auf „Lokal"; alles andere → „Cloud". Der „kein providerHost → Lokal"-Fallback bleibt als Sicherheitsnetz für Legacy-Callers, aber wird durch den Frontend-Fix nicht mehr aktiv erreicht.

## Konsequenzen

**Positiv:**
- Das sichtbare Trust-Cue stimmt jetzt mit der tatsächlichen Konfiguration überein.
- Compliance-Plattform widerspricht sich an dieser Stelle nicht mehr.
- Keine Breaking-Change für API-Konsumenten: `provider_host` ist ein additives Feld.

**Negativ / offen:**
- Die Erkennung beruht auf einer Allow-List. Ein Kunde, der seinen lokalen LLM-Container z.B. `my-cool-ai` nennt, sieht „Cloud" obwohl das Modell lokal läuft. Workaround: Container-Namen umbenennen oder via reverse-proxy auf `ollama.internal` lenken. Eine konfigurative Erweiterung (`VAKT_AI_LOCAL_HOSTS=…`) ist Sprint-57-Kandidat, kein Launch-Blocker.

## Verifikation

- `frontend/src/shared/components/LocalLLMBadge.test.tsx` — 6 Unit-Tests:
  - Local-Hosts (ollama, my-ollama.internal) → Lokal
  - Cloud-Hosts (api.openai.com, api.mistral.ai, api.groq.com) → Cloud
  - Undefined (Legacy) → Lokal (Verhalten gepinnt)
- Backend-Test: `ai.providerHostFromBaseURL` ist mit dem Status-Handler verkettet, Build verifiziert die Pipeline (`go build ./...`).
- Smoke: nach Deploy `curl /api/v1/vaktcomply/ai/status | jq` muss `provider_host` enthalten. Mit `VAKT_AI_BASE_URL=https://api.openai.com/v1` wird `provider_host` = `"api.openai.com"` → Frontend zeigt Cloud-Badge.

## Abgelehnte Alternativen

- **Backend bestimmt Lokal/Cloud zentral und liefert `{"local": true}` aus** — würde die Allow-List am Backend duplizieren und die Frontend-Logik vergessen-anfällig machen. Provider-Host als reinen Tatbestand auszuliefern und die Klassifikation im UI zu lassen ist UI-näher und transparenter.
- **`if (!host) return false` als Fallback (sicher-by-default)** — würde Legacy-Aufrufer im internen UI brechen, ohne dass die fundamentale Frontend-Pipeline-Lücke bei `SecVitalsOverviewPage` gefixt wäre. Wir fixen die Pipeline, behalten den freundlichen Fallback.
