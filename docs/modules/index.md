# Vakt — Modul-Dokumentation

Jedes Modul kann über `VAKT_MODULES_ENABLED` unabhängig aktiviert oder deaktiviert werden.

```env
VAKT_MODULES_ENABLED=vaktscan,vaktcomply,vaktvault,vaktaware,vaktprivacy
```

---

## Module

| Modul | Datei | Zweck |
|---|---|---|
| **Vakt Comply** (`vaktcomply`) | [vaktcomply.md](vaktcomply.md) | Compliance-Hub: Controls, Risiken, Vorfälle, Richtlinien, KI-Berichte |
| **Vakt Scan** (`vaktscan`) | [vaktscan.md](vaktscan.md) | Scanner-Orchestrierung: Trivy, Nuclei, OpenVAS, BSI CERT-Bund |
| **Vakt Vault** (`vaktvault`) | [vaktvault.md](vaktvault.md) | Secrets-Management: AES-256, Git-Scanner, CI/CD-Integration |
| **Vakt Aware** (`vaktaware`) | [vaktaware.md](vaktaware.md) | Security Awareness: Phishing-Simulationen, Micro-Trainings |
| **Vakt Privacy** (`vaktprivacy`) | [vaktprivacy.md](vaktprivacy.md) | DSGVO-Dokumentation: VVT, DPIA, AVV, Datenpannen, DSR |

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
Vakt Scan    →  Finding geschlossen (vaktscan:auto_evidence)    →  Vakt Comply (Patch-Management-Controls)
Vakt Vault   →  Secret rotiert (Rotation-Workflow)              →  Vakt Comply (Access-Control-Controls)
Vakt Aware   →  Training abgeschlossen (Completion-Workflow)    →  Vakt Comply (Awareness-Controls)
Vakt Privacy →  DSR abgeschlossen (UpdateDSR completed)         →  Vakt Comply (Privacy-Controls)
Vakt Privacy →  Datenpanne angelegt (vaktprivacy:breach_incident) →  Vakt Comply (Incident Register)
```

Der Mechanismus: Module stellen Asynq-Tasks in die Queue. Der Worker verarbeitet diese asynchron und schreibt die Evidenz in Vakt Comply — ohne direkten DB-Cross-Read (Modul-Isolation).

---

## API-Pfade

| Modul | Basis-Pfad |
|---|---|
| Vakt Comply | `/api/v1/vaktcomply/` |
| Vakt Scan | `/api/v1/vaktscan/` |
| Vakt Vault | `/api/v1/vaktvault/` |
| Vakt Aware | `/api/v1/vaktaware/` |
| Vakt Privacy | `/api/v1/vaktprivacy/` |

Alle Pfade erfordern einen gültigen Paseto-Token (`Authorization: Bearer <token>`), außer öffentlich markierte Endpunkte (Trust Center, Phishing-Tracking-Pixel, Auditor-Portal).

---

## Weiterführende Dokumentation

- [Setup & Deployment](../setup.md)
- [Konfigurationsreferenz](../configuration.md)
- [Architektur](../architecture.md)
- [Produkt-Anforderungen](../prd.md)
