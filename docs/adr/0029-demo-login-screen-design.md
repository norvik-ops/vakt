# ADR-0029: Demo-Login-Screen UI-Design

**Status:** Accepted
**Datum:** 2026-05-22
**Entscheider:** Stefan Moseler

## Kontext

Die öffentliche Demo (`secdemo.norvikops.de`, `VAKT_DEMO=true`) setzt ADR-0015 um: jeder Visitor
bekommt eine eigene ephemere Org mit Random-Credentials aus `/api/v1/demo/start`. Der technische
Flow ist in ADR-0015 und `docs/concepts/demo-flow.md` definiert.

Offen blieb bisher die Frage: **Was soll der Login-Screen im Demo-Modus konkret zeigen?**
Das fehlende Design-Dokument führte wiederholt zu Verwirrung:

- Nach Code-Änderungen fehlte das Demo-Overlay plötzlich.
- Nach Build-Fehlern lief eine alte Demo-Image; der Unterschied zwischen „Demo-UI fehlt wegen
  kaputtem Build" und „Demo-UI fehlt wegen Backend-Down" war nicht klar.
- CI-Fehler (gitleaks, TypeScript) blockierten `deploy-demo` wochenlang ohne explizites
  Fehlerbild; der Visitor sah nur ein leeres Standard-Login-Formular.

## Entscheidung

Im Demo-Modus (`isDemo === true`) zeigt `Login.tsx` vier Elemente zusätzlich zum Standard-Login-Card:

### 1. Login-Card (immer sichtbar, kein Demo-Spezifika)
Logo + „Anmelden"-Heading + E-Mail/Passwort-Felder + Anmelden-Button.
In Demo-Mode wird das Passwort-Feld per `setPassword()` automatisch vorbefüllt, sobald der
Visitor einen Account-Button anklickt — der Visitor muss nichts händisch eintippen.

### 2. Amber Demo-Banner (direkt unter dem Login-Card)
```
┌────────────────────────────────────┐
│  DEMO-UMGEBUNG                     │
│  Alle Daten werden nach 4 Stunden  │
│  automatisch zurückgesetzt.        │
└────────────────────────────────────┘
```
Styling: `border-amber-500/40 bg-amber-500/10`, Text `text-amber-400` (uppercase) +
`text-amber-300/80`. Kein Close-Button — der Banner ist dauerhaft sichtbar,
solange der Visitor auf der Login-Page ist.

### 3. Credentials-Card (unter dem Amber-Banner)
```
┌────────────────────────────────────┐
│  DEMO-ZUGÄNGE                      │
│  Zum Anmelden einfach einen Account│
│  auswählen.                        │
│                                    │
│  [Spinner]  Demo wird vorbereitet… │  ← nur während Lade-Phase
│                                    │
│  ┌──────────────────────────────┐  │
│  │ Admin                        │  │
│  │ admin@demo-a3f2b1c9.demo     │  │  ← Klick füllt E-Mail + Passwort vor
│  └──────────────────────────────┘  │
│  ┌──────────────────────────────┐  │
│  │ Analyst                      │  │
│  │ analyst@demo-a3f2b1c9.demo   │  │
│  └──────────────────────────────┘  │
└────────────────────────────────────┘
```
Styling: `border-brand/30 bg-brand/5`. Jeder Account-Button ist ein `<button type="button">`
mit `onClick={() => { setEmail(u.email); setPassword(u.password) }}`.
Passwort wird **nicht angezeigt** — nur die E-Mail als `font-mono`. Das Password-Feld im
Login-Card wird automatisch befüllt. Security-Rationale: das Passwort muss nicht sichtbar
sein; der Visitor klickt nur einen Button und dann „Anmelden".

Lade-Zustand: solange `/api/v1/demo/start` noch läuft (`demoStarting === true`), zeigt
die Card einen Mini-Spinner + „Demo wird vorbereitet…". Die Account-Buttons erscheinen
erst wenn `demoUsers != null`.

Fehler-Zustand: wenn `/demo/start` fehlschlägt, bleibt `demoUsers === null`, die
Buttons verschwinden, und ein Toast (`variant: 'error'`) erklärt, dass die Demo
gerade nicht verfügbar ist. Das Login-Formular bleibt benutzbar.

### 4. Disclaimer (unter der Credentials-Card)
```
Vakt ist ein self-hosted Produkt. Diese Demo läuft auf
einem öffentlichen Testserver — niemals echte Daten eingeben.
Mehr: sec.norvikops.de
```
Styling: `text-xs text-secondary text-center`. Link auf `https://sec.norvikops.de` (underline).

