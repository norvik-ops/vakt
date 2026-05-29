# ADR-0047: BSI-Content-Skalierung + i18n-Coverage-Sweep

**Status:** Akzeptiert
**Datum:** 2026-05-27
**Entscheider:** Stefan Moseler
**Sprint:** 59 (Marktreife-Welle 4)
**Related:** Audit-Befund „BSI-Grundschutz = Stub", „i18n in 14% der vaktcomply-Pages"

## Kontext

Zwei Audit-Drift-Befunde aus dem Auditos-Singularity-9-Agent-Audit waren mit Sprint 58 noch nicht adressiert:

1. **BSI IT-Grundschutz war ein Stub.** `vaktcomply/service_helpers.go:bsiControls()` lieferte sieben Titel ohne Beschreibung, ohne Domain-Bezug zu den BSI-Schichten, ohne Evidence-Type-Differenzierung. Marketing-seitig war BSI als gleichwertig zu NIS2 (88 Controls mit Description) und TISAX (59 Controls) positioniert. Pen-Tester sehen das beim ersten Page-Load.

2. **i18n-Coverage war ungleichmäßig.** Audit behauptete „14% der vaktcomply-Pages haben i18n" — Re-Check zeigte 68% (30/44 Pages), aber mit 14 Lücken-Pages und ~61 hardcoded deutschen Strings, davon ~40 auf zwei Pages (AISystems 27, AccessReviews 13).

## Entscheidung

### BSI-Content-Vollausbau

`bsiControls()` wird von 7 Stub-Einträgen auf **34 ausführliche Controls** erweitert, mit folgenden Eigenschaften:

- Alle **10 Kompendium-Schichten** des BSI IT-Grundschutz-Standards 200-2 abgedeckt: ISMS, ORP, CON, OPS, DER, APP, SYS, IND, NET, INF
- Jeder Control mit deutscher Description nach dem gleichen „Anforderung → Nachweis"-Muster, das `craControls` und `doraControls` bereits verwenden
- Domain-Strings konsistent mit der UI-Filterleiste (Sicherheitsmanagement, Organisation, Personal, Konzeption, Betrieb, Detektion, Reaktion, Anwendungen, IT-Systeme, Industrielle IT, Netze, Infrastruktur)
- Evidence-Type-Klassifikation (manual / automated) für die Compliance-Dashboard-Aggregation
- Weight 1-3 wie in den anderen Frameworks

### Strukturelle Test-Anker

Fünf strukturelle Tests (`bsi_controls_test.go`) verhindern Regression:

1. **`TestBSIControls_BaselineSize`** — Floor von 30 Controls (audit-relevant)
2. **`TestBSIControls_AllLayersCovered`** — alle 10 Kompendium-Schichten müssen mindestens einen Control haben
3. **`TestBSIControls_EveryControlHasDescription`** — Description ≥ 60 Zeichen pro Control (Stub-Re-Introduction blockiert)
4. **`TestBSIControls_HasTitleDomainAndEvidenceType`** — strukturelle Vollständigkeit
5. **`TestBSIControls_IDsAreUnique`** — Duplikat-Schutz für DB-INSERT-Phase

### i18n-Sweep — P0 + P1

**P0** (große hardcoded-Mengen):
- `AccessReviewsPage.tsx` — 13 Strings → 30 i18n-Keys mit `vaktcomply.accessReviews.*`
- `AISystemsPage.tsx` — 27 Strings → 30 i18n-Keys mit `vaktcomply.aiSystems.*`

**P1** (kleinere Mengen, weniger sichtbar):
- `ResilienceTestsPage.tsx` — 5 Status-/Kind-Strings → `vaktcomply.resilienceTests.*`
- `ExceptionsPage.tsx` — 4 Strings, plus Module-Level-`statusBadge` zu Component refaktoriert → `vaktcomply.exceptions.*`
- `EvidenceAutoPage.tsx` — 3 Label-Strings → `vaktcomply.evidenceAuto.*`
- `TISAXMappingPage.tsx` — 2 Strings → `vaktcomply.tisaxMapping.*`
- `DSGVOTOMPage.tsx` — 2 Strings → `vaktcomply.dsgvoTom.*`

Alle Keys in **vier Locales** (de, en, fr, nl) gepflegt.

