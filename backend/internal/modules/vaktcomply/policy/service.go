// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/matharnica/vakt/internal/db"
	"github.com/matharnica/vakt/internal/shared/notify"
)

// Service handles the policy domain of vaktcomply: frameworks, controls,
// policies, Statement of Applicability, framework mappings, physical-control
// templates and policy-acceptance campaigns. ADR-0066 sub-package strategy.
type Service struct {
	db       *pgxpool.Pool
	q        *db.Queries
	repo     *Repository
	aiClient aiClientI
	notifSvc notifyService

	// invalidateCache is injected by the parent service so policy-domain writes
	// invalidate the same cached dashboard aggregate.
	invalidateCache func(context.Context, string)

	// triggerWebhookFn is injected by the parent service so policy-domain writes
	// can fire outgoing webhook events without importing the webhook package.
	triggerWebhookFn func(ctx context.Context, orgID, eventType string, payload map[string]any)
}

// aiClientI abstracts the AI client used for policy-draft generation.
type aiClientI interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// notifyService abstracts the notify.Service dependency for testability.
type notifyService interface {
	Notify(ctx context.Context, msg notify.Message) error
}

// NewService creates a new policy-domain service.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{db: pool, q: db.New(pool), repo: NewRepository(pool)}
}

// WithAIClient sets the AI client used for policy draft generation.
func (s *Service) WithAIClient(c aiClientI) { s.aiClient = c }

// WithNotifyService sets the notification service used for external email delivery.
func (s *Service) WithNotifyService(n notifyService) { s.notifSvc = n }

// WithCacheInvalidator injects the dashboard cache-invalidation function from the parent service.
func (s *Service) WithCacheInvalidator(fn func(context.Context, string)) { s.invalidateCache = fn }

// WithWebhookTrigger injects the webhook-trigger function from the parent service.
func (s *Service) WithWebhookTrigger(fn func(ctx context.Context, orgID, eventType string, payload map[string]any)) {
	s.triggerWebhookFn = fn
}

// invalidateDashboardCache invokes the injected cache invalidator (no-op when unset).
func (s *Service) invalidateDashboardCache(ctx context.Context, orgID string) {
	if s.invalidateCache != nil {
		s.invalidateCache(ctx, orgID)
	}
}

// triggerWebhook invokes the injected webhook trigger (no-op when unset).
func (s *Service) triggerWebhook(ctx context.Context, orgID, eventType string, payload map[string]any) {
	if s.triggerWebhookFn != nil {
		s.triggerWebhookFn(ctx, orgID, eventType, payload)
	}
}
