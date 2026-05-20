// Package emaildigest sends a weekly security-digest e-mail to all Admin users
// of an organisation.  It uses only stdlib packages (net/smtp, html/template).
package emaildigest

import (
	"bytes"
	"context"
	"fmt"
	"net/smtp"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// SMTPConfig holds the configuration needed to send outbound e-mails.
type SMTPConfig struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

// findingSeverityCount is a count of open findings per severity level.
type findingSeverityCount struct {
	Severity string
	Count    int
}

// dsrSummary carries DSR counts for the digest body.
type dsrSummary struct {
	DueSoon  int // due_date within the next 7 days
	Overdue  int // due_date already passed, not completed/rejected
}

// adminEmail holds the e-mail address, user ID, and preferred language of an admin user.
type adminEmail struct {
	Email    string
	UserID   string
	Language string
}

// digestI18n holds the translated strings for a single language.
type digestI18n struct {
	Subject  string // printf template with one %s arg (date)
	Score    string
	Findings string
	Capas    string
	Risks    string
	DSR      string
	DueSoon  string
	Overdue  string
	Footer   string
}

// forLang returns the digestI18n for the given BCP-47 language tag, defaulting to "de".
func forLang(lang string) digestI18n {
	switch lang {
	case "en":
		return digestI18n{
			Subject:  "[Vakt] Weekly Security Overview — %s",
			Score:    "Compliance Score",
			Findings: "Open Findings",
			Capas:    "Open CAPAs",
			Risks:    "Open Risks",
			DSR:      "DSR Requests",
			DueSoon:  "Due in 7 days",
			Overdue:  "Overdue",
			Footer:   "This email was sent automatically by Vakt. To unsubscribe, disable the digest in your retention settings.",
		}
	case "fr":
		return digestI18n{
			Subject:  "[Vakt] Aperçu hebdomadaire de sécurité — %s",
			Score:    "Score de conformité",
			Findings: "Constats ouverts",
			Capas:    "CAPAs ouvertes",
			Risks:    "Risques ouverts",
			DSR:      "Demandes DSR",
			DueSoon:  "Échéance 7 jours",
			Overdue:  "En retard",
			Footer:   "Cet e-mail a été envoyé automatiquement par Vakt. Pour vous désabonner, désactivez le résumé dans vos paramètres de rétention.",
		}
	case "nl":
		return digestI18n{
			Subject:  "[Vakt] Wekelijks beveiligingsoverzicht — %s",
			Score:    "Compliance-score",
			Findings: "Openstaande bevindingen",
			Capas:    "Openstaande CAPAs",
			Risks:    "Openstaande risico's",
			DSR:      "DSR-verzoeken",
			DueSoon:  "Vervalt in 7 dagen",
			Overdue:  "Achterstallig",
			Footer:   "Deze e-mail werd automatisch verzonden door Vakt. Schakel de samenvatting uit in uw bewaarinstellingen om u af te melden.",
		}
	default: // "de"
		return digestI18n{
			Subject:  "[Vakt] Wöchentlicher Sicherheits-Überblick — %s",
			Score:    "Compliance-Score",
			Findings: "Offene Findings",
			Capas:    "Offene CAPAs",
			Risks:    "Offene Risiken",
			DSR:      "DSR-Anfragen",
			DueSoon:  "Fällig in 7 Tagen",
			Overdue:  "Überfällig",
			Footer:   "Diese E-Mail wurde automatisch von Vakt versandt. Um den Digest zu deaktivieren, ändern Sie Ihre Aufbewahrungseinstellungen.",
		}
	}
}

// DigestService sends weekly digest e-mails for an organisation.
type DigestService struct {
	db   *pgxpool.Pool
	smtp SMTPConfig
}

// NewDigestService creates a DigestService.
func NewDigestService(db *pgxpool.Pool, smtpCfg SMTPConfig) *DigestService {
	return &DigestService{db: db, smtp: smtpCfg}
}

// SendDigest composes and sends the weekly security digest to all Admin users
// of the given organisation.  It is called every hour for all orgs; it checks
// internally whether the current UTC weekday+hour matches the org's schedule.
func (s *DigestService) SendDigest(ctx context.Context, orgID string) error {
	// 1. Check digest is enabled and the current UTC time matches the org's schedule.
	var digestEnabled bool
	var digestDay, digestHour int16
	err := s.db.QueryRow(ctx,
		`SELECT digest_enabled, digest_day, digest_hour
		 FROM   retention_config
		 WHERE  org_id = $1::uuid`, orgID,
	).Scan(&digestEnabled, &digestDay, &digestHour)
	if err != nil || !digestEnabled {
		// No config row or digest disabled — skip silently.
		return nil
	}
	now := time.Now().UTC()
	// time.Weekday(): 0=Sunday … 6=Saturday — same as our digest_day convention.
	if int16(now.Weekday()) != digestDay || int16(now.Hour()) != digestHour {
		return nil
	}

	// 2. Open findings by severity.
	severityCounts, err := s.fetchSeverityCounts(ctx, orgID)
	if err != nil {
		return fmt.Errorf("emaildigest: fetch severity counts: %w", err)
	}

	// 3. DSR due soon + overdue.
	dsr, err := s.fetchDSRSummary(ctx, orgID)
	if err != nil {
		return fmt.Errorf("emaildigest: fetch dsr summary: %w", err)
	}

	// 4. Admin e-mail addresses.
	admins, err := s.fetchAdminEmails(ctx, orgID)
	if err != nil {
		return fmt.Errorf("emaildigest: fetch admin emails: %w", err)
	}
	if len(admins) == 0 {
		log.Info().Str("org_id", orgID).Msg("emaildigest: no admins found, skipping")
		return nil
	}

	// 5. Send to each admin — build per-user email with their preferred language.
	for _, a := range admins {
		if !s.weeklyDigestEnabled(ctx, a.UserID) {
			log.Debug().Str("org_id", orgID).Str("user_id", a.UserID).Msg("emaildigest: skipped (preference disabled)")
			continue
		}
		subject, body := s.buildEmail(orgID, severityCounts, dsr, a.Language)
		if err := s.send(a.Email, subject, body); err != nil {
			log.Error().Err(err).Str("org_id", orgID).Str("to", a.Email).Msg("emaildigest: send failed")
		} else {
			log.Info().Str("org_id", orgID).Str("to", a.Email).Msg("emaildigest: sent")
		}
	}
	return nil
}

// SendDigestForAllOrgs sends the digest for every org that has digest_enabled = true.
func SendDigestForAllOrgs(ctx context.Context, db *pgxpool.Pool, smtpCfg SMTPConfig) error {
	rows, err := db.Query(ctx, `
		SELECT org_id::text FROM retention_config WHERE digest_enabled = true`)
	if err != nil {
		return fmt.Errorf("emaildigest: list digest orgs: %w", err)
	}
	defer rows.Close()

	svc := NewDigestService(db, smtpCfg)
	for rows.Next() {
		var orgID string
		if err := rows.Scan(&orgID); err != nil {
			log.Error().Err(err).Msg("emaildigest: scan org_id")
			continue
		}
		if err := svc.SendDigest(ctx, orgID); err != nil {
			log.Error().Err(err).Str("org_id", orgID).Msg("emaildigest: digest failed")
		}
	}
	return rows.Err()
}

// ── private helpers ───────────────────────────────────────────────────────────

func (s *DigestService) fetchSeverityCounts(ctx context.Context, orgID string) ([]findingSeverityCount, error) {
	rows, err := s.db.Query(ctx, `
		SELECT severity, COUNT(*) AS cnt
		FROM   vb_findings
		WHERE  org_id = $1::uuid
		  AND  status NOT IN ('resolved','false_positive')
		GROUP  BY severity
		ORDER  BY CASE severity
		    WHEN 'critical' THEN 1
		    WHEN 'high'     THEN 2
		    WHEN 'medium'   THEN 3
		    WHEN 'low'      THEN 4
		    ELSE 5 END`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var counts []findingSeverityCount
	for rows.Next() {
		var fc findingSeverityCount
		if err := rows.Scan(&fc.Severity, &fc.Count); err != nil {
			return nil, err
		}
		counts = append(counts, fc)
	}
	return counts, rows.Err()
}

func (s *DigestService) fetchDSRSummary(ctx context.Context, orgID string) (dsrSummary, error) {
	var dsr dsrSummary

	err := s.db.QueryRow(ctx, `
		SELECT
		    COUNT(*) FILTER (WHERE due_date BETWEEN now() AND now() + INTERVAL '7 days') AS due_soon,
		    COUNT(*) FILTER (WHERE due_date < now() AND status NOT IN ('completed','rejected')) AS overdue
		FROM po_dsr
		WHERE org_id = $1::uuid
		  AND status NOT IN ('completed','rejected')`,
		orgID,
	).Scan(&dsr.DueSoon, &dsr.Overdue)
	if err != nil {
		return dsr, err
	}
	return dsr, nil
}

func (s *DigestService) fetchAdminEmails(ctx context.Context, orgID string) ([]adminEmail, error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.email, u.id::text, COALESCE(u.preferred_language, 'de')
		FROM   org_members om
		JOIN   users u ON u.id = om.user_id
		JOIN   roles r ON r.id = om.role_id
		WHERE  om.org_id = $1::uuid
		  AND  r.name = 'Admin'
		  AND  u.is_active = true`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var admins []adminEmail
	for rows.Next() {
		var a adminEmail
		if err := rows.Scan(&a.Email, &a.UserID, &a.Language); err != nil {
			return nil, err
		}
		admins = append(admins, a)
	}
	return admins, rows.Err()
}

// weeklyDigestEnabled returns true if the user has not disabled the weekly digest.
// Defaults to true when no preference row exists.
func (s *DigestService) weeklyDigestEnabled(ctx context.Context, userID string) bool {
	var enabled bool
	err := s.db.QueryRow(ctx, `
		SELECT email_weekly_digest
		FROM notification_preferences
		WHERE user_id = $1::uuid`,
		userID,
	).Scan(&enabled)
	if err != nil {
		// No row yet → default is true.
		return true
	}
	return enabled
}

func (s *DigestService) buildEmail(orgID string, severityCounts []findingSeverityCount, dsr dsrSummary, lang string) (subject, body string) {
	i18n := forLang(lang)
	subject = fmt.Sprintf(i18n.Subject, time.Now().UTC().Format("2006-01-02"))

	var buf bytes.Buffer
	buf.WriteString(`<!DOCTYPE html><html><body style="font-family:sans-serif;color:#1a202c;">`)
	fmt.Fprintf(&buf, `<h2 style="color:#2b6cb0;">Vakt — %s</h2>`, i18n.Findings)
	fmt.Fprintf(&buf, `<p>Generated: %s UTC</p>`, time.Now().UTC().Format("2006-01-02 15:04"))
	fmt.Fprintf(&buf, `<p style="color:#718096;font-size:0.85em;">Org: %s</p>`, orgID)

	// Findings section
	fmt.Fprintf(&buf, `<h3>%s</h3>`, i18n.Findings)
	if len(severityCounts) == 0 {
		buf.WriteString(`<p style="color:#38a169;">&#10003;</p>`)
	} else {
		buf.WriteString(`<table border="1" cellpadding="6" cellspacing="0" style="border-collapse:collapse;">`)
		buf.WriteString(`<tr style="background:#ebf8ff;"><th>Severity</th><th>Count</th></tr>`)
		for _, fc := range severityCounts {
			color := "#1a202c"
			switch fc.Severity {
			case "critical":
				color = "#c53030"
			case "high":
				color = "#c05621"
			case "medium":
				color = "#b7791f"
			case "low":
				color = "#2f855a"
			}
			fmt.Fprintf(&buf,
				`<tr><td style="color:%s;font-weight:bold;">%s</td><td>%d</td></tr>`,
				color, fc.Severity, fc.Count,
			)
		}
		buf.WriteString(`</table>`)
	}

	// DSR section
	fmt.Fprintf(&buf, `<h3>%s</h3>`, i18n.DSR)
	buf.WriteString(`<table border="1" cellpadding="6" cellspacing="0" style="border-collapse:collapse;">`)
	buf.WriteString(`<tr style="background:#ebf8ff;"><th>Category</th><th>Count</th></tr>`)
	fmt.Fprintf(&buf, `<tr><td>%s</td><td>%d</td></tr>`, i18n.DueSoon, dsr.DueSoon)
	overdueColor := "#1a202c"
	if dsr.Overdue > 0 {
		overdueColor = "#c53030"
	}
	fmt.Fprintf(&buf,
		`<tr><td style="color:%s;">%s</td><td style="color:%s;font-weight:bold;">%d</td></tr>`,
		overdueColor, i18n.Overdue, overdueColor, dsr.Overdue,
	)
	buf.WriteString(`</table>`)

	buf.WriteString(`<hr style="margin-top:24px;"/><p style="font-size:0.75em;color:#a0aec0;">`)
	buf.WriteString(i18n.Footer)
	buf.WriteString(`</p></body></html>`)

	return subject, buf.String()
}

// send delivers the e-mail using stdlib net/smtp.
func (s *DigestService) send(to, subject, htmlBody string) error {
	if s.smtp.Host == "" {
		return fmt.Errorf("emaildigest: SMTP host not configured")
	}

	headers := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n",
		s.smtp.From, to, subject,
	)
	msg := []byte(headers + htmlBody)

	addr := s.smtp.Host + ":" + s.smtp.Port
	if addr == ":" {
		addr = "localhost:25"
	}

	if s.smtp.User != "" && s.smtp.Pass != "" {
		auth := smtp.PlainAuth("", s.smtp.User, s.smtp.Pass, s.smtp.Host)
		return smtp.SendMail(addr, auth, s.smtp.From, []string{to}, msg)
	}
	return smtp.SendMail(addr, nil, s.smtp.From, []string{to}, msg)
}
