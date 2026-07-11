// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Certificate represents a tracked TLS certificate.
type Certificate struct {
	ID            string     `json:"id"`
	OrgID         string     `json:"org_id"`
	Domain        string     `json:"domain"`
	Issuer        string     `json:"issuer"`
	Subject       string     `json:"subject"`
	SANs          []string   `json:"sans"`
	NotBefore     *time.Time `json:"not_before,omitempty"`
	NotAfter      *time.Time `json:"not_after,omitempty"`
	AssetID       *string    `json:"asset_id,omitempty"`
	Source        string     `json:"source"` // manual | scan
	Status        string     `json:"status"` // valid | expiring | expired | error | unknown
	LastCheckedAt *time.Time `json:"last_checked_at,omitempty"`
	ErrorMsg      *string    `json:"error_msg,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// CreateCertificateInput is the request body for adding a certificate to track.
type CreateCertificateInput struct {
	Domain  string  `json:"domain"   validate:"required,min=1,max=255"`
	AssetID *string `json:"asset_id" validate:"omitempty,uuid"`
	Source  string  `json:"source"   validate:"omitempty,oneof=manual scan"`
}

// CertInfo holds raw metadata returned by ScanTLSCertificate.
type CertInfo struct {
	Issuer    string
	Subject   string
	SANs      []string
	NotBefore time.Time
	NotAfter  time.Time
}

// ScanTLSCertificate dials domain:443, reads the leaf certificate metadata and returns it.
// It does NOT verify the chain — we only need metadata (expiry, issuer, SANs).
func ScanTLSCertificate(domain string) (*CertInfo, error) {
	host := domain
	if !strings.Contains(host, ":") {
		host = net.JoinHostPort(host, "443")
	}
	dialer := &tls.Dialer{
		Config: &tls.Config{ // nosemgrep: bypass-tls-verification -- InsecureSkipVerify intentional: scanner inspects cert metadata without chain validation
			InsecureSkipVerify: true,             // #nosec G402 -- scanner must connect without chain validation to inspect cert metadata
			MinVersion:         tls.VersionTLS12, // nosemgrep: missing-ssl-minversion
			ServerName:         strings.Split(domain, ":")[0],
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", host, err)
	}
	defer conn.Close()

	tlsConn := conn.(*tls.Conn)
	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, fmt.Errorf("no certificates presented by %s", host)
	}

	leaf := certs[0]
	info := &CertInfo{
		Issuer:    leaf.Issuer.CommonName,
		Subject:   leaf.Subject.CommonName,
		SANs:      leaf.DNSNames,
		NotBefore: leaf.NotBefore,
		NotAfter:  leaf.NotAfter,
	}
	return info, nil
}

// certStatus computes the certificate status from its expiry time.
func certStatus(notAfter *time.Time) string {
	if notAfter == nil {
		return "unknown"
	}
	now := time.Now().UTC()
	if now.After(*notAfter) {
		return "expired"
	}
	if notAfter.Sub(now) < 30*24*time.Hour {
		return "expiring"
	}
	return "valid"
}

// ── Repository methods ───────────────────────────────────────────────────────

// CreateCertificate inserts a new certificate tracking record.
func (r *Repository) CreateCertificate(ctx context.Context, orgID string, in CreateCertificateInput) (*Certificate, error) {
	source := in.Source
	if source == "" {
		source = "manual"
	}
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO vb_certificates (org_id, domain, source, status)
		VALUES ($1::uuid, $2, $3, 'unknown')
		ON CONFLICT (org_id, domain) DO UPDATE SET updated_at = NOW()
		RETURNING id::text
	`, orgID, in.Domain, source).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}
	return r.GetCertificate(ctx, orgID, id)
}

// GetCertificate returns a single certificate record.
func (r *Repository) GetCertificate(ctx context.Context, orgID, id string) (*Certificate, error) {
	return scanCertRow(r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, domain, issuer, subject, sans,
		       not_before, not_after, asset_id::text, source, status,
		       last_checked_at, error_msg, created_at, updated_at
		FROM vb_certificates
		WHERE id = $1::uuid AND org_id = $2::uuid
	`, id, orgID))
}

// ListCertificates returns all certificate records for an org.
func (r *Repository) ListCertificates(ctx context.Context, orgID string) ([]Certificate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, domain, issuer, subject, sans,
		       not_before, not_after, asset_id::text, source, status,
		       last_checked_at, error_msg, created_at, updated_at
		FROM vb_certificates
		WHERE org_id = $1::uuid
		ORDER BY not_after ASC NULLS LAST, domain ASC
	`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list certificates: %w", err)
	}
	defer rows.Close()

	var out []Certificate
	for rows.Next() {
		cert, scanErr := scanCertRow(rows)
		if scanErr != nil {
			continue
		}
		out = append(out, *cert)
	}
	return out, rows.Err()
}

