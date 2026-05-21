# Vakt Frontend

React 18 + TypeScript + Vite + Tailwind + shadcn/ui. Spricht über `/api/v1/*` mit dem Go-Backend (`../backend`).

## Stack

| Schicht | Wahl | Begründung |
|---|---|---|
| Framework | React 18 | Pflicht für shadcn/ui |
| Build | Vite | schnell, kein Webpack-Ballast |
| Sprache | TypeScript (strict) | Type-safety vom Day 1 |
| Routing | React Router v6 | Standard, gut dokumentiert |
| Styling | Tailwind CSS + shadcn/ui | Konsistent, Customer-Whitelabel via Design-Tokens (Sprint 16) |
| State | Zustand | leicht, keine Redux-Komplexität |
| Server-State | TanStack Query v5 | Caching + Refetch ohne Boilerplate |
| Charts | Recharts | SVG-basiert, accessible |
| Forms | `useFormValidation` Hook (siehe ADR-Anmerkung) | bewusst kein react-hook-form, eigene Hook-Implementierung mit cross-field validation + scroll-to-error |
| i18n | i18next mit de/en/fr/nl | bundled Locales, kein CDN |
| Tests | Vitest + Playwright + vitest-axe | Unit + E2E + Accessibility |
| API-Typen | `frontend/src/api/` — generated client (Ziel: openapi-ts in CI, siehe Sprint 16 S16-2) |

## Modul-Struktur

```
src/
├── modules/         # eine Folder pro Modul
│   ├── secvitals/   # Vakt Comply
│   ├── secpulse/    # Vakt Scan
│   ├── secvault/    # Vakt Vault
│   ├── secreflex/   # Vakt Aware
│   ├── secprivacy/  # Vakt Privacy
│   └── hr/          # Vakt HR
├── shared/          # Layout, Auth, Notifications, Hooks (useFocusTrap, useKeyboardShortcuts, ...)
├── pages/           # Top-Level-Routes (Login, TrustPage, ...)
├── api/             # API-Client (fetch-Wrapper + Typen)
├── components/      # ungrouped/legacy components — werden schrittweise nach shared/ migriert
└── i18n/            # Locale-JSON-Dateien
```

## Lokal entwickeln

```bash
# Backend separat starten (siehe ../backend/README oder docker-compose.dev.yml)
npm install
npm run dev          # Vite dev server auf http://localhost:5173
```

Per Default proxied der Dev-Server `/api/*` an `http://localhost:8080` (siehe `vite.config.ts`).

## Tests

```bash
npm run lint         # ESLint
npm run typecheck    # tsc --noEmit
npm run test         # Vitest (unit + a11y)
npm run e2e          # Playwright (E2E, braucht laufenden Backend-Stack)
npm run build        # Production-Build, prüft TS-Strict
```

E2E-Spezifikationen unter `e2e/`, Vitest-Suites neben den Komponenten als `*.test.ts(x)`.

## Wichtige Hooks und Patterns

- `useKeyboardShortcuts` + `GlobalSearch` — Cmd/Ctrl+K Command Palette (Sprint 12 P3-39).
- `useFocusTrap(open, onClose)` — Modal-Focus-Trap für WCAG 2.1 (Sprint 2 P2-28).
- `useFormValidation` — Cross-Field-Validation + Scroll-to-Error (Sprint 6 P2-21).
- `useDemoMode` — liest `/health.demo` und entscheidet, ob Demo-Banner + Ephemeral-Login-Flow läuft (siehe ADR-0015).
- `useFirstAction` + Hint-Toasts — Onboarding-Hilfe auf Listseiten (Sprint 6 P2-24).
- `SkeletonTable`, `SkeletonCardGrid`, `SkeletonDetailPage` — Loading-Patterns (Sprint 2 P2-23).

## Frontend ↔ Backend Vertrag

Jede Response-Form, die das Frontend liest, MUSS gleichzeitig in der OpenAPI-Spec gepflegt sein:

1. Backend-Handler (`../backend/internal/.../handler.go`)
2. OpenAPI-Schema (`../backend/internal/shared/apidocs/openapi.yaml`)
3. Frontend-Interface (`src/api/*.ts` oder Komponente)

Siehe [ADR-0017](../docs/adr/0017-api-contract-tests.md) für die Strategie und `docs/dev/api-contract-checklist.md` für die manuelle Übergangs-Checkliste, die Drift verhindern soll.

## Browser-Support

Latest 2 Versionen Chrome, Firefox, Safari, Edge. Kein IE-Support, kein Polyfill für `Promise.allSettled` oder `Array.flat`.

## Public Mirror

Beim Sync nach `norvik-ops/vatk` wird das gesamte `frontend/`-Verzeichnis 1:1 übernommen (siehe ADR-0016). Keine Build-Artefakte im Repo committen — `node_modules/` und `dist/` sind in `.gitignore`.
