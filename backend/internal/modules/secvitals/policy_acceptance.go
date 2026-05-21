package secvitals

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"text/template"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
)

// --- Models ---

// PolicyAcceptanceCampaign is a campaign that tracks acceptance of a policy by recipients.
type PolicyAcceptanceCampaign struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	PolicyID  string    `json:"policy_id"`
	Name      string    `json:"name"`
	Message   string    `json:"message,omitempty"`
	Deadline  *string   `json:"deadline,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateCampaignInput holds the request body for creating an acceptance campaign.
type CreateCampaignInput struct {
	PolicyID string           `json:"policy_id"  validate:"required"`
	Name     string           `json:"name"       validate:"required,max=255"`
	Message  string           `json:"message"`
	Deadline *string          `json:"deadline"` // YYYY-MM-DD
	Emails   []RecipientInput `json:"emails"    validate:"required,min=1"`
}

// RecipientInput is a single recipient entry.
type RecipientInput struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name"`
}

// PolicyAcceptanceRequest is a single per-recipient acceptance request.
type PolicyAcceptanceRequest struct {
	ID             string     `json:"id"`
	CampaignID     string     `json:"campaign_id"`
	RecipientEmail string     `json:"recipient_email"`
	RecipientName  string     `json:"recipient_name,omitempty"`
	AcceptedAt     *time.Time `json:"accepted_at,omitempty"`
	SentAt         *time.Time `json:"sent_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// CampaignStats holds aggregated acceptance statistics.
type CampaignStats struct {
	Total    int `json:"total"`
	Accepted int `json:"accepted"`
	Pending  int `json:"pending"`
}

// --- SMTP helpers ---

// policyAcceptanceSMTP holds the SMTP config needed by the acceptance service.
// It is populated from the Service's notifSvc config at call time.
type policyAcceptanceSMTPConfig struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

// sendAcceptanceEmail delivers a single acceptance-request email.
func sendAcceptanceEmail(cfg policyAcceptanceSMTPConfig, to, subject, body string) error {
	from := cfg.From
	if from == "" {
		from = "compliance@vakt.local"
	}

	var msg strings.Builder
	fmt.Fprintf(&msg, "From: Vakt Compliance <%s>\r\n", from)
	fmt.Fprintf(&msg, "To: %s\r\n", to)
	fmt.Fprintf(&msg, "Subject: %s\r\n", subject)
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)
	raw := []byte(msg.String())

	addr := net.JoinHostPort(cfg.Host, cfg.Port)

	if cfg.Port == "587" {
		conn, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("smtp dial: %w", err)
		}
		defer conn.Close()
		if err := conn.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
		if cfg.User != "" {
			if err := conn.Auth(smtp.PlainAuth("", cfg.User, cfg.Pass, cfg.Host)); err != nil {
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
		if _, err := wc.Write(raw); err != nil {
			return fmt.Errorf("smtp write: %w", err)
		}
		return wc.Close()
	}

	if cfg.Port == "465" {
		tlsConn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: cfg.Host})
		if err != nil {
			return fmt.Errorf("smtp tls dial: %w", err)
		}
		client, err := smtp.NewClient(tlsConn, cfg.Host)
		if err != nil {
			return fmt.Errorf("smtp client: %w", err)
		}
		defer client.Close()
		if cfg.User != "" {
			if err := client.Auth(smtp.PlainAuth("", cfg.User, cfg.Pass, cfg.Host)); err != nil {
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
		if _, err := wc.Write(raw); err != nil {
			return err
		}
		return wc.Close()
	}

	// Default — plain SMTP (Mailpit / port 25)
	var auth smtp.Auth
	if cfg.User != "" {
		auth = smtp.PlainAuth("", cfg.User, cfg.Pass, cfg.Host)
	}
	return smtp.SendMail(addr, auth, from, []string{to}, raw)
}

// --- Email template ---

const policyAcceptanceEmailSubjectDE = "Bitte bestätigen Sie: {{.PolicyName}}"

const policyAcceptanceEmailBodyDE = `Sehr geehrte/r {{.Name}},

{{.OrgName}} bittet Sie, die folgende Richtlinie zu bestätigen:

Richtlinie: {{.PolicyName}}
{{if .Message}}
{{.Message}}
{{end}}
Bitte lesen Sie die Richtlinie und bestätigen Sie Ihre Kenntnisnahme:

  {{.AcceptURL}}

Dieser Link ist persönlich und sollte nicht weitergegeben werden.
{{if .DeadlineText}}{{.DeadlineText}}
{{end}}
Mit freundlichen Grüßen,
{{.OrgName}} — Compliance-Team

Diese E-Mail wurde automatisch von Vakt generiert.`

type acceptanceEmailData struct {
	PolicyName   string
	Name         string
	OrgName      string
	Message      string
	AcceptURL    string
	DeadlineText string
}

func renderAcceptanceEmail(data acceptanceEmailData) (subject, body string, err error) {
	subjectTmpl, err := template.New("subject").Parse(policyAcceptanceEmailSubjectDE)
	if err != nil {
		return "", "", fmt.Errorf("parse subject template: %w", err)
	}
	var subjectBuf bytes.Buffer
	if err := subjectTmpl.Execute(&subjectBuf, data); err != nil {
		return "", "", fmt.Errorf("render subject: %w", err)
	}

	bodyTmpl, err := template.New("body").Parse(policyAcceptanceEmailBodyDE)
	if err != nil {
		return "", "", fmt.Errorf("parse body template: %w", err)
	}
	var bodyBuf bytes.Buffer
	if err := bodyTmpl.Execute(&bodyBuf, data); err != nil {
		return "", "", fmt.Errorf("render body: %w", err)
	}

	return subjectBuf.String(), bodyBuf.String(), nil
}

// --- Service methods ---

// CreateAcceptanceCampaign creates a campaign, generates per-recipient tokens, and sends emails.
// smtpCfg and frontendURL are passed in because the Service struct does not hold SMTP config directly.
func (s *Service) CreateAcceptanceCampaign(
	ctx context.Context,
	orgID, userID string,
	in CreateCampaignInput,
	smtpCfg policyAcceptanceSMTPConfig,
	frontendURL string,
) (*PolicyAcceptanceCampaign, error) {
	// Load policy to get title for emails.
	policy, err := s.repo.GetPolicy(ctx, orgID, in.PolicyID)
	if err != nil {
		return nil, fmt.Errorf("policy not found: %w", err)
	}

	// Load org name.
	orgName, err := s.repo.GetOrgName(ctx, orgID)
	if err != nil {
		orgName = "Ihre Organisation"
	}

	// Create campaign record.
	campaign, err := s.repo.CreateAcceptanceCampaign(ctx, orgID, userID, in)
	if err != nil {
		return nil, fmt.Errorf("create campaign: %w", err)
	}

	// For each recipient: generate token, persist request, send email.
	for _, recipient := range in.Emails {
		token, tokenHash, err := generateAcceptanceToken()
		if err != nil {
			log.Error().Err(err).Str("email", recipient.Email).Msg("generate acceptance token failed")
			continue
		}

		requestID, err := s.repo.CreateAcceptanceRequest(ctx, campaign.ID, orgID, recipient, tokenHash)
		if err != nil {
			log.Error().Err(err).Str("email", recipient.Email).Msg("create acceptance request failed")
			continue
		}

		acceptURL := fmt.Sprintf("%s/policy/accept/%s", strings.TrimRight(frontendURL, "/"), token)

		name := recipient.Name
		if name == "" {
			name = recipient.Email
		}

		var deadlineText string
		if in.Deadline != nil && *in.Deadline != "" {
			deadlineText = fmt.Sprintf("Bitte bestätigen Sie bis spätestens: %s", *in.Deadline)
		}

		subject, body, err := renderAcceptanceEmail(acceptanceEmailData{
			PolicyName:   policy.Title,
			Name:         name,
			OrgName:      orgName,
			Message:      in.Message,
			AcceptURL:    acceptURL,
			DeadlineText: deadlineText,
		})
		if err != nil {
			log.Error().Err(err).Str("email", recipient.Email).Msg("render acceptance email failed")
			continue
		}

		if smtpCfg.Host == "" {
			log.Warn().Str("email", recipient.Email).Msg("SMTP not configured — skipping acceptance email")
		} else {
			if err := sendAcceptanceEmail(smtpCfg, recipient.Email, subject, body); err != nil {
				log.Warn().Err(err).Str("email", recipient.Email).Msg("send acceptance email failed")
			}
		}

		if err := s.repo.MarkAcceptanceRequestSent(ctx, requestID); err != nil {
			log.Warn().Err(err).Str("request_id", requestID).Msg("mark acceptance request sent failed")
		}
	}

	return campaign, nil
}

// ListCampaigns returns all acceptance campaigns for a policy within an org.
func (s *Service) ListCampaigns(ctx context.Context, orgID, policyID string) ([]PolicyAcceptanceCampaign, error) {
	return s.repo.ListAcceptanceCampaigns(ctx, orgID, policyID)
}

// GetCampaignStats returns acceptance statistics for a campaign.
func (s *Service) GetCampaignStats(ctx context.Context, campaignID string) (*CampaignStats, error) {
	return s.repo.GetCampaignStats(ctx, campaignID)
}

// ListCampaignRequests returns all individual acceptance requests for a campaign.
func (s *Service) ListCampaignRequests(ctx context.Context, campaignID string) ([]PolicyAcceptanceRequest, error) {
	return s.repo.ListAcceptanceRequests(ctx, campaignID)
}

// AcceptPolicy records an acceptance for the given token and creates evidence in SecVitals.
func (s *Service) AcceptPolicy(ctx context.Context, token, ip string) error {
	tokenHash := hashToken(token)

	req, policyTitle, orgID, err := s.repo.GetRequestByTokenHash(ctx, tokenHash)
	if err != nil {
		return fmt.Errorf("invalid or expired token: %w", err)
	}

	if req.AcceptedAt != nil {
		// Already accepted — idempotent: return OK.
		return nil
	}

	if err := s.repo.RecordAcceptance(ctx, req.ID, ip); err != nil {
		return fmt.Errorf("record acceptance: %w", err)
	}

	// Create ISO 27001 A.5.1 evidence on matching controls.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("goroutine panic recovered")
			}
		}()
		bgCtx, bgCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer bgCancel()
		title := fmt.Sprintf("Richtlinien-Akzeptanz: %s", policyTitle)

		controls, err := s.repo.FindControlsByKeywords(bgCtx, orgID, []string{"policy", "richtlinie", "information security policies", "a.5.1", "a5.1"})
		if err != nil || len(controls) == 0 {
			log.Info().Str("org_id", orgID).Msg("policy acceptance: no matching A.5.1 controls found for evidence")
			return
		}

		collectorData, _ := json.Marshal(map[string]string{
			"source":          "policy_acceptance",
			"request_id":      req.ID,
			"recipient_email": req.RecipientEmail,
			"accepted_ip":     ip,
		})

		for _, ctrl := range controls {
			if _, err := s.repo.AddCollectorEvidence(bgCtx, orgID, ctrl.ID, "", "policy_acceptance", title, collectorData); err != nil {
				log.Warn().Err(err).Str("control_id", ctrl.ID).Msg("policy acceptance: evidence insert failed")
			}
		}

		log.Info().
			Str("org_id", orgID).
			Str("request_id", req.ID).
			Int("controls_updated", len(controls)).
			Msg("policy acceptance evidence recorded")
	}()

	return nil
}

// GetAcceptanceRequestInfo returns public info for the accept-policy page.
func (s *Service) GetAcceptanceRequestInfo(ctx context.Context, token string) (*acceptancePublicInfo, error) {
	tokenHash := hashToken(token)
	return s.repo.GetAcceptancePublicInfo(ctx, tokenHash)
}

type acceptancePublicInfo struct {
	PolicyTitle string     `json:"policy_title"`
	OrgName     string     `json:"org_name"`
	Message     string     `json:"message,omitempty"`
	Deadline    *string    `json:"deadline,omitempty"`
	AcceptedAt  *time.Time `json:"accepted_at,omitempty"`
}

// --- Token helpers ---

// generateAcceptanceToken creates a 32-byte random hex token and its SHA-256 hash.
func generateAcceptanceToken() (plaintext, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	plaintext = hex.EncodeToString(buf)
	hash = hashToken(plaintext) // uses the existing hashToken from service.go
	return plaintext, hash, nil
}

// --- Repository methods ---

// GetOrgName returns the name of an organisation.
func (r *Repository) GetOrgName(ctx context.Context, orgID string) (string, error) {
	var name string
	err := r.db.QueryRow(ctx,
		`SELECT name FROM organizations WHERE id = $1::uuid`, orgID,
	).Scan(&name)
	if err != nil {
		return "", fmt.Errorf("get org name: %w", err)
	}
	return name, nil
}

// CreateAcceptanceCampaign inserts a new acceptance campaign.
func (r *Repository) CreateAcceptanceCampaign(ctx context.Context, orgID, userID string, in CreateCampaignInput) (*PolicyAcceptanceCampaign, error) {
	var c PolicyAcceptanceCampaign
	var deadline *string
	if in.Deadline != nil && *in.Deadline != "" {
		deadline = in.Deadline
	}

	var createdByParam *string
	if userID != "" {
		createdByParam = &userID
	}

	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_policy_acceptance_campaigns (org_id, policy_id, name, message, deadline, created_by)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5::date, $6::uuid)
		RETURNING id::text, org_id::text, policy_id::text, name, COALESCE(message,''),
		          deadline::text, created_at`,
		orgID, in.PolicyID, in.Name, in.Message, deadline, createdByParam,
	).Scan(&c.ID, &c.OrgID, &c.PolicyID, &c.Name, &c.Message, &c.Deadline, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create acceptance campaign: %w", err)
	}
	return &c, nil
}

