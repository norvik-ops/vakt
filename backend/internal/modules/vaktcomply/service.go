package vaktcomply

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/matharnica/vakt/internal/db"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/audit"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/bcm"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/bsi"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/reporting"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/risk"
	"github.com/matharnica/vakt/internal/services/ai"
	"github.com/matharnica/vakt/internal/shared/dashboard"
	"github.com/matharnica/vakt/internal/shared/notify"
	"github.com/matharnica/vakt/internal/shared/platform/webhooks"
	"github.com/matharnica/vakt/internal/shared/safego"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

var ErrDORANotEnabled = errors.New("DORA framework not enabled")

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// Service handles ComplyKit business logic.
type Service struct {
	db         *pgxpool.Pool
	q          *db.Queries
	rdb        *redis.Client
	repo       *Repository
	BCM        *bcm.Service
	BSI        *bsi.Service
	Audit      *audit.Service
	Risk       *risk.Service
	Policy     *policy.Service
	Reporting  *reporting.Service
	notifSvc   notifyService
	aiClient   *ai.AIClient
	webhookSvc webhookTrigger
}

// notifyService abstracts the notify.Service dependency for testability.
type notifyService interface {
	Notify(ctx context.Context, msg notify.Message) error
}

// webhookTrigger abstracts the webhook delivery dependency for testability.
type webhookTrigger interface {
	TriggerEvent(ctx context.Context, orgID, eventType string, payload any)
}

// NewService creates a new ComplyKit service.
func NewService(dbPool *pgxpool.Pool) *Service {
	svc := &Service{
		db:        dbPool,
		q:         db.New(dbPool),
		repo:      NewRepository(dbPool),
		BCM:       bcm.NewService(dbPool),
		BSI:       bsi.NewService(dbPool),
		Audit:     audit.NewService(dbPool),
		Risk:      risk.NewService(dbPool),
		Reporting: reporting.NewService(dbPool),
	}
	svc.Policy = policy.NewService(dbPool)
	// Inject the dashboard cache invalidator so risk-domain writes invalidate
	// the same cached aggregate the parent service maintains.
	svc.Risk.WithCacheInvalidator(svc.invalidateDashboardCache)
	svc.Policy.WithCacheInvalidator(svc.invalidateDashboardCache)
	svc.Policy.WithWebhookTrigger(svc.triggerWebhook)
	return svc
}

// WithRedis sets the Redis client used for dashboard cache invalidation.
func (s *Service) WithRedis(rdb *redis.Client) {
	s.rdb = rdb
}

// invalidateDashboardCache deletes the cached dashboard aggregate for the given
// org from Redis. It is a no-op when Redis is not configured.
func (s *Service) invalidateDashboardCache(ctx context.Context, orgID string) {
	if err := dashboard.InvalidateDashboardCache(ctx, s.rdb, orgID); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("vaktcomply: dashboard cache invalidation failed")
	}
}

// WithNotifyService sets the notification service used for external email delivery.
func (s *Service) WithNotifyService(n notifyService) {
	s.notifSvc = n
	if s.Policy != nil {
		s.Policy.WithNotifyService(n)
	}
}

// WithAIClient sets the AI client used for policy draft generation.
func (s *Service) WithAIClient(c *ai.AIClient) {
	s.aiClient = c
	if s.Policy != nil {
		s.Policy.WithAIClient(c)
	}
}

// WithWebhooks sets the webhook service used to fire outgoing events.
func (s *Service) WithWebhooks(svc *webhooks.WebhookService) {
	s.webhookSvc = svc
}

// triggerWebhook fires a webhook event in a background goroutine so the caller
// is never blocked by network latency or a slow endpoint.
//
// ADR-0018: läuft über safego.Run, parentCtx ist der Request-/Job-Context des
// Aufrufers. WithoutCancel hängt einen unabhängigen Timeout-Lifecycle dran
// damit ein Client-Disconnect das Webhook nicht abbricht (Fire-and-Forget-
// Semantik beibehalten), shutdown-Signale aber respektiert werden.
func (s *Service) triggerWebhook(parentCtx context.Context, orgID, eventType string, payload map[string]any) {
	if s.webhookSvc == nil {
		return
	}
	safego.Run(parentCtx, "vaktcomply.webhook.trigger", func(parent context.Context) error {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 15*time.Second)
		defer cancel()
		s.webhookSvc.TriggerEvent(ctx, orgID, eventType, payload)
		return nil
	})
}

