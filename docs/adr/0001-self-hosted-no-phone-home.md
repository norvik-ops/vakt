# ADR-0001: Self-Hosted Architektur ohne Phone-Home

**Status:** Accepted  
**Datum:** 2026-02-01

## Kontext

Vakt richtet sich an KMU und MSPs im DACH-Raum, die DSGVO-konform Compliance dokumentieren müssen. Wettbewerber (Vanta, Drata, Tugboat Logic) laufen multi-tenant Cloud. Das erfordert einen AVV mit dem Anbieter (Art. 28 DSGVO) und exportiert Compliance-Daten — die ihrerseits wieder Compliance-Daten sind — in ein US-Cloud-Konto. Viele DACH-Kunden lehnen das ab.

## Entscheidung

Vakt ist ausschließlich self-hosted. Jede Instanz läuft komplett in der Infrastruktur des Kunden. Es gibt **keinerlei** ausgehende Verbindungen zu Norvik-Servern, weder Telemetrie noch Lizenz-Heartbeat noch Update-Check (außer optional, opt-in als reiner GET-Request auf die öffentliche GitHub-API).

## Alternativen

- **Multi-Tenant SaaS** — verworfen: erfordert AVV mit Norvik, scheitert am DACH-DSGVO-Argument.
- **Hybrid (self-hosted + Cloud-Komponenten)** — verworfen: jede Cloud-Komponente untergräbt das Kernversprechen. Cleane Trennung ist verkaufbar, hybride ist es nicht.
- **Phone-Home für Lizenz-Validierung** — verworfen: macht die Instanz vom Norvik-Server abhängig. Lizenzen sind kryptographisch signiert (ECDSA-P256) und werden offline validiert.

## Konsequenzen

### Positive

- Klares Differenzierungsmerkmal gegenüber US-Anbietern.
- DSGVO-Vorteil ohne AVV mit Norvik.
- Kunden behalten volle Kontrolle über Backup/Audit-Logs.

### Negative

- Kein zentrales Monitoring der Kunden-Instanzen → Support läuft per Kunden-Bug-Report, nicht proaktiv.
- Keine Telemetrie → Feature-Priorisierung basiert auf Interviews/Surveys, nicht auf Analytics.
- Updates erfordert Kunden-Aktion (oder Watchtower opt-in).

### Neutrale

- Lizenzvalidierung muss offline funktionieren — Architektur ist darauf ausgerichtet.

## Referenzen

- `docs/architecture.md`
- ADR-0008 (MSP-Portal-Verzicht — direkte Konsequenz)
