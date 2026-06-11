package siem

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// OrgSIEMConfig holds the per-org SIEM integration settings.
type OrgSIEMConfig struct {
	Enabled  bool   `json:"enabled"`
	Adapter  string `json:"adapter"` // splunk_hec | elastic | webhook
	Endpoint string `json:"endpoint"`
	Token    string `json:"token"` // write-only: masked on GET
}

// Service provides SIEM forwarding for Vakt audit logs.
type Service struct {
	db *pgxpool.Pool
}

// NewService constructs a SIEM Service.
func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// GetOrgConfig returns the org's SIEM config from org_siem_config.
// If no row exists the default (disabled) config is returned.
func (s *Service) GetOrgConfig(ctx context.Context, orgID string) (*OrgSIEMConfig, error) {
	cfg := &OrgSIEMConfig{Adapter: "webhook"}
	row := s.db.QueryRow(ctx,
		`SELECT enabled, adapter, endpoint, token
		   FROM org_siem_config WHERE org_id = $1::uuid`,
		orgID,
	)
	if err := row.Scan(&cfg.Enabled, &cfg.Adapter, &cfg.Endpoint, &cfg.Token); err != nil {
		// No row → return defaults
		return cfg, nil
	}
	return cfg, nil
}

// validateSIEMEndpoint rejects SIEM endpoint URLs that resolve to loopback,
// private, link-local, or the cloud metadata service (169.254.169.254).
func validateSIEMEndpoint(rawURL string) error {
	if rawURL == "" {
		return nil // empty = disabled, no validation needed
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid SIEM endpoint URL: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("SIEM endpoint URL scheme must be http or https")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("SIEM endpoint URL is missing a host")
	}
	addrs, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("cannot resolve SIEM endpoint host %q: %w", host, err)
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("SIEM endpoint URL resolves to a private/internal address — not allowed")
		}
		if ip.Equal(net.ParseIP("169.254.169.254")) {
			return fmt.Errorf("SIEM endpoint URL resolves to cloud metadata service — not allowed")
		}
	}
	return nil
}

