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
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

const entraidSource = "entraid-collector"

// EntraIDCollector collects identity & access compliance evidence from Microsoft Graph API.
type EntraIDCollector struct {
	db           *pgxpool.Pool
	evidence     EvidenceWriter
	httpClient   *http.Client
	loginBaseURL string // default: "https://login.microsoftonline.com" (overridable in tests)
	graphBaseURL string // default: "https://graph.microsoft.com"       (overridable in tests)
}

// NewEntraIDCollector creates a new EntraIDCollector.
func NewEntraIDCollector(db *pgxpool.Pool, evidence EvidenceWriter) *EntraIDCollector {
	return &EntraIDCollector{
		db:           db,
		evidence:     evidence,
		loginBaseURL: "https://login.microsoftonline.com",
		graphBaseURL: "https://graph.microsoft.com",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Collect runs all Entra ID evidence collectors. Returns number of evidence items created.
// Permission errors on individual endpoints cause partial status; auth errors abort early.
func (c *EntraIDCollector) Collect(ctx context.Context, orgID string, cfg EntraIDConfig) (int, error) {
	token, err := c.getAccessToken(ctx, cfg)
	if err != nil {
		return 0, fmt.Errorf("entraid auth: %w", err)
	}

	identityControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"mfa", "authentication", "access", "identity"})
	accessControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"privileged", "admin", "access", "rights"})
	monitoringControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"monitoring", "risk", "incident"})

	total := 0

	if n, err := c.collectMFAEnrollment(ctx, orgID, token, identityControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("entraid_collector: mfa enrollment failed")
	} else {
		total += n
	}

	if n, err := c.collectConditionalAccess(ctx, orgID, token, accessControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("entraid_collector: conditional access failed")
	} else {
		total += n
	}

	if n, err := c.collectRiskyUsers(ctx, orgID, token, monitoringControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("entraid_collector: risky users failed")
	} else {
		total += n
	}

	if n, err := c.collectAdminRoles(ctx, orgID, token, accessControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("entraid_collector: admin roles failed")
	} else {
		total += n
	}

	if n, err := c.collectInactiveUsers(ctx, orgID, token, accessControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("entraid_collector: inactive users failed")
	} else {
		total += n
	}

	return total, nil
}

// getAccessToken obtains a Bearer token via OAuth2 client_credentials flow against Graph API.
func (c *EntraIDCollector) getAccessToken(ctx context.Context, cfg EntraIDConfig) (string, error) {
	tokenURL := fmt.Sprintf("%s/%s/oauth2/v2.0/token", c.loginBaseURL, cfg.TenantID)

	body := url.Values{}
	body.Set("grant_type", "client_credentials")
	body.Set("client_id", cfg.ClientID)
	body.Set("client_secret", cfg.ClientSecret)
	body.Set("scope", "https://graph.microsoft.com/.default")

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
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(raw))
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

// graphGet performs an authenticated GET against the Microsoft Graph API.
func (c *EntraIDCollector) graphGet(ctx context.Context, token, apiURL string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("graph api %s returned %d: %s", apiURL, resp.StatusCode, string(raw))
	}

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return result, nil
}

// graphGetAll follows @odata.nextLink pagination and returns all value items.
func (c *EntraIDCollector) graphGetAll(ctx context.Context, token, apiURL string) ([]json.RawMessage, error) {
	var all []json.RawMessage
	next := apiURL

	for next != "" {
		result, err := c.graphGet(ctx, token, next)
		if err != nil {
			return all, err
		}
		if vals, ok := result["value"].([]any); ok {
			for _, v := range vals {
				b, _ := json.Marshal(v)
				all = append(all, b)
			}
		}
		if link, ok := result["@odata.nextLink"].(string); ok {
			next = link
		} else {
			next = ""
		}
	}
	return all, nil
}

func (c *EntraIDCollector) addEvidence(ctx context.Context, orgID, controlID, title string, details map[string]any) error {
	data, _ := json.Marshal(details)
	if controlID == "" {
		return nil
	}
	return c.evidence.AddCollectorEvidence(ctx, orgID, controlID, "", entraidSource, title, data)
}

