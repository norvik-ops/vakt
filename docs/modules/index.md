# Vakt — Modul-Dokumentation

Jedes Modul kann über `VAKT_MODULES_ENABLED` unabhängig aktiviert oder deaktiviert werden.

```env
VAKT_MODULES_ENABLED=secpulse,secvitals,secvault,secreflex,secprivacy
```

---

## Module

| Modul | Datei | Zweck |
|---|---|---|
| **Vakt Comply** (`secvitals`) | [secvitals.md](secvitals.md) | Compliance-Hub: Controls, Risiken, Vorfälle, Richtlinien, KI-Berichte |
| **Vakt Scan** (`secpulse`) | [secpulse.md](secpulse.md) | Scanner-Orchestrierung: Trivy, Nuclei, OpenVAS, BSI CERT-Bund |
| **Vakt Vault** (`secvault`) | [secvault.md](secvault.md) | Secrets-Management: AES-256, Git-Scanner, CI/CD-Integration |
| **Vakt Aware** (`secreflex`) | [secreflex.md](secreflex.md) | Security Awareness: Phishing-Simulationen, Micro-Trainings |
| **Vakt Privacy** (`secprivacy`) | [secprivacy.md](secprivacy.md) | DSGVO-Dokumentation: VVT, DPIA, AVV, Datenpannen, DSR |

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
| Vakt Comply | `ck_` |
| Vakt Scan | `vb_` |
| Vakt Vault | `so_` |
| Vakt Aware | `pg_` |
| Vakt Privacy | `po_` |

### Cross-Modul-Evidenz

Alle Module erzeugen automatisch Compliance-Evidenz in Vakt Comply:

```
Vakt Scan    →  Finding geschlossen (secpulse:auto_evidence)    →  Vakt Comply (Patch-Management-Controls)
Vakt Vault   →  Secret rotiert (Rotation-Workflow)              →  Vakt Comply (Access-Control-Controls)
Vakt Aware   →  Training abgeschlossen (Completion-Workflow)    →  Vakt Comply (Awareness-Controls)
Vakt Privacy →  DSR abgeschlossen (UpdateDSR completed)         →  Vakt Comply (Privacy-Controls)
Vakt Privacy →  Datenpanne angelegt (secprivacy:breach_incident) →  Vakt Comply (Incident Register)
```

Der Mechanismus: Module stellen Asynq-Tasks in die Queue. Der Worker verarbeitet diese asynchron und schreibt die Evidenz in Vakt Comply — ohne direkten DB-Cross-Read (Modul-Isolation).

---

## API-Pfade

| Modul | Basis-Pfad |
|---|---|
| Vakt Comply | `/api/v1/secvitals/` |
| Vakt Scan | `/api/v1/secpulse/` |
| Vakt Vault | `/api/v1/secvault/` |
| Vakt Aware | `/api/v1/secreflex/` |
| Vakt Privacy | `/api/v1/secprivacy/` |

Alle Pfade erfordern einen gültigen Paseto-Token (`Authorization: Bearer <token>`), außer öffentlich markierte Endpunkte (Trust Center, Phishing-Tracking-Pixel, Auditor-Portal).

---

## Weiterführende Dokumentation

- [Setup & Deployment](../setup.md)
- [Konfigurationsreferenz](../configuration.md)
- [Architektur](../architecture.md)
- [Produkt-Anforderungen](../prd.md)
