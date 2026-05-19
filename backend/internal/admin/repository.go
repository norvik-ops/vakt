package admin

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles admin data access via pgx.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new admin Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetCurrentOrg fetches summary info for an org by ID, including slug and trust center fields.
func (r *Repository) GetCurrentOrg(ctx context.Context, orgID string) (*CurrentOrg, error) {
	var o CurrentOrg
	var description, contact *string
	err := r.db.QueryRow(ctx, `
		SELECT id::text, name, slug,
		       trust_center_enabled,
		       trust_center_description,
		       trust_center_contact,
		       require_mfa
		FROM organizations
		WHERE id = $1::uuid`, orgID,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.TrustCenterEnabled, &description, &contact, &o.RequireMFA)
	if err != nil {
		return nil, fmt.Errorf("get current org %s: %w", orgID, err)
	}
	if description != nil {
		o.TrustCenterDescription = *description
	}
	if contact != nil {
		o.TrustCenterContact = *contact
	}
	return &o, nil
}

// UpdateOrgTrustCenter updates the trust center settings for an organization.
func (r *Repository) UpdateOrgTrustCenter(ctx context.Context, orgID string, enabled bool, description, contact string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE organizations
		SET trust_center_enabled     = $2,
		    trust_center_description = NULLIF($3, ''),
		    trust_center_contact     = NULLIF($4, ''),
		    updated_at               = NOW()
		WHERE id = $1::uuid`,
		orgID, enabled, description, contact,
	)
	return err
}

// GetOrgSecurity fetches the security policy settings for an organisation.
func (r *Repository) GetOrgSecurity(ctx context.Context, orgID string) (*OrgSecurity, error) {
	var s OrgSecurity
	err := r.db.QueryRow(ctx,
		`SELECT require_mfa FROM organizations WHERE id = $1::uuid`, orgID,
	).Scan(&s.RequireMFA)
	if err != nil {
		return nil, fmt.Errorf("get org security %s: %w", orgID, err)
	}
	return &s, nil
}

// SetOrgRequireMFA updates the require_mfa flag for an organisation.
func (r *Repository) SetOrgRequireMFA(ctx context.Context, orgID string, requireMFA bool) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE organizations SET require_mfa = $2, updated_at = NOW() WHERE id = $1::uuid`,
		orgID, requireMFA,
	)
	if err != nil {
		return fmt.Errorf("set org require_mfa %s: %w", orgID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("org not found: %s", orgID)
	}
	return nil
}
