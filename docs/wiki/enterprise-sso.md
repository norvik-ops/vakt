# SSO, SCIM & SIEM — Setup-Guide

**Gilt ab:** v0.16.0  
**Tier:** SAML Community · SCIM/SIEM Pro

---

## Übersicht

| Feature | Tier | Endpunkt |
|---------|------|----------|
| SAML 2.0 SP (direkter IdP) | Community | `/api/v1/auth/saml/*` |
| SCIM 2.0 Provisioning | Pro | `/api/v1/scim/v2/*` |
| SIEM Audit-Forwarder | Pro | konfigurierbar über Admin-Settings |

---

## 1. SAML 2.0 — Single Sign-On

Vakt agiert als **Service Provider (SP)** und validiert Assertions direkt — kein Casdoor-Sidecar nötig.  
Unterstützte IdPs: Azure Entra ID (Microsoft 365), Okta, OneLogin, Google Workspace.

### 1.1 AzureAD / Microsoft Entra ID

1. **Enterprise-App erstellen**
   - Azure Portal → **Entra ID** → **Enterprise Applications** → **New application** → **Create your own application**
   - Name: `Vakt`, wähle *"Integrate any other application you don't find in the gallery"*

2. **SAML konfigurieren**
   - App → **Single sign-on** → **SAML**
   - **Basic SAML Configuration:**
     - Identifier (Entity ID): `https://DEINE-DOMAIN/saml`
     - Reply URL (ACS URL): `https://DEINE-DOMAIN/api/v1/auth/saml/acs`
     - Sign on URL: `https://DEINE-DOMAIN/api/v1/auth/saml/initiate`

3. **IdP Metadata herunterladen**
   - Abschnitt *SAML Certificates* → **Federation Metadata XML** herunterladen

4. **In Vakt eintragen** (Admin → Einstellungen → Single Sign-On)
   - SP Entity ID: `https://DEINE-DOMAIN/saml`
   - ACS URL: `https://DEINE-DOMAIN/api/v1/auth/saml/acs`
   - IdP Metadata XML: Inhalt der heruntergeladenen Datei einfügen
   - **Speichern** — Vakt generiert automatisch ein SP-Zertifikat

5. **SP Zertifikat in Azure eintragen**
   - Vakt Admin → SSO-Einstellungen → *SP Zertifikat* kopieren
   - Azure → Enterprise App → SAML → *SAML Signing Certificate* → *Upload certificate* → PEM einfügen

6. **Test** — Nutzer über Azure-App zuweisen, dann Login testen

> **Attribute Mapping:** Vakt liest `email` und `displayName`. Azure liefert diese standardmäßig als `http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress` und `displayName`.

---

### 1.2 Okta

1. **App Integration erstellen**
   - Okta Admin → **Applications** → **Create App Integration** → **SAML 2.0**

2. **SAML Settings**
   - Single sign-on URL (ACS): `https://DEINE-DOMAIN/api/v1/auth/saml/acs`
   - Audience URI (Entity ID): `https://DEINE-DOMAIN/saml`
   - Attribute Statements:
     - `email` → `user.email`
     - `displayName` → `user.displayName`

3. **IdP Metadata**
   - App → **Sign On** → **View SAML setup instructions** → **Identity Provider metadata** (XML) kopieren

4. **In Vakt eintragen** wie oben beschrieben.

---

### 1.3 OneLogin

1. **App erstellen**
   - OneLogin Admin → **Applications** → **Add App** → nach "SAML Test Connector" suchen

2. **Configuration**
   - ACS URL: `https://DEINE-DOMAIN/api/v1/auth/saml/acs`
   - Recipient: `https://DEINE-DOMAIN/api/v1/auth/saml/acs`
   - Audience: `https://DEINE-DOMAIN/saml`
   - ACS URL Validator: `https://DEINE-DOMAIN/api/v1/auth/saml/acs`

3. **Metadata herunterladen** → **More Actions** → **SAML Metadata**

---

### 1.4 Google Workspace

1. **SAML-App einrichten**
   - Google Admin → **Apps** → **Web and mobile apps** → **Add app** → **Add custom SAML app**

2. **Service provider details**
   - ACS URL: `https://DEINE-DOMAIN/api/v1/auth/saml/acs`
   - Entity ID: `https://DEINE-DOMAIN/saml`

3. **Attribute mapping**
   - Primary email → `email`
   - First name → `givenName`
   - Last name → `surname`

4. **IdP Metadata** → *Download Metadata* auf der "Google Identity Provider details"-Seite

---

### 1.5 SP-initiierter Login (deep link)

Das Frontend startet den SAML-Flow via:
```
GET /api/v1/auth/saml/initiate
Authorization: Bearer <paseto>
```
Backend gibt `{ "redirect_url": "https://idp.example.com/...?SAMLRequest=..." }` zurück.  
Das Frontend folgt dem `redirect_url`.

---

### 1.6 SP-Zertifikat erneuern

```
POST /api/v1/admin/org/saml-config/regenerate-cert
Authorization: Bearer <admin-paseto>
```

Nach Erneuerung muss das neue Zertifikat im IdP eingetragen werden.

---

## 2. SCIM 2.0 — Automatisches Provisioning (Pro)

SCIM ermöglicht automatisches Anlegen, Aktualisieren und Deaktivieren von Nutzern und Gruppen aus dem IdP.

**Base URL:** `https://DEINE-DOMAIN/api/v1/scim/v2`  
**Auth:** Bearer Token (SCIM-Token aus Admin-Einstellungen, getrennt von API-Keys)

### 2.1 SCIM Token erstellen

