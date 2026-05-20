// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package scheduledreports provides automated periodic report delivery via email.
package scheduledreports

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// TaskProcessScheduledReports is the Asynq task type for the daily report cron.
const TaskProcessScheduledReports = "scheduled_reports:process_due"

// SMTPConfig holds the configuration needed to send outbound e-mails.
type SMTPConfig struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

// ScheduledReport is a persisted report schedule.
type ScheduledReport struct {
	ID         string     `json:"id"`
	OrgID      string     `json:"org_id"`
	Name       string     `json:"name"`
	ReportType string     `json:"report_type"` // compliance | findings | risk
	Schedule   string     `json:"schedule"`    // weekly | monthly | quarterly
	Recipients []string   `json:"recipients"`
	Format     string     `json:"format"` // pdf | csv
	Active     bool       `json:"active"`
	LastRunAt  *time.Time `json:"last_run_at,omitempty"`
	NextRunAt  *time.Time `json:"next_run_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// CreateScheduledReportInput is the validated input for creating a report schedule.
type CreateScheduledReportInput struct {
	Name       string   `json:"name"        validate:"required,min=1,max=255"`
	ReportType string   `json:"report_type" validate:"required,oneof=compliance findings risk board_report"`
	Schedule   string   `json:"schedule"    validate:"required,oneof=weekly monthly quarterly"`
	Recipients []string `json:"recipients"  validate:"required,min=1"`
	Format     string   `json:"format"      validate:"omitempty,oneof=pdf csv"`
	Active     bool     `json:"active"`
}

// UpdateScheduledReportInput is the validated input for updating a report schedule.
type UpdateScheduledReportInput struct {
	Name       *string  `json:"name"        validate:"omitempty,min=1,max=255"`
	ReportType *string  `json:"report_type" validate:"omitempty,oneof=compliance findings risk board_report"`
	Schedule   *string  `json:"schedule"    validate:"omitempty,oneof=weekly monthly quarterly"`
	Recipients []string `json:"recipients"`
	Format     *string  `json:"format"      validate:"omitempty,oneof=pdf csv"`
	Active     *bool    `json:"active"`
}

// BoardReportProvider is implemented by the secvitals Service to generate board
// report PDFs. Using an interface avoids a circular import between scheduledreports
// and secvitals.
type BoardReportProvider interface {
	GetBoardReportPDF(ctx context.Context, orgID string) ([]byte, error)
}

// Service manages scheduled report CRUD and delivery.
type Service struct {
	db          *pgxpool.Pool
	smtp        SMTPConfig
	boardReport BoardReportProvider
}

// NewService creates a new scheduled reports service.
func NewService(db *pgxpool.Pool, smtpCfg SMTPConfig) *Service {
	return &Service{db: db, smtp: smtpCfg}
}

// WithBoardReportProvider attaches a board report provider to the service.
// Call this after NewService when the secvitals service is available.
func (s *Service) WithBoardReportProvider(p BoardReportProvider) *Service {
	s.boardReport = p
	return s
}

// List returns all scheduled reports for the given org.
func (s *Service) List(ctx context.Context, orgID string) ([]ScheduledReport, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id::text, org_id::text, name, report_type, schedule,
		       recipients, format, active, last_run_at, next_run_at, created_at
		FROM scheduled_reports
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list scheduled reports: %w", err)
	}
	defer rows.Close()

	var out []ScheduledReport
	for rows.Next() {
		var r ScheduledReport
		if err := rows.Scan(
			&r.ID, &r.OrgID, &r.Name, &r.ReportType, &r.Schedule,
			&r.Recipients, &r.Format, &r.Active, &r.LastRunAt, &r.NextRunAt, &r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan scheduled report: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scheduled reports: %w", err)
	}
	if out == nil {
		out = []ScheduledReport{}
	}
	return out, nil
}

// Create inserts a new scheduled report for the org.
func (s *Service) Create(ctx context.Context, orgID string, input CreateScheduledReportInput) (*ScheduledReport, error) {
	if input.Format == "" {
		input.Format = "pdf"
	}
	nextRun := ComputeNextRun(input.Schedule)

	var r ScheduledReport
	err := s.db.QueryRow(ctx, `
		INSERT INTO scheduled_reports (org_id, name, report_type, schedule, recipients, format, active, next_run_at)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id::text, org_id::text, name, report_type, schedule,
		          recipients, format, active, last_run_at, next_run_at, created_at`,
		orgID, input.Name, input.ReportType, input.Schedule,
		input.Recipients, input.Format, input.Active, nextRun,
	).Scan(
		&r.ID, &r.OrgID, &r.Name, &r.ReportType, &r.Schedule,
		&r.Recipients, &r.Format, &r.Active, &r.LastRunAt, &r.NextRunAt, &r.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create scheduled report: %w", err)
	}
	return &r, nil
}

// Update applies a partial update to a scheduled report owned by orgID.
func (s *Service) Update(ctx context.Context, id, orgID string, input UpdateScheduledReportInput) (*ScheduledReport, error) {
	// Recompute next_run_at when schedule changes.
	var nextRun *time.Time
	if input.Schedule != nil {
		t := ComputeNextRun(*input.Schedule)
		nextRun = &t
	}

	var r ScheduledReport
	err := s.db.QueryRow(ctx, `
		UPDATE scheduled_reports
		SET
			name        = COALESCE($3, name),
			report_type = COALESCE($4, report_type),
			schedule    = COALESCE($5, schedule),
			recipients  = CASE WHEN $6::boolean THEN $7 ELSE recipients END,
			format      = COALESCE($8, format),
			active      = COALESCE($9, active),
			next_run_at = CASE WHEN $10::boolean THEN $11 ELSE next_run_at END
		WHERE id = $1::uuid AND org_id = $2::uuid
		RETURNING id::text, org_id::text, name, report_type, schedule,
		          recipients, format, active, last_run_at, next_run_at, created_at`,
		id, orgID,
		input.Name, input.ReportType, input.Schedule,
		input.Recipients != nil, input.Recipients,
		input.Format,
		input.Active,
		nextRun != nil, nextRun,
	).Scan(
		&r.ID, &r.OrgID, &r.Name, &r.ReportType, &r.Schedule,
		&r.Recipients, &r.Format, &r.Active, &r.LastRunAt, &r.NextRunAt, &r.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update scheduled report: %w", err)
	}
	return &r, nil
}

// Delete removes a scheduled report owned by orgID.
func (s *Service) Delete(ctx context.Context, id, orgID string) error {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM scheduled_reports WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete scheduled report: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("scheduled report not found")
	}
	return nil
}

// RunNow executes a scheduled report immediately, ignoring schedule timing.
func (s *Service) RunNow(ctx context.Context, id, orgID string) error {
	var r ScheduledReport
	err := s.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, name, report_type, schedule,
		       recipients, format, active, last_run_at, next_run_at, created_at
		FROM scheduled_reports
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
	).Scan(
		&r.ID, &r.OrgID, &r.Name, &r.ReportType, &r.Schedule,
		&r.Recipients, &r.Format, &r.Active, &r.LastRunAt, &r.NextRunAt, &r.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("scheduled report not found: %w", err)
	}
	return s.RunReport(ctx, r)
}

// ComputeNextRun returns the next scheduled run time for the given schedule string.
//   - "weekly"    → next Monday at 08:00 UTC
//   - "monthly"   → 1st of next month at 08:00 UTC
//   - "quarterly" → start of next calendar quarter at 08:00 UTC
func ComputeNextRun(schedule string) time.Time {
	now := time.Now().UTC()
	switch schedule {
	case "weekly":
		// Advance to the next Monday.
		daysUntilMonday := (8 - int(now.Weekday())) % 7
		if daysUntilMonday == 0 {
			daysUntilMonday = 7
		}
		next := now.AddDate(0, 0, daysUntilMonday)
		return time.Date(next.Year(), next.Month(), next.Day(), 8, 0, 0, 0, time.UTC)
	case "monthly":
		// 1st of next month.
		next := time.Date(now.Year(), now.Month()+1, 1, 8, 0, 0, 0, time.UTC)
		return next
	case "quarterly":
		// Start of next quarter: Jan, Apr, Jul, Oct.
		month := now.Month()
		var nextQMonth time.Month
		var nextQYear int
		switch {
		case month < time.April:
			nextQMonth = time.April
			nextQYear = now.Year()
		case month < time.July:
			nextQMonth = time.July
			nextQYear = now.Year()
		case month < time.October:
			nextQMonth = time.October
			nextQYear = now.Year()
		default:
			nextQMonth = time.January
			nextQYear = now.Year() + 1
		}
		return time.Date(nextQYear, nextQMonth, 1, 8, 0, 0, 0, time.UTC)
	default:
		// Unknown schedule — default to one week from now.
		return now.AddDate(0, 0, 7).Truncate(time.Hour)
	}
}

// ProcessDue runs all active reports where next_run_at <= now.
// Called by the daily cron job in the worker.
func (s *Service) ProcessDue(ctx context.Context) error {
	rows, err := s.db.Query(ctx, `
		SELECT id::text, org_id::text, name, report_type, schedule,
		       recipients, format, active, last_run_at, next_run_at, created_at
		FROM scheduled_reports
		WHERE active = true
		  AND next_run_at <= NOW()
		ORDER BY next_run_at ASC`,
	)
	if err != nil {
		return fmt.Errorf("query due reports: %w", err)
	}
	defer rows.Close()

	var reports []ScheduledReport
	for rows.Next() {
		var r ScheduledReport
		if err := rows.Scan(
			&r.ID, &r.OrgID, &r.Name, &r.ReportType, &r.Schedule,
			&r.Recipients, &r.Format, &r.Active, &r.LastRunAt, &r.NextRunAt, &r.CreatedAt,
		); err != nil {
			log.Error().Err(err).Msg("scheduled_reports: scan due report")
			continue
		}
		reports = append(reports, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate due reports: %w", err)
	}

	for _, r := range reports {
		if runErr := s.RunReport(ctx, r); runErr != nil {
			log.Error().Err(runErr).Str("report_id", r.ID).Msg("scheduled_reports: run failed")
		}
		nextRun := ComputeNextRun(r.Schedule)
		if _, err := s.db.Exec(ctx, `
			UPDATE scheduled_reports
			SET last_run_at = NOW(), next_run_at = $2
			WHERE id = $1::uuid`,
			r.ID, nextRun,
		); err != nil {
			log.Error().Err(err).Str("report_id", r.ID).Msg("scheduled_reports: update next_run_at failed")
		}
	}
	return nil
}

// RunReport executes the report and sends it to all recipients.
// For findings+csv it fetches findings and sends a CSV attachment.
// For other types it sends a placeholder email.
func (s *Service) RunReport(ctx context.Context, r ScheduledReport) error {
	if len(r.Recipients) == 0 {
		log.Warn().Str("report_id", r.ID).Msg("scheduled_reports: no recipients configured")
		return nil
	}

	subject := fmt.Sprintf("[Vakt] Geplanter Bericht: %s", r.Name)

	var (
		body     string
		attachFn func() ([]byte, string, error) // returns data, filename, error
	)

	switch {
	case r.ReportType == "findings" && r.Format == "csv":
		csvData, err := s.buildFindingsCSV(ctx, r.OrgID)
		if err != nil {
			log.Error().Err(err).Str("report_id", r.ID).Msg("scheduled_reports: build findings CSV failed")
			csvData = []byte("id,title,severity,status,asset_id,created_at\n")
		}
		body = fmt.Sprintf(
			"<p>Ihr geplanter Findings-Bericht <strong>%s</strong> ist beigefügt.</p>"+
				"<p>Format: CSV | Zeitraum: %s</p>",
			r.Name, r.Schedule,
		)
		csvBytes := csvData
		attachFn = func() ([]byte, string, error) {
			return csvBytes, "findings.csv", nil
		}
	case r.ReportType == "board_report":
		body = fmt.Sprintf(
			"<p>Ihr geplanter Management-Board-Bericht <strong>%s</strong> ist beigefügt.</p>"+
				"<p>Zeitplan: %s</p>",
			r.Name, r.Schedule,
		)
		if s.boardReport != nil {
			orgID := r.OrgID
			attachFn = func() ([]byte, string, error) {
				pdfBytes, err := s.boardReport.GetBoardReportPDF(ctx, orgID)
				if err != nil {
					return nil, "", fmt.Errorf("board report PDF: %w", err)
				}
				filename := fmt.Sprintf("vakt-board-report-%s.pdf", time.Now().UTC().Format("2006-01-02"))
				return pdfBytes, filename, nil
			}
		} else {
			log.Warn().Str("report_id", r.ID).Msg("scheduled_reports: board report provider not configured")
		}
	default:
		body = fmt.Sprintf(
			"<p>Ihr geplanter Bericht <strong>%s</strong> (Typ: %s) wird demnächst angehängt.</p>"+
				"<p>Format: %s | Zeitplan: %s</p>",
			r.Name, r.ReportType, r.Format, r.Schedule,
		)
	}

	for _, to := range r.Recipients {
		var err error
		if attachFn != nil {
			data, filename, attachErr := attachFn()
			if attachErr != nil {
				log.Error().Err(attachErr).Str("to", to).Msg("scheduled_reports: build attachment failed")
				err = s.sendHTML(to, subject, body)
			} else {
				err = s.sendWithAttachment(to, subject, body, filename, data)
			}
		} else {
			err = s.sendHTML(to, subject, body)
		}
		if err != nil {
			log.Error().Err(err).Str("to", to).Str("report_id", r.ID).Msg("scheduled_reports: send failed")
		}
	}
	return nil
}

// buildFindingsCSV queries open findings for the org and returns a CSV byte slice.
func (s *Service) buildFindingsCSV(ctx context.Context, orgID string) ([]byte, error) {
	rows, err := s.db.Query(ctx, `
		SELECT f.id::text, f.title, f.severity, f.status,
		       f.asset_id::text, f.created_at
		FROM vb_findings f
		WHERE f.org_id = $1::uuid
		ORDER BY f.created_at DESC
		LIMIT 5000`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("query findings: %w", err)
	}
	defer rows.Close()

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"id", "title", "severity", "status", "asset_id", "created_at"})
	for rows.Next() {
		var (
			id, title, severity, status, assetID string
			createdAt                            time.Time
		)
		if err := rows.Scan(&id, &title, &severity, &status, &assetID, &createdAt); err != nil {
			continue
		}
		_ = w.Write([]string{id, title, severity, status, assetID, createdAt.Format(time.RFC3339)})
	}
	w.Flush()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// sendHTML sends a plain HTML email (no attachment).
func (s *Service) sendHTML(to, subject, htmlBody string) error {
	if s.smtp.Host == "" {
		log.Warn().Str("to", to).Msg("scheduled_reports: SMTP not configured, skipping send")
		return nil
	}
	headers := strings.Join([]string{
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"From: " + s.smtp.From,
		"To: " + to,
		"Subject: " + subject,
		"",
		"",
	}, "\r\n")
	msg := []byte(headers + htmlBody)
	return s.smtpSend(to, msg)
}

// sendWithAttachment sends an HTML email with a single file attachment.
func (s *Service) sendWithAttachment(to, subject, htmlBody, filename string, data []byte) error {
	if s.smtp.Host == "" {
		log.Warn().Str("to", to).Msg("scheduled_reports: SMTP not configured, skipping send")
		return nil
	}

	boundary := "vakt-report-boundary-001"
	var buf bytes.Buffer

	// Headers
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary)
	fmt.Fprintf(&buf, "From: %s\r\n", s.smtp.From)
	fmt.Fprintf(&buf, "To: %s\r\n", to)
	fmt.Fprintf(&buf, "Subject: %s\r\n", subject)
	fmt.Fprintf(&buf, "\r\n")

	// HTML part
	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Type: text/html; charset=UTF-8\r\n")
	fmt.Fprintf(&buf, "\r\n")
	fmt.Fprintf(&buf, "%s\r\n", htmlBody)

	// Attachment part — encode as base64
	encoded := encodeBase64(data)
	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Type: application/octet-stream\r\n")
	fmt.Fprintf(&buf, "Content-Disposition: attachment; filename=\"%s\"\r\n", filename)
	fmt.Fprintf(&buf, "Content-Transfer-Encoding: base64\r\n")
	fmt.Fprintf(&buf, "\r\n")
	fmt.Fprintf(&buf, "%s\r\n", encoded)
	fmt.Fprintf(&buf, "--%s--\r\n", boundary)

	return s.smtpSend(to, buf.Bytes())
}

// smtpSend delivers a raw MIME message via configured SMTP server.
func (s *Service) smtpSend(to string, msg []byte) error {
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

// encodeBase64 encodes data as MIME-compatible base64 with 76-char line wrapping.
func encodeBase64(data []byte) string {
	const lineLen = 76
	encoded := base64.StdEncoding.EncodeToString(data)
	var sb strings.Builder
	for len(encoded) > lineLen {
		sb.WriteString(encoded[:lineLen])
		sb.WriteString("\r\n")
		encoded = encoded[lineLen:]
	}
	sb.WriteString(encoded)
	return sb.String()
}