// collectMFAEnrollment collects MFA registration stats via credentialUserRegistrationDetails.
func (c *EntraIDCollector) collectMFAEnrollment(ctx context.Context, orgID, token string, controls []ControlMatch) (int, error) {
	items, err := c.graphGetAll(ctx, token,
		c.graphBaseURL+"/v1.0/reports/credentialUserRegistrationDetails")
	if err != nil {
		return 0, err
	}

	total := len(items)
	mfaEnabled := 0
	for _, raw := range items {
		var user struct {
			IsMFARegistered bool `json:"isMfaRegistered"`
		}
		if json.Unmarshal(raw, &user) == nil && user.IsMFARegistered {
			mfaEnabled++
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

	title := fmt.Sprintf("Entra ID MFA-Enrollment: %.0f%% aller User haben MFA aktiviert", pct)
	if err := c.addEvidence(ctx, orgID, firstControlID(controls), title, details); err != nil {
		return 0, err
	}
	return 1, nil
}

// collectConditionalAccess collects active Conditional Access policies.
func (c *EntraIDCollector) collectConditionalAccess(ctx context.Context, orgID, token string, controls []ControlMatch) (int, error) {
	result, err := c.graphGet(ctx, token,
		c.graphBaseURL+"/v1.0/identity/conditionalAccess/policies")
	if err != nil {
		return 0, err
	}

	vals, _ := result["value"].([]any)
	active, total := 0, len(vals)
	var policyNames []string

	for _, v := range vals {
		if p, ok := v.(map[string]any); ok {
			if state, _ := p["state"].(string); state == "enabled" {
				active++
				if name, _ := p["displayName"].(string); name != "" {
					policyNames = append(policyNames, name)
				}
			}
		}
	}

	details := map[string]any{
		"collected_at":    time.Now().UTC().Format(time.RFC3339),
		"total_policies":  total,
		"active_policies": active,
		"policy_names":    policyNames,
	}

	title := fmt.Sprintf("Entra ID Conditional Access: %d aktive Policies", active)
	if err := c.addEvidence(ctx, orgID, firstControlID(controls), title, details); err != nil {
		return 0, err
	}
	return 1, nil
}

// collectRiskyUsers collects users with elevated identity risk.
func (c *EntraIDCollector) collectRiskyUsers(ctx context.Context, orgID, token string, controls []ControlMatch) (int, error) {
	q := url.Values{}
	q.Set("$filter", "riskLevel eq 'high' or riskLevel eq 'medium'")
	result, err := c.graphGet(ctx, token,
		c.graphBaseURL+"/v1.0/identityProtection/riskyUsers?"+q.Encode())
	if err != nil {
		return 0, err
	}

	vals, _ := result["value"].([]any)
	count := len(vals)

	status := "ok"
	if count > 0 {
		status = "warning"
	}

	details := map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"risky_users":  count,
		"status":       status,
	}

	title := fmt.Sprintf("Entra ID Risky Users: %d User mit hohem/mittlerem Risiko", count)
	if err := c.addEvidence(ctx, orgID, firstControlID(controls), title, details); err != nil {
		return 0, err
	}
	return 1, nil
}

// collectAdminRoles collects privileged role assignments.
func (c *EntraIDCollector) collectAdminRoles(ctx context.Context, orgID, token string, controls []ControlMatch) (int, error) {
	result, err := c.graphGet(ctx, token,
		c.graphBaseURL+"/v1.0/roleManagement/directory/roleAssignments?$expand=roleDefinition")
	if err != nil {
		return 0, err
	}

	vals, _ := result["value"].([]any)
	globalAdmins := 0
	other := 0

	for _, v := range vals {
		if assignment, ok := v.(map[string]any); ok {
			if rd, ok := assignment["roleDefinition"].(map[string]any); ok {
				if name, _ := rd["displayName"].(string); name == "Global Administrator" {
					globalAdmins++
				} else {
					other++
				}
			}
		}
	}

	details := map[string]any{
		"collected_at":   time.Now().UTC().Format(time.RFC3339),
		"global_admins":  globalAdmins,
		"other_admins":   other,
		"total_assigned": len(vals),
	}

	title := fmt.Sprintf("Entra ID Privilegierte Accounts: %d Global Admins, %d weitere", globalAdmins, other)
	if err := c.addEvidence(ctx, orgID, firstControlID(controls), title, details); err != nil {
		return 0, err
	}
	return 1, nil
}

// collectInactiveUsers collects users who haven't signed in for more than 90 days.
func (c *EntraIDCollector) collectInactiveUsers(ctx context.Context, orgID, token string, controls []ControlMatch) (int, error) {
	threshold := time.Now().UTC().AddDate(0, 0, -90).Format(time.RFC3339)
	q := url.Values{}
	q.Set("$select", "id,displayName,userPrincipalName,signInActivity")
	q.Set("$filter", "signInActivity/lastSignInDateTime lt "+threshold)
	apiURL := c.graphBaseURL + "/v1.0/users?" + q.Encode()

	items, err := c.graphGetAll(ctx, token, apiURL)
	if err != nil {
		return 0, err
	}

	count := len(items)
	status := "ok"
	if count > 0 {
		status = "warning"
	}

	details := map[string]any{
		"collected_at":   time.Now().UTC().Format(time.RFC3339),
		"inactive_users": count,
		"threshold_days": 90,
		"status":         status,
	}

	title := fmt.Sprintf("Entra ID Inaktive Accounts: %d User seit >90 Tagen nicht eingeloggt", count)
	if err := c.addEvidence(ctx, orgID, firstControlID(controls), title, details); err != nil {
		return 0, err
	}
	return 1, nil
}
