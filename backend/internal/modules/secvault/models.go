// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package secvault provides domain models for secrets management, Git scanning, and secret rotation.
package secvault

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// privateRanges holds the RFC 1918 private IPv4 CIDRs used by ValidateRepoURL
// and its DNS rebinding check. Parsed once at package init to avoid repeated
// allocations per request.
var privateRanges = func() []*net.IPNet {
	cidrs := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, n, _ := net.ParseCIDR(c)
		if n != nil {
			out = append(out, n)
		}
	}
	return out
}()

// ValidateRepoURL returns an error if the URL is not a safe, public HTTPS
// endpoint. It blocks:
//   - non-HTTPS schemes (http, file, git, etc.)
//   - loopback addresses (127.x, ::1, localhost)
//   - unspecified addresses (0.0.0.0, ::)
//   - RFC 1918 private ranges (10.x, 172.16-31.x, 192.168.x)
//   - link-local addresses (169.254.x.x, fe80::/10)
//
// It also resolves the hostname via DNS and applies the same checks to all
// returned IPs to mitigate DNS-rebinding attacks.
func ValidateRepoURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("repo_url is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("repo_url is not a valid URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("repo_url must use the https:// scheme (got %q)", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("repo_url has no host")
	}

	// Reject plain "localhost" or any case variant.
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("repo_url must not target localhost")
	}

	// blockIP centralises the per-IP checks used both for literal IPs and
	// DNS-resolved addresses.
	blockIP := func(ip net.IP) error {
		if ip.IsLoopback() {
			return fmt.Errorf("repo_url must not target a loopback address")
		}
		if ip.IsUnspecified() {
			return fmt.Errorf("repo_url must not target the unspecified address")
		}
		if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("repo_url must not target a link-local address")
		}
		for _, cidr := range privateRanges {
			if cidr.Contains(ip) {
				return fmt.Errorf("repo_url must not target a private IP range")
			}
		}
		return nil
	}

	// Parse as IP to check private/loopback/link-local ranges for literal IPs.
	if ip := net.ParseIP(host); ip != nil {
		if err := blockIP(ip); err != nil {
			return err
		}
		// Literal IPs do not require a DNS lookup.
		return nil
	}

	// DNS rebinding mitigation: resolve the hostname and check every returned IP.
	resolverCtx, resolverCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer resolverCancel()
	resolvedIPs, err := net.DefaultResolver.LookupHost(resolverCtx, host)
	if err != nil {
		return fmt.Errorf("cannot resolve repo host: %w", err)
	}
	for _, rawIP := range resolvedIPs {
		ip := net.ParseIP(rawIP)
		if ip == nil {
			continue
		}
		if err := blockIP(ip); err != nil {
			return fmt.Errorf("repo_url resolves to a blocked address: %w", err)
		}
	}
	return nil
}

// --- Import / Export ---

// ImportInput describes a bulk-import request from various secret sources.
type ImportInput struct {
	Source      string `json:"source"      validate:"required,oneof=dotenv vault aws_secrets_manager"`
	Environment string `json:"environment" validate:"required"`
	// For dotenv: FileContent holds the raw .env file text.
	FileContent string `json:"file_content,omitempty"`
	// For HashiCorp Vault
	VaultAddr  string `json:"vault_addr,omitempty"`
	VaultToken string `json:"vault_token,omitempty"`
	VaultPath  string `json:"vault_path,omitempty"`
	// For AWS Secrets Manager
	AWSRegion          string `json:"aws_region,omitempty"`
	AWSAccessKeyID     string `json:"aws_access_key_id,omitempty"`
	AWSSecretAccessKey string `json:"aws_secret_access_key,omitempty"`
	AWSPrefix          string `json:"aws_prefix,omitempty"`
}

// ImportResult summarises the outcome of a bulk import operation.
type ImportResult struct {
	Imported  int      `json:"imported"`
	Versioned int      `json:"versioned"` // existing secrets that received a new version
	Errors    []string `json:"errors,omitempty"`
}

// --- Rotation ---

// RotateInput describes how a secret should be rotated.
type RotateInput struct {
	Type   string `json:"type"   validate:"required,oneof=random_string uuid db_password"`
	Length int    `json:"length,omitempty"`
}

// RotationPolicy defines the automatic rotation schedule for a single secret.
type RotationPolicy struct {
	ID             string     `json:"id"`
	OrgID          string     `json:"org_id"`
	SecretID       string     `json:"secret_id"`
	IntervalDays   int        `json:"interval_days"`
	LastRotatedAt  *time.Time `json:"last_rotated_at,omitempty"`
	NextRotationAt *time.Time `json:"next_rotation_at,omitempty"`
	IsActive       bool       `json:"is_active"`
	CreatedAt      time.Time  `json:"created_at"`
}

// --- Git scanner ---