### i18n-Contract-Test als Drift-Prävention

`AccessReviewsPage.i18n.test.tsx` pinnt **60 i18n-Keys × 4 Locales = 240 Sub-Tests**. Wenn jemand eine Locale-Datei nachträglich beschneidet oder einen Key umbenennt, schlägt das Vitest fehl. Pattern ist generisch genug, um auf zukünftige Page-Sweeps übertragen zu werden.

### Bewusst NICHT umgestellt (Sprint 60+)

- **P2/P3 Container-Pages** ohne hardcoded-German-Strings (BSIGrundschutzPage, NIS2ChecklistPage, PolicyTemplatesPage, OverdueReviewsPage, AuthorityDirectoryPage, AIAgentPage). Sie rendern primär dynamische Daten aus dem Backend; ein `useTranslation`-Import würde nur die Audit-Metrik aufpolieren ohne UX-Wert.
- **Form-Placeholder in P1-Pages** (z.B. ResilienceTests „Berechnungsergebnis hochladen"). Marginaler Sichtbarkeitswert, große Diff-Fläche.
- **CCMPage** mit englischem Marketing-Titel „Continuous Control Monitoring" und einem isolierten „Konfiguration"-String. Sprint-60-Item zusammen mit der Frage, ob CCM-Marketing-Begriff lokalisiert werden soll.

## Konsequenzen

**Positiv:**
- BSI ist als Framework jetzt **vollwertig** und im Marketing nicht mehr eine Lüge
- 8 Pages mit hardcoded-German auf i18n umgestellt; Strings reduziert von ~61 auf ~25 (alle in P2/P3-Stubs oder Form-Placeholders)
- 240 i18n-Contract-Tests verhindern Locale-Drift
- 5 BSI-Struktur-Tests verhindern Content-Regression

**Negativ / akzeptiert:**
- BSI ist *Baseline* (~34 Controls), nicht der vollständige Grundschutz-Kompendium-Stand (>100 Bausteine). Customer mit Kern- oder Standard-Absicherung erweitern ihre Installation. Realistisch, weil Self-Hosted-Customer ihren Scope selbst definieren
- i18n-Sweep ist 80%-Lösung: P2/P3 + Placeholder warten auf Sprint 60. Die übrig gebliebenen Strings sind nicht in der Haupt-Sicht
- Englische Marketing-Begriffe (CCM, Pentest) bewusst nicht übersetzt — Customer kennen sie so

## Verifikation

- **Backend**: `go test ./internal/modules/vaktcomply/ -run TestBSIControls` — 5 Tests grün
- **Frontend**: `npm test` — 482 Tests grün (242 vorher + 240 neue i18n-Contract-Tests)
- **TypeScript**: `tsc --noEmit -p tsconfig.app.json` clean
- **Smoke-Test** (post-deploy):
  ```bash
  # 1. BSI-Framework installieren, dashboard prüfen
  curl -sX POST http://localhost/api/v1/vaktcomply/frameworks \
    -H "Authorization: Bearer $TOKEN" -d '{"name":"BSI"}'
  curl -s "http://localhost/api/v1/vaktcomply/frameworks?name=BSI" \
    | jq '.controls | length'
  # Erwartet: 34

  # 2. Browser: /vaktcomply/access-reviews mit Sprachumschaltung
  # Erwartet: Volltext-Übersetzung in de/en/fr/nl, keine "vaktcomply.X.Y"-Strings sichtbar

  # 3. AISystemsPage analog
  ```

## Abgelehnte Alternativen

- **BSI-Content als externes YAML-Plugin** — der Framework-Plugin-Mechanismus aus `plugins.go` existiert, aber das ist eine Frage der Verteilung (signed YAML mit Customer-Subscription-Modell), nicht des Sprint-Scopes. Built-In-Controls bleiben Go-Code, Customer-Extensions via Plugin
- **Komplette i18n-Vollabdeckung mit Form-Labels in einem Rutsch** — würde Diff-Surface verdoppeln, ohne sichtbaren UX-Vorteil. Pragmatisch limitiert auf sichtbare Strings
- **i18n-Contract-Test als Type-Check** (TypeScript Discriminated-Union-Type für jeden Key) — viel Code für marginalen Mehrwert. Vitest-Sub-Tests sind verständlicher
