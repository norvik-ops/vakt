// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

const keycloakSource = "keycloak-collector"

// KeycloakCollector collects identity & access compliance evidence from a Keycloak realm.
type KeycloakCollector struct {
	db         *pgxpool.Pool
	evidence   EvidenceWriter
	httpClient *http.Client
}

// NewKeycloakCollector creates a new KeycloakCollector.
func NewKeycloakCollector(db *pgxpool.Pool, evidence EvidenceWriter) *KeycloakCollector {
	return &KeycloakCollector{
		db:       db,
		evidence: evidence,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Collect runs all Keycloak evidence collectors. Returns the number of evidence items created.
func (c *KeycloakCollector) Collect(ctx context.Context, orgID string, cfg KeycloakConfig) (int, error) {
	token, err := c.authenticate(ctx, cfg)
	if err != nil {
		return 0, fmt.Errorf("keycloak auth: %w", err)
	}

	baseURL := strings.TrimRight(cfg.KeycloakURL, "/") + "/admin/realms/" + cfg.Realm
	identityControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"mfa", "authentication", "password", "access"})
	accessControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"privileged", "admin", "access", "rights"})

	total := 0

	users, err := c.getAllUsers(ctx, cfg, token)
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("keycloak_collector: could not load users")
	}

	if n, err := c.collectMFAStatus(ctx, orgID, baseURL, token, users, identityControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("keycloak_collector: mfa status failed")
	} else {
		total += n
	}

	if n, err := c.collectPasswordPolicy(ctx, orgID, baseURL, token, identityControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("keycloak_collector: password policy failed")
	} else {
		total += n
	}

	if n, err := c.collectInactiveUsers(ctx, orgID, users, accessControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("keycloak_collector: inactive users failed")
	} else {
		total += n
	}

	if n, err := c.collectAdminRoles(ctx, orgID, baseURL, token, users, accessControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("keycloak_collector: admin roles failed")
	} else {
		total += n
	}

	if n, err := c.collectSessionPolicy(ctx, orgID, baseURL, token, identityControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("keycloak_collector: session policy failed")
	} else {
		total += n
	}

	return total, nil
}