### Reihenfolge im DOM
```
<Login-Card>
{isDemo && (
  <>
    <Amber-Banner />
    <Credentials-Card />
    <Disclaimer />
  </>
)}
```

Alle vier Elemente erscheinen **nur** wenn `isDemo === true`. Wenn `isDemo === null`
(initialer Lade-Moment) oder `false` (keine Demo-Instanz), ist keines davon sichtbar.

## Wann `isDemo === false` / `null`

`useDemoMode()` fetcht `/health` und prüft `response.demo === true`.

| Szenario | `isDemo`-Wert | Sichtbare UI |
|----------|---------------|--------------|
| Backend antwortet, `demo: true` | `true` | alle 4 Elemente |
| Backend antwortet, `demo: false` | `false` | nur Login-Card |
| Backend down / CORS-Error | `false` | nur Login-Card |
| Fetch noch nicht abgeschlossen | `null` | nur Login-Card |

**Wenn ein Visitor nur den Login-Card ohne Demo-Sektion sieht, liegt KEIN Frontend-Bug vor.**
Es ist fast immer ein Server-Problem:
- `VAKT_DEMO=true` nicht gesetzt
- API-Container down oder restarted-loop
- CI-Fehler blockiert `deploy-demo` → altes Image läuft noch

## Diagnosepfad

```bash
# 1. Demo-Flag prüfen:
curl -s https://secdemo.norvikops.de/health | jq .demo
# Erwartet: true

# 2. Demo-Start testen:
curl -s -X POST https://secdemo.norvikops.de/api/v1/demo/start | jq .
# Erwartet: {admin_email, admin_password, ...}

# 3. CI-Status:
gh run list --branch main --limit 5
# Letzter CI-Run für 'deploy-demo'-Job muss success sein.
```

## Alternativen

- **Credentials in Klartext anzeigen** — verworfen. Das Passwort hat 64-Bit-Entropie und ist
  für Humans nicht lesbar (16 Hex-Chars). Sichtbar zu machen erzeugt unnötige Friction;
  der Click-to-Autofill-Flow ist klarer.
- **Separater Demo-Landing-Page** vor dem Login — verworfen. Eine eigene Route
  (`/demo`) würde den Setup-Flow (auth guard, redirect-after-login) verkomplizieren. Der
  Login-Screen mit Demo-Overlay ist der minimale Eingriffspunkt.
- **Statische Demo-Credentials im UI** (`admin@vakt.local / admin1234demo`) — verworfen.
  Widerspricht ADR-0015 (kein statischer Demo-Account). Auch: 16-hex-char Random-Passwort
  kann nicht statisch vorbekannt sein.
- **Auto-Login ohne Klick** (direkt nach `/demo/start` einloggen und weiterleiten) —
  verworfen. Der Visitor soll bewusst einen Account wählen (Admin vs. Analyst),
  um zu verstehen, mit welcher Rolle er die Demo startet.

## Konsequenzen

### Positive
- Das Design ist vollständig durch `Login.tsx` implementiert — kein separater Routing-Layer.
- Fehler-Zustand (Demo nicht verfügbar) degradiert graceful auf Standard-Login.
- Klares Schuldprinzip: fehlende Demo-Sektion = Server-/Deploy-Problem, nie Frontend-Bug.

### Negative
- Wenn `/health` und `/demo/start` BEIDE langsam sind, sieht der Visitor kurz nur den
  Login-Card. Kein Placeholder/Skeleton für diesen Moment (bewusste Entscheidung:
  kein Flash-of-missing-content bei schnellen Deployments).

### Neutrale
- `useDemoMode()` cached den `/health`-Wert module-level (`let cachedDemo`). Eine SPA-
  Navigation weg und zurück zur Login-Page ohne full reload trifft den Cache.
  Full-Reload setzt den Cache zurück.

## Referenzen

- Frontend: `frontend/src/pages/Login.tsx` — Implementierung
- Frontend: `frontend/src/shared/hooks/useDemoMode.ts` — `isDemo`-Hook
- ADR-0015 — Ephemere Demo-Sessions (Flow-Entscheidung)
- `docs/concepts/demo-flow.md` — technischer Flow
- `docs/wiki/demo-mode.md` — Customer-facing Demo-Setup-Anleitung
