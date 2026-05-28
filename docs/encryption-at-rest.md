# Verschlüsselung at-Rest

Vakt selbst verschlüsselt alle anwendungssensitiven Geheimnisse mit AES-256-GCM
(VAKT_SECRET_KEY → Vakt Vault, OIDC-Refresh-Tokens, API-Key-Hashes, etc.). Die
**rohe Datenbank** — Compliance-Daten, Risiken, Findings, Audit-Logs — speichert
PostgreSQL standardmäßig **unverschlüsselt** auf dem Volume.

Diese Seite beschreibt die drei Wege, at-Rest-Verschlüsselung für deine Vakt-Instanz
herzustellen, geordnet nach Aufwand und Schutzwirkung.

---

## TL;DR — Was wir empfehlen

| Schutzziel | Empfehlung |
|------------|------------|
| Verlorene/gestohlene Hardware | **LUKS-Volume-Verschlüsselung** (Standard für DSGVO Art. 32) |
| Cloud-Hosting (AWS/Azure/GCP) | Cloud-Provider-eigene Volume-Verschlüsselung aktivieren |
| Einzelne hochsensible Spalten | Optional `pgcrypto` (Spalten-Level, bereits in Postgres enthalten) |

Für ISO 27001 / NIS2 / TISAX reicht **eine** dieser Maßnahmen — sie wird als
TOM gemäß DSGVO Art. 32 dokumentiert (Vakt Comply → Framework "DSGVO-TOM").

---

## Variante 1 — LUKS (empfohlen für eigene Hardware)

LUKS verschlüsselt das gesamte Datenträger-Volume transparent. PostgreSQL
merkt nichts davon — und Vakt auch nicht. Das ist der DACH-Standard für
"Verschlüsselung at-Rest" im Sinne der DSGVO und des BSI-Grundschutz.

### Setup auf dem Vakt-Host

```bash
# Vor dem ersten docker compose up:
cryptsetup luksFormat /dev/sdX                     # neues, leeres Volume
cryptsetup open /dev/sdX vakt-data
mkfs.ext4 /dev/mapper/vakt-data
mkdir -p /var/lib/vakt
mount /dev/mapper/vakt-data /var/lib/vakt
echo "/dev/mapper/vakt-data /var/lib/vakt ext4 defaults 0 2" >> /etc/fstab
```

Dann das Docker-Compose-Volume auf `/var/lib/vakt` umbiegen:

```yaml
# docker-compose.override.yml
volumes:
  postgres_data:
    driver: local
    driver_opts:
      type: none
      device: /var/lib/vakt/postgres
      o: bind
```

### Beim Reboot

LUKS fragt beim Bootvorgang nach der Passphrase. Für **headless Server**
empfiehlt sich `crypttab` mit einem Keyfile oder die Anbindung an einen
externen Keyserver (Tang / Clevis für Network-Bound Disk Encryption).

---

## Variante 2 — Cloud-Provider-Volume-Encryption

Wenn Vakt in einer Cloud-VM läuft, aktiviere die provider-eigene
Verschlüsselung. Sie ist transparent, hat keinen Performance-Overhead
und ist im jeweiligen Compliance-Audit-Trail nachweisbar.

| Cloud | Mechanismus | Standard? |
|-------|-------------|-----------|
| **AWS** | EBS-Encryption mit KMS | Empfohlen, opt-in |
| **Azure** | Disk Encryption mit Azure Key Vault | Empfohlen, opt-in |
| **GCP** | CMEK (Customer-Managed Encryption Keys) | Standard-Encryption ist immer aktiv |
| **Hetzner / OVH** | Volume-Encryption beim Anlegen wählen | opt-in |
| **STRATO / IONOS** | Datacenter-Disk-Encryption (oft Standard) | DC-Standard |

Frage beim Provider explizit nach der **Key-Management-Policy** — wer hält die
Master-Keys, gibt es Rotation, gibt es Customer-Managed-Keys.

---

## Variante 3 — `pgcrypto` für sensible Spalten (optional)

`pgcrypto` ist eine Postgres-Extension, die symmetrische Verschlüsselung auf
Spalten-Ebene erlaubt. Anwendungsfall: einzelne, besonders sensible Felder
(z.B. DSR-Antragsteller-PII, Breach-Notification-Details) verschlüsseln,
**ohne** den gesamten Server umzustellen.

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Verschlüsselung beim INSERT:
INSERT INTO sensitive_table (name, encrypted_data)
VALUES ('row1', pgp_sym_encrypt('Klartext', current_setting('app.pgcrypto_key')));

-- Entschlüsselung beim SELECT:
SELECT name, pgp_sym_decrypt(encrypted_data::bytea, current_setting('app.pgcrypto_key'))
FROM sensitive_table;
```

> **Hinweis:** Vakt nutzt `pgcrypto` aktuell nicht von sich aus — es ist ein
> Mechanismus für Kunden mit spezifischen vertraglichen Anforderungen
> ("schreib mir genau dieses Feld verschlüsselt in die DB"). In der Regel ist
> Variante 1 oder 2 ausreichend und einfacher zu betreiben.

---

## Was Vakt **immer** schon verschlüsselt (zusätzlich, nicht statt)

| Datentyp | Verfahren | Wo |
|----------|-----------|----|
| Vakt Vault-Secrets | AES-256-GCM | `secret_values.encrypted_value` |
| OIDC-Client-Secrets | AES-256-GCM | `auth_oidc_configs.encrypted_client_secret` |
| API-Keys (Lookup) | bcrypt (cost 12) | `api_keys.key_hash` |
| TOTP-Secrets | AES-256-GCM | `users.totp_secret_encrypted` |
| TOTP-Recovery-Codes | bcrypt | `auth_recovery_codes.code_hash` |
| Session-Tokens | bcrypt (Hash) | `refresh_sessions.token_hash` |
| Passwörter | bcrypt (cost 12) | `users.password_hash` |

Die Master-Encryption-Key (`VAKT_SECRET_KEY`) wird **nur über Umgebungsvariablen
oder einen externen KMS** an die App gereicht — niemals in der Datenbank
gespeichert. Bei kompromittiertem DB-Backup ohne den Key sind alle obigen
Datenpunkte weiterhin sicher.

---

## Installations-Checklist (DSGVO Art. 32)

Vor dem Go-Live bestätigen:

- [ ] Eine der drei Varianten (LUKS / Cloud-Volume / pgcrypto) ist aktiv
- [ ] `VAKT_SECRET_KEY` wurde mit `openssl rand -hex 32` generiert
- [ ] `VAKT_SECRET_KEY` liegt **nicht** im Git, nicht im Image, nicht im Backup
- [ ] Datenbank-Backups werden separat verschlüsselt (`scripts/backup.sh` macht das automatisch)
- [ ] Eintrag im Verarbeitungsverzeichnis (VVT) → Vakt Privacy → "Verschlüsselung at-Rest"
- [ ] TOM-Eintrag in Vakt Comply (Framework "DSGVO-TOM" → TOM-3 "Datenträger-Verschlüsselung")

---

## Verifikation

```bash
# LUKS aktiv?
lsblk -o NAME,FSTYPE,MOUNTPOINT
# → erwartet "crypto_LUKS" auf dem Datenträger

# Cloud-Volume verschlüsselt?
# AWS:
aws ec2 describe-volumes --volume-ids vol-xxx --query 'Volumes[0].Encrypted'
# Azure:
az disk show --name xxx --query "encryption.type"

# pgcrypto installiert?
docker compose exec postgres psql -U vakt -c "\dx" | grep pgcrypto
```
