# ADR-0015: Ephemere Demo-Sessions pro Visitor

**Status:** Accepted
**Datum:** 2026-05-20
**Entscheider:** Stefan Moseler

## Kontext

Die öffentliche Vakt-Demo läuft auf `secdemo.norvikops.de` und soll Interessenten erlauben, die Plattform „mal kurz auszuprobieren" — ohne Registrierung, ohne Installation, ohne Datenrückstände bei anderen Besuchern.

Eine geteilte Demo-Instanz (alle Besucher loggen mit dem gleichen `admin@vakt.local`-User in dieselbe Org) hat mehrere Probleme:

- Hardcoded Credentials in einem öffentlich erreichbaren System sind ein Brute-Force-Ziel — selbst wenn der Login auf 9-stellige Passwörter limitiert ist, kann ein einziger Angreifer per Skript dauerhaft die Demo-Daten ändern.
- Besucher sehen sich gegenseitig: jeder Klick eines Sales-Prospects manipuliert Demo-Daten, die der nächste Besucher zu sehen bekommt. Vorführungen wirken willkürlich.
- Zurücksetzen über einen Daily-Reset-Job kontaminiert die Demo zwischen Reset-Zeitfenstern.
- Hardcoded Passwörter müssen mit der Passwort-Stärke-Policy kompatibel sein. Die Auth-Validation in `service.go` verlangt min 10 Zeichen — der historische Demo-Default `admin1234` (9 Zeichen) wurde dadurch unbenutzbar und musste mehrfach gefixt werden (siehe CHANGELOG v0.6.2).

## Entscheidung

Wir nutzen **eine eigene ephemere Organisation pro Besucher** als Demo-Login-Mechanismus.

Der Flow:

1. Frontend ruft beim Mount der Login-Page im Demo-Modus automatisch `POST /api/v1/demo/start` auf.
2. Backend (`internal/shared/demo/start.go` → `demoseed.RunEphemeral`) legt eine neue Org mit Random-Slug `demo-XXXXXXXX` (8 hex chars) an, inkl. Admin- und Analyst-User mit Random-Emails (`admin@demo-XXXXXXXX.demo`) und 16-stelligen Random-Hex-Passwörtern via `crypto/rand`.
3. Backend antwortet mit `{admin_email, admin_password, analyst_email, analyst_password, expires_in: 14400}` — Klartext-Passwörter verlassen den Server **genau einmal**, danach existieren nur Bcrypt-Hashes (Cost 12) in der DB.
4. Frontend zeigt die Credentials in der Login-Form als Auto-Fill an.
5. Asynq-Cleanup-Job (`internal/shared/demo/cleanup.go`) löscht alle ephemeren Orgs **nach 4 Stunden** Lebensdauer.

## Alternativen

- **Geteilte Demo-Org mit statischen Credentials** (`admin@vakt.local / admin1234`) — verworfen wegen Brute-Force-Risiko und gegenseitiger Sichtbarkeit der Besucher. Bleibt für lokale Dev-Setups via `demoseed.Run()` erhalten, aber **niemals** auf öffentlich erreichbaren Instanzen empfehlen.
- **Pro-Visitor-Container** (eigener Docker-Compose-Stack pro Demo) — verworfen wegen Komplexität (Orchestrierung, Port-Management, Cleanup, Cold-Start-Zeit > 30 s pro Besucher).
- **Anonyme „View-Only"-Demo** (kein Login, alle Daten read-only) — verworfen weil Vakt-Features (Risk-Editing, CAPA-Workflows, Evidence-Upload) nur write-fähig sinnvoll vorführbar sind.
- **24h statt 4h Lebensdauer** — verworfen, weil die Demo-DB sonst zu schnell aufbläht und 4 h für eine Eval-Session völlig ausreicht.

## Konsequenzen

### Positive

- Besucher haben isolierten, manipulationsfreien Spielraum — gut für Vertriebs-Demos und Self-Eval-Walkthroughs.
- Kein Brute-Force-Risiko: jeder Login ist eine 64-Bit-Random-Combo, nach 4 h gelöscht.
- Kein Bedarf an Daily-Reset-Cron: Asynq-Job räumt natürlich auf.
- Demo-Code wird durch echte Last (jede Org mit eigenem Seed) getestet — Performance-Probleme im Seed fallen früh auf.

### Negative

- Pro Besucher ein voller Seed-Lauf (Migrations + ~50 Findings + Frameworks). Dauer ~5–15 s auf der Demo-VM. Rate-Limit auf `/api/v1/demo/start` (5 req/min pro IP) verhindert Flooding.
- DB-Footprint wächst während der 4-h-Fenster proportional zur Besucherzahl. Eine populäre Launch-Demo könnte 100+ Orgs gleichzeitig haben — Demo-VM braucht entsprechend Headroom.
- Klartext-Passwort muss einmal über die Leitung (HTTPS). Wenn der Visitor das Tab schließt bevor er sich einloggt, ist die ephemere Org „verloren" und wird erst nach 4 h gecleant — kleiner Waste, akzeptabel.
- Custom-Domains in der Test-Email (`@demo-XXXXXXXX.demo`) sind syntaktisch gültige aber nicht zustellbare Adressen — Vakt-Aware-Tests könnten in der Demo-Org fehlschlagen. Akzeptabel, weil Aware in der Demo ohnehin als „Read-only Demonstration" gedacht ist.

### Neutrale

- Wenn die Demo-Instanz später eine Pro-Tier-Showcase bekommen soll, gilt dieselbe Architektur — die ephemere Org einfach mit `org.plan = 'pro'` markieren.
- Cleanup-Job läuft alle 15 Minuten (siehe `internal/shared/demo/cleanup.go`). Effektive Lebensdauer einer Demo-Org ist also zwischen 4 h und 4 h 15 min.

## Anti-Patterns / häufige Verwirrung

Diese Punkte sind das, was in der Vergangenheit mehrfach falsch dokumentiert wurde:

- **`admin@vakt.local / admin1234` ist KEIN Demo-Login.** Das war ein historischer statischer Seed (`demoseed.Run`), der für lokale Dev-Tests existiert. Auf `secdemo.norvikops.de` wird er nie verwendet. Trotzdem wurde er versehentlich in Marketing-Texten und Doku zitiert — siehe CHANGELOG v0.6.2 für die Korrektur.
- **Demo-Credentials erscheinen niemals statisch in einer Doku-Tabelle.** Sie kommen pro Visitor aus dem `/api/v1/demo/start`-Response.
- **Wenn die Demo „Login funktioniert nicht" meldet**, ist die Ursache fast immer eines davon:
  1. API-Container in restart-Loop (gescheiterte Migration) → `/demo/start` antwortet nicht
  2. Backend gibt Passwörter nicht zurück (Regression in `start.go`)
  3. Frontend hardcodet falsche Passwörter (Regression in `Login.tsx`)
  4. Auth-Validation lehnt erzeugte Passwörter ab (z.B. wenn min-Length-Policy nicht zur Demo-Seed-Länge passt)

## Referenzen

- Backend: `backend/internal/shared/demo/start.go`, `backend/internal/shared/demoseed/seed.go`, `backend/internal/shared/demo/cleanup.go`
- Frontend: `frontend/src/pages/Login.tsx`
- Wiki: `docs/wiki/demo-mode.md`
- CHANGELOG: `[v0.6.2] — Behoben` Abschnitt
- Wiki: Demo-Betrieb in `docs/wiki/demo-mode.md` beschrieben
