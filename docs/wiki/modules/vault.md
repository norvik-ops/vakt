# Vakt Vault

Vakt Vault speichert Secrets verschlüsselt mit AES-256-GCM und protokolliert jeden Zugriff in einem unveränderlichen Audit-Log. Zusätzlich scannt das Modul Git-Repositories auf versehentlich eingecheckte Credentials und unterstützt manuelle sowie automatisch geplante Secret-Rotation. CI/CD-Pipelines können per API-Token auf Secrets zugreifen — ohne Benutzer-Account in Vakt.

---

## Aktivierung

Das Modul ist standardmäßig aktiv. Zum Deaktivieren:

```env
VAKT_MODULES_ENABLED=vaktcomply,vaktscan,vaktaware,vaktprivacy
```

---

## Konfiguration

| Variable | Pflicht | Beschreibung |
|----------|---------|--------------|
| `VAKT_SECRET_KEY` | Ja | 32-Byte Hex-Master-Key für AES-256-GCM-Verschlüsselung. Generieren mit `openssl rand -hex 32`. **Nie nach erstem Start ändern.** |

Der Master-Key verschlüsselt alle Secret-Values in der Datenbank. Ohne den richtigen Key sind die Daten nicht mehr lesbar. Den Key sicher aufbewahren (Passwortmanager, separates Vault-System).

---

## Konzept: Projekte und Umgebungen

Secrets sind in einer Hierarchie organisiert:

```
Projekt (z. B. "backend-api")
  └── Umgebung (dev / staging / prod)
        └── Secrets (Key-Value-Paare)
```

Das erlaubt es, die gleichen Secret-Namen in verschiedenen Umgebungen mit unterschiedlichen Werten zu führen.

---

## Secrets speichern und abrufen

Secret-Values werden AES-256-GCM-verschlüsselt abgelegt. In Listen erscheinen nur die Key-Namen — niemals die Values. Den Wert erhält man nur bei einem direkten Abruf, der im Audit-Log protokolliert wird.

Jeder Lese-Zugriff wird mit folgenden Informationen geloggt:

- Benutzername oder Token-Name
- IP-Adresse
- User-Agent
- Zeitstempel

Das Audit-Log ist nicht veränderbar und dient als Nachweis für regulatorische Anforderungen.

---

## Secret-Rotation

Rotation kann manuell oder automatisch nach einem konfigurierbaren Intervall (in Tagen) ausgelöst werden. Drei Rotations-Strategien stehen zur Auswahl:

| Strategie | Ergebnis |
|-----------|----------|
| `random_string` | Zufällige alphanumerische Zeichenkette |
| `uuid` | UUID v4 |
| `db_password` | Zufälliges Passwort im Datenbankformat |

Bei jeder Rotation wird automatisch ein Compliance-Nachweis in Vakt Comply erzeugt — für den entsprechenden Schlüsselmanagement-Control.

---

## Project Health Score

Pro Projekt berechnet Vakt einen Health-Score (0–100) basierend auf:

- Alter der Secrets (wie lange wurden sie nicht geändert?)
- Fehlende Rotation (welche Secrets haben kein Rotationsintervall?)
- Zugriffshäufigkeit (auffällig hohe Zugriffszahlen)

Konkrete Issues werden im Dashboard angezeigt.

---

## Share-Links

Für die einmalige, sichere Weitergabe eines Secrets an Dritte können Share-Links erzeugt werden:

- Einmalig verwendbar — nach dem ersten Abruf ungültig
- Zeitlich begrenzt (konfigurierbare Gültigkeit)
- Kein Vakt-Login erforderlich

Der Empfänger öffnet den Link im Browser und sieht den Secret-Value einmalig.

---

## API-Token für CI/CD

Für den Zugriff aus CI/CD-Pipelines gibt es scoped API-Tokens:

- Der Raw-Key wird nur einmalig bei der Erstellung angezeigt — danach nicht mehr
- Tokens können auf bestimmte Projekte und Umgebungen eingeschränkt werden
- Tokens haben ein optionales Ablaufdatum

Beispiel für eine CI/CD-Pipeline:

```bash
curl -H "Authorization: Bearer $VAKT_API_TOKEN" \
  https://vakt.meine-firma.de/api/v1/vaktvault/projects/backend/envs/prod/secrets/DATABASE_URL
```

---

## Import und Export

### Import

Massenimport aus:
- `.env`-Dateien
- HashiCorp Vault
- AWS Secrets Manager

### Export

Alle Secrets einer Umgebung als verschlüsselte Datei exportieren (für Backups oder Migration).

---

## Git-Repository-Scanner

Vakt Vault kann Git-Repositories auf versehentlich eingecheckte Credentials scannen (powered by [gitleaks](https://github.com/gitleaks/gitleaks)):

1. Repository-URL eingeben
2. Scan starten — läuft asynchron
3. Ergebnisse abrufen

Findings zeigen:
- Pfad und Zeilennummer der betroffenen Datei
- Name des ausgelösten Musters (z. B. "AWS Access Key")
- Redaktierten Wert (erste 4 und letzte 4 Zeichen sichtbar: `AKIA...WXYZ`)
- Schweregrad

Einzelne Ergebnisse können als False Positive verworfen werden.

---

## Compliance-Mapping

| Standard | Control |
|----------|---------|
| NIS2 Art. 21 Abs. 2i | Zugangskontrollen und Authentifizierung |
| NIS2 Art. 21 Abs. 2j | Kryptographie und Schlüsselmanagement |
| ISO 27001:2022 A.8.13 | Informationssicherung |
| ISO 27001:2022 A.8.24 | Kryptographische Verfahren |
| BSI IT-Grundschutz ORP.4 | Identitäts- und Berechtigungsmanagement |

---

## Hintergrund-Jobs

| Job | Auslöser | Beschreibung |
|-----|----------|--------------|
| `vaktvault:git_scan` | API-Aufruf | Git-Repository asynchron mit gitleaks scannen |
