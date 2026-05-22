package admin

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// generateSAMLCertForRepo creates a self-signed RSA-2048 certificate for a SAML SP.
func generateSAMLCertForRepo(orgID string) (certPEM, keyPEM string, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("saml cert: generate key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", fmt.Errorf("saml cert: serial: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "vakt-saml-sp-" + orgID, Organization: []string{"Vakt"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return "", "", fmt.Errorf("saml cert: create: %w", err)
	}
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	return certPEM, keyPEM, nil
}

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

// OrgAISettings holds the per-org AI model configuration (S32-3).
type OrgAISettings struct {
	ModelOverride   string `json:"model_override"`    // empty = use system default
	BaseURLOverride string `json:"base_url_override"` // empty = use system default (Pro only)
}

// GetOrgAISettings returns the per-org AI model configuration.
func (r *Repository) GetOrgAISettings(ctx context.Context, orgID string) (*OrgAISettings, error) {
	var s OrgAISettings
	var model, baseURL *string
	err := r.db.QueryRow(ctx,
		`SELECT ai_model_override, ai_base_url_override FROM organizations WHERE id = $1::uuid`,
		orgID,
	).Scan(&model, &baseURL)
	if err != nil {
		return nil, fmt.Errorf("get org ai settings %s: %w", orgID, err)
	}
	if model != nil {
		s.ModelOverride = *model
	}
	if baseURL != nil {
		s.BaseURLOverride = *baseURL
	}
	return &s, nil
}

// SetOrgAISettings updates the per-org AI model configuration.
func (r *Repository) SetOrgAISettings(ctx context.Context, orgID, modelOverride, baseURLOverride string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE organizations
		SET ai_model_override    = NULLIF($2, ''),
		    ai_base_url_override = NULLIF($3, ''),
		    updated_at           = NOW()
		WHERE id = $1::uuid`,
		orgID, modelOverride, baseURLOverride,
	)
	if err != nil {
		return fmt.Errorf("set org ai settings %s: %w", orgID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("org not found: %s", orgID)
	}
	return nil
}

// OrgSecurityExtensions holds Pro-tier security settings (S21-5 + S21-6).
type OrgSecurityExtensions struct {
	AdminIPAllowlist          string `json:"admin_ip_allowlist"`            // comma-separated CIDRs; empty = allow all
	RequireMFASensitiveCalls  bool   `json:"require_mfa_sensitive_calls"`
}

// GetOrgSecurityExtensions returns the org's Pro security settings.
func (r *Repository) GetOrgSecurityExtensions(ctx context.Context, orgID string) (*OrgSecurityExtensions, error) {
	var s OrgSecurityExtensions
	var allowlist *string
	err := r.db.QueryRow(ctx, `
		SELECT admin_ip_allowlist, require_mfa_sensitive_calls
		FROM organizations WHERE id = $1::uuid`, orgID,
	).Scan(&allowlist, &s.RequireMFASensitiveCalls)
	if err != nil {
		return nil, fmt.Errorf("get org security ext %s: %w", orgID, err)
	}
	if allowlist != nil {
		s.AdminIPAllowlist = *allowlist
	}
	return &s, nil
}

// SetOrgIPAllowlist updates the org's admin IP allowlist.
func (r *Repository) SetOrgIPAllowlist(ctx context.Context, orgID, allowlist string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE organizations SET admin_ip_allowlist = NULLIF($2,''), updated_at = NOW()
		WHERE id = $1::uuid`, orgID, allowlist)
	return err
}

// SetOrgRequireMFASensitiveCalls updates the require_mfa_sensitive_calls flag.
func (r *Repository) SetOrgRequireMFASensitiveCalls(ctx context.Context, orgID string, require bool) error {
	_, err := r.db.Exec(ctx, `
		UPDATE organizations SET require_mfa_sensitive_calls = $2, updated_at = NOW()
		WHERE id = $1::uuid`, orgID, require)
	return err
}

// ─── S21-4: SCIM Token Management ────────────────────────────────────────────

