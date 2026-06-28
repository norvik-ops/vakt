package admin

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
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

// OrgAISettings holds the per-org AI model configuration (S32-3, S52-4).
type OrgAISettings struct {
	ModelOverride       string `json:"model_override"`        // empty = use system default
	BaseURLOverride     string `json:"base_url_override"`     // empty = use system default (Pro only)
	WeeklyDigestEnabled bool   `json:"weekly_digest_enabled"` // S52-4: Monday AI digest
}

// GetOrgAISettings returns the per-org AI model configuration.
func (r *Repository) GetOrgAISettings(ctx context.Context, orgID string) (*OrgAISettings, error) {
	var s OrgAISettings
	var model, baseURL *string
	err := r.db.QueryRow(ctx,
		`SELECT ai_model_override, ai_base_url_override, ai_weekly_digest_enabled FROM organizations WHERE id = $1::uuid`,
		orgID,
	).Scan(&model, &baseURL, &s.WeeklyDigestEnabled)
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
func (r *Repository) SetOrgAISettings(ctx context.Context, orgID, modelOverride, baseURLOverride string, weeklyDigest bool) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE organizations
		SET ai_model_override         = NULLIF($2, ''),
		    ai_base_url_override      = NULLIF($3, ''),
		    ai_weekly_digest_enabled  = $4,
		    updated_at                = NOW()
		WHERE id = $1::uuid`,
		orgID, modelOverride, baseURLOverride, weeklyDigest,
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
	AdminIPAllowlist         string `json:"admin_ip_allowlist"` // comma-separated CIDRs; empty = allow all
	RequireMFASensitiveCalls bool   `json:"require_mfa_sensitive_calls"`
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
	ID         string     `json:"id"`
	OrgID      string     `json:"org_id"`
	Name       string     `json:"name"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
}

