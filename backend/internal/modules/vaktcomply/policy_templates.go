package vaktcomply

// PolicyTemplate is a pre-built policy template with German content.
type PolicyTemplate struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Content     string `json:"content"` // the policy body text
}

// BuiltinPolicyTemplates returns the built-in German policy templates.
func BuiltinPolicyTemplates() []PolicyTemplate {
	return []PolicyTemplate{
		{
			ID:          "isms-policy",
			Title:       "Informationssicherheitsrichtlinie",
			Category:    "ISMS",
			Description: "Grundlegende Richtlinie für das Informationssicherheitsmanagementsystem der Organisation.",
			Content: `# Informationssicherheitsrichtlinie

## 1. Zweck und Geltungsbereich
Diese Richtlinie legt die grundlegenden Anforderungen an die Informationssicherheit innerhalb der Organisation fest. Sie gilt für alle Mitarbeiter, externen Dienstleister und Systeme, die auf Informationen der Organisation zugreifen.

## 2. Grundsätze
Die Organisation verpflichtet sich zur Wahrung der Vertraulichkeit, Integrität und Verfügbarkeit aller Informationswerte. Informationssicherheit ist eine gemeinsame Verantwortung aller Mitarbeiter.

## 3. Verantwortlichkeiten
- Geschäftsführung: Bereitstellung von Ressourcen und Unterstützung des ISMS
- IT-Sicherheitsbeauftragter (ISB): Koordination und Überwachung der Umsetzung
- Mitarbeiter: Einhaltung aller Sicherheitsrichtlinien und Meldung von Vorfällen

## 4. Risikomanagement
Die Organisation identifiziert, bewertet und behandelt Informationssicherheitsrisiken systematisch gemäß der Risikorichtlinie.

## 5. Überprüfung
Diese Richtlinie wird jährlich oder bei wesentlichen Änderungen überprüft und bei Bedarf aktualisiert.`,
		},
		{
			ID:          "password-policy",
			Title:       "Passwort-Richtlinie",
			Category:    "Zugangskontrolle",
			Description: "Anforderungen an sichere Passwörter und Authentifizierung.",
			Content: `# Passwort-Richtlinie

## 1. Mindestanforderungen an Passwörter
- Mindestlänge: 12 Zeichen
- Kombination aus Groß- und Kleinbuchstaben, Zahlen und Sonderzeichen
- Keine vollständigen Wörter aus dem Wörterbuch
- Kein Bezug zu persönlichen Daten (Name, Geburtsdatum)

## 2. Passwort-Verwaltung
- Passwörter dürfen nicht schriftlich notiert oder unverschlüsselt gespeichert werden
- Nutzung eines genehmigten Passwort-Managers ist empfohlen
- Passwörter dürfen nicht mit anderen Personen geteilt werden

## 3. Änderungsintervalle
- Systempasswörter: alle 90 Tage
- Privilegierte Konten: alle 60 Tage
- Bei Verdacht auf Kompromittierung: sofortige Änderung

## 4. Multi-Faktor-Authentifizierung
Für alle administrativen Zugänge und Remote-Zugriffe ist eine Multi-Faktor-Authentifizierung (MFA) verpflichtend.`,
		},
		{
			ID:          "acceptable-use-policy",
			Title:       "Richtlinie zur akzeptablen Nutzung",
			Category:    "Nutzung",
			Description: "Regelungen zur erlaubten Nutzung von IT-Systemen und Ressourcen.",
			Content: `# Richtlinie zur akzeptablen Nutzung von IT-Ressourcen

## 1. Erlaubte Nutzung
IT-Ressourcen der Organisation dürfen ausschließlich für dienstliche Zwecke genutzt werden. Eine eingeschränkte private Nutzung ist toleriert, sofern sie die dienstliche Arbeit nicht beeinträchtigt.

## 2. Verbotene Aktivitäten
- Download oder Verbreitung von urheberrechtlich geschütztem Material ohne Genehmigung
- Zugriff auf pornografische, extremistische oder anderweitig illegale Inhalte
- Installation nicht genehmigter Software
- Überbrückung von Sicherheitsmaßnahmen
- Nutzung für kommerzielle Eigeninteressen

## 3. Internet und E-Mail
- E-Mail-Kommunikation repräsentiert die Organisation
- Phishing-E-Mails und verdächtige Anhänge sind dem IT-Support zu melden
- Vertrauliche Daten dürfen nicht unverschlüsselt per E-Mail übertragen werden

## 4. Monitoring
Die Organisation behält sich vor, die Nutzung von IT-Ressourcen im gesetzlich zulässigen Rahmen zu überwachen.`,
		},
		{
			ID:          "home-office-policy",
			Title:       "Homeoffice- und Fernarbeitsrichtlinie",
			Category:    "Fernarbeit",
			Description: "Sicherheitsanforderungen für das Arbeiten außerhalb der Unternehmensräume.",
			Content: `# Homeoffice- und Fernarbeitsrichtlinie

## 1. Voraussetzungen
Fernarbeit ist nur mit ausdrücklicher Genehmigung und über gesicherte Verbindungen (VPN) zulässig.

## 2. Technische Anforderungen
- Aktuelles Betriebssystem mit automatischen Sicherheitsupdates
- Aktive Firewall und Antivirensoftware
- Verschlüsselte Festplatte (BitLocker / FileVault)
- VPN-Verbindung für den Zugriff auf interne Ressourcen

## 3. Physische Sicherheit
- Bildschirm vor unbefugten Blicken schützen (Clean Screen)
- Dokumente mit vertraulichen Informationen sicher aufbewahren
- Bei Verlassen des Arbeitsplatzes: Bildschirm sperren

## 4. Nutzung privater Geräte
Die Nutzung privater Endgeräte (BYOD) für dienstliche Zwecke ist nur nach Genehmigung und Einhaltung der MDM-Anforderungen zulässig.`,
		},
		{
			ID:          "data-classification-policy",
			Title:       "Datenklassifizierungsrichtlinie",
			Category:    "Datenschutz",
			Description: "Klassifizierung und Handhabung von Informationen nach Schutzbedarf.",
			Content: `# Datenklassifizierungsrichtlinie

## 1. Klassifizierungsstufen
| Klasse | Beschreibung | Beispiele |
|--------|-------------|-----------|
| Öffentlich | Frei zugänglich | Pressemitteilungen, Produktbroschüren |
| Intern | Nur für Mitarbeiter | Interne Rundschreiben, Prozessdokumente |
| Vertraulich | Eingeschränkter Zugang | Kundendaten, Verträge, Finanzdaten |
| Streng vertraulich | Sehr eingeschränkt | Geschäftsgeheimnisse, M&A-Informationen |

## 2. Handhabungsanforderungen
- Vertrauliche Daten: nur über verschlüsselte Kanäle übertragen
- Streng vertrauliche Daten: Zwei-Augen-Prinzip bei Zugriff
- Alle Klassen: Daten nur für den dienstlichen Zweck verwenden

## 3. Kennzeichnung
Dokumente müssen ihrer Klassifizierungsstufe entsprechend gekennzeichnet werden (Kopf-/Fußzeile).`,
		},
		{
			ID:          "incident-response-policy",
			Title:       "Incident-Response-Richtlinie",
			Category:    "Vorfallsmanagement",
			Description: "Verfahren zur Erkennung, Meldung und Behandlung von Sicherheitsvorfällen.",
			Content: `# Incident-Response-Richtlinie

## 1. Definition eines Sicherheitsvorfalls
Ein Sicherheitsvorfall ist jedes Ereignis, das die Vertraulichkeit, Integrität oder Verfügbarkeit von Informationen gefährdet oder gefährden könnte.

## 2. Meldepflicht
Alle Mitarbeiter sind verpflichtet, erkannte oder vermutete Sicherheitsvorfälle unverzüglich zu melden:
- Intern: security@[organisation].de oder Ticket im IT-Helpdesk
- Hotline: [Telefonnummer eintragen]

## 3. Reaktionsphasen
1. **Erkennung**: Identifikation und erste Bewertung
2. **Eindämmung**: Begrenzung des Schadens
3. **Beseitigung**: Entfernung der Ursache
4. **Wiederherstellung**: Rückkehr zum Normalbetrieb
5. **Nachbereitung**: Lessons-learned und Dokumentation

## 4. DSGVO-Meldepflicht
Bei Datenpannen: Meldung an die Aufsichtsbehörde innerhalb von 72 Stunden (Art. 33 DSGVO).`,
		},
		{
			ID:          "change-management-policy",
			Title:       "Änderungsmanagement-Richtlinie",
			Category:    "Betrieb",
			Description: "Kontrolle und Steuerung von Änderungen an IT-Systemen und Prozessen.",
			Content: `# Änderungsmanagement-Richtlinie

## 1. Geltungsbereich
Diese Richtlinie gilt für alle geplanten Änderungen an Produktivsystemen, Anwendungen, Netzwerken und kritischen Prozessen.

## 2. Änderungskategorien
- **Standard**: Vorab definierte, risikoarme Änderungen (keine Freigabe erforderlich)
- **Normal**: Geplante Änderungen mit mittlerem Risiko (Freigabe durch IT-Leiter)
- **Notfall**: Ungeplante Änderungen bei kritischen Vorfällen (nachträgliche Dokumentation)

## 3. Prozess
1. Änderungsantrag mit Beschreibung, Risikobewertung und Rollback-Plan
2. Review und Genehmigung
3. Test in Nicht-Produktivumgebung
4. Implementierung mit Dokumentation
5. Post-Implementation Review

## 4. Dokumentation
Alle Änderungen werden im Change-Log dokumentiert und für mindestens 3 Jahre aufbewahrt.`,
		},
		{
			ID:          "access-control-policy",
			Title:       "Zugangs- und Zugriffskontrollrichtlinie",
			Category:    "Zugangskontrolle",
			Description: "Regelungen für die Vergabe und Verwaltung von Zugriffsrechten.",
			Content: `# Zugangs- und Zugriffskontrollrichtlinie

## 1. Grundsatz
Jeder Mitarbeiter erhält nur die Zugriffsrechte, die für seine Aufgaben zwingend erforderlich sind (Minimalprinzip / Need-to-know).

## 2. Benutzerkonten
- Persönliche Benutzerkonten sind nicht übertragbar
- Administrationskonten werden nur für administrative Tätigkeiten genutzt
- Gemeinsam genutzte Konten sind grundsätzlich verboten

## 3. Berechtigungsverwaltung
- Zugriffsrechte werden bei Rollenwechsel oder Ausscheiden sofort angepasst
- Quartalsweise Überprüfung aller Berechtigungen
- Privilegierte Zugriffsrechte werden halbjährlich rezertifiziert

## 4. Fernzugriff
Fernzugriff auf interne Systeme ist nur über VPN mit MFA zulässig. Alle Fernzugriffe werden protokolliert.`,
		},
		{
			ID:          "backup-policy",
			Title:       "Datensicherungsrichtlinie",
			Category:    "Verfügbarkeit",
			Description: "Anforderungen an Datensicherung und Wiederherstellung.",
			Content: `# Datensicherungsrichtlinie

## 1. Backup-Strategie
Die Organisation folgt der 3-2-1-Regel:
- **3** Kopien der Daten
- **2** verschiedene Speichermedien
- **1** Kopie an einem externen Standort

## 2. Backup-Intervalle
| Datenklasse | Frequenz | Aufbewahrung |
|-------------|----------|--------------|
| Kritische Produktivdaten | Täglich | 90 Tage |
| Datenbanken | Stündlich (inkrementell) | 30 Tage |
| Konfigurationen | Wöchentlich | 1 Jahr |
| Archivdaten | Monatlich | 7 Jahre |

## 3. Wiederherstellungstests
Backups werden mindestens quartalsweise auf Wiederherstellbarkeit getestet. Ergebnisse werden dokumentiert.

## 4. RTO und RPO
- Recovery Time Objective (RTO): max. 4 Stunden für kritische Systeme
- Recovery Point Objective (RPO): max. 1 Stunde Datenverlust`,
		},
		{
			ID:          "supplier-security-policy",
			Title:       "Lieferanten- und Dienstleistersicherheit",
			Category:    "Lieferantenmanagement",
			Description: "Sicherheitsanforderungen an externe Dienstleister und Lieferanten.",
			Content: `# Richtlinie zur Lieferanten- und Dienstleistersicherheit

## 1. Geltungsbereich
Diese Richtlinie gilt für alle externen Dienstleister, die Zugang zu Systemen, Netzwerken oder Daten der Organisation haben.

## 2. Sicherheitsanforderungen
Dienstleister müssen:
- Nachweisbare Informationssicherheitsmaßnahmen implementiert haben (z. B. ISO 27001)
- Die Datenschutz-Grundverordnung (DSGVO) einhalten
- Sicherheitsvorfälle innerhalb von 24 Stunden melden

## 3. Vertragsliche Anforderungen
- Auftragsverarbeitungsvertrag (AVV) gemäß Art. 28 DSGVO
- Vertraulichkeitsvereinbarung (NDA)
- Recht auf Auditierung oder Zertifikatsnachweise

## 4. Laufende Überwachung
- Jährliche Überprüfung aller aktiven Dienstleister
- Einholen aktueller Sicherheitsnachweise (Zertifikate, Auditberichte)
- Sofortige Kündigung bei schwerwiegenden Sicherheitsverstößen`,
		},
	}
}
