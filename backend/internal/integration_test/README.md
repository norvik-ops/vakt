# Cross-Module Integration Tests

Diese Tests prüfen Verhaltensweisen, die über Modul-Grenzen hinweg gehen:

- HR Checklist-Run wird abgeschlossen → SecVitals erhält Evidenz (`ck_evidence` Eintrag)
- SecPulse Finding-Status ändert sich → Webhook feuert
- SecPrivacy Breach-Event → SecVitals Incident erstellt
- Auth-Logout → Token in Redis-Deny-List → nachfolgende Requests 401

Sie sind **bewusst getrennt** von den per-Modul Unit-Tests (`internal/modules/<m>/*_test.go`), weil sie:

- Eine echte Postgres + Redis hochfahren (testcontainers-go)
- Mehrere Module gleichzeitig instanzieren
- Deutlich langsamer sind (~30 s pro Run)

CI-Strategie: laufen nur auf `main` und vor Releases, nicht auf jedem PR.

## Voraussetzungen

- Docker oder Podman muss verfügbar sein (testcontainers-go nutzt das daemon).
- Go 1.22+

## Pattern

```go
//go:build integration

package integration_test

import (
    "context"
    "testing"

    "github.com/testcontainers/testcontainers-go/modules/postgres"
    "github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestHRChecklistCompletion_CreatesEvidence(t *testing.T) {
    ctx := context.Background()

    // 1) Postgres + Redis hochfahren
    pgC, err := postgres.Run(ctx, "postgres:16-alpine",
        postgres.WithDatabase("vakt_test"),
        postgres.WithUsername("vakt"),
        postgres.WithPassword("test"),
    )
    if err != nil { t.Fatal(err) }
    defer pgC.Terminate(ctx)

    // 2) Migrationen anwenden
    // 3) Module wire-up (HR + SecVitals)
    // 4) Aktion ausführen (Checklist-Run abschließen)
    // 5) Assert: ck_evidence Eintrag existiert
}
```

## Build-Tag

Alle Files in diesem Verzeichnis tragen `//go:build integration`. So laufen sie nicht bei `go test ./...` mit, sondern nur bei `go test -tags=integration ./internal/integration_test/...`.

## Status

Aktuell als **Skeleton + Pattern-Doku** angelegt. Test-Cases werden hinzugefügt, sobald konkrete Cross-Modul-Bug-Klassen auftreten — siehe ADR-0012 (risiko-priorisiertes Testen statt Coverage-Quote).

Erste Kandidaten (in Reihenfolge des Risikos):

1. **HR → SecVitals Evidence** — beim Run-Complete wird ein `ck_evidence` Eintrag erzeugt. Regression: Refactor von `HREvidenceWriter` würde stillschweigend brechen.
2. **Auth-Logout → Deny-List → 401** — Race-Conditions zwischen Token-Generierung und Deny-List wurden in der Security-Wave gefunden; ein Integration-Test schließt das Re-Auftreten aus.
3. **SecPrivacy Breach → SecVitals Incident** — der Cross-Module-Event-Bus läuft aktuell nicht aktiv (interface ist da, Wire-Up fehlt). Bei Aktivierung: Test als erster Schritt.

## Wann ergänzen?

- Beim ersten konkreten Bug der Modulgrenzen überschritt
- Vor jedem Release-Tag (smoke-Suite)
- Wenn ein Cross-Modul-Refactor ansteht (z.B. neuer Event-Bus)

Globaler vorab-Aufbau aller theoretisch möglichen Cross-Modul-Tests ist explizit **nicht** das Ziel — das wäre Coverage-Junk. Tests werden hinzugefügt, sobald sie einen konkreten Wert (Regression schließen / Bug verhindern) versprechen.
