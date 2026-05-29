# ADR-0007: Betriebsrat-Modus — Anonymisierung beim Schreiben

**Status:** Accepted  
**Datum:** 2026-05-20

## Kontext

Vakt Aware (Package `vaktaware`) führt Phishing-Simulationen mit Mitarbeitern durch. §87 BetrVG Abs. 1 Nr. 6 verlangt Mitbestimmung des Betriebsrats bei „Einführung und Anwendung von technischen Einrichtungen, die dazu bestimmt sind, das Verhalten oder die Leistung der Arbeitnehmer zu überwachen".

Viele DACH-Kunden brauchen daher eine **anonyme** Variante der Phishing-Simulation, in der weder Klickdaten noch IP-Adressen einzelnen Mitarbeitern zugeordnet werden können.

Die ursprüngliche Implementierung speicherte alle Tracking-Events vollständig in `pg_events` (IP, User-Agent, target_id) und versteckte personenbezogene Auswertung nur im PDF-Export. Das ist **rechtlich unzureichend**: der Betriebsrat müsste der Speicherung zustimmen, nicht nur der Anzeige.

## Entscheidung

**Anonymisierung greift beim Schreiben**, nicht beim Anzeigen. Bei `pg_campaigns.betriebsrat_mode = true`:

- IP-Adresse und User-Agent werden gar nicht erst in `pg_events` geschrieben (leerer String/NULL).
- `target_id` wird auf NULL gesetzt — keine Verknüpfung zu einer konkreten Person.
- Department-Aggregat bleibt erhalten — für Statistik („Marketing: 3 von 10 haben geklickt").

Eine spätere Umstellung des Betriebsrat-Modus kann keine Daten zurückholen, die nie gespeichert wurden. Das ist datenschutzrechtlich belastbarer als ein Display-Filter.

## Alternativen

- **Display-Filter im PDF-Export** (ursprüngliche Implementierung) — verworfen: Daten existieren weiterhin in der DB, Betriebsrat-Zustimmung wäre für die Speicherung nötig.
- **Krypto-Anonymisierung** (z.B. HMAC der Personen-ID) — verworfen: zu komplex, mit ausreichend Kontext (Kampagne + Department) ggf. dennoch re-identifizierbar.
- **Pro-Kampagne wählbar mit Default false** — verworfen: Default-Sicher heißt opt-in für personenbezogene Daten, nicht opt-out. Default ist `true` (anonym).

## Konsequenzen

### Positive

- BetrVG-konform: keine personenbezogenen Daten ohne Vereinbarung.
- DSGVO Art. 5 (1c) Datenminimierung explizit eingehalten.
- Audit-fest gegenüber DPO-Reviews.

### Negative

- Im Anonym-Modus ist Re-Test einzelner Personen nicht möglich („Wer hat geklickt, dem schicke ich Training") — bewusste Trade-off-Entscheidung.

### Neutrale

- Die `anonymizeForBetriebsrat`-Funktion in `service.go` ist die einzige Code-Stelle, die diese Logik anwendet — zentralisiert, einfach zu auditieren.

## Referenzen

- `backend/internal/modules/vaktaware/service.go` — `anonymizeForBetriebsrat()`
- `backend/internal/modules/vaktaware/service_test.go` — Tests für on/off/empty
- `docs/wiki/modules/aware.md` — Kunden-Dokumentation der Garantie
- §87 BetrVG Abs. 1 Nr. 6
- DSGVO Art. 5 (1c)
