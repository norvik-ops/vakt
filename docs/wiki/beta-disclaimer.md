# Vakt — Early Access: Status, Support & Erwartungsmanagement

**Status: Early Access.** Vakt ist funktional umfangreich und wird aktiv
weiterentwickelt, befindet sich aber noch in der **Early-Access-Phase**. Dieses
Dokument macht transparent, was das für Support, Betrieb und
Geschäftskontinuität bedeutet — damit es vorab klar ist, statt im Fragebogen.

## Was „Early Access" bedeutet

- **Funktionsumfang:** breit und produktiv nutzbar (NIS2, ISO 27001:2022, BSI
  IT-Grundschutz, DSGVO), aber einzelne Workflows können sich noch ändern.
- **Stabilität:** Kern-Pfade (Auth, Krypto, Migrationen, Audit-Log) sind getestet
  und durch CI-Gates abgesichert. Trotzdem gilt Beta-Vorbehalt: vor Produktivnutzung
  eigene Tests fahren und **Backups** einrichten (siehe unten).
- **Breaking Changes:** werden im `CHANGELOG.md` dokumentiert; Migrationen laufen
  über `golang-migrate`. Vor Upgrades das Changelog lesen.

## Support — Erwartungsmanagement

- **Best-Effort-Support, keine zugesicherte Reaktionszeit, kein 24/7-SLA.**
  Im Early Access gibt es **keine** vertragliche Service-Level-Zusage.
- Kanal: Issues/Anfragen werden best-effort und in der Regel innerhalb weniger
  Werktage bearbeitet — ohne Garantie.
- Sicherheitsrelevante Meldungen siehe `SECURITY.md` /
  `/.well-known/security.txt`.

## Datensicherung — Verantwortung des Betreibers

Vakt ist **self-hosted**: alle Daten liegen in deiner Infrastruktur. Die
Datensicherung liegt damit in **deiner** Verantwortung.

- Nutze die mitgelieferten Skripte `scripts/backup.sh` / `restore.sh` /
  `backup-verify.sh` (signiert + verschlüsselt) und richte geplante Backups ein.
- Teste den **Restore-Pfad** regelmäßig — ein ungetestetes Backup ist kein Backup.
- Runbook: [`docs/runbooks/disaster-recovery.md`](../runbooks/disaster-recovery.md)
  (inkl. letztem verifizierten Restore-Drill + gemessener RTO).

## Geschäftskontinuität (ehrlich, kein Vertrag)

Vakt wird aktuell maßgeblich von **einer Schlüsselperson** entwickelt und
betrieben (niedriger „Bus-Faktor"). Das ist im Early Access vertretbar,
ist aber transparent zu nennen:

- Der Quellcode ist **source-available** (Elastic License v2). Selbst im Fall, dass
  der Maintainer ausfällt, bleibt dein Self-Hosted-Betrieb lauffähig, und der Code
  ist auditierbar/forkbar — es gibt keine proprietäre Cloud-Abhängigkeit.
- Es gibt **keine Telemetrie** und **keine** zentrale Norvik-Infrastruktur, von der
  dein Betrieb abhängt. Die einzige Verbindung zu uns ist die Pro-Lizenz-Erneuerung
  (nur der Lizenz-Token, bei Jahreslizenz ~1×/Jahr, abschaltbar mit
  `VAKT_LICENSE_AUTORENEW=false`) — und selbst wenn wir morgen verschwinden, läuft
  dein Schlüssel bis zum Ende des Zeitraums, den du bezahlt hast. Die Community
  Edition ruft nie an.

## Kurzfassung für Procurement/Security-Fragebögen

> Vakt befindet sich in Early Access. Es gibt aktuell keinen vertraglichen
> Support-SLA (Best-Effort). Datensicherung und Restore liegen beim Betreiber
> (Skripte + Runbook mitgeliefert). Der Code ist source-available (ELv2), self-
> hosted, ohne Telemetrie — Geschäftskontinuität ist dadurch unabhängig vom
> Anbieter gegeben.
