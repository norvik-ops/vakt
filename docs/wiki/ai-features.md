# Vakt KI-Funktionen

Vakt enthält einen **lokalen KI-Berater**, der auf deiner eigenen Infrastruktur läuft — **kein Cloud-API-Key, kein GPU, keine Daten verlassen die Instanz** (siehe [ADR-0001](../adr/0001-self-hosted-no-phone-home.md)).

> **Community-Feature:** Seit v0.6.x sind alle KI-Funktionen in jeder Vakt-Instanz enthalten — keine Pro-Lizenz nötig. Mit dem Default-Modell `qwen2.5:7b` (Apache 2.0, ~4.5 GB RAM, CPU-tauglich, 8 GB RAM) läuft die KI lokal — auf VMs mit < 8 GB RAM alternativ `qwen2.5:3b` (~1.9 GB); ein Pro-Gate würde nur Marketing-Limit ohne echten Schutz bedeuten. **Premium-Compliance-Features** (TISAX-Maturitätsanalyse, DORA, NIS2-Meldungsassistent, EU AI Act, AuditPDF, SSO, API-Access, Vakt Aware Advanced, Vakt Scan Advanced, Granular-Permissions, Supplier-Portal) bleiben dem Pro-Plan vorbehalten.

---

## Die 5 KI-Funktionen

| Funktion | Endpoint | Wofür |
|---|---|---|
| **Compliance-Berater** | `POST /vaktcomply/ai/advice` | „Was sollte ich diese Woche tun?" — analysiert offene Controls, überfällige Tasks, kritische Risiken und gibt eine priorisierte Wochenplan-Empfehlung. |
| **Bericht-Generator** | `POST /vaktcomply/ai/report` | Generiert Gap-Analyse, Risiko-Übersicht oder Executive Summary als Markdown — als Vorbereitung für Audit-Termine oder Management-Reviews. |
| **Policy-Drafting** *(neu)* | `POST /vaktcomply/ai/draft-policy` | „Erstelle eine Passwort-Richtlinie für ISO 27001 A.5.17" — generiert einen Markdown-Entwurf in deutscher Sprache mit Standard-Abschnitten (Zweck, Geltungsbereich, Anforderungen, Verantwortlichkeiten). Admin reviewt und veröffentlicht. |
| **Incident-Response-Guide** *(neu)* | `POST /vaktcomply/ai/incident-guide` | Aus einer Vorfalls-Beschreibung erzeugt die KI eine nummerierte Sofort-Checkliste mit gesetzlichen Fristen-Hinweisen (NIS2 T+24h / T+72h, DSGVO Art. 33 72 h, DORA T+4h). Direkt im UI per „KI-Sofortmaßnahmen"-Button anwendbar. |
| **AI-System-Dokumentation** | (UI in EU AI Act-Modul) | Hilft beim Ausfüllen der technischen Dokumentation nach EU AI Act Art. 11 / Annex IV. |

---

## System-Anforderungen

### Lokales Modell (Standard, empfohlen)

| Setup | RAM | CPU | Geschwindigkeit | Anwendbar für |
|-------|-----|-----|-----------------|----------------|
| **Minimum** | 8 GB (+4.5 GB Modell) | 4 vCPU | 5–9 tok/s | Funktioniert, bei Lastspitzen etwas zäh |
| **Empfohlen** | 8 GB (+4.5 GB Modell qwen2.5:7b) | 4 vCPU | 5–9 tok/s | KMU-Standard, komfortabel nutzbar |
| **Komfort** | 16 GB+ | NVIDIA T4 oder besser | 50–100+ tok/s | Wenn die KI häufig genutzt wird |

**Konkrete Erwartung (Default-Modell `qwen2.5:7b` auf 4 vCPU):**

- Compliance-Berater-Antwort (~300 Tokens): **35–60 Sekunden**
- Policy-Draft (~600 Tokens): **70–120 Sekunden**
- Incident-Guide (~250 Tokens): **28–50 Sekunden**

→ Faustregel: Wenn die KI 5–10× pro Tag genutzt wird, ist CPU-Inference tolerabel. Bei intensiver Nutzung lohnt eine GPU oder Mistral EU.

### Cloud-Modell (Mistral AI EU)

| Setup | Geschwindigkeit | Daten-Standort |
|-------|-----------------|----------------|
| Mistral Small | 80–150 tok/s | EU (Frankreich, ISO 27001) |
| Mistral Medium | 50–100 tok/s | EU |

