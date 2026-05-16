package admin

import "time"

// Organization is a full organization record including MSP fields.
type Organization struct {
	ID                  string            `json:"id"`
	Name                string            `json:"name"`
	Slug                string            `json:"slug"`
	Plan                string            `json:"plan"`
	ParentOrgID         *string           `json:"parent_org_id,omitempty"`
	MSPBrandLogo        *string           `json:"msp_brand_logo,omitempty"`
	MSPBrandColors      map[string]string `json:"msp_brand_colors,omitempty"`
	ScheduledDeletionAt *time.Time        `json:"scheduled_deletion_at,omitempty"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
}

// CreateManagedOrgInput is the request body for POST /admin/organizations.
type CreateManagedOrgInput struct {
	Name string `json:"name" validate:"required"`
	Plan string `json:"plan" validate:"required,oneof=msp_managed standard enterprise"`
}

// ManagedOrgSummary is a lightweight org view returned in list responses.
type ManagedOrgSummary struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	Plan                string     `json:"plan"`
	CreatedAt           time.Time  `json:"created_at"`
	ScheduledDeletionAt *time.Time `json:"scheduled_deletion_at,omitempty"`
}

// BrandingInput is the request body for PUT /admin/organizations/:id/branding.
type BrandingInput struct {
	LogoURL string            `json:"logo_url"`
	Colors  map[string]string `json:"colors"`
}

// BrandingConfig is the response body for GET /admin/organizations/:id/branding.
type BrandingConfig struct {
	OrgID   string            `json:"org_id"`
	LogoURL string            `json:"logo_url"`
	Colors  map[string]string `json:"colors"`
}

// CurrentOrg is a lightweight view of the caller's own organisation.
type CurrentOrg struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	Slug                   string `json:"slug"`
	TrustCenterEnabled     bool   `json:"trust_center_enabled"`
	TrustCenterDescription string `json:"trust_center_description"`
	TrustCenterContact     string `json:"trust_center_contact"`
}