// authenticate obtains a Bearer token via client_credentials from the realm token endpoint.
func (c *KeycloakCollector) authenticate(ctx context.Context, cfg KeycloakConfig) (string, error) {
	tokenURL := strings.TrimRight(cfg.KeycloakURL, "/") +
		"/realms/" + cfg.Realm + "/protocol/openid-connect/token"

	body := url.Values{}
	body.Set("grant_type", "client_credentials")
	body.Set("client_id", cfg.ClientID)
	body.Set("client_secret", cfg.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("keycloak token endpoint returned %d: %s", resp.StatusCode, string(raw))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(raw, &tokenResp); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if tokenResp.Error != "" {
		return "", fmt.Errorf("token error: %s — %s", tokenResp.Error, tokenResp.ErrorDesc)
	}
	return tokenResp.AccessToken, nil
}

type keycloakUser struct {
	ID               string `json:"id"`
	Username         string `json:"username"`
	NotBefore        int64  `json:"notBefore"`
	CreatedTimestamp int64  `json:"createdTimestamp"` // ms since epoch
}

// getAllUsers fetches all users from the realm using pagination.
func (c *KeycloakCollector) getAllUsers(ctx context.Context, cfg KeycloakConfig, token string) ([]keycloakUser, error) {
	baseURL := strings.TrimRight(cfg.KeycloakURL, "/") + "/admin/realms/" + cfg.Realm

	var all []keycloakUser
	batchSize := 100
	first := 0

	for {
		apiURL := fmt.Sprintf("%s/users?max=%d&first=%d&briefRepresentation=true", baseURL, batchSize, first)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			return all, fmt.Errorf("build users request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return all, fmt.Errorf("get users: %w", err)
		}

		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
		_ = resp.Body.Close()
		if readErr != nil {
			return all, fmt.Errorf("read users response: %w", readErr)
		}

		if resp.StatusCode == http.StatusForbidden {
			return all, fmt.Errorf("insufficient permissions — Service Account benötigt 'view-users' Rolle")
		}
		if resp.StatusCode != http.StatusOK {
			return all, fmt.Errorf("get users returned %d: %s", resp.StatusCode, string(raw))
		}

		var batch []keycloakUser
		if err := json.Unmarshal(raw, &batch); err != nil {
			return all, fmt.Errorf("parse users: %w", err)
		}

		all = append(all, batch...)
		if len(batch) < batchSize {
			break
		}
		first += batchSize
	}

	return all, nil
}

// collectMFAStatus checks OTP credentials for each user.
func (c *KeycloakCollector) collectMFAStatus(ctx context.Context, orgID, baseURL, token string, users []keycloakUser, controls []ControlMatch) (int, error) {
	total := len(users)
	mfaEnabled := 0

	for _, user := range users {
		apiURL := fmt.Sprintf("%s/users/%s/credentials", baseURL, user.ID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		var creds []struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &creds); err != nil {
			continue
		}

		for _, cred := range creds {
			if cred.Type == "otp" || cred.Type == "totp" {
				mfaEnabled++
				break
			}
		}
	}

	pct := 0.0
	if total > 0 {
		pct = float64(mfaEnabled) / float64(total) * 100
	}

	status := "ok"
	if pct < 80 {
		status = "warning"
	}

	details := map[string]any{
		"collected_at":       time.Now().UTC().Format(time.RFC3339),
		"total_users":        total,
		"mfa_enabled":        mfaEnabled,
		"mfa_enrollment_pct": pct,
		"status":             status,
	}

	title := fmt.Sprintf("Keycloak MFA-Enrollment: %d von %d Usern haben OTP aktiviert (%.0f%%)", mfaEnabled, total, pct)
	if err := c.evidence.AddCollectorEvidence(ctx, orgID, firstControlID(controls), "", keycloakSource, title, mustMarshal(details)); err != nil {
		return 0, err
	}
	return 1, nil
}

// collectPasswordPolicy evaluates the realm password policy string.
func (c *KeycloakCollector) collectPasswordPolicy(ctx context.Context, orgID, baseURL, token string, controls []ControlMatch) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return 0, fmt.Errorf("build realm request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("get realm: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return 0, fmt.Errorf("read realm: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("get realm returned %d", resp.StatusCode)
	}

	var realm struct {
		PasswordPolicy        string `json:"passwordPolicy"`
		SSOSessionMaxLifespan int    `json:"ssoSessionMaxLifespan"` // seconds
	}
	if err := json.Unmarshal(raw, &realm); err != nil {
		return 0, fmt.Errorf("parse realm: %w", err)
	}

	policyLen := parsePasswordPolicyLength(realm.PasswordPolicy)
	status := "ok"
	if policyLen < 8 {
		status = "warning"
	}

	details := map[string]any{
		"collected_at":  time.Now().UTC().Format(time.RFC3339),
		"policy_string": realm.PasswordPolicy,
		"min_length":    policyLen,
		"status":        status,
	}

	title := fmt.Sprintf("Keycloak Password-Policy: %s", realm.PasswordPolicy)
	if realm.PasswordPolicy == "" {
		title = "Keycloak Password-Policy: nicht konfiguriert"
	}
	if err := c.evidence.AddCollectorEvidence(ctx, orgID, firstControlID(controls), "", keycloakSource, title, mustMarshal(details)); err != nil {
		return 0, err
	}
	return 1, nil
}

// collectInactiveUsers identifies users not seen for more than 90 days.
// Keycloak doesn't expose last-login in the user list directly; we use createdTimestamp
// plus notBefore as a proxy for never-logged-in + old accounts.
func (c *KeycloakCollector) collectInactiveUsers(ctx context.Context, orgID string, users []keycloakUser, controls []ControlMatch) (int, error) {
	threshold := time.Now().UTC().AddDate(0, 0, -90)
	thresholdMs := threshold.UnixMilli()

	inactive := 0
	for _, u := range users {
		// Never logged in and account older than 90 days
		if u.NotBefore == 0 && u.CreatedTimestamp > 0 && u.CreatedTimestamp < thresholdMs {
			inactive++
		}
	}

	status := "ok"
	if inactive > 0 {
		status = "warning"
	}

	details := map[string]any{
		"collected_at":   time.Now().UTC().Format(time.RFC3339),
		"inactive_users": inactive,
		"threshold_days": 90,
		"status":         status,
	}

	title := fmt.Sprintf("Keycloak Inaktive Accounts: %d User seit >90 Tagen nie eingeloggt", inactive)
	if err := c.evidence.AddCollectorEvidence(ctx, orgID, firstControlID(controls), "", keycloakSource, title, mustMarshal(details)); err != nil {
		return 0, err
	}
	return 1, nil
}

// collectAdminRoles finds users with realm-admin, manage-users, or manage-realm roles.
func (c *KeycloakCollector) collectAdminRoles(ctx context.Context, orgID, baseURL, token string, users []keycloakUser, controls []ControlMatch) (int, error) {
	adminRoles := map[string]bool{
		"realm-admin":  true,
		"manage-users": true,
		"manage-realm": true,
	}

	adminCount := 0
	for _, user := range users {
		apiURL := fmt.Sprintf("%s/users/%s/role-mappings/realm", baseURL, user.ID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		var roles []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &roles); err != nil {
			continue
		}

		for _, r := range roles {
			if adminRoles[r.Name] {
				adminCount++
				break
			}
		}
	}

	details := map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"admin_count":  adminCount,
	}

	title := fmt.Sprintf("Keycloak Privilegierte Accounts: %d Admins im Realm", adminCount)
	if err := c.evidence.AddCollectorEvidence(ctx, orgID, firstControlID(controls), "", keycloakSource, title, mustMarshal(details)); err != nil {
		return 0, err
	}
	return 1, nil
}