// Repo exposes the underlying repository for use by ancillary services (e.g. EvidenceFileService).
func (s *Service) Repo() *Repository {
	return s.repo
}

func (s *Service) CreateOrVersionISMSScope(ctx context.Context, orgID, userID string, in CreateISMSScopeInput) (ISMSScope, error) {
	return s.repo.CreateOrVersionISMSScope(ctx, orgID, userID, in)
}

// GetCurrentISMSScope returns the latest ISMS scope version for the org.
func (s *Service) GetCurrentISMSScope(ctx context.Context, orgID string) (ISMSScope, error) {
	return s.repo.GetCurrentISMSScope(ctx, orgID)
}

// ListISMSScopeVersions returns all ISMS scope versions for the org.
func (s *Service) ListISMSScopeVersions(ctx context.Context, orgID string) ([]ISMSScope, error) {
	return s.repo.ListISMSScopeVersions(ctx, orgID)
}

// ApproveISMSScope approves the specified ISMS scope version.
// Only users with the "admin" role may approve.
func (s *Service) ApproveISMSScope(ctx context.Context, orgID, id, approverID, userRole string) (ISMSScope, error) {
	if userRole != "admin" {
		return ISMSScope{}, fmt.Errorf("only admins may approve the ISMS scope")
	}
	return s.repo.ApproveISMSScope(ctx, orgID, id, approverID)
}

func (s *Service) LinkRiskControl(ctx context.Context, orgID, riskID, controlID string) error {
	return s.repo.LinkRiskControl(ctx, orgID, riskID, controlID)
}

func (s *Service) UnlinkRiskControl(ctx context.Context, orgID, riskID, controlID string) error {
	return s.repo.UnlinkRiskControl(ctx, orgID, riskID, controlID)
}

func (s *Service) ListRiskControls(ctx context.Context, orgID, riskID string) ([]Control, error) {
	controls, err := s.repo.ListRiskControls(ctx, orgID, riskID)
	if err != nil {
		return nil, fmt.Errorf("list risk controls: %w", err)
	}
	if controls == nil {
		controls = []Control{}
	}
	return controls, nil
}

func (s *Service) ListAccessReviewCampaigns(ctx context.Context, orgID string) ([]AccessReviewCampaign, error) {
	return s.repo.ListAccessReviewCampaigns(ctx, orgID)
}

func (s *Service) GetAccessReviewCampaign(ctx context.Context, orgID, id string) (*AccessReviewCampaign, error) {
	return s.repo.GetAccessReviewCampaign(ctx, orgID, id)
}

func (s *Service) CreateAccessReviewCampaign(ctx context.Context, orgID string, in CreateAccessReviewCampaignInput) (*AccessReviewCampaign, error) {
	return s.repo.CreateAccessReviewCampaign(ctx, orgID, in)
}

func (s *Service) UpdateAccessReviewCampaign(ctx context.Context, orgID, id string, in UpdateAccessReviewCampaignInput) (*AccessReviewCampaign, error) {
	return s.repo.UpdateAccessReviewCampaign(ctx, orgID, id, in)
}

func (s *Service) DeleteAccessReviewCampaign(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteAccessReviewCampaign(ctx, orgID, id)
}

// --- Access Review Items ---

func (s *Service) ListAccessReviewItems(ctx context.Context, orgID, campaignID string) ([]AccessReviewItem, error) {
	return s.repo.ListAccessReviewItems(ctx, orgID, campaignID)
}

func (s *Service) CreateAccessReviewItem(ctx context.Context, orgID string, in CreateAccessReviewItemInput) (*AccessReviewItem, error) {
	return s.repo.CreateAccessReviewItem(ctx, orgID, in)
}

func (s *Service) UpdateAccessReviewItem(ctx context.Context, orgID, id string, in UpdateAccessReviewItemInput) (*AccessReviewItem, error) {
	return s.repo.UpdateAccessReviewItem(ctx, orgID, id, in)
}

var scanEvidenceKeywords = []string{"vulnerabilit", "schwachstell", "configuration", "konfiguration", "patch", "hardening"}

