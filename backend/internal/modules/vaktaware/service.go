package vaktaware

import (
	"bufio"
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/smtp"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/microcosm-cc/bluemonday"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/services/crossevidence"
	"github.com/matharnica/vakt/internal/services/evidence_auto"
	"github.com/matharnica/vakt/internal/shared/platform/events"
)

// Service handles SecReflex business logic.
type Service struct {
	repo        *Repository
	db          *pgxpool.Pool
	smtpCfg     SMTPConfig
	asynqClient *asynq.Client
}

// NewService creates a new SecReflex service.
func NewService(db *pgxpool.Pool, smtpCfg SMTPConfig, asynqOpt ...asynq.RedisClientOpt) *Service {
	svc := &Service{repo: NewRepository(db), db: db, smtpCfg: smtpCfg}
	if len(asynqOpt) > 0 && asynqOpt[0].Addr != "" {
		svc.asynqClient = asynq.NewClient(asynqOpt[0])
	}
	return svc
}

// presetTemplates returns the curated DACH-specific phishing-simulation template
// library (50 templates in 5 categories). Each template is in German and uses
// realistic DACH social-engineering patterns.
//
// Placeholders resolved at send time:
//
//	{{first_name}}   — Vorname des Empfängers
//	{{last_name}}    — Nachname
//	{{company}}      — Unternehmensname (aus Org-Settings)
//	{{tracking_url}} — Tracking-Link mit eindeutigem Token
//	{{open_pixel}}   — 1×1 transparentes Pixel zur Open-Erkennung
func presetTemplates() []Template {
	ph := func(pp ...string) []string { return pp }
	type t = Template
	return []t{
		// ── Kategorie: credential (10) ────────────────────────────────────────
		{ID: "preset-it-passwort-de", Name: "IT-Helpdesk Passwort-Reset", Category: "credential", Difficulty: "easy", Language: "de",
			Subject: "Ihr Passwort läuft heute ab", FromName: "IT-Helpdesk {{company}}", FromEmail: "helpdesk@{{company}}-it.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{last_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Sehr geehrte/r {{first_name}} {{last_name}},</p><p>Ihr Microsoft-365-Passwort läuft <b>heute um 17:00 Uhr</b> ab. Um eine Sperrung Ihres Kontos zu vermeiden, bitte <a href="{{tracking_url}}">jetzt neues Passwort setzen</a>.</p><p>Bei Fragen wenden Sie sich an Ihren IT-Helpdesk.</p>{{open_pixel}}`},
		{ID: "preset-microsoft-mfa-de", Name: "Microsoft 365 MFA-Warnung", Category: "credential", Difficulty: "medium", Language: "de",
			Subject: "Ungewöhnliche Anmeldung in Ihrem Microsoft-Konto", FromName: "Microsoft Sicherheit", FromEmail: "account-security@microsoft-365-de.com", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>wir haben eine ungewöhnliche Anmeldung in Ihrem Microsoft-365-Konto bemerkt:</p><p><b>Standort:</b> Moskau, Russland<br/><b>IP:</b> 185.220.101.47<br/><b>Zeit:</b> vor 12 Minuten</p><p>Falls das nicht Sie waren: <a href="{{tracking_url}}">Konto jetzt sperren</a></p>{{open_pixel}}`},
		{ID: "preset-vpn-access-de", Name: "VPN Zugangsdaten ablaufen", Category: "credential", Difficulty: "easy", Language: "de",
			Subject: "Ihr VPN-Zugang wird gesperrt – jetzt erneuern", FromName: "IT-Security {{company}}", FromEmail: "it-security@{{company}}-vpn.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>Ihr VPN-Zugang für <b>{{company}}</b> läuft in <b>24 Stunden</b> ab. Um unterbrechungsfreies Arbeiten im Homeoffice zu gewährleisten, müssen Sie Ihre Zugangsdaten jetzt bestätigen.</p><p><a href="{{tracking_url}}">Zugangsdaten bestätigen</a></p>{{open_pixel}}`},
		{ID: "preset-azure-aad-de", Name: "Azure Active Directory Verifikation", Category: "credential", Difficulty: "medium", Language: "de",
			Subject: "Ihre Azure AD-Sitzung ist abgelaufen", FromName: "Microsoft Azure", FromEmail: "noreply@azure-notifications-de.com", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Sehr geehrte/r {{first_name}},</p><p>aus Sicherheitsgründen wurde Ihre Azure Active Directory-Sitzung beendet. Bitte melden Sie sich erneut an, um auf Ihre Unternehmensressourcen zuzugreifen.</p><p><a href="{{tracking_url}}">Jetzt anmelden</a></p>{{open_pixel}}`},
		{ID: "preset-zoom-reaktivierung-de", Name: "Zoom-Account Reaktivierung", Category: "credential", Difficulty: "easy", Language: "de",
			Subject: "Ihr Zoom-Account wurde deaktiviert", FromName: "Zoom Sicherheitsteam", FromEmail: "security@zoom-accounts-de.com", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>wir haben verdächtige Aktivitäten in Ihrem Zoom-Account festgestellt und diesen vorübergehend gesperrt. Bitte bestätigen Sie Ihre Identität, um den Account wieder freizuschalten.</p><p><a href="{{tracking_url}}">Account entsperren</a></p>{{open_pixel}}`},
		{ID: "preset-google-workspace-de", Name: "Google Workspace Login-Alert", Category: "credential", Difficulty: "medium", Language: "de",
			Subject: "Neues Gerät hat sich in Ihr Google-Konto eingeloggt", FromName: "Google Sicherheit", FromEmail: "no-reply@google-security-alert.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>ein neues Gerät hat sich in Ihr Google-Konto eingeloggt:<br/><b>Gerät:</b> Windows 11 · Chrome<br/><b>Standort:</b> Bukarest, Rumänien</p><p>Nicht Sie? <a href="{{tracking_url}}">Zugriff sofort blockieren</a></p>{{open_pixel}}`},
		{ID: "preset-slack-verifizierung-de", Name: "Slack Workspace Verifizierung", Category: "credential", Difficulty: "easy", Language: "de",
			Subject: "Aktion erforderlich: Slack Workspace-Zugang", FromName: "Slack", FromEmail: "noreply@slack-workspace-verify.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>der Slack-Workspace von <b>{{company}}</b> wird auf ein neues Sicherheitsprotokoll umgestellt. Sie müssen Ihren Zugang bis <b>Freitag 18:00 Uhr</b> bestätigen, sonst wird Ihr Konto deaktiviert.</p><p><a href="{{tracking_url}}">Jetzt bestätigen</a></p>{{open_pixel}}`},
		{ID: "preset-sharepoint-zugriff-de", Name: "SharePoint Zugriffsanfrage", Category: "credential", Difficulty: "medium", Language: "de",
			Subject: "{{first_name}}, Sie haben eine SharePoint-Einladung erhalten", FromName: "SharePoint {{company}}", FromEmail: "sharepoint@{{company}}-docs.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>eine neue SharePoint-Seite wurde mit Ihnen geteilt. Klicken Sie auf den Link, um auf die Dokumente zuzugreifen. <b>Der Link läuft in 72 Stunden ab.</b></p><p><a href="{{tracking_url}}">Zu SharePoint</a></p>{{open_pixel}}`},
		{ID: "preset-cisco-vpn-de", Name: "Cisco AnyConnect Erneuerung", Category: "credential", Difficulty: "hard", Language: "de",
			Subject: "Cisco AnyConnect Zertifikat abgelaufen", FromName: "IT-Infrastruktur {{company}}", FromEmail: "it-infra@{{company}}-noc.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>das SSL-Zertifikat für den Cisco AnyConnect VPN-Client ist abgelaufen. Sie müssen das Client-Zertifikat erneuern, um weiterhin auf das {{company}}-Netzwerk zugreifen zu können.</p><p><a href="{{tracking_url}}">Zertifikat erneuern</a></p>{{open_pixel}}`},
		{ID: "preset-okta-mfa-de", Name: "Okta MFA Push abgelaufen", Category: "credential", Difficulty: "hard", Language: "de",
			Subject: "Ihre Okta MFA-Konfiguration erfordert Erneuerung", FromName: "Okta IT", FromEmail: "it@okta-enterprise-de.com", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Sehr geehrte/r {{first_name}},</p><p>Ihre Okta Verify-App muss neu registriert werden. Dies ist ein standardmäßiger Sicherheitsprozess, der alle 90 Tage durchgeführt wird.</p><p><a href="{{tracking_url}}">Jetzt neu registrieren</a></p>{{open_pixel}}`},

		// ── Kategorie: bec (10) ───────────────────────────────────────────────
		{ID: "preset-ceo-fraud-de", Name: "CEO Fraud – Überweisung", Category: "bec", Difficulty: "medium", Language: "de",
			Subject: "Vertraulich – kurze Rückmeldung erforderlich", FromName: "{{company}} Geschäftsführung", FromEmail: "geschaeftsfuehrung@{{company}}.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>ich bin gerade in einem Meeting und brauche bitte kurz Ihre Hilfe. Können Sie mir die Bankverbindung für die ausstehende Überweisung kurz bestätigen? Bitte <a href="{{tracking_url}}">hier klicken</a> und die Daten gegenchecken — es ist eilig.</p><p>Danke und beste Grüße</p>{{open_pixel}}`},
		{ID: "preset-steuerberater-daten-de", Name: "Steuerberater Datenweitergabe", Category: "bec", Difficulty: "medium", Language: "de",
			Subject: "Dringende Übermittlung Jahresabschluss-Unterlagen", FromName: "Steuerkanzlei Müller & Partner", FromEmail: "partner@steuerberatung-mueller-de.com", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Sehr geehrte/r {{first_name}},</p><p>für die fristgerechte Erstellung Ihres Jahresabschlusses benötige ich noch einige Unterlagen. Bitte laden Sie diese bis morgen 12:00 Uhr in unser Mandantenportal hoch.</p><p><a href="{{tracking_url}}">Zum Mandantenportal</a></p>{{open_pixel}}`},
		{ID: "preset-finanzabteilung-iban-de", Name: "Finanzabteilung IBAN-Änderung", Category: "bec", Difficulty: "hard", Language: "de",
			Subject: "Wichtig: Neue Bankverbindung für Lieferant", FromName: "Buchhaltung {{company}}", FromEmail: "buchhaltung@{{company}}-finance.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>ein wichtiger Lieferant hat uns mitgeteilt, dass sich seine Bankverbindung geändert hat. Bitte aktualisieren Sie die IBAN in unserem ERP-System. Die neue Bankverbindung finden Sie im angehängten Formular.</p><p><a href="{{tracking_url}}">Formular öffnen</a></p>{{open_pixel}}`},
		{ID: "preset-cfo-sofortueberweisung-de", Name: "CFO – Sofortüberweisung Akquisition", Category: "bec", Difficulty: "hard", Language: "de",
			Subject: "Streng vertraulich: Akquisition erfordert sofortige Zahlung", FromName: "{{company}} CFO", FromEmail: "cfo@{{company}}-mgmt.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>{{first_name}},</p><p>wir befinden uns in einer streng vertraulichen Akquisitionsphase. Ich brauche Ihre sofortige Unterstützung bei der Überweisung eines Betrags. Details erhalten Sie nach Bestätigung Ihrer Bereitschaft.</p><p><a href="{{tracking_url}}">Bereitschaft bestätigen</a></p>{{open_pixel}}`},
		{ID: "preset-lieferantenrechnung-de", Name: "Gefälschte Lieferantenrechnung", Category: "bec", Difficulty: "easy", Language: "de",
			Subject: "Rechnung RE-2026-0847 — Fälligkeitsdatum überschritten", FromName: "Rechnungswesen Lieferant GmbH", FromEmail: "rechnungen@lieferant-gmbh-buchhaltung.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Sehr geehrte/r {{first_name}},</p><p>die Rechnung RE-2026-0847 über <b>12.450,00 €</b> ist seit 14 Tagen überfällig. Bitte veranlassen Sie die Zahlung umgehend, um Mahngebühren zu vermeiden.</p><p><a href="{{tracking_url}}">Rechnung anzeigen</a></p>{{open_pixel}}`},
		{ID: "preset-anwalt-compliance-de", Name: "Anwalt Compliance-Prüfung", Category: "bec", Difficulty: "hard", Language: "de",
			Subject: "Vertraulich: Compliance-Untersuchung – Dringende Rückmeldung", FromName: "RA Dr. Schmitt & Kollegen", FromEmail: "kanzlei@schmitt-anwaelte-de.com", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Sehr geehrte/r {{first_name}},</p><p>im Rahmen einer internen Compliance-Prüfung müssen wir einige Transaktionen der letzten 12 Monate überprüfen. Bitte antworten Sie bis morgen und stellen Sie folgende Unterlagen bereit.</p><p><a href="{{tracking_url}}">Sichere Upload-Plattform</a></p>{{open_pixel}}`},
		{ID: "preset-personalchef-gehaltsanpassung-de", Name: "HR-Chef Gehaltsanpassung", Category: "bec", Difficulty: "medium", Language: "de",
			Subject: "Vertraulich: Ihre Gehaltsanpassung zum 01.07.", FromName: "Personalleitung {{company}}", FromEmail: "personalleitung@{{company}}-hr.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>im Rahmen der jährlichen Gehaltsrunde wurde für Sie eine Anpassung beschlossen. Bitte bestätigen Sie Ihre aktuelle Kontoverbindung für die Gehaltsüberweisung.</p><p><a href="{{tracking_url}}">Kontoverbindung bestätigen</a></p>{{open_pixel}}`},
		{ID: "preset-vorstand-notfall-de", Name: "Vorstand – Notfall außerhalb Büro", Category: "bec", Difficulty: "medium", Language: "de",
			Subject: "Dringend – Ich bin im Ausland und brauche Ihre Hilfe", FromName: "{{company}} Geschäftsführung", FromEmail: "gf@{{company}}-international.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>{{first_name}}, ich bin derzeit auf einer Geschäftsreise in Seoul und habe keinen Zugriff auf mein reguläres E-Mail-Konto. Ich benötige dringend Ihre Unterstützung. Bitte melden Sie sich.</p><p><a href="{{tracking_url}}">Sicher antworten</a></p>{{open_pixel}}`},
		{ID: "preset-einkauf-bestellung-de", Name: "Einkauf – Eilbestellung bestätigen", Category: "bec", Difficulty: "easy", Language: "de",
			Subject: "Bestätigung Eilbestellung – sofortige Freigabe nötig", FromName: "Einkauf {{company}}", FromEmail: "einkauf@{{company}}-procurement.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>für ein dringendes Projekt muss eine Bestellung im Wert von <b>4.800 €</b> noch heute freigegeben werden. Der Lieferant benötigt eine sofortige Bestellbestätigung.</p><p><a href="{{tracking_url}}">Bestellung freigeben</a></p>{{open_pixel}}`},
		{ID: "preset-buchhalter-audit-de", Name: "Wirtschaftsprüfer Dokumentenanfrage", Category: "bec", Difficulty: "hard", Language: "de",
			Subject: "Jahresabschlussprüfung: Unterlagen bis Freitag benötigt", FromName: "WP Kanzlei Heinz & Braun", FromEmail: "audit@heinz-braun-wp.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Sehr geehrte/r {{first_name}},</p><p>im Rahmen der laufenden Jahresabschlussprüfung benötigen wir verschiedene Belege und Kontoauszüge. Bitte laden Sie die Unterlagen über unser sicheres Revisionsportal hoch.</p><p><a href="{{tracking_url}}">Zum Revisionsportal</a></p>{{open_pixel}}`},

		// ── Kategorie: it_software (10) ───────────────────────────────────────
		{ID: "preset-github-notification-de", Name: "GitHub Security Alert", Category: "it_software", Difficulty: "hard", Language: "de",
			Subject: "Security vulnerability found in your repository", FromName: "GitHub Security", FromEmail: "security@github-notifications-de.com", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hi {{first_name}},</p><p>A critical security vulnerability has been detected in one of your repositories. Immediate action is required to prevent unauthorized access.</p><p><a href="{{tracking_url}}">View security alert</a></p>{{open_pixel}}`},
		{ID: "preset-windows-update-de", Name: "Windows Kritisches Sicherheitsupdate", Category: "it_software", Difficulty: "easy", Language: "de",
			Subject: "Kritisches Windows-Sicherheitsupdate – sofortige Installation erforderlich", FromName: "Microsoft Windows Update", FromEmail: "updates@windows-security-patch.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>Microsoft hat einen kritischen Sicherheitspatch (KB5034763) veröffentlicht, der eine aktiv ausgenutzte Schwachstelle behebt. Installieren Sie das Update umgehend.</p><p><a href="{{tracking_url}}">Update jetzt installieren</a></p>{{open_pixel}}`},
		{ID: "preset-antivirus-warnung-de", Name: "Antivirusprogramm Warnung", Category: "it_software", Difficulty: "easy", Language: "de",
			Subject: "WARNUNG: Virus auf Ihrem Computer gefunden!", FromName: "IT Security {{company}}", FromEmail: "it-security@{{company}}-antivirus.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>unser Sicherheitssystem hat eine verdächtige Datei auf Ihrem Arbeitsrechner entdeckt. Um eine weitere Ausbreitung zu verhindern, muss Ihr PC sofort gescannt werden.</p><p><a href="{{tracking_url}}">Sicherheitsscan starten</a></p>{{open_pixel}}`},
		{ID: "preset-jira-ticket-de", Name: "Jira Ticket zugewiesen", Category: "it_software", Difficulty: "medium", Language: "de",
			Subject: "Jira: Kritisches Ticket an Sie zugewiesen – SLA läuft ab", FromName: "Jira Notifications {{company}}", FromEmail: "jira@{{company}}-atlassian.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>Ihnen wurde soeben ein kritisches Jira-Ticket zugewiesen (Priorität: Blocker). Das SLA läuft in <b>2 Stunden</b> ab. Bitte reagieren Sie sofort.</p><p><a href="{{tracking_url}}">Ticket öffnen</a></p>{{open_pixel}}`},
		{ID: "preset-ssl-zertifikat-de", Name: "SSL-Zertifikat läuft ab", Category: "it_software", Difficulty: "hard", Language: "de",
			Subject: "Dringlich: SSL-Zertifikat Ihrer Domain läuft morgen ab", FromName: "SSL-Zertifikat Service", FromEmail: "certs@ssl-renew-service-de.com", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>das SSL-Zertifikat für Ihre Domain läuft in <b>24 Stunden</b> ab. Danach wird Ihre Website als unsicher markiert. Erneuern Sie das Zertifikat jetzt.</p><p><a href="{{tracking_url}}">Zertifikat erneuern</a></p>{{open_pixel}}`},
		{ID: "preset-docker-hub-de", Name: "DockerHub Security Scan", Category: "it_software", Difficulty: "hard", Language: "de",
			Subject: "Critical vulnerabilities detected in your Docker images", FromName: "Docker Security Team", FromEmail: "security@dockerhub-notifications-de.com", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hi {{first_name}},</p><p>Our automated security scanner detected <b>3 critical CVEs</b> in your Docker images. Affected images may be exploited in production. Please review and update immediately.</p><p><a href="{{tracking_url}}">View vulnerability report</a></p>{{open_pixel}}`},
		{ID: "preset-aws-rechnung-de", Name: "AWS Ungewöhnliche Kosten", Category: "it_software", Difficulty: "medium", Language: "de",
			Subject: "Ungewöhnliche AWS-Nutzung festgestellt – sofortige Aktion", FromName: "Amazon Web Services", FromEmail: "billing-alert@aws-notifications-de.com", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>wir haben ungewöhnliche Kosten in Ihrem AWS-Konto festgestellt. Ihr Konto wurde möglicherweise kompromittiert. Bitte überprüfen Sie Ihre Aktivitäten sofort.</p><p><a href="{{tracking_url}}">AWS Konsole öffnen</a></p>{{open_pixel}}`},
		{ID: "preset-confluence-doku-de", Name: "Confluence Dokumentenfreigabe", Category: "it_software", Difficulty: "medium", Language: "de",
			Subject: "{{first_name}}, wichtige Änderungen an geteiltem Dokument", FromName: "Confluence {{company}}", FromEmail: "confluence@{{company}}-wiki.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>an einem Confluence-Dokument, das mit Ihnen geteilt ist, wurden wichtige Änderungen vorgenommen. Überprüfen Sie die Änderungen und geben Sie Ihr Feedback ab.</p><p><a href="{{tracking_url}}">Dokument prüfen</a></p>{{open_pixel}}`},
		{ID: "preset-lastpass-breach-de", Name: "Passwort-Manager Sicherheitswarnung", Category: "it_software", Difficulty: "medium", Language: "de",
			Subject: "Sicherheitswarnung: Ihr Master-Passwort könnte kompromittiert sein", FromName: "Passwort-Manager Support", FromEmail: "security@vault-security-alert.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>wir haben festgestellt, dass Ihr Passwort-Manager-Account in einem Datenleck aufgetaucht sein könnte. Ändern Sie sofort Ihr Master-Passwort und aktivieren Sie die 2-Faktor-Authentifizierung.</p><p><a href="{{tracking_url}}">Jetzt absichern</a></p>{{open_pixel}}`},
		{ID: "preset-backup-fehler-de", Name: "Backup-System Fehlerbenachrichtigung", Category: "it_software", Difficulty: "easy", Language: "de",
			Subject: "FEHLER: Backup fehlgeschlagen – sofortiger Handlungsbedarf", FromName: "Backup System {{company}}", FromEmail: "backup-alerts@{{company}}-storage.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>das letzte Backup Ihres Systems ist fehlgeschlagen. <b>Seit 3 Tagen existiert kein aktuelles Backup.</b> Bei einem Datenverlust wären alle Daten der letzten 3 Tage verloren. Bitte beheben Sie den Fehler sofort.</p><p><a href="{{tracking_url}}">Backup-Status prüfen</a></p>{{open_pixel}}`},

		// ── Kategorie: hr_payroll (10) ────────────────────────────────────────
		{ID: "preset-personalabteilung-de", Name: "Personalabteilung Gehaltsabrechnung", Category: "hr_payroll", Difficulty: "easy", Language: "de",
			Subject: "Ihre Gehaltsabrechnung Dezember 2025 (überarbeitet)", FromName: "Personalabteilung {{company}}", FromEmail: "personal@{{company}}-hr.com", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>aufgrund einer Korrektur der Sondervergütung haben wir Ihre Gehaltsabrechnung für Dezember 2025 angepasst. Bitte <a href="{{tracking_url}}">die neue Version hier ansehen</a> und bestätigen.</p><p>Viele Grüße<br/>Personalabteilung</p>{{open_pixel}}`},
		{ID: "preset-urlaubsantrag-portal-de", Name: "Urlaubsantrags-Portal Einladung", Category: "hr_payroll", Difficulty: "easy", Language: "de",
			Subject: "Neues HR-Portal: Urlaubsantrag jetzt online stellen", FromName: "HR {{company}}", FromEmail: "hr@{{company}}-urlaub.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>{{company}} hat ein neues HR-Self-Service-Portal eingeführt. Ab sofort können Sie Urlaubsanträge, Krankmeldungen und Gleitzeitanpassungen direkt online verwalten.</p><p><a href="{{tracking_url}}">Zum HR-Portal</a></p>{{open_pixel}}`},
		{ID: "preset-benefits-portal-de", Name: "Mitarbeiter-Benefits Portal", Category: "hr_payroll", Difficulty: "easy", Language: "de",
			Subject: "Ihr persönliches Benefits-Paket 2026 wartet auf Sie", FromName: "HR Benefits {{company}}", FromEmail: "benefits@{{company}}-mitarbeiter.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>Ihr individuelles Mitarbeiter-Benefits-Paket für 2026 steht bereit. Wählen Sie jetzt aus Sachbezügen, Mobilität, Weiterbildung und mehr.</p><p><a href="{{tracking_url}}">Benefits auswählen</a></p>{{open_pixel}}`},
		{ID: "preset-homeoffice-ausstattung-de", Name: "Homeoffice-Ausstattungs-Antrag", Category: "hr_payroll", Difficulty: "easy", Language: "de",
			Subject: "Beantragen Sie jetzt Ihre Homeoffice-Ausstattung (500€ Budget)", FromName: "IT & HR {{company}}", FromEmail: "homeoffice@{{company}}-equipment.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>{{company}} stellt jedem Mitarbeiter im Homeoffice ein Budget von <b>500 €</b> für Ausstattung zur Verfügung. Stellen Sie bis zum 30.06. Ihren Antrag.</p><p><a href="{{tracking_url}}">Antrag stellen</a></p>{{open_pixel}}`},
		{ID: "preset-onboarding-unterlagen-de", Name: "Onboarding-Unterlagen ausstehend", Category: "hr_payroll", Difficulty: "easy", Language: "de",
			Subject: "Wichtig: Ihre Onboarding-Unterlagen fehlen noch", FromName: "HR Onboarding {{company}}", FromEmail: "onboarding@{{company}}-hr.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Liebe/r {{first_name}},</p><p>für Ihre vollständige Registrierung im HR-System fehlen noch einige Unterlagen. Bitte laden Sie diese bis Freitag hoch, damit Ihre erste Gehaltsüberweisung termingerecht erfolgen kann.</p><p><a href="{{tracking_url}}">Unterlagen hochladen</a></p>{{open_pixel}}`},
		{ID: "preset-betriebsrat-umfrage-de", Name: "Betriebsrats-Umfrage", Category: "hr_payroll", Difficulty: "easy", Language: "de",
			Subject: "Anonyme Umfrage: Ihre Meinung zählt", FromName: "Betriebsrat {{company}}", FromEmail: "betriebsrat@{{company}}-survey.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Liebe/r {{first_name}},</p><p>wir möchten Ihre Meinung zur Arbeitssituation hören. Die Teilnahme dauert nur 3 Minuten und ist <b>vollständig anonym</b>.</p><p><a href="{{tracking_url}}">Zur Umfrage</a></p>{{open_pixel}}`},
		{ID: "preset-rentenversicherung-de", Name: "Rentenversicherung Mitteilung", Category: "hr_payroll", Difficulty: "medium", Language: "de",
			Subject: "Ihre Rentenversicherungs-Auskunft ist verfügbar", FromName: "Deutsche Rentenversicherung", FromEmail: "service@drv-online-portal.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Sehr geehrte/r {{first_name}},</p><p>Ihre aktuelle Rentenauskunft steht in Ihrem persönlichen Rentenversicherungsportal bereit. Sie enthält Ihren aktuellen Rentenanspruch und die voraussichtliche Rentenhöhe.</p><p><a href="{{tracking_url}}">Rentenauskunft abrufen</a></p>{{open_pixel}}`},
		{ID: "preset-jobwechsel-angebot-de", Name: "Vertrauliches Job-Angebot", Category: "hr_payroll", Difficulty: "medium", Language: "de",
			Subject: "Exklusives Stellenangebot für Sie", FromName: "Executive Recruitment Partner", FromEmail: "executive@karriere-netzwerk-de.com", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>aufgrund Ihres beeindruckenden Profils möchten wir Ihnen ein vertrauliches Karriereangebot unterbreiten. Die Position bietet ein Jahresgehalt von <b>25% über Ihrem aktuellen Niveau</b>.</p><p><a href="{{tracking_url}}">Angebot vertraulich ansehen</a></p>{{open_pixel}}`},
		{ID: "preset-krankenversicherung-de", Name: "Krankenversicherungs-Bonus", Category: "hr_payroll", Difficulty: "easy", Language: "de",
			Subject: "Ihr Prämienwegfall-Bonus von Ihrer Krankenkasse", FromName: "TK Techniker Krankenkasse", FromEmail: "service@tk-praemienbonus-de.com", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>Sie haben Anspruch auf einen Bonus von <b>150 €</b> im Rahmen unseres Gesundheitsbonus-Programms. Um den Bonus zu erhalten, bestätigen Sie bitte Ihre Bankverbindung.</p><p><a href="{{tracking_url}}">Bonus jetzt beantragen</a></p>{{open_pixel}}`},
		{ID: "preset-zeiterfassung-de", Name: "Zeiterfassungs-System Migration", Category: "hr_payroll", Difficulty: "easy", Language: "de",
			Subject: "Wichtig: Zugangsdaten für neues Zeiterfassungs-System", FromName: "IT HR {{company}}", FromEmail: "it-hr@{{company}}-zeiterfassung.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{company}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>{{company}} wechselt zum 01.07. auf ein neues Zeiterfassungs-System. Bitte aktivieren Sie Ihren Zugang bis spätestens Freitag, damit Ihre Überstunden korrekt übertragen werden.</p><p><a href="{{tracking_url}}">Zugang aktivieren</a></p>{{open_pixel}}`},

		// ── Kategorie: dach_specific (10) ─────────────────────────────────────
		{ID: "preset-datev-portal-de", Name: "DATEV Online-Portal Sicherheitsprüfung", Category: "dach_specific", Difficulty: "hard", Language: "de",
			Subject: "DATEV: Ihre Zugangsdaten müssen verifiziert werden", FromName: "DATEV eG Service", FromEmail: "service@datev-online-portal.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Sehr geehrte/r {{first_name}},</p><p>im Rahmen einer Sicherheitsprüfung müssen alle DATEV Online-Nutzer ihre Zugangsdaten verifizieren. Dieser Prozess ist verpflichtend und muss bis zum 20.06.2026 abgeschlossen werden.</p><p><a href="{{tracking_url}}">Jetzt verifizieren</a></p>{{open_pixel}}`},
		{ID: "preset-telekom-rechnung-de", Name: "Deutsche Telekom Rechnung", Category: "dach_specific", Difficulty: "medium", Language: "de",
			Subject: "Ihre Telekom-Rechnung für Mai 2026 ist verfügbar", FromName: "Telekom Kundenservice", FromEmail: "rechnungen@telekom-online-service.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>Ihre Monatsrechnung für Mai 2026 ist verfügbar. Aufgrund einer ungewöhnlich hohen Nutzung wurden zusätzliche Gebühren berechnet. Bitte prüfen Sie die Rechnung.</p><p><a href="{{tracking_url}}">Rechnung ansehen</a></p>{{open_pixel}}`},
		{ID: "preset-finanzamt-steuer-de", Name: "Finanzamt Steuerrückerstattung", Category: "dach_specific", Difficulty: "hard", Language: "de",
			Subject: "Ihre Steuererstattung von 847 € ist bereit", FromName: "Bundeszentralamt für Steuern", FromEmail: "erstattung@bzst-steuern.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Sehr geehrte/r {{first_name}},</p><p>nach Prüfung Ihrer Steuererklärung hat das Finanzamt eine Erstattung in Höhe von <b>847,00 €</b> festgestellt. Für die Auszahlung benötigen wir Ihre aktuelle Bankverbindung.</p><p><a href="{{tracking_url}}">IBAN jetzt eingeben</a></p>{{open_pixel}}`},
		{ID: "preset-elster-de", Name: "ELSTER Online Steuererklärung", Category: "dach_specific", Difficulty: "hard", Language: "de",
			Subject: "ELSTER: Ihre Steuererklärung muss korrigiert werden", FromName: "ELSTER Online Portal", FromEmail: "elster@finanzamt-online-portal.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Sehr geehrte/r {{first_name}},</p><p>bei der Verarbeitung Ihrer Steuererklärung wurden Unstimmigkeiten festgestellt. Bitte korrigieren Sie Ihre Angaben bis zum 30.06. im ELSTER-Portal, um Nachzahlungen zu vermeiden.</p><p><a href="{{tracking_url}}">ELSTER Portal öffnen</a></p>{{open_pixel}}`},
		{ID: "preset-vodafone-de", Name: "Vodafone Vertragsverlängerung", Category: "dach_specific", Difficulty: "easy", Language: "de",
			Subject: "Ihr Vodafone-Vertrag endet – jetzt verlängern und sparen", FromName: "Vodafone Kundenservice", FromEmail: "vertraege@vodafone-kundenservice-de.com", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>Ihr Vodafone-Vertrag läuft bald aus. Verlängern Sie jetzt und profitieren Sie von einem Treue-Bonus von <b>200 € in Ihrem nächsten Handyvertrag</b>.</p><p><a href="{{tracking_url}}">Jetzt verlängern</a></p>{{open_pixel}}`},
		{ID: "preset-o2-sicherheit-de", Name: "O2 Konto-Sicherheitswarnung", Category: "dach_specific", Difficulty: "medium", Language: "de",
			Subject: "O2: Verdächtiger Login auf Ihrem Konto", FromName: "O2 Sicherheit", FromEmail: "sicherheit@o2-account-security.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>auf Ihr O2-Konto wurde von einem unbekannten Gerät zugegriffen. Falls das nicht Sie waren, sperren Sie sofort den Zugang.</p><p><a href="{{tracking_url}}">Zugang sperren</a></p>{{open_pixel}}`},
		{ID: "preset-bundesagentur-arbeit-de", Name: "Bundesagentur für Arbeit Mitteilung", Category: "dach_specific", Difficulty: "hard", Language: "de",
			Subject: "Wichtige Mitteilung der Bundesagentur für Arbeit", FromName: "Bundesagentur für Arbeit", FromEmail: "service@ba-arbeitsagentur-online.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Sehr geehrte/r {{first_name}},</p><p>bezüglich Ihres Antrags liegt eine wichtige Nachricht vor. Für die weitere Bearbeitung benötigen wir die Bestätigung Ihrer persönlichen Daten in unserem Sicherheitsportal.</p><p><a href="{{tracking_url}}">Sicher einloggen</a></p>{{open_pixel}}`},
		{ID: "preset-dhl-paket-de", Name: "DHL Paketzustellung", Category: "dach_specific", Difficulty: "easy", Language: "de",
			Subject: "Ihr Paket konnte nicht zugestellt werden", FromName: "DHL Paket", FromEmail: "noreply@dhl-paket-tracking.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>Ihr Paket konnte heute nicht zugestellt werden. Bitte <a href="{{tracking_url}}">hier den neuen Zustelltermin auswählen</a>.</p><p>Mit freundlichen Grüßen<br/>Ihr DHL-Team</p>{{open_pixel}}`},
		{ID: "preset-post-sendung-de", Name: "Deutsche Post Einschreiben", Category: "dach_specific", Difficulty: "easy", Language: "de",
			Subject: "Einschreiben für Sie bereit – Zustellungsbenachrichtigung", FromName: "Deutsche Post", FromEmail: "benachrichtigung@post-benachrichtigung-de.com", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Hallo {{first_name}},</p><p>ein Einschreiben für Sie liegt in der nächsten Postfiliale zur Abholung bereit. Um eine Rücksendung zu vermeiden, vereinbaren Sie bitte einen neuen Zustellungstermin.</p><p><a href="{{tracking_url}}">Neuen Termin wählen</a></p>{{open_pixel}}`},
		{ID: "preset-svb-beitrag-de", Name: "Sozialversicherung Beitragsprüfung", Category: "dach_specific", Difficulty: "hard", Language: "de",
			Subject: "Ihre Sozialversicherungsbeiträge werden geprüft", FromName: "Deutsche Rentenversicherung Bund", FromEmail: "pruefung@drv-beitragspruefung.de", AttackType: "phishing", IsPreset: true,
			Placeholders: ph("{{first_name}}", "{{tracking_url}}"),
			HTMLBody:     `<p>Sehr geehrte/r {{first_name}},</p><p>im Rahmen einer routinemäßigen Beitragsprüfung bitten wir Sie, Ihre aktuellen Arbeitgeberdaten zu verifizieren. Die Prüfung ist gesetzlich vorgeschrieben (§ 28p SGB IV).</p><p><a href="{{tracking_url}}">Daten verifizieren</a></p>{{open_pixel}}`},
	}
}

// presetTrainingModules returns the bundled awareness-training curriculum.
// Modules cover the four attack types and serve as starting templates that an
// admin can clone and customize. content_url points to in-app Markdown lessons
// served via the vaktaware/training-content asset bundle.
func presetTrainingModules() []TrainingModule {
	return []TrainingModule{
		{
			ID:              "preset-train-phishing-basics",
			Title:           "Phishing-Grundlagen: Die 5 Warnsignale",
			Type:            "quiz",
			AttackType:      "phishing",
			ContentURL:      "/training/de/phishing-basics.md",
			DurationSeconds: 360,
			PassingScore:    80,
			Questions: []Question{
				{Text: "Welches dieser Merkmale ist KEIN typisches Phishing-Warnsignal?", Options: []string{"Dringlichkeit / Zeitdruck", "Persönliche Anrede mit korrektem Namen", "Generische Anrede ('Sehr geehrter Kunde')", "Rechtschreibfehler"}, Answer: 1},
				{Text: "Sie erhalten eine E-Mail vom 'CEO' mit einer dringenden Überweisungsbitte. Was tun?", Options: []string{"Sofort überweisen", "Telefonisch beim CEO rückfragen über die bekannte Nummer", "An IT weiterleiten ohne Rückfrage", "Ignorieren"}, Answer: 1},
				{Text: "Ein Link führt zu 'mircosoft-login.com'. Ist das verdächtig?", Options: []string{"Ja — Tippfehler in der Domain ist ein klassisches Phishing-Indiz", "Nein — leichter Tippfehler ist normal"}, Answer: 0},
			},
		},
		{
			ID:              "preset-train-mfa-erklaert",
			Title:           "Multi-Faktor-Authentifizierung verstehen",
			Type:            "video",
			AttackType:      "phishing",
			ContentURL:      "/training/de/mfa-erklaert.md",
			DurationSeconds: 420,
			PassingScore:    80,
			Questions: []Question{
				{Text: "Warum schützt MFA auch, wenn das Passwort gestohlen wird?", Options: []string{"Das Passwort wird länger", "Ein zweiter Faktor (Gerät/Token) ist erforderlich", "Der Login wird verzögert"}, Answer: 1},
				{Text: "Sind SMS-TAN sicher als zweiter Faktor?", Options: []string{"Ja, immer", "Nein, SIM-Swapping möglich — TOTP-App oder Hardware-Token besser"}, Answer: 1},
			},
		},
		{
			ID:              "preset-train-smishing-de",
			Title:           "Smishing — Phishing per SMS",
			Type:            "quiz",
			AttackType:      "smishing",
			ContentURL:      "/training/de/smishing.md",
			DurationSeconds: 300,
			PassingScore:    75,
			Questions: []Question{
				{Text: "Eine SMS Ihrer Bank fordert Sie auf, eine TAN per Link zu verifizieren. Was ist richtig?", Options: []string{"TAN per Link verifizieren", "SMS ignorieren — Banken senden niemals TAN-Links", "Bei der Bank zurückrufen über die offizielle Hotline auf der Rückseite Ihrer EC-Karte"}, Answer: 2},
			},
		},
		{
			ID:              "preset-train-usb-koder-de",
			Title:           "USB-Köder am Arbeitsplatz",
			Type:            "quiz",
			AttackType:      "usb",
			ContentURL:      "/training/de/usb-koder.md",
			DurationSeconds: 240,
			PassingScore:    75,
			Questions: []Question{
				{Text: "Sie finden einen USB-Stick auf dem Parkplatz. Korrektes Vorgehen?", Options: []string{"Anstecken, schauen wem er gehört", "An die IT-Abteilung abgeben — niemals an einen Firmen-Rechner anschließen", "Wegwerfen"}, Answer: 1},
				{Text: "Welches Risiko ist bei einem präparierten USB-Stick am gefährlichsten?", Options: []string{"BadUSB-Tastatur-Emulation: Stick gibt sich als Tastatur aus und tippt Schadcode", "Optisch defektes Gehäuse", "Speichergröße"}, Answer: 0},
			},
		},
		{
			ID:              "preset-train-vishing-de",
			Title:           "Vishing — Phishing per Telefon",
			Type:            "quiz",
			AttackType:      "vishing",
			ContentURL:      "/training/de/vishing.md",
			DurationSeconds: 360,
			PassingScore:    80,
			Questions: []Question{
				{Text: "Ein angeblicher 'Microsoft-Support' ruft Sie wegen eines Computer-Problems an. Was tun?", Options: []string{"Helfen lassen, Remote-Zugriff geben", "Auflegen — Microsoft ruft niemals unaufgefordert an", "Nach Mitarbeiter-Nummer fragen und dann mitmachen"}, Answer: 1},
			},
		},
	}
}

// validateTemplateHTML rejects templates that embed external image trackers.
func validateTemplateHTML(html string) error {
	re := regexp.MustCompile(`(?i)<img[^>]+src\s*=\s*["']?(https?://[^"'\s>]+)`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return fmt.Errorf("external image URL not allowed: %s", matches[1])
	}
	return nil
}

// ── Templates ─────────────────────────────────────────────────────────────────

func (s *Service) CreateTemplate(ctx context.Context, orgID, userID string, input CreateTemplateInput) (*Template, error) {
	if err := validateTemplateHTML(input.HTMLBody); err != nil {
		return nil, err
	}
	return s.repo.CreateTemplate(ctx, orgID, userID, input)
}

func (s *Service) ListTemplates(ctx context.Context, orgID string) ([]Template, error) {
	return s.repo.ListTemplates(ctx, orgID)
}

func (s *Service) GetPresetTemplates() []Template { return presetTemplates() }

// GetPresetTrainingModules returns the bundled awareness-training curriculum
// (read-only — admins clone these as a starting point for their own modules).
func (s *Service) GetPresetTrainingModules() []TrainingModule { return presetTrainingModules() }

// ── Target groups ─────────────────────────────────────────────────────────────

func (s *Service) CreateTargetGroup(ctx context.Context, orgID, name, source string) (*TargetGroup, error) {
	return s.repo.CreateTargetGroup(ctx, orgID, name, source)
}

func (s *Service) ListTargetGroups(ctx context.Context, orgID string) ([]TargetGroup, error) {
	return s.repo.ListTargetGroups(ctx, orgID)
}

// DeleteTargetGroup removes a target group and, via DB cascade, its targets.
func (s *Service) DeleteTargetGroup(ctx context.Context, orgID, groupID string) error {
	return s.repo.DeleteTargetGroup(ctx, orgID, groupID)
}

// DeleteTemplate removes an org-owned phishing template. S121-D3 (C9).
func (s *Service) DeleteTemplate(ctx context.Context, orgID, templateID string) error {
	return s.repo.DeleteTemplate(ctx, orgID, templateID)
}

// AddTarget adds a single target to a group (manual entry, as opposed to CSV import).
func (s *Service) AddTarget(ctx context.Context, orgID, groupID, email, firstName, lastName, department string) (*Target, error) {
	return s.repo.CreateTarget(ctx, orgID, groupID, email, firstName, lastName, department)
}

// ImportTargetsCSV parses a CSV string and upserts targets into the given group.
// Returns the number of successfully imported rows and a slice of per-row errors.
func (s *Service) ImportTargetsCSV(ctx context.Context, orgID, groupID, csvContent string) (int, []string) {
	var imported int
	var errs []string
	scanner := bufio.NewScanner(strings.NewReader(csvContent))
	lineNum := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineNum++
		if lineNum == 1 {
			continue // skip header
		}
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 1 {
			errs = append(errs, fmt.Sprintf("line %d: invalid", lineNum))
			continue
		}
		email := strings.TrimSpace(parts[0])
		firstName, lastName, dept := "", "", ""
		if len(parts) > 1 {
			firstName = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			lastName = strings.TrimSpace(parts[2])
		}
		if len(parts) > 3 {
			dept = strings.TrimSpace(parts[3])
		}
		if _, err := s.repo.CreateTarget(ctx, orgID, groupID, email, firstName, lastName, dept); err != nil {
			errs = append(errs, fmt.Sprintf("line %d: %v", lineNum, err))
		} else {
			imported++
		}
	}
	return imported, errs
}

func (s *Service) ListTargets(ctx context.Context, orgID, groupID string) ([]Target, error) {
	return s.repo.ListTargets(ctx, orgID, groupID)
}

// ── Landing pages ─────────────────────────────────────────────────────────────

// landingPagePolicy defines the HTML elements/attributes allowed in
// phishing-simulation landing pages. Arbitrary JavaScript (onclick, onerror,
// javascript: hrefs, <script> tags) is stripped before content reaches the DB.
var landingPagePolicy = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	// UGCPolicy already allows <a href>, <img src>, <p>, <br>, <strong>, <em>.
	// Allow id/class/style so custom branded pages render correctly.
	p.AllowAttrs("id", "class", "style").OnElements("div", "p", "span", "table", "tr", "td", "th", "h1", "h2", "h3", "h4", "img", "a")
	return p
}()

func (s *Service) CreateLandingPage(ctx context.Context, orgID, name, html string) (*LandingPage, error) {
	// Sanitize before storing — prevents Stored XSS via phishing landing pages.
	sanitized := landingPagePolicy.Sanitize(html)
	return s.repo.CreateLandingPage(ctx, orgID, name, sanitized)
}

func (s *Service) ListLandingPages(ctx context.Context, orgID string) ([]LandingPage, error) {
	return s.repo.ListLandingPages(ctx, orgID)
}

// ── Campaigns ─────────────────────────────────────────────────────────────────

func (s *Service) CreateCampaign(ctx context.Context, orgID, userID string, input CreateCampaignInput) (*Campaign, error) {
	return s.repo.CreateCampaign(ctx, orgID, userID, input)
}

func (s *Service) GetCampaign(ctx context.Context, orgID, campaignID string) (*Campaign, error) {
	return s.repo.GetCampaign(ctx, orgID, campaignID)
}

func (s *Service) ListCampaigns(ctx context.Context, orgID string) ([]Campaign, error) {
	return s.repo.ListCampaigns(ctx, orgID)
}

func (s *Service) LaunchCampaign(ctx context.Context, orgID, campaignID string) error {
	if s.smtpCfg.Host == "" {
		return fmt.Errorf("SMTP not configured")
	}
	if err := s.repo.UpdateCampaignStatus(ctx, orgID, campaignID, "running"); err != nil {
		return err
	}
	if s.asynqClient != nil {
		payload, _ := json.Marshal(map[string]string{
			"campaign_id": campaignID,
			"org_id":      orgID,
		})
		task := asynq.NewTask(TaskSendCampaign, payload)
		if _, err := s.asynqClient.EnqueueContext(ctx, task, asynq.Queue(Queue)); err != nil {
			log.Warn().Err(err).Str("campaign_id", campaignID).Msg("failed to enqueue send_campaign job")
		}
	}
	return nil
}

func (s *Service) AbortCampaign(ctx context.Context, orgID, campaignID string) error {
	return s.repo.UpdateCampaignStatus(ctx, orgID, campaignID, "aborted")
}

func (s *Service) GetCampaignStats(ctx context.Context, orgID, campaignID string) (*CampaignStats, error) {
	return s.repo.GetCampaignStats(ctx, orgID, campaignID)
}

// anonymizeForBetriebsrat redacts PII (IP, User-Agent) from tracking-event input
// when the campaign was configured with betriebsrat_mode=true. Department info
// is kept (aggregate statistics) but only if department buckets stay above a
// minimum size — that aggregation is enforced at report-rendering time.
//
// Why: §87 BetrVG and DSGVO Art. 22 require that phishing-simulation results
// cannot be attributed to individual employees by management. Storing PII
// "just in case the Betriebsrat agrees later" violates the principle of data
// minimisation. The toggle is binding from event-write time onward.
func anonymizeForBetriebsrat(betriebsratMode bool, ip, ua string) (string, string) {
	if betriebsratMode {
		return "", ""
	}
	return ip, ua
}

// RecordEvent records a tracking event (click or form_submission) for the given
// token and returns the landing page HTML to render (or a default awareness message).
func (s *Service) RecordEvent(ctx context.Context, token, eventType, ip, ua string) (string, error) {
	campaign, err := s.repo.GetCampaignByTrackingToken(ctx, token)
	if err != nil {
		return "", fmt.Errorf("invalid tracking token")
	}
	storeIP, storeUA := anonymizeForBetriebsrat(campaign.BetriebsratMode, ip, ua)
	if err := s.repo.CreateTrackingEvent(ctx, campaign.OrgID, campaign.ID, nil, "", token, eventType, storeIP, storeUA); err != nil {
		log.Warn().Err(err).Msg("failed to record tracking event")
	}
	lp, err := s.repo.GetLandingPageForCampaign(ctx, campaign.ID)
	if err != nil {
		return "<p>You have been phished. This was a security awareness simulation.</p>", nil
	}
	return lp.HTMLContent, nil
}

// RecordOpen records an email-open event for the given tracking token.
// Unlike RecordEvent it returns nothing — the caller serves the pixel directly.
func (s *Service) RecordOpen(ctx context.Context, token, ip, ua string) {
	campaign, err := s.repo.GetCampaignByTrackingToken(ctx, token)
	if err != nil {
		return
	}
	storeIP, storeUA := anonymizeForBetriebsrat(campaign.BetriebsratMode, ip, ua)
	if err := s.repo.CreateTrackingEvent(ctx, campaign.OrgID, campaign.ID, nil, "", token, "open", storeIP, storeUA); err != nil {
		log.Warn().Err(err).Msg("failed to record open event")
	}
}

// ── Training modules ──────────────────────────────────────────────────────────

func (s *Service) CreateModule(ctx context.Context, orgID, userID string, input CreateModuleInput) (*TrainingModule, error) {
	if input.PassingScore == 0 {
		input.PassingScore = 80
	}
	return s.repo.CreateModule(ctx, orgID, userID, input)
}

func (s *Service) ListModules(ctx context.Context, orgID string) ([]TrainingModule, error) {
	return s.repo.ListModules(ctx, orgID)
}

// assignmentDefaultDueDays is how far out a manually-assigned module is due
// when the caller (TrainingPage.tsx's "Assign" dialog) doesn't collect a
// due date — no other default exists anywhere else in this codebase to
// mirror, so two weeks was chosen as a reasonable default completion window.
const assignmentDefaultDueDays = 14

// AssignModule assigns a training module to a list of user emails, resolving
// each to an existing target (anywhere in the org) or creating one in a
// reserved "Manuelle Zuweisungen" group. Emails that fail to resolve/assign
// are skipped and reported back rather than failing the whole batch.
func (s *Service) AssignModule(ctx context.Context, orgID, moduleID string, emails []string) (assigned int, failed []string) {
	dueDate := time.Now().UTC().AddDate(0, 0, assignmentDefaultDueDays)
	for _, email := range emails {
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}
		target, err := s.repo.FindOrCreateTargetByEmail(ctx, orgID, email)
		if err != nil {
			failed = append(failed, email)
			continue
		}
		if _, err := s.repo.UpsertAssignment(ctx, orgID, moduleID, &target.ID, "", dueDate); err != nil {
			failed = append(failed, email)
			continue
		}
		assigned++
	}
	return assigned, failed
}

// ListAssignmentsByModule returns per-target assignment detail for a module.
func (s *Service) ListAssignmentsByModule(ctx context.Context, orgID, moduleID string) ([]AssignmentDetail, error) {
	return s.repo.ListAssignmentsByModule(ctx, orgID, moduleID)
}

// evaluateQuiz scores the submitted answers against the module's questions.
func evaluateQuiz(module *TrainingModule, answers []int) (score int, passed bool) {
	if len(module.Questions) == 0 {
		return 100, true
	}
	correct := 0
	for i, q := range module.Questions {
		if i < len(answers) && answers[i] == q.Answer {
			correct++
		}
	}
	score = correct * 100 / len(module.Questions)
	return score, score >= module.PassingScore
}

func (s *Service) CompleteAssignment(ctx context.Context, orgID, assignmentID string, input CompleteAssignmentInput) (*Completion, error) {
	assignment, err := s.repo.GetAssignment(ctx, orgID, assignmentID)
	if err != nil {
		return nil, err
	}

	modules, err := s.repo.ListModules(ctx, orgID)
	if err != nil {
		return nil, err
	}
	var module *TrainingModule
	for i := range modules {
		if modules[i].ID == assignment.ModuleID {
			module = &modules[i]
			break
		}
	}

	var score *int
	passed := true
	if module != nil && module.Type == "quiz" && len(input.Answers) > 0 {
		s, p := evaluateQuiz(module, input.Answers)
		score = &s
		passed = p
	}
	completion, err := s.repo.CreateCompletion(ctx, orgID, assignmentID, score, passed)
	if err != nil {
		return nil, err
	}

	// Enqueue cross-module evidence for SecVitals awareness controls.
	if s.asynqClient != nil && passed {
		if task, taskErr := crossevidence.NewRecordEvidenceTask(events.TrainingCompleted(orgID, assignmentID)); taskErr == nil {
			_, _ = s.asynqClient.EnqueueContext(ctx, task)
		}
	}

	return completion, nil
}

func (s *Service) ListAssignments(ctx context.Context, orgID, status string) ([]Assignment, error) {
	return s.repo.ListAssignments(ctx, orgID, status)
}

// SendCampaignEmails sends phishing simulation emails to all targets in the campaign group.
// Each email is personalised with the target's name and a unique tracking token.
func (s *Service) SendCampaignEmails(ctx context.Context, orgID, campaignID string) error {
	campaign, err := s.repo.GetCampaign(ctx, orgID, campaignID)
	if err != nil {
		return fmt.Errorf("get campaign: %w", err)
	}
	if campaign.TemplateID == nil {
		return fmt.Errorf("campaign has no template")
	}
	if campaign.GroupID == nil {
		return fmt.Errorf("campaign has no target group")
	}

	tmpl, err := s.repo.GetTemplate(ctx, orgID, *campaign.TemplateID)
	if err != nil {
		return fmt.Errorf("get template: %w", err)
	}

	targets, err := s.repo.ListTargets(ctx, orgID, *campaign.GroupID)
	if err != nil {
		return fmt.Errorf("list targets: %w", err)
	}

	// Parse once; re-execute per target.
	bodyTmpl, err := template.New("body").Parse(tmpl.HTMLBody)
	if err != nil {
		return fmt.Errorf("parse template body: %w", err)
	}

	type pendingMsg struct {
		from, to string
		body     []byte
	}
	var msgs []pendingMsg
	failed := 0

	for _, target := range targets {
		if target.IsBounced {
			continue
		}
		trackingToken := uuid.New().String()

		var bodyBuf bytes.Buffer
		data := map[string]string{
			"FirstName":   target.FirstName,
			"LastName":    target.LastName,
			"Email":       target.Email,
			"TrackingURL": s.smtpCfg.trackingURL(trackingToken),
		}
		if err := bodyTmpl.Execute(&bodyBuf, data); err != nil {
			log.Warn().Err(err).Str("target", target.Email).Msg("template render failed, skipping target")
			failed++
			continue
		}

		subject := campaign.Subject
		if subject == "" {
			subject = tmpl.Subject
		}
		fromName := campaign.FromName
		fromEmail := campaign.FromEmail
		if fromEmail == "" {
			fromEmail = s.smtpCfg.from()
		}

		body := buildMIMEMessage(fromName, fromEmail, target.Email, subject, bodyBuf.String(), trackingToken, s.smtpCfg.AppURL, campaign.TrackOpens)
		msgs = append(msgs, pendingMsg{from: fromEmail, to: target.Email, body: body})
	}

	// Send all messages over a single SMTP connection.
	sent := 0
	if len(msgs) > 0 {
		client, closeClient, err := s.openSMTPClient(msgs[0].from)
		if err != nil {
			log.Error().Err(err).Str("campaign_id", campaignID).Msg("smtp open failed")
			failed += len(msgs)
		} else {
			for _, m := range msgs {
				if err := sendViaClient(client, m.from, m.to, m.body); err != nil {
					log.Warn().Err(err).Str("target", m.to).Msg("smtp send failed")
					failed++
				} else {
					sent++
				}
			}
			closeClient()
		}
	}

	log.Info().
		Str("campaign_id", campaignID).
		Int("sent", sent).
		Int("failed", failed).
		Msg("campaign email delivery complete")

	if err := s.repo.SetCampaignCompleted(ctx, orgID, campaignID); err != nil {
		return err
	}

	// Collect auto-evidence into the unassigned inbox (best-effort).
	if autoErr := evidence_auto.CollectSecReflexEvidence(ctx, s.db, orgID, campaignID); autoErr != nil {
		log.Error().Err(autoErr).Str("campaign_id", campaignID).Msg("evidence_auto: vaktaware collection failed")
	}
	return nil
}

// openSMTPClient opens an authenticated SMTP connection and returns the client
// plus a close function. The caller must call close() when done.
func (s *Service) openSMTPClient(from string) (*smtp.Client, func(), error) {
	addr := net.JoinHostPort(s.smtpCfg.Host, s.smtpCfg.Port)

	var client *smtp.Client

	switch s.smtpCfg.Port {
	case "587": // STARTTLS
		conn, err := smtp.Dial(addr)
		if err != nil {
			return nil, nil, fmt.Errorf("smtp dial: %w", err)
		}
		if err := conn.StartTLS(&tls.Config{ServerName: s.smtpCfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			_ = conn.Close()
			return nil, nil, fmt.Errorf("starttls: %w", err)
		}
		if s.smtpCfg.User != "" {
			auth := smtp.PlainAuth("", s.smtpCfg.User, s.smtpCfg.Pass, s.smtpCfg.Host)
			if err := conn.Auth(auth); err != nil {
				_ = conn.Close()
				return nil, nil, fmt.Errorf("smtp auth: %w", err)
			}
		}
		client = conn

	case "465": // implicit TLS
		tlsConn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: s.smtpCfg.Host, MinVersion: tls.VersionTLS12})
		if err != nil {
			return nil, nil, fmt.Errorf("smtp tls dial: %w", err)
		}
		c, err := smtp.NewClient(tlsConn, s.smtpCfg.Host)
		if err != nil {
			_ = tlsConn.Close()
			return nil, nil, fmt.Errorf("smtp client: %w", err)
		}
		if s.smtpCfg.User != "" {
			auth := smtp.PlainAuth("", s.smtpCfg.User, s.smtpCfg.Pass, s.smtpCfg.Host)
			if err := c.Auth(auth); err != nil {
				_ = c.Close()
				return nil, nil, fmt.Errorf("smtp auth: %w", err)
			}
		}
		client = c

	default: // plain / port 25 (Mailpit dev)
		// smtp.SendMail handles the full lifecycle; wrap in a minimal client.
		conn, err := smtp.Dial(addr)
		if err != nil {
			return nil, nil, fmt.Errorf("smtp dial: %w", err)
		}
		if s.smtpCfg.User != "" {
			auth := smtp.PlainAuth("", s.smtpCfg.User, s.smtpCfg.Pass, s.smtpCfg.Host)
			if err := conn.Auth(auth); err != nil {
				_ = conn.Close()
				return nil, nil, fmt.Errorf("smtp auth: %w", err)
			}
		}
		client = conn
	}

	return client, func() { client.Quit() }, nil //nolint:errcheck
}

// sendViaClient delivers a single message through an already-open SMTP client.
// Each call issues MAIL FROM / RCPT TO / DATA against the existing connection.
func sendViaClient(client *smtp.Client, from, to string, msg []byte) error {
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp RCPT: %w", err)
	}
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := wc.Write(msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	return wc.Close()
}

// sendSMTP opens a connection, delivers one message, and closes. Used for
// single-recipient sends (training reminders, test emails).
func (s *Service) sendSMTP(from, to string, msg []byte) error {
	client, close, err := s.openSMTPClient(from)
	if err != nil {
		return err
	}
	defer close()
	return sendViaClient(client, from, to, msg)
}

// sanitizeHeader removes CR and LF characters from an email header value to
// prevent CRLF injection attacks (CWE-93). Callers must apply this to every
// user-supplied value that appears in a raw MIME header line.
func sanitizeHeader(v string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(v)
}

// buildMIMEMessage constructs a minimal HTML email with optional open-tracking pixel.
func buildMIMEMessage(fromName, fromEmail, to, subject, htmlBody, trackingToken, appURL string, trackOpens bool) []byte {
	body := htmlBody
	if trackOpens && trackingToken != "" {
		pixelURL := appURL + "/api/v1/vaktaware/track/" + trackingToken + "?event=open"
		pixel := fmt.Sprintf(`<img src="%s" width="1" height="1" style="display:none" alt="" />`, pixelURL)
		if idx := strings.LastIndex(body, "</body>"); idx >= 0 {
			body = body[:idx] + pixel + body[idx:]
		} else {
			body = body + pixel
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "From: %s <%s>\r\n", sanitizeHeader(fromName), sanitizeHeader(fromEmail))
	fmt.Fprintf(&b, "To: %s\r\n", sanitizeHeader(to))
	fmt.Fprintf(&b, "Subject: %s\r\n", sanitizeHeader(subject))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(body)
	return []byte(b.String())
}

// trackingURL builds the absolute URL embedded in campaign emails for click tracking.
func (c SMTPConfig) trackingURL(token string) string {
	return c.AppURL + "/api/v1/vaktaware/track/" + token
}

// from returns the configured From address or a safe default.
func (c SMTPConfig) from() string {
	if c.From != "" {
		return c.From
	}
	return "vaktaware@" + c.Host
}

// ── Phish-Button (Feature 5) ──────────────────────────────────────────────────

// RecordPhishReport handles an incoming webhook from the mail add-in.
// It validates the org token, checks whether the reported email matches an active
// campaign, creates the record, and returns the result with the is_simulation flag.
func (s *Service) RecordPhishReport(ctx context.Context, in PhishReportWebhookInput) (*PhishReport, error) {
	orgID, err := s.repo.GetOrgByPhishToken(ctx, in.OrgToken)
	if err != nil {
		return nil, fmt.Errorf("invalid org token")
	}

	campaignID, err := s.repo.findActiveCampaignForReporter(ctx, orgID, in.ReporterEmail)
	if err != nil {
		return nil, fmt.Errorf("campaign lookup: %w", err)
	}
	isSimulation := campaignID != nil

	return s.repo.CreatePhishReport(ctx, orgID, campaignID, in, isSimulation)
}

// ListPhishReports returns phishing reports for the given org.
func (s *Service) ListPhishReports(ctx context.Context, orgID string) ([]PhishReport, error) {
	return s.repo.ListPhishReports(ctx, orgID)
}

// GetPhishReportStats returns aggregate stats for an org's phishing reports.
func (s *Service) GetPhishReportStats(ctx context.Context, orgID string) (*PhishReportStats, error) {
	return s.repo.GetPhishReportStats(ctx, orgID)
}

// RegeneratePhishToken creates a new 32-byte hex token, persists it, and returns it.
func (s *Service) RegeneratePhishToken(ctx context.Context, orgID string) (string, error) {
	raw := make([]byte, 32)
	if _, err := cryptorand.Read(raw); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(raw)
	if err := s.repo.SetPhishReportToken(ctx, orgID, token); err != nil {
		return "", fmt.Errorf("store token: %w", err)
	}
	return token, nil
}

// SendTrainingReminderEmail sends a single reminder email to an employee who has
// not completed their training in the last 14 days. The email is built inline
// and delivered through the service's configured SMTP transport.
func (s *Service) SendTrainingReminderEmail(ctx context.Context, orgID, email, firstName string) error {
	if s.smtpCfg.Host == "" {
		return fmt.Errorf("SMTP not configured")
	}

	greeting := firstName
	if greeting == "" {
		greeting = email
	}

	subject := "Erinnerung: Bitte schließe dein Security-Awareness-Training ab"
	htmlBody := fmt.Sprintf(`<p>Hallo %s,</p>
<p>Du hast in den letzten 14 Tagen kein Security-Awareness-Training abgeschlossen.
Bitte melde dich in der Vakt-Plattform an und schließe dein zugewiesenes Training ab.</p>
<p>Dein IT-Sicherheitsteam</p>`, greeting)

	msg := buildMIMEMessage("Security Awareness", s.smtpCfg.from(), email, subject, htmlBody, "", s.smtpCfg.AppURL, false)
	return s.sendSMTP(s.smtpCfg.from(), email, msg)
}

// GetAssignmentCertificate generates a PDF training certificate for a completed assignment.
// Returns (pdfBytes, filename, error). Returns an error if the assignment has no completion record.
func (s *Service) GetAssignmentCertificate(ctx context.Context, orgID, assignmentID string) ([]byte, string, error) {
	assignment, err := s.repo.GetAssignment(ctx, orgID, assignmentID)
	if err != nil {
		return nil, "", fmt.Errorf("get assignment: %w", err)
	}

	completion, err := s.repo.GetCompletionByAssignment(ctx, orgID, assignmentID)
	if err != nil {
		return nil, "", fmt.Errorf("no completion found: %w", err)
	}

	module, err := s.repo.GetModuleByID(ctx, orgID, assignment.ModuleID)
	if err != nil {
		return nil, "", fmt.Errorf("get module: %w", err)
	}

	orgName := s.repo.GetOrganizationName(ctx, orgID)
	if orgName == "" {
		orgName = "Ihre Organisation"
	}

	// Determine user email from the assignment's target.
	userEmail := "Unbekannt"
	if assignment.TargetID != nil {
		if email := s.repo.GetTargetEmail(ctx, *assignment.TargetID); email != "" {
			userEmail = email
		}
	} else if assignment.Department != "" {
		userEmail = assignment.Department
	}

	pdfBytes, err := GenerateTrainingCertificatePDF(module.Title, userEmail, completion.Score, completion.Passed, completion.CompletedAt, orgName)
	if err != nil {
		return nil, "", fmt.Errorf("generate certificate pdf: %w", err)
	}

	filename := "certificate-" + assignmentID + ".pdf"
	return pdfBytes, filename, nil
}

// ExportCampaignReport generates a PDF report for the given campaign.
// Returns (pdfBytes, filename, error).
func (s *Service) ExportCampaignReport(ctx context.Context, orgID, campaignID string) ([]byte, string, error) {
	campaign, err := s.repo.GetCampaign(ctx, orgID, campaignID)
	if err != nil {
		return nil, "", fmt.Errorf("get campaign: %w", err)
	}
	stats, err := s.repo.GetCampaignStats(ctx, orgID, campaignID)
	if err != nil {
		return nil, "", fmt.Errorf("get campaign stats: %w", err)
	}
	orgName := s.repo.GetOrganizationName(ctx, orgID)

	pdf, err := GenerateCampaignReportPDF(campaign, stats, orgName)
	if err != nil {
		return nil, "", fmt.Errorf("generate pdf: %w", err)
	}
	safeName := strings.Map(func(r rune) rune {
		switch r {
		case '"', '\n', '\r', '\x00', '/', '\\':
			return '_'
		}
		return r
	}, campaign.Name)
	filename := safeName + ".pdf"
	return pdf, filename, nil
}

// ListCampaignsCursor returns campaigns using keyset pagination.
func (s *Service) ListCampaignsCursor(ctx context.Context, orgID string, cursorID string, cursorTS time.Time, limit int) ([]Campaign, error) {
	return s.repo.ListCampaignsCursor(ctx, orgID, cursorID, cursorTS, limit)
}