// UpdateCertificateScan updates a certificate with fresh scan results.
func (r *Repository) UpdateCertificateScan(ctx context.Context, orgID, id string, info *CertInfo, scanErr error) error {
	if scanErr != nil {
		errMsg := scanErr.Error()
		_, err := r.db.Exec(ctx, `
			UPDATE vb_certificates
			SET status = 'error', error_msg = $1, last_checked_at = NOW(), updated_at = NOW()
			WHERE id = $2::uuid AND org_id = $3::uuid
		`, errMsg, id, orgID)
		return err
	}
	status := certStatus(&info.NotAfter)
	_, err := r.db.Exec(ctx, `
		UPDATE vb_certificates
		SET issuer = $1, subject = $2, sans = $3, not_before = $4, not_after = $5,
		    status = $6, error_msg = NULL, last_checked_at = NOW(), updated_at = NOW()
		WHERE id = $7::uuid AND org_id = $8::uuid
	`, info.Issuer, info.Subject, info.SANs, info.NotBefore, info.NotAfter,
		status, id, orgID)
	return err
}

// DeleteCertificate removes a certificate record.
func (r *Repository) DeleteCertificate(ctx context.Context, orgID, id string) error {
	n, err := r.db.Exec(ctx, `
		DELETE FROM vb_certificates WHERE id = $1::uuid AND org_id = $2::uuid
	`, id, orgID)
	if err != nil {
		return fmt.Errorf("delete certificate: %w", err)
	}
	if n.RowsAffected() == 0 {
		return fmt.Errorf("certificate not found")
	}
	return nil
}

// ScanAllCertificatesForOrg rescans every certificate in vb_certificates for the org,
// updating status and metadata. Returns the number of certs scanned.
func ScanAllCertificatesForOrg(ctx context.Context, pool *pgxpool.Pool, orgID string) (int, error) {
	type certRow struct {
		id     string
		domain string
	}

	rows, err := pool.Query(ctx, `
		SELECT id::text, domain FROM vb_certificates WHERE org_id = $1::uuid
	`, orgID)
	if err != nil {
		return 0, fmt.Errorf("list certs for scan: %w", err)
	}
	defer rows.Close()

	var certs []certRow
	for rows.Next() {
		var cr certRow
		if e := rows.Scan(&cr.id, &cr.domain); e == nil {
			certs = append(certs, cr)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	repo := &Repository{db: pool}
	for _, cert := range certs {
		info, scanErr := ScanTLSCertificate(cert.domain)
		if updateErr := repo.UpdateCertificateScan(ctx, orgID, cert.id, info, scanErr); updateErr != nil {
			log.Error().Err(updateErr).Str("cert_id", cert.id).Msg("cert_scan: update failed")
		}
	}
	return len(certs), nil
}

// DB returns the underlying connection pool. Used for ad-hoc raw-SQL operations.
func (r *Repository) DB() *pgxpool.Pool { return r.db }

// ListExpiringCertificates returns certificates whose not_after is within the next [days] days.
func (r *Repository) ListExpiringCertificates(ctx context.Context, orgID string, days int) ([]Certificate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, domain, issuer, subject, sans,
		       not_before, not_after, asset_id::text, source, status,
		       last_checked_at, error_msg, created_at, updated_at
		FROM vb_certificates
		WHERE org_id = $1::uuid
		  AND not_after IS NOT NULL
		  AND not_after <= NOW() + make_interval(days => $2::int)
		ORDER BY not_after ASC
	`, orgID, days)
	if err != nil {
		return nil, fmt.Errorf("list expiring certificates: %w", err)
	}
	defer rows.Close()

	var out []Certificate
	for rows.Next() {
		cert, scanErr := scanCertRow(rows)
		if scanErr != nil {
			continue
		}
		out = append(out, *cert)
	}
	return out, rows.Err()
}

// ── Row scanner ──────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCertRow(row rowScanner) (*Certificate, error) {
	var c Certificate
	var sans []string
	var notBefore, notAfter, lastChecked *time.Time
	var assetID, errorMsg *string

	err := row.Scan(
		&c.ID, &c.OrgID, &c.Domain, &c.Issuer, &c.Subject, &sans,
		&notBefore, &notAfter, &assetID, &c.Source, &c.Status,
		&lastChecked, &errorMsg, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("certificate not found")
		}
		return nil, fmt.Errorf("scan certificate row: %w", err)
	}

	c.SANs = sans
	c.NotBefore = notBefore
	c.NotAfter = notAfter
	c.AssetID = assetID
	c.LastCheckedAt = lastChecked
	c.ErrorMsg = errorMsg
	return &c, nil
}
