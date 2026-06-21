package vaktcomply

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

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
)

// ErrDORANotEnabled is returned when DORA framework is not enabled for the organisation.
var ErrDORANotEnabled = errors.New("DORA framework not enabled")

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// Service handles ComplyKit business logic.
type Service struct {
	db         *pgxpool.Pool
	q          *db.Queries
	rdb        *redis.Client
	repo       *Repository
	BCM       *bcm.Service
	BSI       *bsi.Service
	Audit     *audit.Service
	Risk      *risk.Service
	Policy    *policy.Service
	Reporting *reporting.Service
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
		db:    dbPool,
		q:     db.New(dbPool),
		repo:  NewRepository(dbPool),
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