// RecordScanFindingEvidence attaches a scanner finding as evidence to the matching
// vulnerability/configuration controls. The (org, finding, control) tuple is
// recorded in ck_scan_evidence_map; evidence is only written for tuples that are
// newly inserted, making re-delivery of the same finding a no-op (idempotent).
// findingID is an opaque key — vaktcomply never reads vaktscan tables.
func (s *Service) RecordScanFindingEvidence(ctx context.Context, orgID, findingID, title string) (int, error) {
	if orgID == "" || findingID == "" {
		return 0, fmt.Errorf("scan bridge: org_id and finding_id required")
	}
	controls, err := s.repo.FindControlsByKeywords(ctx, orgID, scanEvidenceKeywords)
	if err != nil {
		return 0, fmt.Errorf("scan bridge: find controls: %w", err)
	}
	written := 0
	for _, ctrl := range controls {
		// Idempotency guard: only the first delivery for this (finding, control)
		// inserts a row; ON CONFLICT makes re-scans a no-op.
		tag, err := s.db.Exec(ctx, `
			INSERT INTO ck_scan_evidence_map (org_id, finding_id, control_id)
			VALUES ($1, $2, $3::uuid)
			ON CONFLICT (org_id, finding_id, control_id) DO NOTHING`,
			orgID, findingID, ctrl.ID)
		if err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Str("control_id", ctrl.ID).Msg("scan bridge: map insert")
			continue
		}
		if tag.RowsAffected() == 0 {
			continue // already mapped — skip (idempotent)
		}
		payload := []byte(fmt.Sprintf(`{"source":"scan_bridge","finding_id":%q}`, findingID))
		if _, evErr := s.repo.AddCollectorEvidence(ctx, orgID, ctrl.ID, "", "automated",
			"Scan-Finding: "+title, payload); evErr != nil {
			log.Warn().Err(evErr).Str("control_id", ctrl.ID).Msg("scan bridge: add evidence")
			continue
		}
		written++
	}
	return written, nil
}

func (s *Service) CreatePentest(ctx context.Context, orgID, userID string, in CreatePentestInput) (Pentest, error) {
	return s.repo.CreatePentest(ctx, orgID, userID, in)
}

// GetPentest returns a single pentest by ID for the organisation.
func (s *Service) GetPentest(ctx context.Context, orgID, id string) (Pentest, error) {
	return s.repo.GetPentest(ctx, orgID, id)
}

// ListPentests returns all pentest records for the organisation.
// If testerType is non-nil and non-empty, only records matching that type are returned.
func (s *Service) ListPentests(ctx context.Context, orgID string, testerType *string) ([]Pentest, error) {
	return s.repo.ListPentests(ctx, orgID, testerType)
}

// UpdatePentest updates an existing pentest record.
func (s *Service) UpdatePentest(ctx context.Context, orgID, id string, in UpdatePentestInput) (Pentest, error) {
	return s.repo.UpdatePentest(ctx, orgID, id, in)
}

// DeletePentest removes a pentest record.
func (s *Service) DeletePentest(ctx context.Context, orgID, id string) error {
	return s.repo.DeletePentest(ctx, orgID, id)
}

// GetLastPentest returns the most recent pentest for the organisation, or nil if none exist.
func (s *Service) GetLastPentest(ctx context.Context, orgID string) (*Pentest, error) {
	return s.repo.GetLastPentest(ctx, orgID)
}

func (s *Service) LinkResilienceTestAsEvidence(ctx context.Context, orgID, testID, controlID, userID string) (*Evidence, error) {
	test, err := s.repo.GetResilienceTest(ctx, orgID, testID)
	if err != nil {
		return nil, fmt.Errorf("get resilience test: %w", err)
	}

	// Build a human-readable summary for the evidence title/description.
	title := fmt.Sprintf("DORA Resilienztest: %s (%s)", test.Type, test.TestDate.Format("02.01.2006"))
	description := fmt.Sprintf("Resilienztest vom %s. Typ: %s. Abhilfestatus: %s.",
		test.TestDate.Format("02.01.2006"), test.Type, test.RemediationStatus)
	if test.Provider != "" {
		description += fmt.Sprintf(" Durchgeführt von: %s.", test.Provider)
	}
	if test.Summary != "" {
		description += " Zusammenfassung: " + test.Summary
	}

	// Set expiry to +1 year from test date (DORA annual retesting requirement).
	expiry := test.TestDate.Add(365 * 24 * time.Hour)

	ev, err := s.AddEvidence(ctx, orgID, controlID, userID, AddEvidenceInput{
		Title:       title,
		Description: description,
		Source:      "manual",
		ExpiresAt:   &expiry,
	})
	if err != nil {
		return nil, fmt.Errorf("add evidence from resilience test: %w", err)
	}
	return ev, nil
}