// ListSCIMTokens returns all non-revoked SCIM tokens for an org.
func (r *Repository) ListSCIMTokens(ctx context.Context, orgID string) ([]SCIMToken, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, name, last_used_at, created_at, revoked_at, expires_at
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
		if err := rows.Scan(&t.ID, &t.OrgID, &t.Name, &t.LastUsedAt, &t.CreatedAt, &t.RevokedAt, &t.ExpiresAt); err != nil {
			return nil, fmt.Errorf("scan scim token: %w", err)
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// CreateSCIMToken inserts a new SCIM token record and returns the stored row.
// tokenHash is the sha256 hex of the raw Bearer value. expiresAt is optional;
// pass nil for tokens that should never expire.
func (r *Repository) CreateSCIMToken(ctx context.Context, orgID, name, tokenHash string, expiresAt *time.Time) (*SCIMToken, error) {
	var t SCIMToken
	err := r.db.QueryRow(ctx, `
		INSERT INTO scim_tokens (org_id, name, token_hash, expires_at)
		VALUES ($1::uuid, $2, $3, $4)
		RETURNING id::text, org_id::text, name, last_used_at, created_at, revoked_at, expires_at`,
		orgID, name, tokenHash, expiresAt,
	).Scan(&t.ID, &t.OrgID, &t.Name, &t.LastUsedAt, &t.CreatedAt, &t.RevokedAt, &t.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("insert scim token: %w", err)
	}
	return &t, nil
}

// RevokeExpiredSCIMTokens sets revoked_at = NOW() for all tokens that have
// passed their expires_at timestamp. Returns the count of tokens revoked.
func (r *Repository) RevokeExpiredSCIMTokens(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE scim_tokens
		SET revoked_at = NOW()
		WHERE expires_at IS NOT NULL
		  AND expires_at < NOW()
		  AND revoked_at IS NULL`)
	if err != nil {
		return 0, fmt.Errorf("revoke expired scim tokens: %w", err)
	}
	return tag.RowsAffected(), nil
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
	OrgID           string
	EntityID        string
	ACSURL          string
	IDPMetadata     string
	CertPEM         string
	Enabled         bool
	JITProvisioning bool
}

// GetOrgSAMLConfigPublic returns the SAML config for an org (no key PEM).
// Returns nil, nil when no row exists.
func (r *Repository) GetOrgSAMLConfigPublic(ctx context.Context, orgID string) (*OrgSAMLConfigPublic, error) {
	var c OrgSAMLConfigPublic
	err := r.db.QueryRow(ctx,
		`SELECT org_id::text, entity_id, acs_url, idp_metadata, cert_pem, enabled, jit_provisioning
		 FROM org_saml_configs WHERE org_id = $1::uuid`, orgID,
	).Scan(&c.OrgID, &c.EntityID, &c.ACSURL, &c.IDPMetadata, &c.CertPEM, &c.Enabled, &c.JITProvisioning)
	if err != nil {
		return nil, nil //nolint:nilerr
	}
	return &c, nil
}

// UpsertOrgSAMLConfig writes entity_id, acs_url, idp_metadata, enabled, jit_provisioning.
// If no cert/key row exists yet, a new self-signed cert is generated and stored.
func (r *Repository) UpsertOrgSAMLConfig(ctx context.Context, orgID, entityID, acsURL, idpMetadata string, enabled, jitProvisioning bool) error {
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
		INSERT INTO org_saml_configs (org_id, entity_id, acs_url, idp_metadata, cert_pem, key_pem, enabled, jit_provisioning, updated_at)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (org_id) DO UPDATE SET
			entity_id        = EXCLUDED.entity_id,
			acs_url          = EXCLUDED.acs_url,
			idp_metadata     = EXCLUDED.idp_metadata,
			cert_pem         = EXCLUDED.cert_pem,
			key_pem          = EXCLUDED.key_pem,
			enabled          = EXCLUDED.enabled,
			jit_provisioning = EXCLUDED.jit_provisioning,
			updated_at       = NOW()`,
		orgID, entityID, acsURL, idpMetadata, certPEM, []byte(keyPEM), enabled, jitProvisioning,
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

// ─── S105-2: OIDC/Casdoor Config ─────────────────────────────────────────────

// OrgOIDCConfig is the public view of org_oidc_configs (secret never returned).
type OrgOIDCConfig struct {
	OrgID       string    `json:"org_id"`
	ProviderURL string    `json:"provider_url"`
	ClientID    string    `json:"client_id"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GetOrgOIDCConfig returns the OIDC config for an org. Returns nil, nil when absent.
func (r *Repository) GetOrgOIDCConfig(ctx context.Context, orgID string) (*OrgOIDCConfig, error) {
	var c OrgOIDCConfig
	err := r.db.QueryRow(ctx,
		`SELECT org_id::text, provider_url, client_id, enabled, created_at, updated_at
		 FROM org_oidc_configs WHERE org_id = $1::uuid`, orgID,
	).Scan(&c.OrgID, &c.ProviderURL, &c.ClientID, &c.Enabled, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, nil //nolint:nilerr
	}
	return &c, nil
}

// UpsertOrgOIDCConfig writes or updates the OIDC configuration for an org.
// clientSecretEnc must already be AES-256-GCM encrypted.
func (r *Repository) UpsertOrgOIDCConfig(ctx context.Context, orgID, providerURL, clientID string, clientSecretEnc []byte, enabled bool) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO org_oidc_configs (org_id, provider_url, client_id, client_secret_enc, enabled, updated_at)
		VALUES ($1::uuid, $2, $3, $4, $5, NOW())
		ON CONFLICT (org_id) DO UPDATE SET
			provider_url        = EXCLUDED.provider_url,
			client_id           = EXCLUDED.client_id,
			client_secret_enc   = EXCLUDED.client_secret_enc,
			enabled             = EXCLUDED.enabled,
			updated_at          = NOW()`,
		orgID, providerURL, clientID, clientSecretEnc, enabled,
	)
	return err
}

// DisableOrgOIDCConfig sets enabled=false for an org's OIDC config.
func (r *Repository) DisableOrgOIDCConfig(ctx context.Context, orgID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE org_oidc_configs SET enabled = FALSE, updated_at = NOW() WHERE org_id = $1::uuid`,
		orgID,
	)
	return err
}

// OIDCEnabledExists returns true when any org has an active OIDC config in the DB.
// Used by the /health endpoint to determine sso_enabled at runtime.
func (r *Repository) OIDCEnabledExists(ctx context.Context) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM org_oidc_configs WHERE enabled = TRUE)`,
	).Scan(&exists)
	return exists, err
}

// OrgSMTPSettings holds the per-org SMTP configuration stored in the DB.
// The password is never returned in plaintext; HasPass signals whether one is set.
type OrgSMTPSettings struct {
	Host    string `json:"host"`
	Port    string `json:"port"`
	User    string `json:"user"`
	From    string `json:"from"`
	TLS     bool   `json:"tls"`
	HasPass bool   `json:"has_pass"` // true if an encrypted password is stored
}

// GetOrgSMTPSettings returns the per-org SMTP configuration.
func (r *Repository) GetOrgSMTPSettings(ctx context.Context, orgID string) (*OrgSMTPSettings, error) {
	var s OrgSMTPSettings
	var host, port, user, from *string
	var passEnc []byte
	err := r.db.QueryRow(ctx,
		`SELECT smtp_host, smtp_port, smtp_user, smtp_pass_enc, smtp_from, smtp_tls
		 FROM organizations WHERE id = $1::uuid`,
		orgID,
	).Scan(&host, &port, &user, &passEnc, &from, &s.TLS)
	if err != nil {
		return nil, fmt.Errorf("get org smtp settings %s: %w", orgID, err)
	}
	if host != nil {
		s.Host = *host
	}
	if port != nil {
		s.Port = *port
	}
	if user != nil {
		s.User = *user
	}
	if from != nil {
		s.From = *from
	}
	s.HasPass = len(passEnc) > 0
	return &s, nil
}

// SetOrgSMTPSettings updates the per-org SMTP configuration.
// passEnc may be nil to leave the existing encrypted password unchanged.
func (r *Repository) SetOrgSMTPSettings(ctx context.Context, orgID, host, port, user, from string, tls bool, passEnc []byte) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE organizations
		SET smtp_host     = NULLIF($2, ''),
		    smtp_port     = NULLIF($3, ''),
		    smtp_user     = NULLIF($4, ''),
		    smtp_from     = NULLIF($5, ''),
		    smtp_tls      = $6,
		    smtp_pass_enc = COALESCE($7, smtp_pass_enc),
		    updated_at    = NOW()
		WHERE id = $1::uuid`,
		orgID, host, port, user, from, tls, passEnc,
	)
	if err != nil {
		return fmt.Errorf("set org smtp settings %s: %w", orgID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("org not found: %s", orgID)
	}
	return nil
}

// ─── Migration 230: Backup Configuration ─────────────────────────────────────

// OrgBackupConfig holds the per-org backup configuration.
// Encrypted fields (passphrase, notify webhook) are never returned in plaintext;
// HasPassphrase and HasNotifyWebhook signal whether values are stored.
type OrgBackupConfig struct {
	Schedule         string `json:"schedule"`       // cron expr, "" = use env default
	RetentionDays    int    `json:"retention_days"` // 0 = use env default
	OffsiteCmd       string `json:"offsite_cmd"`
	NotifyCmd        string `json:"notify_cmd"`
	HasPassphrase    bool   `json:"has_passphrase"`
	HasNotifyWebhook bool   `json:"has_notify_webhook"`
}

// GetOrgBackupConfig returns the per-org backup configuration.
// The raw encrypted bytes for passphrase and notify webhook are returned
// separately so callers can decrypt them when needed (e.g. the internal endpoint).
func (r *Repository) GetOrgBackupConfig(ctx context.Context, orgID string) (*OrgBackupConfig, []byte, []byte, error) {
	var s OrgBackupConfig
	var schedule, offsiteCmd, notifyCmd *string
	var retentionDays *int
	var passphraseEnc, webhookEnc []byte
	err := r.db.QueryRow(ctx, `
		SELECT backup_schedule, backup_retention_days,
		       backup_passphrase_enc, backup_notify_webhook_enc,
		       backup_offsite_cmd, backup_notify_cmd
		FROM organizations WHERE id = $1::uuid`,
		orgID,
	).Scan(&schedule, &retentionDays, &passphraseEnc, &webhookEnc, &offsiteCmd, &notifyCmd)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get org backup config %s: %w", orgID, err)
	}
	if schedule != nil {
		s.Schedule = *schedule
	}
	if retentionDays != nil {
		s.RetentionDays = *retentionDays
	}
	if offsiteCmd != nil {
		s.OffsiteCmd = *offsiteCmd
	}
	if notifyCmd != nil {
		s.NotifyCmd = *notifyCmd
	}
	s.HasPassphrase = len(passphraseEnc) > 0
	s.HasNotifyWebhook = len(webhookEnc) > 0
	return &s, passphraseEnc, webhookEnc, nil
}

// SetOrgBackupConfig updates the per-org backup configuration.
// passphraseEnc and webhookEnc may be nil to leave existing encrypted values unchanged (COALESCE).
// An empty schedule or notifyCmd/offsiteCmd is stored as NULL (NULLIF).
// retentionDays of 0 is stored as NULL (NULLIF), meaning "use env default".
func (r *Repository) SetOrgBackupConfig(ctx context.Context, orgID, schedule string, retentionDays int, passphraseEnc, webhookEnc []byte, offsiteCmd, notifyCmd string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE organizations
		SET backup_schedule           = NULLIF($2, ''),
		    backup_retention_days     = NULLIF($3, 0),
		    backup_passphrase_enc     = COALESCE($4, backup_passphrase_enc),
		    backup_notify_webhook_enc = COALESCE($5, backup_notify_webhook_enc),
		    backup_offsite_cmd        = NULLIF($6, ''),
		    backup_notify_cmd         = NULLIF($7, ''),
		    updated_at                = NOW()
		WHERE id = $1::uuid`,
		orgID, schedule, retentionDays, passphraseEnc, webhookEnc, offsiteCmd, notifyCmd,
	)
	if err != nil {
		return fmt.Errorf("set org backup config %s: %w", orgID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("org not found: %s", orgID)
	}
	return nil
}