// collectSessionPolicy evaluates the SSO session timeout configuration.
func (c *KeycloakCollector) collectSessionPolicy(ctx context.Context, orgID, baseURL, token string, controls []ControlMatch) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return 0, fmt.Errorf("build realm request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("get realm: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return 0, fmt.Errorf("read realm: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("get realm returned %d", resp.StatusCode)
	}

	var realm struct {
		SSOSessionMaxLifespan int `json:"ssoSessionMaxLifespan"` // seconds
	}
	if err := json.Unmarshal(raw, &realm); err != nil {
		return 0, fmt.Errorf("parse realm: %w", err)
	}

	const recommendedMaxHours = 8
	timeoutHours := realm.SSOSessionMaxLifespan / 3600

	status := "ok"
	if timeoutHours > recommendedMaxHours {
		status = "warning"
	}

	details := map[string]any{
		"collected_at":          time.Now().UTC().Format(time.RFC3339),
		"sso_timeout_seconds":   realm.SSOSessionMaxLifespan,
		"sso_timeout_hours":     timeoutHours,
		"recommended_max_hours": recommendedMaxHours,
		"status":                status,
	}

	title := fmt.Sprintf("Keycloak Session-Timeout: %dh konfiguriert", timeoutHours)
	if status == "warning" {
		title += " (Empfehlung: ≤8h)"
	}
	if err := c.evidence.AddCollectorEvidence(ctx, orgID, firstControlID(controls), "", keycloakSource, title, mustMarshal(details)); err != nil {
		return 0, err
	}
	return 1, nil
}

// parsePasswordPolicyLength extracts the minimum length from a Keycloak password policy string.
// e.g. "length(12) and upperCase(1)" → 12; "" → 0
func parsePasswordPolicyLength(policy string) int {
	re := regexp.MustCompile(`length\((\d+)\)`)
	m := re.FindStringSubmatch(policy)
	if len(m) < 2 {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
