// Package admin provides admin panel endpoints for audit logs, user management,
// module status, and MSP multi-tenancy management.
package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
)

const (
	// TaskDeleteOrg is the Asynq task type for deferred org deletion.
	TaskDeleteOrg = "admin:org:delete"

	// orgDeletionGrace is the grace period before a managed org is hard-deleted.
	orgDeletionGrace = 30 * 24 * time.Hour
)

// MSPService implements MSP multi-tenancy business logic.
// It is embedded in Service so that all admin service methods share one receiver.
type MSPService struct {
	repo        *Repository
	asynqClient *asynq.Client // may be nil if Redis is unavailable
}

// newMSPService constructs an MSPService.
// asynqClient may be nil; the service degrades gracefully.
func newMSPService(repo *Repository, asynqClient *asynq.Client) *MSPService {
	return &MSPService{repo: repo, asynqClient: asynqClient}
}

// CreateManagedOrg creates a new child organization under the MSP org.
// mspOrgID is used only for authorization context at the handler/middleware layer;
// the service trusts the caller to have validated that the requestor is an Admin of mspOrgID.
func (s *MSPService) CreateManagedOrg(ctx context.Context, mspOrgID, _ string, input CreateManagedOrgInput) (*Organization, error) {
	org, err := s.repo.CreateOrg(ctx, input.Name, input.Plan, mspOrgID)
	if err != nil {
		return nil, fmt.Errorf("create managed org: %w", err)
	}

	log.Info().
		Str("msp_org_id", mspOrgID).
		Str("new_org_id", org.ID).
		Str("plan", org.Plan).
		Msg("managed org created")

	return org, nil
}

// ListManagedOrgs returns a summary list of all child orgs owned by the MSP org.
func (s *MSPService) ListManagedOrgs(ctx context.Context, mspOrgID string) ([]ManagedOrgSummary, error) {
	orgs, err := s.repo.ListChildOrgs(ctx, mspOrgID)
	if err != nil {
		return nil, fmt.Errorf("list managed orgs: %w", err)
	}
	return orgs, nil
}

// DeleteManagedOrg schedules the target org for deletion after a 30-day grace period.
// If Asynq is available the deletion task is enqueued; otherwise the org is soft-deleted
// immediately by setting scheduled_deletion_at = now (zero grace).
func (s *MSPService) DeleteManagedOrg(ctx context.Context, mspOrgID, targetOrgID string) error {
	// Verify the target org is a child of the requesting MSP org.
	org, err := s.repo.GetOrg(ctx, targetOrgID)
	if err != nil {
		return fmt.Errorf("get target org: %w", err)
	}
	if org.ParentOrgID == nil || *org.ParentOrgID != mspOrgID {
		return fmt.Errorf("org %s is not managed by msp org %s", targetOrgID, mspOrgID)
	}

	deletionAt := time.Now().UTC().Add(orgDeletionGrace)

	if err := s.repo.ScheduleOrgDeletion(ctx, targetOrgID, deletionAt); err != nil {
		return fmt.Errorf("schedule org deletion: %w", err)
	}

	// Enqueue the deferred hard-delete task if Asynq is available.
	if s.asynqClient != nil {
		payload := []byte(fmt.Sprintf(`{"org_id":%q}`, targetOrgID))
		task := asynq.NewTask(TaskDeleteOrg, payload,
			asynq.ProcessAt(deletionAt),
			asynq.TaskID(fmt.Sprintf("delete-org-%s", targetOrgID)),
		)
		if _, err := s.asynqClient.EnqueueContext(ctx, task); err != nil {
			// Non-fatal: the DB record already has the scheduled_deletion_at timestamp;
			// a periodic job can pick it up as a fallback.
			log.Warn().Err(err).Str("org_id", targetOrgID).Msg("enqueue org deletion task failed; will rely on fallback sweep")
		}
	}

	log.Info().
		Str("msp_org_id", mspOrgID).
		Str("target_org_id", targetOrgID).
		Time("deletion_at", deletionAt).
		Msg("managed org deletion scheduled")

	return nil
}

// UpdateOrgBranding stores logo and color branding for a managed org.
func (s *MSPService) UpdateOrgBranding(ctx context.Context, mspOrgID, targetOrgID string, input BrandingInput) error {
	// Verify ownership before mutating.
	org, err := s.repo.GetOrg(ctx, targetOrgID)
	if err != nil {
		return fmt.Errorf("get target org: %w", err)
	}
	if org.ParentOrgID == nil || *org.ParentOrgID != mspOrgID {
		return fmt.Errorf("org %s is not managed by msp org %s", targetOrgID, mspOrgID)
	}

	colors := input.Colors
	if colors == nil {
		colors = map[string]string{}
	}

	if err := s.repo.UpdateOrgBranding(ctx, targetOrgID, input.LogoURL, colors); err != nil {
		return fmt.Errorf("update org branding: %w", err)
	}
	return nil
}

// GetOrgBranding returns the current branding configuration for an org.
// Any org member can call this; the MSP ownership check is omitted here and
// enforced at the route middleware level instead.
func (s *MSPService) GetOrgBranding(ctx context.Context, orgID string) (*BrandingConfig, error) {
	org, err := s.repo.GetOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("get org: %w", err)
	}

	logoURL := ""
	if org.MSPBrandLogo != nil {
		logoURL = *org.MSPBrandLogo
	}

	colors := org.MSPBrandColors
	if colors == nil {
		colors = map[string]string{}
	}

	return &BrandingConfig{
		OrgID:   org.ID,
		LogoURL: logoURL,
		Colors:  colors,
	}, nil
}

// SwitchContext validates that the MSP org has access to the target org, returning
// nil if access is permitted. Handlers use this to confirm cross-org context switches.
func (s *MSPService) SwitchContext(ctx context.Context, mspOrgID, targetOrgID string) error {
	org, err := s.repo.GetOrg(ctx, targetOrgID)
	if err != nil {
		return fmt.Errorf("get target org: %w", err)
	}
	if org.ParentOrgID == nil || *org.ParentOrgID != mspOrgID {
		return fmt.Errorf("msp org %s does not have access to org %s", mspOrgID, targetOrgID)
	}
	return nil
}