// ─── Migration 231: LDAP/AD Configuration ────────────────────────────────────

// OrgLDAPConfig holds the per-org LDAP/AD configuration stored in the DB.
// The bind password is never returned in plaintext; HasBindPass signals whether one is set.
// Enabled is true when the minimum required fields (URL, BindDN, BaseDN) are all non-empty.
type OrgLDAPConfig struct {
	URL         string `json:"url"`
	BindDN      string `json:"bind_dn"`
	BaseDN      string `json:"base_dn"`
	UserFilter  string `json:"user_filter"`
	GroupFilter string `json:"group_filter"`
	TLS         bool   `json:"tls"`
	HasBindPass bool   `json:"has_bind_pass"`
	Enabled     bool   `json:"enabled"`
}

// GetOrgLDAPConfig returns the per-org LDAP configuration.
// The raw encrypted bind password bytes are returned separately so callers can
// decrypt them when needed (e.g. test/sync endpoints).
func (r *Repository) GetOrgLDAPConfig(ctx context.Context, orgID string) (*OrgLDAPConfig, []byte, error) {
	var s OrgLDAPConfig
	var url, bindDN, baseDN, userFilter, groupFilter *string
	var bindPassEnc []byte
	err := r.db.QueryRow(ctx, `
		SELECT ldap_url, ldap_bind_dn, ldap_bind_pass_enc,
		       ldap_base_dn, ldap_user_filter, ldap_group_filter, ldap_tls
		FROM organizations WHERE id = $1::uuid`,
		orgID,
	).Scan(&url, &bindDN, &bindPassEnc, &baseDN, &userFilter, &groupFilter, &s.TLS)
	if err != nil {
		return nil, nil, fmt.Errorf("get org ldap config %s: %w", orgID, err)
	}
	if url != nil {
		s.URL = *url
	}
	if bindDN != nil {
		s.BindDN = *bindDN
	}
	if baseDN != nil {
		s.BaseDN = *baseDN
	}
	if userFilter != nil {
		s.UserFilter = *userFilter
	}
	if groupFilter != nil {
		s.GroupFilter = *groupFilter
	}
	s.HasBindPass = len(bindPassEnc) > 0
	s.Enabled = s.URL != "" && s.BindDN != "" && s.BaseDN != ""
	return &s, bindPassEnc, nil
}