Konfiguration: siehe [Konfigurations-Abschnitt](#cloud-alternative-mistral-eu) unten.

---

## Modell-Auswahl

### Standard: `qwen2.5:7b`

Vakt verwendet **`qwen2.5:7b`** als Default-Modell. Begründung:

- **Apache 2.0 Lizenz** — keine Einschränkungen für kommerzielle Nutzung
- **7 Mrd. Parameter** — spürbar bessere Qualität bei deutschen Compliance-Texten
- **~4.5 GB RAM-Footprint** mit Q4-Quantisierung
- **Deutsche Sprache** — deutlich bessere DE-Performance als alle Llama-Modelle gleicher Größe
- **CPU-fähig** — ~5–9 Token/Sekunde auf 4 vCPU; für 5–10 Anfragen/Tag gut tolerabel

### Alternative Modelle

Alle laufen über Ollama, alle CPU-fähig:

| Modell | RAM | Lizenz | Wann sinnvoll |
|--------|-----|--------|---------------|
| `qwen2.5:7b` *(Default)* | 4.5 GB | Apache 2.0 | Standard — beste DE-Compliance-Qualität |
| `qwen2.5:3b` | 1.9 GB | Apache 2.0 | VMs mit knapp 8 GB RAM |
| `phi3.5:mini` | 2.3 GB | MIT | Sehr gut bei strukturierten Outputs (Policies) |
| `llama3.2:3b` | 2.0 GB | Llama Community | Alternative zu qwen2.5:3b |
| `llama3.2:1b` | 1.3 GB | Llama Community | VMs mit < 8 GB RAM, spürbar geringere Qualität |
| `gemma2:2b` | 1.6 GB | Gemma Terms (restriktiv) | Nicht empfohlen — restriktivere Lizenz |

**Modell wechseln:**

```bash
# 1. Modell ziehen (ollama läuft default seit v0.6.x — kein --profile mehr nötig)
docker compose exec ollama ollama pull phi3.5:mini

# 2. .env anpassen
echo 'VAKT_AI_MODEL=phi3.5:mini' >> .env

# 3. API neu starten
docker compose restart api worker
```

Beim ersten `docker compose up` zieht ein einmaliger `ollama-init`-Container
das in `.env`-Datei konfigurierte Modell (Default `qwen2.5:7b`). Wenn das
Modell schon im Volume ist, ist der Pull ein No-Op.

---

## Datenschutz-Garantie

Vakt's KI-Funktionen verletzen **niemals** das self-hosted-Prinzip (ADR-0001):

| Lokales Modell (Default) | Cloud-Modell (opt-in) |
|--------------------------|----------------------|
| ✅ Daten verlassen die Instanz nicht | ⚠️ Daten gehen an den Cloud-Anbieter |
| ✅ Kein AVV mit Norvik / OpenAI / Anthropic nötig | ⚠️ AVV mit Cloud-Anbieter erforderlich (DSGVO Art. 28) |
| ✅ Funktioniert offline | ❌ Internet-Verbindung Pflicht |
| ✅ Modell-Lizenz prüfbar | ⚠️ Anbieter-Terms prüfen |

Das Kern-Versprechen ist: **wenn du das Default-Setup nutzt, sieht außer dir und deinen Mitarbeitern niemand die Inhalte.**

---

## Cloud-Alternative: Mistral EU

Für Kunden, denen Geschwindigkeit wichtiger ist als die Lokalität — Mistral AI ist DSGVO-konform (Sitz Frankreich, ISO 27001 zertifiziert):

```env
VAKT_AI_PROVIDER=openai          # OpenAI-kompatibles Protokoll
VAKT_AI_BASE_URL=https://api.mistral.ai/v1
VAKT_AI_API_KEY=sk-...           # Mistral-Account anlegen
VAKT_AI_MODEL=mistral-small-latest
```

**Wichtig:** Vor dem Aktivieren AVV mit Mistral AI abschließen. Mistral bietet eine standardmäßige DSGVO-AVV-Vorlage auf https://mistral.ai/legal/.

Andere OpenAI-kompatible EU-Anbieter funktionieren genauso (z.B. Hetzner AI, Aleph Alpha API). Anbieter außerhalb der EU (OpenAI, Anthropic) sind technisch nutzbar, erfordern aber zusätzliche Drittland-Übermittlungs-Maßnahmen (SCC, TIA).

---

## Deaktivieren

```env
VAKT_AI_PROVIDER=disabled
```

Beim Start gibt Vakt einen klaren Log-Hinweis aus, dass KI deaktiviert ist. Im UI sind die KI-Buttons dann ausgeblendet.

---

## Performance-Tuning

Wenn die CPU-Inference zu langsam ist:

1. **Modell kleiner wählen** — `llama3.2:1b` (1.3 GB) statt `qwen2.5:3b` (1.9 GB) — Qualität sinkt aber gering
2. **mehr vCPU zuweisen** — Ollama profitiert deutlich von zusätzlichen Cores
3. **GPU spendieren** — eine günstige RTX 4060 (~300 €) macht alles 5–10× schneller
4. **Cloud nutzen** — Mistral EU für Geschwindigkeit, Datenschutz erhalten
5. **Caching** — gleiche Anfragen kommen aus dem App-Cache (vakt cached Antworten 24h)

---

## Häufige Fragen

**Q: Funktioniert das wirklich ohne GPU?**
Ja. Die Anfragen sind etwas langsamer (sec statt ms), aber für die typischen 5-10 KI-Anfragen pro Tag voll funktional.

**Q: Welches Modell hat die beste Qualität?**
Für Compliance-Texte: `qwen2.5:7b` > `phi3.5:mini` > `qwen2.5:3b` > `llama3.2:3b`. Wenn du 4 GB Modell-RAM hast, lohnt qwen2.5:7b.

**Q: Welches ist am schnellsten?**
Lokal: `llama3.2:1b` (klein → schnell). Cloud: `mistral-small-latest`.

**Q: Was passiert mit dem Compliance-Kontext beim KI-Aufruf?**
Vakt sendet kompakte Daten-Zusammenfassungen an die KI: z.B. „17 offene Controls, davon 3 überfällig, 5 kritische Risiken offen". **Niemals Klartext-Risiken, niemals Personenbezug, niemals Audit-Log-Einträge.** Die Prompts sind in [`backend/internal/services/ai/client.go`](../../backend/internal/services/ai/client.go) im Klartext nachlesbar.

**Q: Kann ich eigene Modelle einbinden?**
Ja — jedes OpenAI-kompatible Endpoint funktioniert. LM Studio, vLLM, LocalAI etc. lassen sich als `VAKT_AI_BASE_URL` setzen.
