# ADR-0030: i18n-konforme Datumsformatierung via useFormatDate

**Status:** Akzeptiert
**Datum:** 2026-05-23
**Entscheider:** Stefan Moseler
**Bezieht sich auf:** S13-27 (Hook-Erstellung), S16-10 (Bulk-Migration)

## Kontext

Vakt unterstützt vier Sprachen (DE/EN/FR/NL) seit v0.19.0 (Sprint 40, i18n-Infrastruktur Phase 1).
Die Sprachumschaltung über den Locale-Umschalter in User-Settings funktionierte für UI-Strings
korrekt via `i18next`. Datumsformatierung blieb jedoch davon unberührt, weil React-Komponenten
direkt auf native Browser-APIs oder date-fns-Funktionen zugriefen:

```ts
// Verbreitete Muster im Codebase vor der Migration
date.toLocaleDateString('de-DE')          // hardcoded Locale
date.toLocaleString('de-DE', {...})       // hardcoded Locale
date.toLocaleTimeString('de-DE')          // hardcoded Locale
format(date, 'dd.MM.yyyy')               // deutsches Format, kein Locale-Param
```

Konkrete Auswirkung: Ein Nutzer mit Locale-Einstellung "EN" sah trotzdem `23.05.2026` statt
`05/23/2026`. Datumsangaben in Audit-Trails, Finding-Listen, Session-Tabellen, Login-History
und Compliance-Reports ignorierten die gewählte Sprache vollständig.

In Sprint 13 (S13-27) wurde der `useFormatDate`-Hook in
`frontend/src/shared/hooks/useFormatDate.ts` erstellt und an zwei Demo-Stellen
(`AdminSecurityPage`, `SecVitalsOverviewPage`) eingesetzt. Die Bulk-Migration der verbleibenden
rund 60 Call-Sites wurde auf Sprint 16 (S16-10) verschoben, weil die Pattern-Etablierung für
v0.7.0 ausreichte.

In Sprint 23 (2026-05-23) wurden alle 62 verbliebenen Stellen auf den Hook umgestellt und
`shared/utils/date.ts` auf `navigator.language` umgestellt (kein Hardcode `de-DE` mehr als
Fallback). Die Migration ist damit abgeschlossen.

## Entscheidung

Alle Datumsformatierungen im Frontend laufen ausschließlich über den `useFormatDate`-Hook.
Direkte Aufrufe von `.toLocaleDateString()`, `.toLocaleString()`, `.toLocaleTimeString()` und
`format()` aus date-fns mit hardcoded Locale-Strings in React-Komponenten sind verboten.

Der Hook liest die aktive Sprache aus dem i18next-Kontext und mappt BCP-47-Locale-Codes
für alle vier unterstützten Sprachen:

| i18next-Key | BCP-47-Locale |
|-------------|---------------|
| `de`        | `de-DE`       |
| `en`        | `en-US`       |
| `fr`        | `fr-FR`       |
| `nl`        | `nl-NL`       |

Der Hook stellt vier Formatierungsfunktionen bereit:
- `formatDate(date)` — kurzes Datum ohne Uhrzeit
- `formatDateTime(date)` — Datum + Uhrzeit
- `formatTime(date)` — nur Uhrzeit
- `formatRelative(date)` — relative Zeit (z.B. „vor 3 Stunden") via `Intl.RelativeTimeFormat`

Utility-Funktionen außerhalb von React-Komponenten (z.B. in `shared/utils/`) dürfen
`navigator.language` direkt lesen — der Hook-Kontext ist dort nicht verfügbar. Auch diese
Utility-Funktionen dürfen keine hardcodierten Locale-Strings verwenden.

## Alternativen

- **Hardcoded Locale-Parameter an jedem Call-Site** (z.B. `toLocaleDateString(i18n.language)`)
  — verworfen. Keine Single Source of Truth; jede neue Komponente muss das Muster erneut
  kennen. Fehlergefahr bei künftigen Locale-Erweiterungen (z.B. `nl`-Support hätte alle
  Call-Sites erfordert). Hook kapselt das BCP-47-Mapping an einer Stelle.

- **date-fns mit Locale-Objekten** (z.B. `import { de } from 'date-fns/locale'`) — verworfen.
  date-fns-Locale-Objekte müssen statisch importiert werden; dynamisches Laden erfordert Code-
  Splitting oder große Bundle-Includes aller Locales. `Intl`-APIs sind browser-nativ und
  benötigen keine zusätzlichen Locale-Daten-Bundles. date-fns bleibt für reine Datum-
  Arithmetik (`addDays`, `differenceInDays` etc.) erlaubt.

- **Globaler Zustand / Zustand-Store (Zustand)** für die aktive Locale — verworfen.
  i18next ist bereits der authoritative Locale-Store. Ein zweites System für dieselbe
  Information wäre Drift-anfällig (analog zum API-Contract-Drift in ADR-0017).

## Konsequenzen

### Positive

- **Locale-korrekte Datumsangaben in allen 4 Sprachen** — Nutzer sehen Datumsformate
  entsprechend ihrer Spracheinstellung in allen Modulen: Audit-Trail, Findings, Sessions,
  Login-History, Compliance-Reports, Supplier-Portal.
- **Single Source of Truth** — BCP-47-Locale-Mapping und Format-Optionen sind an einer
  Stelle gepflegt. Neue Sprachen werden nur im Hook erweitert.
- **Konsistentes Format** — kein Mismatch mehr zwischen Deutsch und Englisch innerhalb
  derselben Seite (war möglich, wenn ein Teil einer Seite den Hook nutzte und ein anderer
  Teil direkt `de-DE` hardcodierte).
- **CI-Schutz via ESLint** — Eine ESLint-Regel (`no-restricted-syntax` auf
  `.toLocaleDateString(` / `.toLocaleString(` mit Literal-Locale-Argument) kann künftige
  Regressionen abfangen.

### Negative

- **Hook-only-Constraint** — `useFormatDate` ist ein React-Hook und darf nur in
  Komponenten und Custom Hooks aufgerufen werden. Utility-Funktionen außerhalb des
  React-Kontexts (z.B. Datei-Export-Funktionen, Chart-Label-Formatter in Recharts
  `tickFormatter`) müssen `navigator.language` direkt lesen oder als Parameter
  übergeben bekommen. Das erfordert leicht mehr Boilerplate in diesen Fällen.
- **Migrationslast einmalig** — 62 Call-Sites wurden migriert. Dabei mussten einige
  Komponenten von reinen Funktionen auf Hooks umgestellt werden, was kleine
  Refactors erforderte. Dieser Aufwand ist einmalig; zukünftige Komponenten beginnen
  direkt mit dem Hook.

### Neutrale

- date-fns bleibt als Dependency für Datum-Arithmetik im Bundle. Es werden keine
  date-fns-Locale-Objekte importiert; die `format()`-Funktion aus date-fns ist in
  React-Komponenten für Datumsformatierung nicht mehr zulässig (nur Arithmetik).
- `Intl.DateTimeFormat` ist in allen Ziel-Browsern (Chrome 88+, Firefox 85+, Safari 14+)
  vollständig unterstützt. Kein Polyfill nötig.

## Referenzen

- Hook-Implementation: `frontend/src/shared/hooks/useFormatDate.ts`
- Utility-Funktion: `frontend/src/shared/utils/date.ts`
- Sprint-Items: S13-27 (Hook-Erstellung), S16-10 (Bulk-Migration-Plan), S23 (Abschluss)
- i18n-Infrastruktur: ADR-0040 (i18n Foundation Phase 1 — v0.19.0)