// ListAcceptanceCampaigns returns all campaigns for a policy.
func (r *Repository) ListAcceptanceCampaigns(ctx context.Context, orgID, policyID string) ([]PolicyAcceptanceCampaign, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, policy_id::text, name, COALESCE(message,''),
		       deadline::text, created_at
		FROM ck_policy_acceptance_campaigns
		WHERE org_id = $1::uuid AND policy_id = $2::uuid
		ORDER BY created_at DESC`,
		orgID, policyID,
	)
	if err != nil {
		return nil, fmt.Errorf("list acceptance campaigns: %w", err)
	}
	defer rows.Close()

	var campaigns []PolicyAcceptanceCampaign
	for rows.Next() {
		var c PolicyAcceptanceCampaign
		if err := rows.Scan(&c.ID, &c.OrgID, &c.PolicyID, &c.Name, &c.Message, &c.Deadline, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan acceptance campaign: %w", err)
		}
		campaigns = append(campaigns, c)
	}
	return campaigns, rows.Err()
}

// CreateAcceptanceRequest inserts a new acceptance request and returns its ID.
func (r *Repository) CreateAcceptanceRequest(ctx context.Context, campaignID, orgID string, recipient RecipientInput, tokenHash string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_policy_acceptance_requests (campaign_id, org_id, recipient_email, recipient_name, token_hash)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5)
		RETURNING id::text`,
		campaignID, orgID, recipient.Email, recipient.Name, tokenHash,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create acceptance request: %w", err)
	}
	return id, nil
}