// SCIMToken is the DB record for a SCIM Bearer token (hash only, no raw value).
type SCIMToken struct {
	ID          string     `json:"id"`
	OrgID       string     `json:"org_id"`
	Name        string     `json:"name"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	CreatedAt   time.Time  `json:"created_at"`
	RevokedAt   *time.Time `json:"revoked_at"`
}

// ListSCIMTokens returns all non-revoked SCIM tokens for an org.
func (r *Repository) ListSCIMTokens(ctx context.Context, orgID string) ([]SCIMToken, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, name, last_used_at, created_at, revoked_at
		FROM scim_tokens
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list scim tokens: %w", err)
	}
	defer rows.Close()

	var tokens []SCIMToken
	for rows.Next() {
		var t SCIMToken
		if err := rows.Scan(&t.ID, &t.OrgID, &t.Name, &t.LastUsedAt, &t.CreatedAt, &t.RevokedAt); err != nil {
			return nil, fmt.Errorf("scan scim token: %w", err)
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// CreateSCIMToken inserts a new SCIM token record and returns the stored row.
// tokenHash is the sha256 hex of the raw Bearer value.
func (r *Repository) CreateSCIMToken(ctx context.Context, orgID, name, tokenHash string) (*SCIMToken, error) {
	var t SCIMToken
	err := r.db.QueryRow(ctx, `
		INSERT INTO scim_tokens (org_id, name, token_hash)
		VALUES ($1::uuid, $2, $3)
		RETURNING id::text, org_id::text, name, last_used_at, created_at, revoked_at`,
		orgID, name, tokenHash,
	).Scan(&t.ID, &t.OrgID, &t.Name, &t.LastUsedAt, &t.CreatedAt, &t.RevokedAt)
	if err != nil {
		return nil, fmt.Errorf("insert scim token: %w", err)
	}
	return &t, nil
}

// RevokeSCIMToken sets revoked_at = NOW() for the given token, scoped to the org.
func (r *Repository) RevokeSCIMToken(ctx context.Context, orgID, tokenID string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE scim_tokens SET revoked_at = NOW()
		WHERE id = $2::uuid AND org_id = $1::uuid AND revoked_at IS NULL`,
		orgID, tokenID)
	if err != nil {
		return fmt.Errorf("revoke scim token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("scim token not found or already revoked")
	}
	return nil
}

// ─── S21-1: SAML Direct SP Config ────────────────────────────────────────────

// OrgSAMLConfigPublic is the DB record for org_saml_configs without the private key.
type OrgSAMLConfigPublic struct {
	OrgID       string
	EntityID    string
	ACSURL      string
	IDPMetadata string
	CertPEM     string
	Enabled     bool
}

// GetOrgSAMLConfigPublic returns the SAML config for an org (no key PEM).
// Returns nil, nil when no row exists.
func (r *Repository) GetOrgSAMLConfigPublic(ctx context.Context, orgID string) (*OrgSAMLConfigPublic, error) {
	var c OrgSAMLConfigPublic
	err := r.db.QueryRow(ctx,
		`SELECT org_id::text, entity_id, acs_url, idp_metadata, cert_pem, enabled
		 FROM org_saml_configs WHERE org_id = $1::uuid`, orgID,
	).Scan(&c.OrgID, &c.EntityID, &c.ACSURL, &c.IDPMetadata, &c.CertPEM, &c.Enabled)
	if err != nil {
		return nil, nil //nolint:nilerr
	}
	return &c, nil
}

// UpsertOrgSAMLConfig writes entity_id, acs_url, idp_metadata, enabled.
// If no cert/key row exists yet, a new self-signed cert is generated and stored.
func (r *Repository) UpsertOrgSAMLConfig(ctx context.Context, orgID, entityID, acsURL, idpMetadata string, enabled bool) error {
	// Check if cert already exists
	var existingCert, existingKey []byte
	_ = r.db.QueryRow(ctx,
		`SELECT cert_pem, key_pem FROM org_saml_configs WHERE org_id = $1::uuid`, orgID,
	).Scan(&existingCert, &existingKey)

	certPEM := string(existingCert)
	keyPEM := string(existingKey)
	if certPEM == "" || keyPEM == "" {
		var err error
		certPEM, keyPEM, err = generateSAMLCertForRepo(orgID)
		if err != nil {
			return fmt.Errorf("upsert saml config: generate cert: %w", err)
		}
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO org_saml_configs (org_id, entity_id, acs_url, idp_metadata, cert_pem, key_pem, enabled, updated_at)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (org_id) DO UPDATE SET
			entity_id    = EXCLUDED.entity_id,
			acs_url      = EXCLUDED.acs_url,
			idp_metadata = EXCLUDED.idp_metadata,
			cert_pem     = EXCLUDED.cert_pem,
			key_pem      = EXCLUDED.key_pem,
			enabled      = EXCLUDED.enabled,
			updated_at   = NOW()`,
		orgID, entityID, acsURL, idpMetadata, certPEM, []byte(keyPEM), enabled,
	)
	return err
}

// RegenerateSAMLCert generates a new self-signed cert and updates the DB.
// Returns the new certPEM (public) for display in the admin UI.
func (r *Repository) RegenerateSAMLCert(ctx context.Context, orgID string) (string, error) {
	certPEM, keyPEM, err := generateSAMLCertForRepo(orgID)
	if err != nil {
		return "", err
	}
	_, err = r.db.Exec(ctx,
		`UPDATE org_saml_configs SET cert_pem = $2, key_pem = $3, updated_at = NOW()
		 WHERE org_id = $1::uuid`,
		orgID, certPEM, []byte(keyPEM),
	)
	return certPEM, err
}