// SetOrgConfig upserts the org's SIEM config.
func (s *Service) SetOrgConfig(ctx context.Context, orgID string, cfg OrgSIEMConfig) error {
	if err := validateSIEMEndpoint(cfg.Endpoint); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO org_siem_config (org_id, enabled, adapter, endpoint, token, updated_at)
		      VALUES ($1::uuid, $2, $3, $4, $5, NOW())
		 ON CONFLICT (org_id) DO UPDATE
		   SET enabled    = EXCLUDED.enabled,
		       adapter    = EXCLUDED.adapter,
		       endpoint   = EXCLUDED.endpoint,
		       token      = CASE WHEN $5 = '' THEN org_siem_config.token ELSE EXCLUDED.token END,
		       updated_at = NOW()`,
		orgID, cfg.Enabled, cfg.Adapter, cfg.Endpoint, cfg.Token,
	)
	if err != nil {
		return fmt.Errorf("upsert org_siem_config: %w", err)
	}
	return nil
}

// ForwardPending fetches up to 100 unforwarded audit entries for each org
// that has SIEM enabled, forwards them via the configured adapter, and marks
// forwarded_to_siem = NOW() for the successfully sent rows.
func (s *Service) ForwardPending(ctx context.Context) error {
	// Find all orgs with SIEM enabled.
	rows, err := s.db.Query(ctx,
		`SELECT org_id::text, adapter, endpoint, token FROM org_siem_config WHERE enabled = true`,
	)
	if err != nil {
		return fmt.Errorf("query enabled siem configs: %w", err)
	}
	defer rows.Close()

	type orgConfig struct {
		orgID    string
		adapter  string
		endpoint string
		token    string
	}
	var configs []orgConfig
	for rows.Next() {
		var oc orgConfig
		if err := rows.Scan(&oc.orgID, &oc.adapter, &oc.endpoint, &oc.token); err != nil {
			return fmt.Errorf("scan siem config row: %w", err)
		}
		configs = append(configs, oc)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate siem config rows: %w", err)
	}

	for _, oc := range configs {
		if err := s.forwardOrgPending(ctx, oc.orgID, OrgSIEMConfig{
			Adapter:  oc.adapter,
			Endpoint: oc.endpoint,
			Token:    oc.token,
		}); err != nil {
			log.Error().Err(err).Str("org_id", oc.orgID).Msg("siem forward pending failed for org")
			// continue with other orgs
		}
	}
	return nil
}

func (s *Service) forwardOrgPending(ctx context.Context, orgID string, cfg OrgSIEMConfig) error {
	adapter, err := newAdapter(cfg)
	if err != nil {
		return fmt.Errorf("build adapter: %w", err)
	}

	// Fetch up to 100 unforwarded entries ordered oldest-first.
	rows, err := s.db.Query(ctx,
		`SELECT
			id::text,
			org_id::text,
			action,
			resource_type,
			COALESCE(resource_id, ''),
			COALESCE(user_id::text, ''),
			COALESCE(ip_address, ''),
			COALESCE(details, 'null'::jsonb),
			created_at
		 FROM audit_log
		 WHERE org_id = $1::uuid
		   AND forwarded_to_siem IS NULL
		   AND deleted_at IS NULL
		 ORDER BY created_at ASC
		 LIMIT 100`,
		orgID,
	)
	if err != nil {
		return fmt.Errorf("query pending audit entries: %w", err)
	}
	defer rows.Close()

	var entries []AuditEntry
	var ids []string
	for rows.Next() {
		var e AuditEntry
		var detailsRaw []byte
		if err := rows.Scan(
			&e.ID, &e.OrgID, &e.Action, &e.ResourceType,
			&e.ResourceID, &e.UserEmail, &e.IPAddress,
			&detailsRaw, &e.CreatedAt,
		); err != nil {
			return fmt.Errorf("scan audit entry: %w", err)
		}
		e.Details = json.RawMessage(detailsRaw)
		entries = append(entries, e)
		ids = append(ids, e.ID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate audit entries: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}

	if err := adapter.Forward(ctx, entries); err != nil {
		return fmt.Errorf("adapter forward (%s): %w", adapter.Name(), err)
	}

	// Mark forwarded.
	now := time.Now().UTC()
	for _, id := range ids {
		if _, err := s.db.Exec(ctx,
			`UPDATE audit_log SET forwarded_to_siem = $1 WHERE id = $2::uuid`,
			now, id,
		); err != nil {
			log.Error().Err(err).Str("entry_id", id).Msg("failed to mark audit entry as forwarded")
		}
	}

	log.Info().
		Str("org_id", orgID).
		Str("adapter", adapter.Name()).
		Int("count", len(entries)).
		Msg("siem audit entries forwarded")
	return nil
}

// TestForward sends a single synthetic test event to verify the org's SIEM config.
func (s *Service) TestForward(ctx context.Context, orgID string) error {
	cfg, err := s.GetOrgConfig(ctx, orgID)
	if err != nil {
		return fmt.Errorf("get siem config: %w", err)
	}
	if cfg.Endpoint == "" {
		return fmt.Errorf("siem endpoint is not configured")
	}

	adapter, err := newAdapter(*cfg)
	if err != nil {
		return fmt.Errorf("build adapter: %w", err)
	}

	testEntry := AuditEntry{
		ID:           "00000000-0000-0000-0000-000000000000",
		OrgID:        orgID,
		Action:       "siem_test",
		ResourceType: "system",
		ResourceID:   "test",
		UserEmail:    "system@vakt.local",
		IPAddress:    "127.0.0.1",
		Details:      json.RawMessage(`{"message":"SIEM connectivity test from Vakt"}`),
		CreatedAt:    time.Now().UTC(),
	}

	return adapter.Forward(ctx, []AuditEntry{testEntry})
}