// MarkAcceptanceRequestSent updates sent_at to now.
func (r *Repository) MarkAcceptanceRequestSent(ctx context.Context, requestID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ck_policy_acceptance_requests SET sent_at = now() WHERE id = $1::uuid`,
		requestID,
	)
	return err
}

// GetCampaignStats returns total / accepted / pending counts for a campaign.
func (r *Repository) GetCampaignStats(ctx context.Context, campaignID string) (*CampaignStats, error) {
	var stats CampaignStats
	err := r.db.QueryRow(ctx, `
		SELECT
			COUNT(*) AS total,
			COUNT(accepted_at) AS accepted,
			COUNT(*) - COUNT(accepted_at) AS pending
		FROM ck_policy_acceptance_requests
		WHERE campaign_id = $1::uuid`,
		campaignID,
	).Scan(&stats.Total, &stats.Accepted, &stats.Pending)
	if err != nil {
		return nil, fmt.Errorf("get campaign stats: %w", err)
	}
	return &stats, nil
}

// ListAcceptanceRequests returns all requests for a campaign.
func (r *Repository) ListAcceptanceRequests(ctx context.Context, campaignID string) ([]PolicyAcceptanceRequest, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, campaign_id::text, recipient_email, COALESCE(recipient_name,''),
		       accepted_at, sent_at, created_at
		FROM ck_policy_acceptance_requests
		WHERE campaign_id = $1::uuid
		ORDER BY created_at ASC`,
		campaignID,
	)
	if err != nil {
		return nil, fmt.Errorf("list acceptance requests: %w", err)
	}
	defer rows.Close()

	var requests []PolicyAcceptanceRequest
	for rows.Next() {
		var req PolicyAcceptanceRequest
		if err := rows.Scan(
			&req.ID, &req.CampaignID, &req.RecipientEmail, &req.RecipientName,
			&req.AcceptedAt, &req.SentAt, &req.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan acceptance request: %w", err)
		}
		requests = append(requests, req)
	}
	return requests, rows.Err()
}

