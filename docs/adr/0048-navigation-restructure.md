# ADR-0048: Navigations-Restrukturierung (TopBar + Settings/Admin-Hub + Comply-Gruppen)

**Status:** Akzeptiert
**Datum:** 2026-05-28
**Entscheider:** Stefan Moseler
**Related:** User-Feedback „Sidebar muss ich ellenlang nach unten scrollen" (Sprint 60)

## Kontext

Nach Login (Dashboard, kein Modul expandiert) staute die Sidebar ~25 vertikale
Items, was auf 1366×768-Laptops ein dauerhaftes Scrollen erzwang. Ursache war
historisches Wachstum:

1. **Vakt Comply** listete 22 flache Sub-Items ohne Gruppierung — Frameworks,
   Operations, Dokumentation und Drittparteien völlig vermischt; Reihenfolge
   willkürlich (NIS2 + NIS2-Assistent als zwei Top-Level-Einträge, ISO/BSI/CIS
   alle mit demselben `Shield`-Icon).
2. **System-Sektion** verteilte 10–14 flache Links über Settings, Account und
   Admin (Audit-Log, Tenants, Health, Security), ohne klare Trennung.
3. **Bottom-Stack** (Collapse, Hilfe, Bell, Changelog, Docs, Theme, Email,
   Logout, ©) addierte ~210px Vertikalraum, der nicht zur Navigation gehört.
4. **Vakt Vault** hatte vier echte Routen (`projects`, `tokens`, `git-scans`)
   aber keine `children` in der Sidebar — Inkonsistenz zu allen anderen
   Modulen.
5. **Mobile Bottom-Nav** versteckte 2 von 6 Modulen (Vault + Aware) komplett;
   mobile Top-Bar hatte keinen Such-Einstieg.
6. **Versteckte Pages**: `/settings/api-keys`, `/settings/score-config` waren
   nur per Direkt-URL erreichbar — kein Einstieg in der Hub-Seite.
7. **Doppeldeutige Naming**: Sowohl `/settings/alerting` als auch
   `/settings/notifications` mit `Bell`-Icon und ähnlichem Label.

## Entscheidung

### 1. Desktop-TopBar einführen
Neue Komponente `frontend/src/shared/components/TopBar.tsx`. Sitzt im
`<main>`-Container (oberhalb des Content-Bereichs, unterhalb der Banner),
nur auf `lg:` sichtbar. Enthält:

- Globaler Suchtrigger (linksbündig, ⌘K-Shortcut bleibt erhalten)
- `NotificationBell` (verschoben aus Sidebar)
- `ChangelogPopover` (verschoben aus Sidebar)
- Tastaturkürzel-Button (?)
- Theme-Toggle
- **User-Menü** als Dropdown — E-Mail/Name + Mein Account, Sitzungen,
  Dokumentation, Logout

Vorteil: Globale Aktionen sind dort, wo sie konventionell erwartet werden.
Sidebar verliert ~210px Bottom-Stack.

### 2. Sidebar verschlanken
- Bottom-Stack reduziert auf **Collapse + © Footer**
- `MODULE`-Sektion: 7 Items (Dashboard + 6 Module). Integrationen verlässt
  Modul-Block.
- `SYSTEM`-Sektion: **3 Items** statt 10–14
  - `Einstellungen` → `/settings` (Hub mit Tabs)
  - `Integrationen` → `/integrations`
  - `Administration` → `/admin` (Hub, nur Admin/Owner)
- Search-Trigger bleibt nur im **collapsed**-Modus (TopBar deckt expanded ab)

### 3. Vakt Comply in 4 visuelle Gruppen
NavItem-Typ um `childGroups: NavGroup[]` erweitert. Comply nutzt das, alle
anderen Module bleiben bei flachem `children`. Gruppen:

- **Frameworks** — Übersicht, NIS2, ISO 27001, BSI, CIS v8, CCM, DORA, EU AI Act
- **Operations** — Risiken, Vorfälle, Audits, Maßnahmen, Genehmigungen, Überfällig
- **Dokumentation** — Richtlinien, SoA, Nachweise, Zert.-Plan
- **Drittparteien** — Lieferanten, KI-Systeme, Resilience

