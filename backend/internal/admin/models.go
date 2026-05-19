package admin

// CurrentOrg is a lightweight view of the caller's own organisation.
type CurrentOrg struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	Slug                   string `json:"slug"`
	TrustCenterEnabled     bool   `json:"trust_center_enabled"`
	TrustCenterDescription string `json:"trust_center_description"`
	TrustCenterContact     string `json:"trust_center_contact"`
	RequireMFA             bool   `json:"require_mfa"`
}

// OrgSecurity holds the security policy settings for an organisation.
type OrgSecurity struct {
	RequireMFA bool `json:"require_mfa"`
}
