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
// library. Each template is in German, uses realistic Absender/Betreff patterns
// observed in BSI / CERT-Bund phishing reports, and rotates the social-engineering
// angle (Authority, Urgency, Reward, Fear, Curiosity).
//
// Templates use mustache-style placeholders that the campaign sender resolves:
//
//	{{first_name}}   — Vorname des Empfängers
//	{{last_name}}    — Nachname
//	{{company}}      — Unternehmensname (aus Org-Settings)
//	{{tracking_url}} — Tracking-Link mit eindeutigem Token
//	{{open_pixel}}   — 1×1 transparentes Pixel zur Open-Erkennung
func presetTemplates() []Template {
	return []Template{
		{
			ID:         "preset-ceo-fraud-de",
			Name:       "CEO Fraud (Deutsch)",
			Subject:    "Vertraulich – kurze Rückmeldung erforderlich",
			FromName:   "{{company}} Geschäftsführung",
			FromEmail:  "geschaeftsfuehrung@{{company}}.de",
			HTMLBody:   `<p>Hallo {{first_name}},</p><p>ich bin gerade in einem Meeting und brauche bitte kurz Ihre Hilfe. Können Sie mir die Bankverbindung für die ausstehende Überweisung kurz bestätigen? Bitte <a href="{{tracking_url}}">hier klicken</a> und die Daten gegenchecken — es ist eilig.</p><p>Danke und beste Grüße</p>{{open_pixel}}`,
			AttackType: "phishing",
			IsPreset:   true,
		},
		{
			ID:         "preset-it-passwort-de",
			Name:       "IT-Helpdesk Passwort-Reset",
			Subject:    "Ihr Passwort läuft heute ab",
			FromName:   "IT-Helpdesk {{company}}",
			FromEmail:  "helpdesk@{{company}}-it.de",
			HTMLBody:   `<p>Sehr geehrte/r {{first_name}} {{last_name}},</p><p>Ihr Microsoft-365-Passwort läuft <b>heute um 17:00 Uhr</b> ab. Um eine Sperrung Ihres Kontos zu vermeiden, bitte <a href="{{tracking_url}}">jetzt neues Passwort setzen</a>.</p><p>Bei Fragen wenden Sie sich an Ihren IT-Helpdesk.</p>{{open_pixel}}`,
			AttackType: "phishing",
			IsPreset:   true,
		},
		{
			ID:         "preset-dhl-paket-de",
			Name:       "DHL Paketzustellung",
			Subject:    "Ihr Paket konnte nicht zugestellt werden",
			FromName:   "DHL Paket",
			FromEmail:  "noreply@dhl-paket-tracking.de",
			HTMLBody:   `<p>Hallo {{first_name}},</p><p>Ihr Paket mit der Sendungsnummer DHL-2026-{{first_name}}-08471 konnte heute nicht zugestellt werden. Bitte <a href="{{tracking_url}}">hier den neuen Zustelltermin auswählen</a>.</p><p>Mit freundlichen Grüßen<br/>Ihr DHL-Team</p>{{open_pixel}}`,
			AttackType: "phishing",
			IsPreset:   true,
		},
		{
			ID:         "preset-microsoft-mfa-de",
			Name:       "Microsoft 365 MFA-Warnung",
			Subject:    "Ungewöhnliche Anmeldung in Ihrem Microsoft-Konto",
			FromName:   "Microsoft Sicherheit",
			FromEmail:  "account-security@microsoft-365-de.com",
			HTMLBody:   `<p>Hallo {{first_name}},</p><p>wir haben eine ungewöhnliche Anmeldung in Ihrem Microsoft-365-Konto bemerkt:</p><p><b>Standort:</b> Moskau, Russland<br/><b>IP:</b> 185.220.101.47<br/><b>Zeit:</b> vor 12 Minuten</p><p>Falls das nicht Sie waren: <a href="{{tracking_url}}">Konto jetzt sperren</a></p>{{open_pixel}}`,
			AttackType: "phishing",
			IsPreset:   true,
		},
		{
			ID:         "preset-rechnung-de",
			Name:       "Offene Rechnung Mahnung",
			Subject:    "Letzte Mahnung – Rechnung {{first_name}}-2026-3471",
			FromName:   "Buchhaltung",
			FromEmail:  "mahnung@inkasso-services-de.com",
			HTMLBody:   `<p>Sehr geehrte/r Herr/Frau {{last_name}},</p><p>trotz mehrfacher Mahnung ist die Rechnung Nr. {{first_name}}-2026-3471 über <b>847,32 €</b> bis heute nicht beglichen worden. Sie finden die Rechnung als PDF: <a href="{{tracking_url}}">Rechnung_3471.pdf öffnen</a></p><p>Bei Nichtzahlung leiten wir den Vorgang an unser Inkasso weiter.</p>{{open_pixel}}`,
			AttackType: "phishing",
			IsPreset:   true,
		},
		{
			ID:         "preset-personalabteilung-de",
			Name:       "Personalabteilung Gehaltsabrechnung",
			Subject:    "Ihre Gehaltsabrechnung Dezember 2025 (überarbeitet)",
			FromName:   "Personalabteilung {{company}}",
			FromEmail:  "personal@{{company}}-hr.com",
			HTMLBody:   `<p>Hallo {{first_name}},</p><p>aufgrund einer Korrektur der Sondervergütung haben wir Ihre Gehaltsabrechnung für Dezember 2025 angepasst. Bitte <a href="{{tracking_url}}">die neue Version hier ansehen</a> und bestätigen.</p><p>Viele Grüße<br/>Personalabteilung</p>{{open_pixel}}`,
			AttackType: "phishing",
			IsPreset:   true,
		},
		{
			ID:         "preset-shared-drive-de",
			Name:       "Shared Drive Einladung",
			Subject:    "{{first_name}}, ein Dokument wurde mit Ihnen geteilt",
			FromName:   "OneDrive",
			FromEmail:  "no-reply@onedrive-share.de",
			HTMLBody:   `<p>Hallo {{first_name}},</p><p>ein Kollege hat das Dokument <b>"Strategie_2026_VERTRAULICH.xlsx"</b> mit Ihnen geteilt.</p><p><a href="{{tracking_url}}">Dokument öffnen</a></p><p>Dieser Link läuft in 48 Stunden ab.</p>{{open_pixel}}`,
			AttackType: "phishing",
			IsPreset:   true,
		},
		{
			ID:         "preset-betriebsrat-umfrage-de",
			Name:       "Betriebsrats-Umfrage (Mitarbeiterzufriedenheit)",
			Subject:    "Anonyme Umfrage: Ihre Meinung zählt",
			FromName:   "Betriebsrat {{company}}",
			FromEmail:  "betriebsrat@{{company}}-survey.de",
			HTMLBody:   `<p>Liebe/r {{first_name}},</p><p>wir möchten Ihre Meinung zur Arbeitssituation hören. Die Teilnahme dauert nur 3 Minuten und ist <b>vollständig anonym</b>.</p><p><a href="{{tracking_url}}">Zur Umfrage</a></p><p>Vielen Dank für Ihre Mithilfe<br/>Ihr Betriebsrat</p>{{open_pixel}}`,
			AttackType: "phishing",
			IsPreset:   true,
		},
		{
			ID:         "preset-smishing-bank-de",
			Name:       "SMS Sparkasse TAN",
			Subject:    "[SMS-Vorlage] Sparkasse Sicherheits-TAN",
			FromName:   "Sparkasse",
			FromEmail:  "sms@sparkasse-tan.de",
			HTMLBody:   `<p>Sparkasse: Ungewöhnliche Aktivität auf Ihrem Konto. Bitte verifizieren: {{tracking_url}}</p>{{open_pixel}}`,
			AttackType: "smishing",
			IsPreset:   true,
		},
		{
			ID:         "preset-usb-fund-de",
			Name:       "USB-Stick auf Parkplatz (Köder)",
			Subject:    "[USB-Köder-Szenario] Bonus-Liste 2026",
			FromName:   "(USB-Stick)",
			FromEmail:  "(physical)",
			HTMLBody:   `<p>Dies ist ein USB-Drop-Szenario. Auf dem präparierten USB-Stick liegt eine Datei <code>Bonus_Liste_2026.pdf.lnk</code>, die beim Öffnen <a href="{{tracking_url}}">eine Awareness-Seite öffnet</a>.</p>{{open_pixel}}`,
			AttackType: "usb",
			IsPreset:   true,
		},
	}
}

// presetTrainingModules returns the bundled awareness-training curriculum.
// Modules cover the four attack types and serve as starting templates that an
// admin can clone and customize. content_url points to in-app Markdown lessons
// served via the secreflex/training-content asset bundle.
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
		log.Error().Err(autoErr).Str("campaign_id", campaignID).Msg("evidence_auto: secreflex collection failed")
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
		if err := conn.StartTLS(&tls.Config{ServerName: s.smtpCfg.Host}); err != nil {
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
		tlsConn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: s.smtpCfg.Host})
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
	fmt.Fprintf(&b, "From: %s <%s>\r\n", fromName, fromEmail)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
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
