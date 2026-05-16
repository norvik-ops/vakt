package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles MSP-related data access via pgx.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new admin Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateOrg inserts a new organization as a child of parentOrgID and returns the created record.
func (r *Repository) CreateOrg(ctx context.Context, name, plan, parentOrgID string) (*Organization, error) {
	// Derive a slug from the name (lowercase, spaces → hyphens, append random suffix via uuid fragment).
	slug := slugify(name)

	var org Organization
	var colorsRaw []byte

	err := r.db.QueryRow(ctx, `
		INSERT INTO organizations (name, slug, plan, parent_org_id)
		VALUES ($1, $2, $3, $4::uuid)
		RETURNING id::text, name, slug, plan,
		          parent_org_id::text,
		          msp_brand_logo,
		          msp_brand_colors,
		          scheduled_deletion_at,
		          created_at, updated_at`,
		name, slug, plan, parentOrgID,
	).Scan(
		&org.ID, &org.Name, &org.Slug, &org.Plan,
		&org.ParentOrgID,
		&org.MSPBrandLogo,
		&colorsRaw,
		&org.ScheduledDeletionAt,
		&org.CreatedAt, &org.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert managed org: %w", err)
	}

	if len(colorsRaw) > 0 {
		_ = json.Unmarshal(colorsRaw, &org.MSPBrandColors)
	}

	return &org, nil
}

// ListChildOrgs returns summary rows for all child organizations of parentOrgID.
func (r *Repository) ListChildOrgs(ctx context.Context, parentOrgID string) ([]ManagedOrgSummary, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, name, plan, created_at, scheduled_deletion_at
		FROM organizations
		WHERE parent_org_id = $1::uuid
		ORDER BY created_at ASC`, parentOrgID)
	if err != nil {
		return nil, fmt.Errorf("query child orgs: %w", err)
	}
	defer rows.Close()

	var orgs []ManagedOrgSummary
	for rows.Next() {
		var o ManagedOrgSummary
		if err := rows.Scan(&o.ID, &o.Name, &o.Plan, &o.CreatedAt, &o.ScheduledDeletionAt); err != nil {
			return nil, fmt.Errorf("scan child org row: %w", err)
		}
		orgs = append(orgs, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate child org rows: %w", err)
	}
	return orgs, nil
}

// ScheduleOrgDeletion sets the scheduled_deletion_at timestamp on an organization.
func (r *Repository) ScheduleOrgDeletion(ctx context.Context, orgID string, at time.Time) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE organizations SET scheduled_deletion_at = $1, updated_at = NOW()
		WHERE id = $2::uuid`, at, orgID)
	if err != nil {
		return fmt.Errorf("schedule org deletion: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("org not found: %s", orgID)
	}
	return nil
}

// GetOrg fetches a single organization by ID.
func (r *Repository) GetOrg(ctx context.Context, orgID string) (*Organization, error) {
	var org Organization
	var colorsRaw []byte

	err := r.db.QueryRow(ctx, `
		SELECT id::text, name, slug, plan,
		       parent_org_id::text,
		       msp_brand_logo,
		       msp_brand_colors,
		       scheduled_deletion_at,
		       created_at, updated_at
		FROM organizations
		WHERE id = $1::uuid`, orgID,
	).Scan(
		&org.ID, &org.Name, &org.Slug, &org.Plan,
		&org.ParentOrgID,
		&org.MSPBrandLogo,
		&colorsRaw,
		&org.ScheduledDeletionAt,
		&org.CreatedAt, &org.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get org %s: %w", orgID, err)
	}

	if len(colorsRaw) > 0 {
		_ = json.Unmarshal(colorsRaw, &org.MSPBrandColors)
	}

	return &org, nil
}

// UpdateOrgBranding stores logo URL and brand colors on an organization.
func (r *Repository) UpdateOrgBranding(ctx context.Context, orgID, logoURL string, colors map[string]string) error {
	colorsJSON, err := json.Marshal(colors)
	if err != nil {
		return fmt.Errorf("marshal brand colors: %w", err)
	}

	tag, err := r.db.Exec(ctx, `
		UPDATE organizations
		SET msp_brand_logo = $1, msp_brand_colors = $2, updated_at = NOW()
		WHERE id = $3::uuid`, logoURL, colorsJSON, orgID)
	if err != nil {
		return fmt.Errorf("update org branding: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("org not found: %s", orgID)
	}
	return nil
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

// GetCurrentOrg fetches summary info for an org by ID, including slug and trust center fields.
func (r *Repository) GetCurrentOrg(ctx context.Context, orgID string) (*CurrentOrg, error) {
	var o CurrentOrg
	var description, contact *string
	err := r.db.QueryRow(ctx, `
		SELECT id::text, name, slug,
		       trust_center_enabled,
		       trust_center_description,
		       trust_center_contact
		FROM organizations
		WHERE id = $1::uuid`, orgID,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.TrustCenterEnabled, &description, &contact)
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

// slugify converts a name into a lowercase hyphen-separated slug.
// Identical to simple slug helpers used elsewhere in the project.
func slugify(name string) string {
	out := make([]byte, 0, len(name))
	prevHyphen := false
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'A' && c <= 'Z':
			out = append(out, c+('a'-'A'))
			prevHyphen = false
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			out = append(out, c)
			prevHyphen = false
		default:
			if !prevHyphen && len(out) > 0 {
				out = append(out, '-')
				prevHyphen = true
			}
		}
	}
	// Trim trailing hyphen
	for len(out) > 0 && out[len(out)-1] == '-' {
		out = out[:len(out)-1]
	}
	return string(out)
}
