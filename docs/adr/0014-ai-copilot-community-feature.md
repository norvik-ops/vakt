# ADR-0014: AI Copilot ist Community-Feature, kein Pro-Gate

**Status:** Accepted
**Datum:** 2026-05-20

## Kontext

Vor v0.6.x war der AI Copilot (5 Endpoints unter `/vaktcomply/ai/*` plus
`/vaktcomply/policies/generate-draft`) durch `license.Require(license.FeatureAIAdvisor)`
hinter dem Pro-Plan-Gate. Die ursprüngliche Annahme: AI braucht GPU oder API-Key
für ein Cloud-LLM und ist damit ein kostenintensives Feature, das den Pro-Plan
rechtfertigt.

Mit der Umstellung auf `qwen2.5:3b` als Default-Modell (Apache 2.0, ~1.9 GB RAM,
CPU-tauglich) ist diese Annahme nicht mehr gültig:

- Das Modell läuft lokal auf jeder Vakt-Instanz ohne zusätzliche Kosten
- Vakt ist source-available unter ELv2 — der Backend-Code für die AI-Endpoints
  ist sichtbar und lauffähig
- Ein Lizenz-Gate verhindert nicht die Nutzung, nur die offizielle Unterstützung
- Customer-Forks oder lokale Patches könnten das Gate trivial umgehen

Effekt: Das Gate war Marketing-Limitierung ohne echten Schutz.

## Entscheidung

**AI Copilot ist Community-Feature seit v0.6.x.** Konkret:

1. `license.Require(license.FeatureAIAdvisor)` wurde aus den AI-Routes entfernt
   (`backend/internal/shared/ai/routes.go`,
   `backend/internal/modules/vaktcomply/routes.go`).
2. Die `FeatureAIAdvisor`-Konstante bleibt erhalten — ausgegebene Lizenzen alter
   Kunden führen sie noch im `features`-Array, und die Lizenz-Validierung soll
   weiterhin erfolgreich bleiben.
3. Frontend-`ProGate`-Komponente nennt KI-Berater nicht mehr in der Pro-Feature-
   Aufzählung; `Settings.tsx` markiert das Label als „legacy seit v0.6.x".
4. Ollama-Service in `docker-compose.yml` ist nicht mehr hinter `profiles: ["ai"]`
   versteckt — startet default-on. Ein `ollama-init`-Container zieht das Modell
   einmalig beim ersten Start.
5. Helm-Chart hat Ollama-StatefulSet + Service + Init-Job als default-on
   (`ollama.enabled: true`).

## Alternativen

- **Pro-Gate behalten** — verworfen: kein echter Schutz, Marketing-Nachteil
  („eigentlich kostenlos, aber wir lassen dich nicht"). Schwächt die
  „Self-hosted, du hast alles"-Position gegenüber Vanta/Drata.
- **Nur Backend-Gate öffnen, Frontend behalten** — verworfen: inkonsistent.
  Kunden sehen das Feature nicht, obwohl es technisch verfügbar wäre.
- **Komplettes License-System abschaffen** — verworfen: für TISAX, DORA,
  NIS2-Reporting, EU AI Act, AuditPDF, SSO, API-Access, SecReflex-Advanced,
  SecPulse-Advanced, Granular-Permissions, Supplier-Portal ist die
  Gate-Differenzierung weiterhin sinnvoll (echte Premium-Compliance-Features
  mit Mehraufwand auf Entwicklungs- und Support-Seite).

## Konsequenzen

### Positive

- Out-of-the-Box-Erlebnis: nach `docker compose up` ist AI direkt nutzbar
- Pricing-Story wird ehrlicher: Pro-Plan ist für Compliance-Premium-Features,
  nicht für „freischalten was eh da ist"
- Stärkt Differenzierung gegenüber Vanta/Drata (deren AI cloud-basiert ist
  und nicht ohne Cloud-Verträge läuft)

### Negative

- Pro-Plan-Wert sinkt (ein Feature weniger im Pro-Bucket) — muss durch
  Schärfung der anderen Pro-Features ausgeglichen werden
- Customer-Erwartung bei AI-Qualität wird größer — bisher konnten wir
  „Pro-Feature, nicht für Community optimiert" als Ausrede nehmen
- Ollama-Container braucht Ressourcen auf jeder Default-Instanz (4 GB RAM-
  Limit; ~2 GB Live-Footprint); kleinere VMs müssen explizit opt-outen

### Neutrale

- `FeatureAIAdvisor`-Konstante bleibt im License-System — keine Breaking-Change
  für ausgegebene Lizenzen, aber nicht mehr im Routing aktiv

## Referenzen

- Commit `0e88299` — Backend Routes
- Commit `697e8eb` — Frontend ProGate + Settings
- Commit `4a9c736` — Ollama default-on in Docker Compose
- Commit `3742699` — Helm-Chart-Integration
- `docs/wiki/ai-features.md` — Kunden-Doku
- `docs/landingpage-ai-briefing.md` — Marketing-Briefing
- ADR-0001 (Self-Hosted ohne Phone-Home) — Voraussetzung für lokale AI
