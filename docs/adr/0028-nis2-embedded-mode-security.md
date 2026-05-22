# ADR-0028: NIS2 Embedded-Mode Security — frame-ancestors * bewusst gewählt

**Status:** Accepted
**Datum:** 2026-05-22
**Entscheider:** Stefan (Maintainer)

## Kontext

Sprint 28 (S28-1) hat den NIS2-Self-Assessment-Wizard um einen Embedded-Mode erweitert:
Partner und Kunden können den Wizard via `<iframe>` in ihre eigenen Websites einbetten
(`/nis2-check`-Route + `/api/v1/public/nis2-assessment`-Endpoints).

Das globale Secure-Middleware-Setup setzt für alle Vakt-Routen:
- `X-Frame-Options: DENY`
- `Content-Security-Policy: frame-ancestors 'none'`

Für die Embedded-Routen muss davon abgewichen werden.

## Entscheidung

Für alle Pfade unter `/nis2-check*` und `/api/v1/public/nis2-assessment*` werden:

1. `X-Frame-Options`-Header entfernt
2. `Content-Security-Policy` überschrieben auf:
   ```
   frame-ancestors *
   ```
   (erlaubt Einbettung von beliebigen Origins)
3. `Referrer-Policy: strict-origin-when-cross-origin` gesetzt, um Leak des Vakt-Hostnames bei Navigation aus dem iframe zu minimieren

## Angriffsvektoren und Akzeptanz

### Clickjacking
**Risiko:** Ein Angreifer bettet `/nis2-check` in eine täuschende Seite ein und verleitet
Nutzer zu Klicks.

**Akzeptanz:** Der NIS2-Embedded-Wizard hat **keine autentifizierten Aktionen** — er ist
rein dateneingabe-basiert und erzeugt nur kurzlebige Assessment-Sessions. Es gibt keine
Button-Interaktion, die einen angemeldeten Nutzer schädigen könnte. Clickjacking-Angriffe
auf diesen Wizard hätten keinen praktischen Nutzen für einen Angreifer.

Für alle anderen Vakt-Seiten (Dashboard, Controls, Incidents, etc.) bleibt `frame-ancestors 'none'` aktiv.

### Information Disclosure via Referrer
**Risiko:** Beim Klick auf externe Links aus dem iframe könnte der Referer-Header den
Vakt-Hostname an Dritte leaken.

**Mitigation:** `Referrer-Policy: strict-origin-when-cross-origin` — sendet nur den
Origin (kein Pfad) bei Cross-Origin-Requests. Für Same-Origin-Requests bleibt der
vollständige Referrer.

### Cross-Origin-Datenexfiltration
**Risiko:** Eingebettete Seite kann via postMessage oder andere Mechanismen Daten an
beliebige Origins senden.

**Akzeptanz:** Der Embedded-Wizard hat **keinen Zugriff auf Vakt-interne Daten** —
er ist eine öffentliche, nicht-authentifizierte Route. Die Assessment-Daten gehören
dem eingebettenden Partner.

### Session-Fixation via iframe
**Risiko:** Cookies könnten über iframe gesetzt werden und eine Session vorbereiten.

**Mitigation:** Paseto-Tokens (kein Cookie-basiertes Auth) für Vakt-intern. Der
Embedded-Wizard hat eigene kurzlebige Assessment-Sessions (`session_token` im
Response-Body), nicht Cookie-basiert.

## Bewusst nicht abgeminderte Risiken

- `frame-ancestors *` erlaubt jedem beliebigen Host die Einbettung — kein Allowlist für
  bekannte Partner. Begründung: Der Wizard ist als öffentliches Marketing-Asset gedacht
  ("jeder kann einbetten"). Eine Allowlist würde den Mehrwert für Partner zerstören.
- Origin-Validation für Partner-Requests ist nicht implementiert.

## Alternativen verworfen

**`frame-ancestors 'self' https://partner1.de https://partner2.de`** — wurde verworfen,
da eine statische Allowlist den Selbst-Deploy-Charakter des Produkts zerstört. Jeder
Vakt-Betreiber hätte seine eigenen Partner eintragen müssen.

**Separater öffentlicher Endpunkt ohne Vakt-Origin** — zu hoher Aufwand für das
aktuelle Stadium des Produkts.

## Implementierung

`backend/cmd/api/main.go` — Middleware nach dem NIS2-Route-Registration-Block:

```go
e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        p := c.Request().URL.Path
        isNIS2Public := strings.HasPrefix(p, "/nis2-check") ||
            strings.HasPrefix(p, "/api/v1/public/nis2-assessment")
        if isNIS2Public {
            c.Response().Header().Del("X-Frame-Options")
            c.Response().Header().Set("Content-Security-Policy",
                "default-src 'self'; script-src 'self'; ...; frame-ancestors *; ...")
            c.Response().Header().Set("Referrer-Policy",
                "strict-origin-when-cross-origin")
        }
        return next(c)
    }
})
```

## Konsequenzen

- Partner können `/nis2-check` ohne Rückfrage einbetten
- Vakt-interne Seiten bleiben vollständig gegen Clickjacking geschützt
- Bei zukünftiger Erweiterung des Embedded-Modes um authentifizierte Features **muss**
  dieser ADR neu bewertet werden und ggf. eine Origin-Allowlist eingeführt werden
