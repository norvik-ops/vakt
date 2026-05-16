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

// adminEmail holds the e-mail address of an admin user.
type adminEmail struct {
	Email string
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
// of the given organisation.
func (s *DigestService) SendDigest(ctx context.Context, orgID string) error {
	// 1. Check digest is enabled for this org.
	var digestEnabled bool
	err := s.db.QueryRow(ctx,
		`SELECT digest_enabled FROM retention_config WHERE org_id = $1::uuid`, orgID,
	).Scan(&digestEnabled)
	if err != nil || !digestEnabled {
		// No config row or digest disabled — skip silently.
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

	// 5. Build e-mail body.
	subject, body := s.buildEmail(orgID, severityCounts, dsr)

	// 6. Send to each admin.
	for _, a := range admins {
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
		SELECT u.email
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
		if err := rows.Scan(&a.Email); err != nil {
			return nil, err
		}
		admins = append(admins, a)
	}
	return admins, rows.Err()
}

func (s *DigestService) buildEmail(orgID string, severityCounts []findingSeverityCount, dsr dsrSummary) (subject, body string) {
	subject = fmt.Sprintf("[Vakt] Weekly Security Digest — %s", time.Now().UTC().Format("2006-01-02"))

	var buf bytes.Buffer
	buf.WriteString(`<!DOCTYPE html><html><body style="font-family:sans-serif;color:#1a202c;">`)
	buf.WriteString(`<h2 style="color:#2b6cb0;">Vakt — Weekly Security Digest</h2>`)
	buf.WriteString(fmt.Sprintf(`<p>Generated: %s UTC</p>`, time.Now().UTC().Format("2006-01-02 15:04")))
	buf.WriteString(fmt.Sprintf(`<p style="color:#718096;font-size:0.85em;">Org: %s</p>`, orgID))

	// Findings section
	buf.WriteString(`<h3>Open Findings</h3>`)
	if len(severityCounts) == 0 {
		buf.WriteString(`<p style="color:#38a169;">No open findings — great work!</p>`)
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
			buf.WriteString(fmt.Sprintf(
				`<tr><td style="color:%s;font-weight:bold;">%s</td><td>%d</td></tr>`,
				color, fc.Severity, fc.Count,
			))
		}
		buf.WriteString(`</table>`)
	}

	// DSR section
	buf.WriteString(`<h3>DSR Requests</h3>`)
	buf.WriteString(`<table border="1" cellpadding="6" cellspacing="0" style="border-collapse:collapse;">`)
	buf.WriteString(`<tr style="background:#ebf8ff;"><th>Category</th><th>Count</th></tr>`)
	buf.WriteString(fmt.Sprintf(`<tr><td>Due within 7 days</td><td>%d</td></tr>`, dsr.DueSoon))
	overdueColor := "#1a202c"
	if dsr.Overdue > 0 {
		overdueColor = "#c53030"
	}
	buf.WriteString(fmt.Sprintf(
		`<tr><td style="color:%s;">Overdue</td><td style="color:%s;font-weight:bold;">%d</td></tr>`,
		overdueColor, overdueColor, dsr.Overdue,
	))
	buf.WriteString(`</table>`)

	buf.WriteString(`<hr style="margin-top:24px;"/><p style="font-size:0.75em;color:#a0aec0;">`)
	buf.WriteString(`This digest was sent automatically by Vakt. To unsubscribe, disable the digest in your retention settings.</p>`)
	buf.WriteString(`</body></html>`)

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