// SetOrgLDAPConfig updates the per-org LDAP configuration.
// bindPassEnc may be nil to leave the existing encrypted password unchanged (COALESCE).
// Empty string fields are stored as NULL (NULLIF).
func (r *Repository) SetOrgLDAPConfig(ctx context.Context, orgID, url, bindDN, baseDN, userFilter, groupFilter string, tls bool, bindPassEnc []byte) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE organizations
		SET ldap_url           = NULLIF($2, ''),
		    ldap_bind_dn       = NULLIF($3, ''),
		    ldap_base_dn       = NULLIF($4, ''),
		    ldap_user_filter   = NULLIF($5, ''),
		    ldap_group_filter  = NULLIF($6, ''),
		    ldap_tls           = $7,
		    ldap_bind_pass_enc = COALESCE($8, ldap_bind_pass_enc),
		    updated_at         = NOW()
		WHERE id = $1::uuid`,
		orgID, url, bindDN, baseDN, userFilter, groupFilter, tls, bindPassEnc,
	)
	if err != nil {
		return fmt.Errorf("set org ldap config %s: %w", orgID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("org not found: %s", orgID)
	}
	return nil
}

// ─── Migration 232: Guided Backup Destination ─────────────────────────────────

// BackupDestConfig is the JSON payload stored encrypted in backup_dest_config_enc.
// Sensitive string fields (Pass, SecretKey) are plaintext inside the encrypted blob.
type BackupDestConfig struct {
	// nextcloud / webdav
	URL        string `json:"url,omitempty"`
	User       string `json:"user,omitempty"`
	Pass       string `json:"pass,omitempty"`
	RemotePath string `json:"remote_path,omitempty"`
	// s3
	Endpoint  string `json:"endpoint,omitempty"`
	Bucket    string `json:"bucket,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	AccessKey string `json:"access_key,omitempty"`
	SecretKey string `json:"secret_key,omitempty"`
	// sftp
	Host string `json:"host,omitempty"`
	Port int    `json:"port,omitempty"`
	// custom (raw command, backward compat with offsite_cmd)
	Cmd string `json:"cmd,omitempty"`
}