Icons sind pro Item eindeutig: `Landmark` (BSI), `ListChecks` (CIS), `Cloud`
(CCM), `Banknote` (DORA), `Cpu` (KI-Systeme). NIS2-Assistent verlässt die
Sidebar — Einstieg dafür kommt als prominenter `actions`-CTA auf der
`NIS2ChecklistPage` (Sparkles-Button „NIS2-Assistent öffnen").

### 4. Vakt Vault Sub-Children
`children: Projekte / Tokens / Git-Scans` — konsistent zu Scan/Aware/Privacy/HR.

### 5. Mobile-Verbesserungen
- **Top-Bar**: Such-Icon links neben Theme-Toggle → öffnet GlobalSearch
- **Bottom-Nav**: 4 Kern-Module (Home, Comply, Scan, Privacy) + Button
  **„Mehr"**, der die Sidebar-Drawer öffnet. Damit alle 6 Module + System per
  zwei Taps erreichbar.

### 6. Settings-Hub mit Tabs
`Settings.tsx` bekommt Tab-Navigation (`#platform`, `#access`,
`#notifications`, `#integrations`, `#privacy`, `#ai`, `#public`, `#system`)
mit Hash-Deep-Linking. Verwaiste Sidebar-Items werden **Hub-Cards**:
Branding, Score-Konfiguration, Team, Auditoren, Aufbewahrung, API-Keys.
Bell-Doppelung aufgelöst:

- `/settings/alerting` → Card **„Alarm-Regeln"** mit `Siren`-Icon (Routing
  von System-Events an Slack/Teams/Webhook)
- `/settings/notifications` → Card **„Persönliche Benachrichtigungen"** mit
  `Bell`-Icon (User-Präferenzen: Wochendigest, Findings, Vorfälle)

### 7. Admin-Hub
Neue Seite `frontend/src/pages/AdminHubPage.tsx`, gemountet auf `/admin`.
Card-Grid mit 4 Tiles: System-Status (`/admin/health`), Mandanten
(`/admin/tenants`), Sicherheitsereignisse (`/admin/security`), Audit-Log
(`/settings/audit-log`). Non-Admins werden mit `<Navigate to="/" />`
umgelenkt.

## Konsequenzen

**Positiv**
- Sidebar bei Standard-Auflösung (1366×768) **ohne Scroll** sichtbar.
- Konsistente Modul-Hierarchie: jedes Modul hat entweder flache `children`
  oder gruppierte `childGroups`.
- Versteckte Pages haben jetzt einen Hub-Einstieg.
- Globale Aktionen (Theme, User, Bell) folgen Konventionen aus Linear/GitHub/
  Notion.

**Risiken**
- User-Muscle-Memory: Logout war Bottom-Sidebar, jetzt im User-Menü oben
  rechts. Auf der Login-Seite und im AppTour-Onboarding muss das ggf.
  reflektiert werden.
- TopBar kollabiert auf Mobile zur bestehenden mobilen Top-Bar — Tests prüfen
  Sichtbarkeit per `hidden lg:flex`.
- `NIS2-Assistent` ist nicht mehr direkt aus der Sidebar erreichbar. Falls
  Pen-Tester/Auditoren das vermissen, bringt Cmd+K + die NIS2-Page-CTA den
  Wegfall auf.

**Reverse-Path**
Rückbau möglich, indem `TopBar`-Komponente entfernt, Bottom-Stack der
Sidebar wiederhergestellt und `MODULES_NAV.vaktcomply.childGroups` zu
flachem `children` zurückkonvertiert wird. Settings-Tabs lassen sich
durch Entfernen des `Tabs`-Wrappers wieder als flat-Cards rendern.

## Verifikation

`docs/reviews/2026-05-28-nav-restructure-verify.md` — vitest, vite build,
ESLint, manueller Smoke (Demo-Mode, Mobile-Drawer, Admin-Gate, Settings-
Tab-Deeplink).

## Anhang: Vorher/Nachher in Zahlen

| Metrik | Vorher | Nachher |
|---|---|---|
| Sidebar-Items (Default-Login, Dashboard aktiv) | ~25 | ~14 |
| Sidebar Bottom-Stack-Items | 9 | 2 |
| Comply Sub-Items pro Block | 22 flach | 4 Gruppen × Ø 5 |
| Mobile-Bottom-Nav abdeckte Module | 4 / 6 | 6 / 6 (via Mehr) |
| Settings-Sub-Pages mit Hub-Einstieg | 6 / 11 | 11 / 11 |
| System-Sidebar-Items | 10–14 | 3 |
