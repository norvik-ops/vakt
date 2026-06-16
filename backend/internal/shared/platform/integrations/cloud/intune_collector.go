// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S88-7: Microsoft Intune / MDM device-posture collector. Pulls
// deviceManagement/managedDevices from the Microsoft Graph API (customer's own
// tenant, OAuth2 client credentials) and records endpoint compliance as evidence
// for ISO A.8.1 (user end point devices), A.8.9 (configuration management) and
// NIS2 cyber-hygiene. Read-only — no device management.

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

const intuneSource = "intune-collector"

// IntuneCollector collects MDM device-posture evidence from Microsoft Graph.
type IntuneCollector struct {
	db           *pgxpool.Pool
	evidence     EvidenceWriter
	httpClient   *http.Client
	loginBaseURL string
	graphBaseURL string
}

// NewIntuneCollector creates a new IntuneCollector.
func NewIntuneCollector(db *pgxpool.Pool, evidence EvidenceWriter) *IntuneCollector {
	return &IntuneCollector{
		db:           db,
		evidence:     evidence,
		loginBaseURL: "https://login.microsoftonline.com",
		graphBaseURL: "https://graph.microsoft.com",
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// managedDevice is the subset of Graph's managedDevices we evaluate.
type managedDevice struct {
	DeviceName       string `json:"deviceName"`
	ComplianceState  string `json:"complianceState"` // compliant | noncompliant | ...
	OperatingSystem  string `json:"operatingSystem"`
	OSVersion        string `json:"osVersion"`
	IsEncrypted      bool   `json:"isEncrypted"`
	LastSyncDateTime string `json:"lastSyncDateTime"`
}

// DevicePosture is the aggregate posture computed from managedDevices.
type DevicePosture struct {
	Total         int     `json:"total"`
	Compliant     int     `json:"compliant"`
	NonCompliant  int     `json:"non_compliant"`
	Encrypted     int     `json:"encrypted"`
	CompliancePct float64 `json:"compliance_pct"`
	EncryptionPct float64 `json:"encryption_pct"`
}

// computePosture aggregates a managedDevices list into a DevicePosture.
// Exposed (lower-case, package-internal) for unit testing without HTTP.
func computePosture(devices []managedDevice) DevicePosture {
	var p DevicePosture
	p.Total = len(devices)
	for _, d := range devices {
		if strings.EqualFold(d.ComplianceState, "compliant") {
			p.Compliant++
		} else {
			p.NonCompliant++
		}
		if d.IsEncrypted {
			p.Encrypted++
		}
	}
	if p.Total > 0 {
		p.CompliancePct = float64(p.Compliant) / float64(p.Total) * 100
		p.EncryptionPct = float64(p.Encrypted) / float64(p.Total) * 100
	}
	return p
}

// parseManagedDevices extracts the managedDevice list from a Graph response body.
func parseManagedDevices(raw []byte) ([]managedDevice, error) {
	var resp struct {
		Value []managedDevice `json:"value"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse managedDevices: %w", err)
	}
	return resp.Value, nil
}

// Collect pulls device posture and writes it as evidence. Returns evidence count.
func (c *IntuneCollector) Collect(ctx context.Context, orgID string, cfg IntuneConfig) (int, error) {
	token, err := c.getAccessToken(ctx, cfg)
	if err != nil {
		return 0, fmt.Errorf("intune auth: %w", err)
	}

	apiURL := c.graphBaseURL + "/v1.0/deviceManagement/managedDevices?$select=deviceName,complianceState,operatingSystem,osVersion,isEncrypted,lastSyncDateTime&$top=200"
	raw, err := c.graphGetRaw(ctx, token, apiURL)
	if err != nil {
		return 0, fmt.Errorf("intune managedDevices: %w", err)
	}
	devices, err := parseManagedDevices(raw)
	if err != nil {
		return 0, err
	}
	posture := computePosture(devices)

	endpointControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"endpoint", "device", "mobile", "end point"})
	configControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"configuration", "hardening", "compliance", "cyber hygiene"})

	total := 0
	title := fmt.Sprintf("Intune Device-Compliance: %.0f%% compliant (%d/%d Geräte)", posture.CompliancePct, posture.Compliant, posture.Total)
	if err := c.evidence.AddCollectorEvidence(ctx, orgID, firstControlID(endpointControls), "", intuneSource, title, mustMarshal(posture)); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("intune_collector: endpoint evidence failed")
	} else {
		total++
	}
	encTitle := fmt.Sprintf("Intune Geräteverschlüsselung: %.0f%% (%d/%d)", posture.EncryptionPct, posture.Encrypted, posture.Total)
	if err := c.evidence.AddCollectorEvidence(ctx, orgID, firstControlID(configControls), "", intuneSource, encTitle, mustMarshal(posture)); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("intune_collector: config evidence failed")
	} else {
		total++
	}
	return total, nil
}

func (c *IntuneCollector) getAccessToken(ctx context.Context, cfg IntuneConfig) (string, error) {
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
		return "", fmt.Errorf("token endpoint returned %d", resp.StatusCode)
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

func (c *IntuneCollector) graphGetRaw(ctx context.Context, token, apiURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("graph get: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("graph api returned %d", resp.StatusCode)
	}
	return raw, nil
}