// GitScan represents a single git repository scan run.
type GitScan struct {
	ID             string     `json:"id"`
	OrgID          string     `json:"org_id"`
	RepoURL        string     `json:"repo_url"`
	Branch         string     `json:"branch"`
	Status         string     `json:"status"`
	FindingCount   int        `json:"finding_count"`
	OpenCount      int        `json:"open_count"`
	DismissedCount int        `json:"dismissed_count"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	ScannedAt      *time.Time `json:"scanned_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// ScanResult is a single secret-like finding from a git scan.
type ScanResult struct {
	ID            string    `json:"id"`
	OrgID         string    `json:"org_id"`
	ScanID        string    `json:"scan_id"`
	RepoURL       string    `json:"repo_url"`
	CommitHash    string    `json:"commit_hash,omitempty"`
	FilePath      string    `json:"file_path"`
	LineNumber    int       `json:"line_number,omitempty"`
	PatternName   string    `json:"pattern_name"`
	MatchPreview  string    `json:"match_preview"` // always redacted: first4...last4
	Severity      string    `json:"severity"`
	Status        string    `json:"status"`
	DismissReason string    `json:"dismiss_reason,omitempty"`
	DismissCount  int       `json:"dismiss_count"`
	CreatedAt     time.Time `json:"created_at"`
}

// TriggerGitScanInput is the request body for triggering a new git scan.
// RepoURL is validated by ValidateRepoURL (HTTPS-only, no private/loopback IPs)
// rather than the generic "url" validator tag to prevent SSRF.
type TriggerGitScanInput struct {
	RepoURL     string               `json:"repo_url" validate:"required"`
	Branch      string               `json:"branch"   validate:"required"`
	Credentials *GitScanCredentials  `json:"credentials,omitempty"`
}

// GitScanCredentials holds optional authentication for cloning private repositories.
// Credentials are used only during the clone subprocess and are never stored.
type GitScanCredentials struct {
	Type  string `json:"type"  validate:"required,oneof=github_token basic"`
	Token string `json:"token,omitempty"`
	User  string `json:"user,omitempty"`
	Pass  string `json:"pass,omitempty"`
}

// DismissScanResultInput is the request body for dismissing a finding.
type DismissScanResultInput struct {
	Reason string `json:"reason" validate:"required"`
}

// Project is a top-level grouping of secrets within an organisation.
type Project struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Environment is a named environment (e.g. dev, staging, prod) within a project.
type Environment struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Secret represents an encrypted key-value pair stored in an environment.
// Value is only populated on direct Get calls — list operations omit it.
type Secret struct {
	ID              string     `json:"id"`
	Key             string     `json:"key"`
	Value           string     `json:"value,omitempty"` // only populated on GetSecret
	Version         int        `json:"version"`
	RotationDueAt   *time.Time `json:"rotation_due_at,omitempty"`
	LastRotatedAt   *time.Time `json:"last_rotated_at,omitempty"`
	LastAccessedAt  *time.Time `json:"last_accessed_at,omitempty"`
	AccessCount     int64      `json:"access_count"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// AccessLogEntry is a single record from the secret access audit trail.
type AccessLogEntry struct {
	ID         string    `json:"id"`
	SecretID   string    `json:"secret_id"`
	AccessedBy *string   `json:"accessed_by,omitempty"`
	AccessVia  string    `json:"access_via"`
	IPAddress  *string   `json:"ip_address,omitempty"`
	UserAgent  *string   `json:"user_agent,omitempty"`
	AccessedAt time.Time `json:"accessed_at"`
}

// ProjectAccessLogEntry is an access log entry enriched with the secret key name,
// returned by the project-level access log endpoint.
type ProjectAccessLogEntry struct {
	ID         string    `json:"id"`
	SecretKey  string    `json:"secret_key"`
	AccessVia  string    `json:"access_via"`
	AccessedBy *string   `json:"accessed_by,omitempty"`
	IPAddress  *string   `json:"ip_address,omitempty"`
	AccessedAt time.Time `json:"accessed_at"`
}

// ShareLink is a time-limited URL token for sharing a single secret.
type ShareLink struct {
	ID        string     `json:"id"`
	SecretID  string     `json:"secret_id"`
	ShareURL  string     `json:"share_url,omitempty"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// SecretHealth holds computed health metadata for a single secret.
type SecretHealth struct {
	SecretID          string   `json:"secret_id"`
	Key               string   `json:"key"`
	AgeInDays         int      `json:"age_in_days"`
	DaysSinceRotation int      `json:"days_since_rotation"`
	AccessCount       int64    `json:"access_count"`
	HealthScore       int      `json:"health_score"` // 0-100
	Issues            []string `json:"issues"`
}

// APIToken represents a SecretOps-scoped API key.
type APIToken struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	KeyPrefix   string     `json:"key_prefix"`
	Scopes      []string   `json:"scopes"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	// RawKey is only returned once on creation and never stored.
	RawKey string `json:"key,omitempty"`
}
