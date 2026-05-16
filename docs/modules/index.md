# SecHealth — Modul-Dokumentation

Jedes Modul kann über `VAKT_MODULES_ENABLED` unabhängig aktiviert oder deaktiviert werden.

```env
VAKT_MODULES_ENABLED=secpulse,secvitals,secvault,secreflex,secprivacy
```

---

## Module

| Modul | Datei | Zweck |
|---|---|---|
| **SecVitals** | [secvitals.md](secvitals.md) | Compliance-Hub: Controls, Risiken, Vorfälle, Richtlinien, KI-Berichte |
| **SecPulse** | [secpulse.md](secpulse.md) | Scanner-Orchestrierung: Trivy, Nuclei, OpenVAS, BSI CERT-Bund |
| **SecVault** | [secvault.md](secvault.md) | Secrets-Management: AES-256, Git-Scanner, CI/CD-Integration |
| **SecReflex** | [secreflex.md](secreflex.md) | Security Awareness: Phishing-Simulationen, Micro-Trainings |
| **SecPrivacy** | [secprivacy.md](secprivacy.md) | DSGVO-Dokumentation: VVT, DPIA, AVV, Datenpannen, DSR |

---

## Architektur-Prinzipien

### Modul-Isolation
Kein Modul importiert direkt aus einem anderen Modul. Die Kommunikation erfolgt ausschließlich über:
- Shared Services (`internal/shared/`)
- Asynq-Tasks (asynchrone Ereignisse)
- Gemeinsame Event-Interfaces

### Datenbank-Präfixe
Jedes Modul hat sein eigenes Tabellen-Präfix, um Schema-Konflikte zu vermeiden:

| Modul | Präfix |
|---|---|
| SecVitals | `ck_` |
| SecPulse | `vb_` |
| SecVault | `so_` |
| SecReflex | `pg_` |
| SecPrivacy | `po_` |

### Cross-Modul-Evidenz

Alle Module erzeugen automatisch Compliance-Evidenz in SecVitals:

```
SecPulse   →  Finding geschlossen (secpulse:auto_evidence)    →  SecVitals (Patch-Management-Controls)
SecVault   →  Secret rotiert (Rotation-Workflow)              →  SecVitals (Access-Control-Controls)
SecReflex  →  Training abgeschlossen (Completion-Workflow)    →  SecVitals (Awareness-Controls)
SecPrivacy →  DSR abgeschlossen (UpdateDSR completed)         →  SecVitals (Privacy-Controls)
SecPrivacy →  Datenpanne angelegt (secprivacy:breach_incident) →  SecVitals (Incident Register)
```

Der Mechanismus: Module stellen Asynq-Tasks in die Queue. Der Worker verarbeitet diese asynchron und schreibt die Evidenz in SecVitals — ohne direkten DB-Cross-Read (Modul-Isolation).

---

## API-Pfade

| Modul | Basis-Pfad |
|---|---|
| SecVitals | `/api/v1/secvitals/` |
| SecPulse | `/api/v1/secpulse/` |
| SecVault | `/api/v1/secvault/` |
| SecReflex | `/api/v1/secreflex/` |
| SecPrivacy | `/api/v1/secprivacy/` |

Alle Pfade erfordern einen gültigen Paseto-Token (`Authorization: Bearer <token>`), außer öffentlich markierte Endpunkte (Trust Center, Phishing-Tracking-Pixel, Auditor-Portal).

---

## Weiterführende Dokumentation

- [Setup & Deployment](../setup.md)
- [Konfigurationsreferenz](../configuration.md)
- [Architektur](../architecture.md)
- [Produkt-Anforderungen](../prd.md)