1. Vakt Admin → **Einstellungen** → **SCIM-Provisioning**
2. **Token erstellen** → Name vergeben (z.B. "Okta Provisioning")
3. Token **einmalig kopieren** — danach nicht mehr abrufbar

### 2.2 AzureAD SCIM-Setup

1. Enterprise App → **Provisioning** → **Automatic**
2. Tenant URL: `https://DEINE-DOMAIN/api/v1/scim/v2`
3. Secret Token: erzeugten SCIM-Token einfügen
4. **Test Connection** → **Save**
5. Provisioning Scope → **Sync only assigned users and groups**

**Attribute Mapping (automatisch übernommen):**
- `userName` ↔ `mail`
- `displayName` ↔ `displayName`
- `active` ↔ Kontostatus

### 2.3 Okta SCIM-Setup

1. App Integration → **Provisioning** → **Configure API Integration**
2. SCIM connector base URL: `https://DEINE-DOMAIN/api/v1/scim/v2`
3. Unique identifier field: `userName`
4. Authentication Mode: **HTTP Header**
5. Header: `Authorization: Bearer <SCIM-TOKEN>`

### 2.4 Unterstützte Operationen

| Operation | Endpunkt | Beschreibung |
|-----------|----------|--------------|
| List Users | `GET /Users` | Paginiert, Filter mit `?filter=userName+eq+"alice"` |
| Get User | `GET /Users/:id` | |
| Create User | `POST /Users` | Legt User + Org-Mitgliedschaft an |
| Update User | `PUT /Users/:id` | Vollständiges Update |
| Patch User | `PATCH /Users/:id` | Partial Update (active, displayName) |
| Deactivate | `PATCH /Users/:id` `{"active":false}` | Entfernt Org-Mitgliedschaft |
| List Groups | `GET /Groups` | |
| Create Group | `POST /Groups` | Erzeugt Rolle/Gruppe in Vakt |
| Patch Group | `PATCH /Groups/:id` | Mitglieder hinzufügen/entfernen |

### 2.5 ServiceProviderConfig

```
GET /api/v1/scim/v2/ServiceProviderConfig
```
Liefert JSON mit unterstützten Features (`filter`, `patch`, `bulk`).

---

## 3. SIEM Audit-Forwarder (Pro)

Vakt forwarded Audit-Log-Einträge automatisch alle 5 Minuten an ein SIEM.

**Unterstützte Adapter:**
- **Splunk HEC** (HTTP Event Collector)
- **Elasticsearch** (Bulk API)
- **Generic Webhook** (JSON Array, optional Bearer Token)

### 3.1 Splunk HEC Setup

1. Splunk → **Settings** → **Data Inputs** → **HTTP Event Collector** → **New Token**
2. Token kopieren
3. Vakt Admin → **Einstellungen** → **SIEM-Integration**:
   - Adapter: `Splunk HEC`
   - Endpunkt: `https://splunk.company.com:8088`
   - Token: HEC-Token einfügen
4. **Test-Event** senden → prüfen ob Event in Splunk erscheint

### 3.2 Elasticsearch Setup

1. Elasticsearch API-Key erstellen:
   ```bash
   POST /_security/api_key
   {"name": "vakt-siem", "role_descriptors": {"vakt-writer": {"cluster": ["monitor"], "index": [{"names": ["vakt-audit-*"], "privileges": ["create", "create_index"]}]}}}
   ```
2. Vakt Admin → SIEM-Integration:
   - Adapter: `Elasticsearch`
   - Endpunkt: `https://elastic.company.com:9200/vakt-audit-events/_bulk`
   - Token: `<id>:<api_key>` (Base64 wird intern erledigt)

### 3.3 Webhook Setup

Vakt sendet ein JSON Array mit Audit-Events an den konfigurierten Endpunkt:
```json
[
  {"event_type": "login", "actor": "user@example.com", "timestamp": "...", "ip": "..."},
  ...
]
```

Optional: Bearer Token für Authentifizierung.

### 3.4 Forwarding-Logik

- Worker-Job läuft alle **5 Minuten**
- Forwarded bis zu **100 Einträge pro Durchlauf** pro Org
- Markiert `forwarded_to_siem = NOW()` nach Erfolg
- Bei Fehler: keine Retry-Queue (Best-Effort; nächster Lauf versucht es erneut)
- **Test-Event** über Admin-UI sendet einen synthetischen Event ohne DB-Eintrag

---

## 4. Audit-Trail

Jeder SAML-Login erscheint im Audit-Log mit:
- `source: "saml_direct"` (direkter SP) oder `source: "saml"` (Casdoor-Proxy)
- `result: "ok"` oder Fehlercode

SCIM-Operationen erscheinen als `actor: "scim_provisioner"` mit der jeweiligen Operation.

---

## 5. Troubleshooting

| Problem | Mögliche Ursache | Lösung |
|---------|-----------------|--------|
| SAML: "assertion validation failed" | Zertifikat-Mismatch oder abgelaufenes SP-Cert | Zertifikat in Vakt und IdP synchronisieren |
| SAML: "missing email claim" | IdP sendet NameID statt email-Attribut | Attribute Mapping im IdP prüfen |
| SCIM: 401 Unauthorized | Token falsch oder widerrufen | Neues SCIM-Token erstellen |
| SCIM: User wird nicht angelegt | Filter-Syntax falsch | `?filter=userName+eq+"user%40example.com"` |
| SIEM: Kein Event in Splunk | HEC-Token oder Endpunkt falsch | Test-Event in Admin-UI senden und Splunk-HEC-Logs prüfen |

---

*Weitere Fragen? → [GitHub Issues](https://github.com/norvik-ops/vatk/issues) oder Community-Forum.*