// OrgBackupDest is the API-facing struct (no plaintext secrets).
type OrgBackupDest struct {
	Type         string `json:"type"` // "none"|"nextcloud"|"s3"|"sftp"|"custom"
	URL          string `json:"url"`
	User         string `json:"user"`
	RemotePath   string `json:"remote_path"`
	HasPass      bool   `json:"has_pass"`
	Endpoint     string `json:"endpoint"`
	Bucket       string `json:"bucket"`
	Prefix       string `json:"prefix"`
	AccessKey    string `json:"access_key"`
	HasSecretKey bool   `json:"has_secret_key"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Cmd          string `json:"cmd"` // only for custom type
}

// GetOrgBackupDest returns the backup destination config and raw encrypted blob.
func (r *Repository) GetOrgBackupDest(ctx context.Context, orgID string) (*OrgBackupDest, []byte, error) {
	var destType sql.NullString
	var configEnc []byte
	err := r.db.QueryRow(ctx, `
		SELECT backup_dest_type, backup_dest_config_enc
		FROM organizations WHERE id = $1`, orgID).
		Scan(&destType, &configEnc)
	if err != nil {
		return nil, nil, fmt.Errorf("get org backup dest %s: %w", orgID, err)
	}
	out := &OrgBackupDest{Type: destType.String}
	return out, configEnc, nil
}

// SetOrgBackupDest stores the encrypted backup destination config.
func (r *Repository) SetOrgBackupDest(ctx context.Context, orgID, destType string, configEnc []byte) error {
	_, err := r.db.Exec(ctx, `
		UPDATE organizations
		SET backup_dest_type       = NULLIF($2, ''),
		    backup_dest_config_enc = COALESCE($3, backup_dest_config_enc)
		WHERE id = $1`, orgID, destType, configEnc)
	if err != nil {
		return fmt.Errorf("set org backup dest %s: %w", orgID, err)
	}
	return nil
}