// GetRequestByTokenHash looks up a request by its token hash.
// Returns request, policy title, org_id, and any error.
func (r *Repository) GetRequestByTokenHash(ctx context.Context, tokenHash string) (*PolicyAcceptanceRequest, string, string, error) {
	var req PolicyAcceptanceRequest
	var policyTitle, orgID string
	err := r.db.QueryRow(ctx, `
		SELECT
			par.id::text, par.campaign_id::text, par.recipient_email, COALESCE(par.recipient_name,''),
			par.accepted_at, par.sent_at, par.created_at,
			p.title,
			pac.org_id::text
		FROM ck_policy_acceptance_requests par
		JOIN ck_policy_acceptance_campaigns pac ON pac.id = par.campaign_id
		JOIN ck_policies p ON p.id = pac.policy_id
		WHERE par.token_hash = $1`,
		tokenHash,
	).Scan(
		&req.ID, &req.CampaignID, &req.RecipientEmail, &req.RecipientName,
		&req.AcceptedAt, &req.SentAt, &req.CreatedAt,
		&policyTitle, &orgID,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, "", "", fmt.Errorf("token not found")
		}
		return nil, "", "", fmt.Errorf("get request by token: %w", err)
	}
	return &req, policyTitle, orgID, nil
}

// RecordAcceptance sets accepted_at and accepted_ip for a request.
func (r *Repository) RecordAcceptance(ctx context.Context, requestID, ip string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE ck_policy_acceptance_requests
		SET accepted_at = now(), accepted_ip = $2
		WHERE id = $1::uuid AND accepted_at IS NULL`,
		requestID, ip,
	)
	if err != nil {
		return fmt.Errorf("record acceptance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Already accepted — treat as success.
		return nil
	}
	return nil
}

// GetAcceptancePublicInfo returns policy/org/message info for the public accept page.
func (r *Repository) GetAcceptancePublicInfo(ctx context.Context, tokenHash string) (*acceptancePublicInfo, error) {
	var info acceptancePublicInfo
	err := r.db.QueryRow(ctx, `
		SELECT
			p.title,
			o.name,
			COALESCE(pac.message,''),
			pac.deadline::text,
			par.accepted_at
		FROM ck_policy_acceptance_requests par
		JOIN ck_policy_acceptance_campaigns pac ON pac.id = par.campaign_id
		JOIN ck_policies p ON p.id = pac.policy_id
		JOIN organizations o ON o.id = pac.org_id
		WHERE par.token_hash = $1`,
		tokenHash,
	).Scan(&info.PolicyTitle, &info.OrgName, &info.Message, &info.Deadline, &info.AcceptedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("token not found")
		}
		return nil, fmt.Errorf("get acceptance public info: %w", err)
	}
	return &info, nil
}
