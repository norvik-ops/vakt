package secreflex

import (
	"bufio"
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/smtp"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/sechealth-app/sechealth/internal/shared/crossevidence"
	"github.com/sechealth-app/sechealth/internal/shared/evidence_auto"
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

// presetTemplates returns hardcoded preset phishing templates.
func presetTemplates() []Template {
	return []Template{
		{
			ID:         "preset-ceo-fraud",
			Name:       "CEO Fraud",
			Subject:    "Urgent: Wire Transfer Required",
			FromName:   "{{company}} CEO",
			FromEmail:  "ceo@{{company}}.com",
			HTMLBody:   `<p>Hi {{first_name}}, I need you to process a wire transfer urgently...</p>`,
			AttackType: "phishing",
			IsPreset:   true,
		},
		{
			ID:         "preset-it-helpdesk",
			Name:       "IT Helpdesk",
			Subject:    "Your password expires today",
			FromName:   "IT Helpdesk",
			FromEmail:  "helpdesk@{{company}}.com",
			HTMLBody:   `<p>Hi {{first_name}}, your password expires today. Click <a href="{{tracking_url}}">here</a> to reset it.</p>`,
			AttackType: "phishing",
			IsPreset:   true,
		},
		{
			ID:         "preset-package-delivery",
			Name:       "Package Delivery",
			Subject:    "Your package could not be delivered",
			FromName:   "DHL Express",
			FromEmail:  "noreply@dhl-delivery.com",
			HTMLBody:   `<p>Dear {{first_name}}, your package could not be delivered. <a href="{{tracking_url}}">Track here</a></p>`,
			AttackType: "phishing",
			IsPreset:   true,
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

// ── Target groups ─────────────────────────────────────────────────────────────

func (s *Service) CreateTargetGroup(ctx context.Context, orgID, name, source string) (*TargetGroup, error) {
	return s.repo.CreateTargetGroup(ctx, orgID, name, source)
}

func (s *Service) ListTargetGroups(ctx context.Context, orgID string) ([]TargetGroup, error) {
	return s.repo.ListTargetGroups(ctx, orgID)
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

func (s *Service) CreateLandingPage(ctx context.Context, orgID, name, html string) (*LandingPage, error) {
	return s.repo.CreateLandingPage(ctx, orgID, name, html)
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
		if _, err := s.asynqClient.EnqueueContext(ctx, task, asynq.Queue("default")); err != nil {
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

// RecordEvent records a tracking event (click or form_submission) for the given
// token and returns the landing page HTML to render (or a default awareness message).
func (s *Service) RecordEvent(ctx context.Context, token, eventType, ip, ua string) (string, error) {
	campaign, err := s.repo.GetCampaignByTrackingToken(ctx, token)
	if err != nil {
		return "", fmt.Errorf("invalid tracking token")
	}
	if err := s.repo.CreateTrackingEvent(ctx, campaign.OrgID, campaign.ID, nil, "", token, eventType, ip, ua); err != nil {
		log.Warn().Err(err).Msg("failed to record tracking event")
	}
	lp, err := s.repo.GetLandingPageForCampaign(ctx, campaign.ID)
	if err != nil {
		return "<p>You have been phished. This was a security awareness simulation.</p>", nil
	}
	return lp.HTMLContent, nil
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
		p := crossevidence.EvidencePayload{
			OrgID:        orgID,
			Source:       "secreflex",
			ResourceType: "training_completion",
			ResourceID:   assignmentID,
			Title:        "Security Awareness Training abgeschlossen",
			Description:  "Ein Mitarbeiter hat ein Security Awareness Training erfolgreich absolviert.",
			OccurredAt:   time.Now(),
		}
		if task, taskErr := crossevidence.NewRecordEvidenceTask(p); taskErr == nil {
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

	sent, failed := 0, 0
	for _, target := range targets {
		if target.IsBounced {
			continue
		}
		trackingToken := uuid.New().String()

		var bodyBuf bytes.Buffer
		data := map[string]string{
			"FirstName":    target.FirstName,
			"LastName":     target.LastName,
			"Email":        target.Email,
			"TrackingURL":  s.smtpCfg.trackingURL(trackingToken),
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

		msg := buildMIMEMessage(fromName, fromEmail, target.Email, subject, bodyBuf.String(), trackingToken, s.smtpCfg.AppURL, campaign.TrackOpens)

		if err := s.sendSMTP(fromEmail, target.Email, msg); err != nil {
			log.Warn().Err(err).Str("target", target.Email).Msg("smtp send failed")
			failed++
			continue
		}
		sent++
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
		log.Error().Err(autoErr).Str("campaign_id", campaignID).Msg("evidence_auto: secreflex collection failed")
	}
	return nil
}

// sendSMTP connects to the configured SMTP server and delivers a single message.
func (s *Service) sendSMTP(from, to string, msg []byte) error {
	addr := net.JoinHostPort(s.smtpCfg.Host, s.smtpCfg.Port)

	// Port 587 — STARTTLS
	if s.smtpCfg.Port == "587" {
		conn, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("smtp dial: %w", err)
		}
		defer conn.Close()

		if err := conn.StartTLS(&tls.Config{ServerName: s.smtpCfg.Host}); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
		if s.smtpCfg.User != "" {
			auth := smtp.PlainAuth("", s.smtpCfg.User, s.smtpCfg.Pass, s.smtpCfg.Host)
			if err := conn.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
		if err := conn.Mail(from); err != nil {
			return fmt.Errorf("smtp MAIL: %w", err)
		}
		if err := conn.Rcpt(to); err != nil {
			return fmt.Errorf("smtp RCPT: %w", err)
		}
		wc, err := conn.Data()
		if err != nil {
			return fmt.Errorf("smtp DATA: %w", err)
		}
		if _, err := wc.Write(msg); err != nil {
			return fmt.Errorf("smtp write: %w", err)
		}
		return wc.Close()
	}

	// Port 465 — implicit TLS
	if s.smtpCfg.Port == "465" {
		tlsConn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: s.smtpCfg.Host})
		if err != nil {
			return fmt.Errorf("smtp tls dial: %w", err)
		}
		client, err := smtp.NewClient(tlsConn, s.smtpCfg.Host)
		if err != nil {
			return fmt.Errorf("smtp client: %w", err)
		}
		defer client.Close()

		if s.smtpCfg.User != "" {
			auth := smtp.PlainAuth("", s.smtpCfg.User, s.smtpCfg.Pass, s.smtpCfg.Host)
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
		if err := client.Mail(from); err != nil {
			return err
		}
		if err := client.Rcpt(to); err != nil {
			return err
		}
		wc, err := client.Data()
		if err != nil {
			return err
		}
		if _, err := wc.Write(msg); err != nil {
			return err
		}
		return wc.Close()
	}

	// Default — plain (Mailpit dev / port 25)
	var auth smtp.Auth
	if s.smtpCfg.User != "" {
		auth = smtp.PlainAuth("", s.smtpCfg.User, s.smtpCfg.Pass, s.smtpCfg.Host)
	}
	return smtp.SendMail(addr, auth, from, []string{to}, msg)
}

// buildMIMEMessage constructs a minimal HTML email with optional open-tracking pixel.
func buildMIMEMessage(fromName, fromEmail, to, subject, htmlBody, trackingToken, appURL string, trackOpens bool) []byte {
	body := htmlBody
	if trackOpens && trackingToken != "" {
		pixelURL := appURL + "/api/v1/secreflex/track/" + trackingToken + "?event=open"
		pixel := fmt.Sprintf(`<img src="%s" width="1" height="1" style="display:none" alt="" />`, pixelURL)
		if idx := strings.LastIndex(body, "</body>"); idx >= 0 {
			body = body[:idx] + pixel + body[idx:]
		} else {
			body = body + pixel
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("From: %s <%s>\r\n", fromName, fromEmail))
	b.WriteString(fmt.Sprintf("To: %s\r\n", to))
	b.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(body)
	return []byte(b.String())
}

// trackingURL builds the absolute URL embedded in campaign emails for click tracking.
func (c SMTPConfig) trackingURL(token string) string {
	return c.AppURL + "/api/v1/secreflex/track/" + token
}

// from returns the configured From address or a safe default.
func (c SMTPConfig) from() string {
	if c.From != "" {
		return c.From
	}
	return "secreflex@" + c.Host
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
	var orgName string
	_ = s.db.QueryRow(ctx, `SELECT name FROM organizations WHERE id=$1::uuid`, orgID).Scan(&orgName)

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
