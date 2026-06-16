// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"
	"github.com/matharnica/vakt/internal/shared/httputil"
)

// Service handles cloud integration business logic (config persistence + evidence sync).
type Service struct {
	db        *pgxpool.Pool
	repo      *Repository
	masterKey []byte
	evidence  EvidenceWriter
}

// NewService creates a new cloud integration service.
func NewService(db *pgxpool.Pool, masterKey []byte, evidence EvidenceWriter) *Service {
	return &Service{
		db:        db,
		repo:      NewRepository(db),
		masterKey: masterKey,
		evidence:  evidence,
	}
}

// --- AWS ---

// GetAWSConfig returns the AWS config for an org with secrets masked.
func (s *Service) GetAWSConfig(ctx context.Context, orgID string) (*AWSConfigResponse, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, "aws")
	if err != nil {
		return nil, err
	}

	resp := &AWSConfigResponse{}
	if raw == nil {
		return resp, nil
	}

	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return resp, nil
	}

	resp.AccessKeyID = stored["access_key_id"]
	resp.Region = stored["region"]
	resp.AccountID = stored["account_id"]
	if stored["secret_access_key"] != "" {
		resp.SecretAccessKey = "****"
	}
	resp.IsConfigured = resp.AccessKeyID != "" && resp.SecretAccessKey == "****"
	return resp, nil
}

// SaveAWSConfig persists the AWS config, encrypting the secret key.
// If secret_access_key == "****", the existing value is kept unchanged.
func (s *Service) SaveAWSConfig(ctx context.Context, orgID string, in SaveAWSConfigInput) error {
	existing, err := s.getDecryptedAWSConfig(ctx, orgID)
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("cloud: could not load existing AWS config (will overwrite)")
	}

	secretKey := in.SecretAccessKey
	if secretKey == "****" {
		if existing != nil {
			secretKey = existing.SecretAccessKey
		} else {
			secretKey = ""
		}
	}

	encryptedSecret := ""
	if secretKey != "" {
		ct, encErr := sharedcrypto.Encrypt(s.masterKey, []byte(secretKey))
		if encErr != nil {
			return fmt.Errorf("encrypt secret key: %w", encErr)
		}
		encryptedSecret = hex.EncodeToString(ct)
	}

	config := map[string]any{
		"access_key_id":     in.AccessKeyID,
		"secret_access_key": encryptedSecret, // hex-encoded ciphertext
		"region":            in.Region,
		"account_id":        in.AccountID,
	}
	return s.repo.UpsertConfig(ctx, orgID, "aws", config)
}

// getDecryptedAWSConfig loads and decrypts the AWS config for internal use.
func (s *Service) getDecryptedAWSConfig(ctx context.Context, orgID string) (*AWSConfig, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, "aws")
	if err != nil || raw == nil {
		return nil, err
	}

	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, fmt.Errorf("parse aws config: %w", err)
	}

	cfg := &AWSConfig{
		AccessKeyID: stored["access_key_id"],
		Region:      stored["region"],
		AccountID:   stored["account_id"],
	}

	if stored["secret_access_key"] != "" {
		ct, decodeErr := hex.DecodeString(stored["secret_access_key"])
		if decodeErr != nil {
			return nil, fmt.Errorf("decode secret key hex: %w", decodeErr)
		}
		plain, decErr := sharedcrypto.Decrypt(s.masterKey, ct)
		if decErr != nil {
			return nil, fmt.Errorf("decrypt secret key: %w", decErr)
		}
		cfg.SecretAccessKey = string(plain)
	}
	return cfg, nil
}

// TestAWSConnection tests AWS connectivity using STS GetCallerIdentity via IAM.
func (s *Service) TestAWSConnection(ctx context.Context, orgID string) error {
	cfg, err := s.getDecryptedAWSConfig(ctx, orgID)
	if err != nil {
		return fmt.Errorf("load aws config: %w", err)
	}
	if cfg == nil || cfg.AccessKeyID == "" {
		return fmt.Errorf("AWS nicht konfiguriert")
	}

	collector := NewAWSCollector(s.db, s.evidence)
	// Test by calling IAM GetAccountPasswordPolicy (lightweight, non-destructive)
	_, testErr := collector.collectPasswordPolicy(ctx, orgID, buildAWSConfig(cfg), nil)
	if testErr != nil {
		// Return a user-friendly error
		return fmt.Errorf("AWS-Verbindung fehlgeschlagen: %w", testErr)
	}
	return nil
}

// SyncAWS runs the AWS collector immediately and updates sync status.
func (s *Service) SyncAWS(ctx context.Context, orgID string) (int, error) {
	cfg, err := s.getDecryptedAWSConfig(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("load aws config: %w", err)
	}
	if cfg == nil || cfg.AccessKeyID == "" {
		return 0, fmt.Errorf("AWS nicht konfiguriert")
	}

	collector := NewAWSCollector(s.db, s.evidence)
	count, syncErr := collector.Collect(ctx, orgID, *cfg)

	status := "success"
	if syncErr != nil {
		status = "error"
	}
	if updateErr := s.repo.UpdateSyncResult(ctx, orgID, "aws", status, syncErr); updateErr != nil {
		log.Warn().Err(updateErr).Str("org_id", orgID).Msg("cloud: failed to update AWS sync result")
	}

	return count, syncErr
}

// GetAWSStatus returns sync status and recent evidence for AWS.
func (s *Service) GetAWSStatus(ctx context.Context, orgID string) (*SyncStatus, error) {
	ci, err := s.repo.GetIntegration(ctx, orgID, "aws")
	if err != nil {
		return nil, err
	}

	st := &SyncStatus{Provider: "aws", Enabled: true}
	if ci != nil {
		st.Enabled = ci.Enabled
		st.LastSyncAt = ci.LastSyncAt
		st.LastSyncStatus = ci.LastSyncStatus
		st.LastSyncError = ci.LastSyncError
	}

	count, err := s.repo.CountEvidence(ctx, orgID, awsSource)
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("cloud: count aws evidence failed")
	}
	st.EvidenceCount = count
	return st, nil
}

// --- Azure ---

// GetAzureConfig returns the Azure config for an org with secrets masked.
func (s *Service) GetAzureConfig(ctx context.Context, orgID string) (*AzureConfigResponse, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, "azure")
	if err != nil {
		return nil, err
	}

	resp := &AzureConfigResponse{}
	if raw == nil {
		return resp, nil
	}

	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return resp, nil
	}

	resp.TenantID = stored["tenant_id"]
	resp.ClientID = stored["client_id"]
	resp.SubscriptionID = stored["subscription_id"]
	if stored["client_secret"] != "" {
		resp.ClientSecret = "****"
	}
	resp.IsConfigured = resp.TenantID != "" && resp.ClientID != "" && resp.ClientSecret == "****" && resp.SubscriptionID != ""
	return resp, nil
}

// SaveAzureConfig persists the Azure config, encrypting the client secret.
func (s *Service) SaveAzureConfig(ctx context.Context, orgID string, in SaveAzureConfigInput) error {
	existing, err := s.getDecryptedAzureConfig(ctx, orgID)
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("cloud: could not load existing Azure config (will overwrite)")
	}

	clientSecret := in.ClientSecret
	if clientSecret == "****" {
		if existing != nil {
			clientSecret = existing.ClientSecret
		} else {
			clientSecret = ""
		}
	}

	encryptedSecret := ""
	if clientSecret != "" {
		ct, encErr := sharedcrypto.Encrypt(s.masterKey, []byte(clientSecret))
		if encErr != nil {
			return fmt.Errorf("encrypt client secret: %w", encErr)
		}
		encryptedSecret = hex.EncodeToString(ct)
	}

	config := map[string]any{
		"tenant_id":       in.TenantID,
		"client_id":       in.ClientID,
		"client_secret":   encryptedSecret,
		"subscription_id": in.SubscriptionID,
	}
	return s.repo.UpsertConfig(ctx, orgID, "azure", config)
}

// getDecryptedAzureConfig loads and decrypts the Azure config for internal use.
func (s *Service) getDecryptedAzureConfig(ctx context.Context, orgID string) (*AzureConfig, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, "azure")
	if err != nil || raw == nil {
		return nil, err
	}

	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, fmt.Errorf("parse azure config: %w", err)
	}

	cfg := &AzureConfig{
		TenantID:       stored["tenant_id"],
		ClientID:       stored["client_id"],
		SubscriptionID: stored["subscription_id"],
	}

	if stored["client_secret"] != "" {
		ct, decodeErr := hex.DecodeString(stored["client_secret"])
		if decodeErr != nil {
			return nil, fmt.Errorf("decode client secret hex: %w", decodeErr)
		}
		plain, decErr := sharedcrypto.Decrypt(s.masterKey, ct)
		if decErr != nil {
			return nil, fmt.Errorf("decrypt client secret: %w", decErr)
		}
		cfg.ClientSecret = string(plain)
	}
	return cfg, nil
}

// TestAzureConnection tests Azure connectivity by requesting an access token.
func (s *Service) TestAzureConnection(ctx context.Context, orgID string) error {
	cfg, err := s.getDecryptedAzureConfig(ctx, orgID)
	if err != nil {
		return fmt.Errorf("load azure config: %w", err)
	}
	if cfg == nil || cfg.TenantID == "" {
		return fmt.Errorf("Azure nicht konfiguriert")
	}

	collector := NewAzureCollector(s.db, s.evidence)
	_, tokenErr := collector.getAccessToken(ctx, *cfg)
	if tokenErr != nil {
		return fmt.Errorf("Azure-Verbindung fehlgeschlagen: %w", tokenErr)
	}
	return nil
}

// SyncAzure runs the Azure collector immediately and updates sync status.
func (s *Service) SyncAzure(ctx context.Context, orgID string) (int, error) {
	cfg, err := s.getDecryptedAzureConfig(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("load azure config: %w", err)
	}
	if cfg == nil || cfg.TenantID == "" {
		return 0, fmt.Errorf("Azure nicht konfiguriert")
	}

	collector := NewAzureCollector(s.db, s.evidence)
	count, syncErr := collector.Collect(ctx, orgID, *cfg)

	status := "success"
	if syncErr != nil {
		status = "error"
	}
	if updateErr := s.repo.UpdateSyncResult(ctx, orgID, "azure", status, syncErr); updateErr != nil {
		log.Warn().Err(updateErr).Str("org_id", orgID).Msg("cloud: failed to update Azure sync result")
	}

	return count, syncErr
}

// GetAzureStatus returns sync status and recent evidence for Azure.
func (s *Service) GetAzureStatus(ctx context.Context, orgID string) (*SyncStatus, error) {
	ci, err := s.repo.GetIntegration(ctx, orgID, "azure")
	if err != nil {
		return nil, err
	}

	st := &SyncStatus{Provider: "azure", Enabled: true}
	if ci != nil {
		st.Enabled = ci.Enabled
		st.LastSyncAt = ci.LastSyncAt
		st.LastSyncStatus = ci.LastSyncStatus
		st.LastSyncError = ci.LastSyncError
	}

	count, err := s.repo.CountEvidence(ctx, orgID, azureSource)
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("cloud: count azure evidence failed")
	}
	st.EvidenceCount = count
	return st, nil
}

// --- Hetzner ---

// GetHetznerConfig returns the Hetzner config for an org with secrets masked.
func (s *Service) GetHetznerConfig(ctx context.Context, orgID string) (*HetznerConfigResponse, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderHetzner)
	if err != nil {
		return nil, err
	}
	resp := &HetznerConfigResponse{}
	if raw == nil {
		return resp, nil
	}
	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return resp, nil
	}
	resp.Location = stored["location"]
	if stored["api_token"] != "" {
		resp.APIToken = "****"
	}
	resp.IsConfigured = resp.APIToken == "****"
	return resp, nil
}

// SaveHetznerConfig persists the Hetzner config, encrypting the API token.
func (s *Service) SaveHetznerConfig(ctx context.Context, orgID string, in SaveHetznerConfigInput) error {
	existing, _ := s.getDecryptedHetznerConfig(ctx, orgID)

	token := in.APIToken
	if token == "****" && existing != nil {
		token = existing.APIToken
	}

	encToken := ""
	if token != "" {
		ct, err := sharedcrypto.Encrypt(s.masterKey, []byte(token))
		if err != nil {
			return fmt.Errorf("encrypt hetzner token: %w", err)
		}
		encToken = hex.EncodeToString(ct)
	}

	return s.repo.UpsertConfig(ctx, orgID, ProviderHetzner, map[string]any{
		"api_token": encToken,
		"location":  in.Location,
	})
}

func (s *Service) getDecryptedHetznerConfig(ctx context.Context, orgID string) (*HetznerConfig, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderHetzner)
	if err != nil || raw == nil {
		return nil, err
	}
	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, err
	}
	cfg := &HetznerConfig{Location: stored["location"]}
	if stored["api_token"] != "" {
		ct, err := hex.DecodeString(stored["api_token"])
		if err != nil {
			return nil, fmt.Errorf("decode hetzner token: %w", err)
		}
		plain, err := sharedcrypto.Decrypt(s.masterKey, ct)
		if err != nil {
			return nil, fmt.Errorf("decrypt hetzner token: %w", err)
		}
		cfg.APIToken = string(plain)
	}
	return cfg, nil
}

// SyncHetzner runs the Hetzner collector and updates sync status.
func (s *Service) SyncHetzner(ctx context.Context, orgID string) (int, error) {
	cfg, err := s.getDecryptedHetznerConfig(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("load hetzner config: %w", err)
	}
	if cfg == nil || cfg.APIToken == "" {
		return 0, fmt.Errorf("Hetzner nicht konfiguriert")
	}
	collector := NewHetznerCollector(s.db, s.evidence)
	count, syncErr := collector.Collect(ctx, orgID, *cfg)
	status := "success"
	if syncErr != nil {
		status = "error"
	}
	if updateErr := s.repo.UpdateSyncResult(ctx, orgID, ProviderHetzner, status, syncErr); updateErr != nil {
		log.Warn().Err(updateErr).Str("org_id", orgID).Msg("cloud: failed to update Hetzner sync result")
	}
	return count, syncErr
}

// GetHetznerStatus returns sync status and server count for Hetzner.
func (s *Service) GetHetznerStatus(ctx context.Context, orgID string) (*HetznerStatus, error) {
	ci, err := s.repo.GetIntegration(ctx, orgID, ProviderHetzner)
	if err != nil {
		return nil, err
	}
	st := &HetznerStatus{SyncStatus: SyncStatus{Provider: ProviderHetzner, Enabled: true}}
	if ci != nil {
		st.Enabled = ci.Enabled
		st.LastSyncAt = ci.LastSyncAt
		st.LastSyncStatus = ci.LastSyncStatus
		st.LastSyncError = ci.LastSyncError
	}
	count, _ := s.repo.CountEvidence(ctx, orgID, hetznerSource)
	st.EvidenceCount = count
	return st, nil
}

// --- IONOS ---

// GetIONOSConfig returns the IONOS config for an org with secrets masked.
func (s *Service) GetIONOSConfig(ctx context.Context, orgID string) (*IONOSConfigResponse, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderIONOS)
	if err != nil {
		return nil, err
	}
	resp := &IONOSConfigResponse{}
	if raw == nil {
		return resp, nil
	}
	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return resp, nil
	}
	resp.Username = stored["username"]
	if stored["password"] != "" {
		resp.Password = "****"
	}
	if stored["token"] != "" {
		resp.Token = "****"
	}
	resp.IsConfigured = resp.Username != "" || resp.Token == "****"
	return resp, nil
}

// SaveIONOSConfig persists the IONOS config, encrypting credentials.
func (s *Service) SaveIONOSConfig(ctx context.Context, orgID string, in SaveIONOSConfigInput) error {
	existing, _ := s.getDecryptedIONOSConfig(ctx, orgID)

	encryptField := func(value, existingValue string) (string, error) {
		if value == "****" && existing != nil {
			value = existingValue
		}
		if value == "" {
			return "", nil
		}
		ct, err := sharedcrypto.Encrypt(s.masterKey, []byte(value))
		if err != nil {
			return "", err
		}
		return hex.EncodeToString(ct), nil
	}

	var existingPw, existingToken string
	if existing != nil {
		existingPw = existing.Password
		existingToken = existing.Token
	}

	encPw, err := encryptField(in.Password, existingPw)
	if err != nil {
		return fmt.Errorf("encrypt ionos password: %w", err)
	}
	encToken, err := encryptField(in.Token, existingToken)
	if err != nil {
		return fmt.Errorf("encrypt ionos token: %w", err)
	}

	return s.repo.UpsertConfig(ctx, orgID, ProviderIONOS, map[string]any{
		"username": in.Username,
		"password": encPw,
		"token":    encToken,
	})
}

func (s *Service) getDecryptedIONOSConfig(ctx context.Context, orgID string) (*IONOSConfig, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderIONOS)
	if err != nil || raw == nil {
		return nil, err
	}
	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, err
	}
	cfg := &IONOSConfig{Username: stored["username"]}
	decryptField := func(enc string) (string, error) {
		if enc == "" {
			return "", nil
		}
		ct, err := hex.DecodeString(enc)
		if err != nil {
			return "", err
		}
		plain, err := sharedcrypto.Decrypt(s.masterKey, ct)
		if err != nil {
			return "", err
		}
		return string(plain), nil
	}
	cfg.Password, _ = decryptField(stored["password"])
	cfg.Token, _ = decryptField(stored["token"])
	return cfg, nil
}

// SyncIONOS runs the IONOS collector and updates sync status.
func (s *Service) SyncIONOS(ctx context.Context, orgID string) (int, error) {
	cfg, err := s.getDecryptedIONOSConfig(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("load ionos config: %w", err)
	}
	if cfg == nil || (cfg.Username == "" && cfg.Token == "") {
		return 0, fmt.Errorf("IONOS nicht konfiguriert")
	}
	collector := NewIONOSCollector(s.db, s.evidence)
	count, syncErr := collector.Collect(ctx, orgID, *cfg)
	status := "success"
	if syncErr != nil {
		status = "error"
	}
	if updateErr := s.repo.UpdateSyncResult(ctx, orgID, ProviderIONOS, status, syncErr); updateErr != nil {
		log.Warn().Err(updateErr).Str("org_id", orgID).Msg("cloud: failed to update IONOS sync result")
	}
	return count, syncErr
}

// GetIONOSStatus returns sync status for IONOS.
func (s *Service) GetIONOSStatus(ctx context.Context, orgID string) (*IONOSStatus, error) {
	ci, err := s.repo.GetIntegration(ctx, orgID, ProviderIONOS)
	if err != nil {
		return nil, err
	}
	st := &IONOSStatus{SyncStatus: SyncStatus{Provider: ProviderIONOS, Enabled: true}}
	if ci != nil {
		st.Enabled = ci.Enabled
		st.LastSyncAt = ci.LastSyncAt
		st.LastSyncStatus = ci.LastSyncStatus
		st.LastSyncError = ci.LastSyncError
	}
	count, _ := s.repo.CountEvidence(ctx, orgID, ionosSource)
	st.EvidenceCount = count
	return st, nil
}

// --- Wazuh ---

// GetWazuhConfig returns the Wazuh config for an org with secrets masked.
func (s *Service) GetWazuhConfig(ctx context.Context, orgID string) (*WazuhConfigResponse, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderWazuh)
	if err != nil {
		return nil, err
	}
	resp := &WazuhConfigResponse{}
	if raw == nil {
		return resp, nil
	}
	var stored map[string]any
	if err := json.Unmarshal(raw, &stored); err != nil {
		return resp, nil
	}
	if v, ok := stored["base_url"].(string); ok {
		resp.BaseURL = v
	}
	if v, ok := stored["username"].(string); ok {
		resp.Username = v
	}
	if v, ok := stored["password"].(string); ok && v != "" {
		resp.Password = "****"
	}
	if v, ok := stored["verify_tls"].(bool); ok {
		resp.VerifyTLS = v
	}
	resp.IsConfigured = resp.BaseURL != "" && resp.Username != ""
	return resp, nil
}

// SaveWazuhConfig persists the Wazuh config, encrypting password.
func (s *Service) SaveWazuhConfig(ctx context.Context, orgID string, in SaveWazuhConfigInput) error {
	if err := httputil.ValidateOutboundURL(in.BaseURL, in.AllowPrivateTarget); err != nil {
		return fmt.Errorf("wazuh base_url: %w", err)
	}
	existing, _ := s.getDecryptedWazuhConfig(ctx, orgID)

	pw := in.Password
	if pw == "****" && existing != nil {
		pw = existing.Password
	}

	encPw := ""
	if pw != "" {
		ct, err := sharedcrypto.Encrypt(s.masterKey, []byte(pw))
		if err != nil {
			return fmt.Errorf("encrypt wazuh password: %w", err)
		}
		encPw = hex.EncodeToString(ct)
	}

	return s.repo.UpsertConfig(ctx, orgID, ProviderWazuh, map[string]any{
		"base_url":   in.BaseURL,
		"username":   in.Username,
		"password":   encPw,
		"verify_tls": in.VerifyTLS,
	})
}

func (s *Service) getDecryptedWazuhConfig(ctx context.Context, orgID string) (*WazuhConfig, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderWazuh)
	if err != nil || raw == nil {
		return nil, err
	}
	var stored map[string]any
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, err
	}
	cfg := &WazuhConfig{}
	if v, ok := stored["base_url"].(string); ok {
		cfg.BaseURL = v
	}
	if v, ok := stored["username"].(string); ok {
		cfg.Username = v
	}
	if v, ok := stored["verify_tls"].(bool); ok {
		cfg.VerifyTLS = v
	}
	if encPw, ok := stored["password"].(string); ok && encPw != "" {
		ct, err := hex.DecodeString(encPw)
		if err != nil {
			return nil, fmt.Errorf("decode wazuh password: %w", err)
		}
		plain, err := sharedcrypto.Decrypt(s.masterKey, ct)
		if err != nil {
			return nil, fmt.Errorf("decrypt wazuh password: %w", err)
		}
		cfg.Password = string(plain)
	}
	return cfg, nil
}

// SyncWazuh runs the Wazuh collector and updates sync status.
func (s *Service) SyncWazuh(ctx context.Context, orgID string) (int, error) {
	cfg, err := s.getDecryptedWazuhConfig(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("load wazuh config: %w", err)
	}
	if cfg == nil || cfg.BaseURL == "" {
		return 0, fmt.Errorf("Wazuh nicht konfiguriert")
	}
	collector := NewWazuhPullCollector(s.db, s.evidence)
	count, syncErr := collector.Collect(ctx, orgID, *cfg)
	status := "success"
	if syncErr != nil {
		status = "error"
	}
	if updateErr := s.repo.UpdateSyncResult(ctx, orgID, ProviderWazuh, status, syncErr); updateErr != nil {
		log.Warn().Err(updateErr).Str("org_id", orgID).Msg("cloud: failed to update Wazuh sync result")
	}
	return count, syncErr
}

// GetWazuhStatus returns sync status and agent counts for Wazuh.
func (s *Service) GetWazuhStatus(ctx context.Context, orgID string) (*WazuhStatus, error) {
	ci, err := s.repo.GetIntegration(ctx, orgID, ProviderWazuh)
	if err != nil {
		return nil, err
	}
	st := &WazuhStatus{SyncStatus: SyncStatus{Provider: ProviderWazuh, Enabled: true}}
	if ci != nil {
		st.Enabled = ci.Enabled
		st.LastSyncAt = ci.LastSyncAt
		st.LastSyncStatus = ci.LastSyncStatus
		st.LastSyncError = ci.LastSyncError
	}
	count, _ := s.repo.CountEvidence(ctx, orgID, wazuhSource)
	st.EvidenceCount = count
	return st, nil
}

// --- Prometheus ---

// GetPrometheusConfig returns the Prometheus config for an org with secret masked.
func (s *Service) GetPrometheusConfig(ctx context.Context, orgID string) (*PrometheusConfigResponse, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderPrometheus)
	if err != nil {
		return nil, err
	}
	resp := &PrometheusConfigResponse{}
	if raw == nil {
		return resp, nil
	}
	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return resp, nil
	}
	resp.PrometheusURL = stored["prometheus_url"]
	resp.AlertmanagerURL = stored["alertmanager_url"]
	if stored["token"] != "" {
		resp.Token = "****"
	}
	resp.IsConfigured = resp.PrometheusURL != ""
	return resp, nil
}

// SavePrometheusConfig persists the Prometheus config, encrypting the token.
func (s *Service) SavePrometheusConfig(ctx context.Context, orgID string, in SavePrometheusConfigInput) error {
	if err := httputil.ValidateOutboundURL(in.PrometheusURL, in.AllowPrivateTarget); err != nil {
		return fmt.Errorf("prometheus_url: %w", err)
	}
	if in.AlertmanagerURL != "" {
		if err := httputil.ValidateOutboundURL(in.AlertmanagerURL, in.AllowPrivateTarget); err != nil {
			return fmt.Errorf("alertmanager_url: %w", err)
		}
	}
	existing, _ := s.getDecryptedPrometheusConfig(ctx, orgID)

	token := in.Token
	if token == "****" && existing != nil {
		token = existing.Token
	}

	encToken := ""
	if token != "" {
		ct, err := sharedcrypto.Encrypt(s.masterKey, []byte(token))
		if err != nil {
			return fmt.Errorf("encrypt prometheus token: %w", err)
		}
		encToken = hex.EncodeToString(ct)
	}

	return s.repo.UpsertConfig(ctx, orgID, ProviderPrometheus, map[string]any{
		"prometheus_url":   in.PrometheusURL,
		"alertmanager_url": in.AlertmanagerURL,
		"token":            encToken,
	})
}

func (s *Service) getDecryptedPrometheusConfig(ctx context.Context, orgID string) (*PrometheusConfig, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderPrometheus)
	if err != nil || raw == nil {
		return nil, err
	}
	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, err
	}
	cfg := &PrometheusConfig{
		PrometheusURL:   stored["prometheus_url"],
		AlertmanagerURL: stored["alertmanager_url"],
	}
	if stored["token"] != "" {
		ct, err := hex.DecodeString(stored["token"])
		if err != nil {
			return nil, fmt.Errorf("decode prometheus token: %w", err)
		}
		plain, err := sharedcrypto.Decrypt(s.masterKey, ct)
		if err != nil {
			return nil, fmt.Errorf("decrypt prometheus token: %w", err)
		}
		cfg.Token = string(plain)
	}
	return cfg, nil
}

// SyncPrometheus runs the Prometheus collector and updates sync status.
func (s *Service) SyncPrometheus(ctx context.Context, orgID string) (int, error) {
	cfg, err := s.getDecryptedPrometheusConfig(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("load prometheus config: %w", err)
	}
	if cfg == nil || cfg.PrometheusURL == "" {
		return 0, fmt.Errorf("Prometheus nicht konfiguriert")
	}
	collector := NewPrometheusCollector(s.db, s.evidence)
	count, syncErr := collector.Collect(ctx, orgID, *cfg)
	status := "success"
	if syncErr != nil {
		status = "error"
	}
	if updateErr := s.repo.UpdateSyncResult(ctx, orgID, ProviderPrometheus, status, syncErr); updateErr != nil {
		log.Warn().Err(updateErr).Str("org_id", orgID).Msg("cloud: failed to update Prometheus sync result")
	}
	return count, syncErr
}

// GetPrometheusStatus returns sync status and metric counts for Prometheus.
func (s *Service) GetPrometheusStatus(ctx context.Context, orgID string) (*PrometheusStatus, error) {
	ci, err := s.repo.GetIntegration(ctx, orgID, ProviderPrometheus)
	if err != nil {
		return nil, err
	}
	st := &PrometheusStatus{SyncStatus: SyncStatus{Provider: ProviderPrometheus, Enabled: true}}
	if ci != nil {
		st.Enabled = ci.Enabled
		st.LastSyncAt = ci.LastSyncAt
		st.LastSyncStatus = ci.LastSyncStatus
		st.LastSyncError = ci.LastSyncError
	}
	count, _ := s.repo.CountEvidence(ctx, orgID, prometheusSource)
	st.EvidenceCount = count
	return st, nil
}

// --- Entra ID ---

// GetEntraIDConfig returns the Entra ID config for an org with secrets masked.
func (s *Service) GetEntraIDConfig(ctx context.Context, orgID string) (*EntraIDConfigResponse, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderEntraID)
	if err != nil {
		return nil, err
	}
	resp := &EntraIDConfigResponse{}
	if raw == nil {
		return resp, nil
	}
	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return resp, nil
	}
	resp.TenantID = stored["tenant_id"]
	resp.ClientID = stored["client_id"]
	if stored["client_secret"] != "" {
		resp.ClientSecret = "****"
	}
	resp.IsConfigured = resp.TenantID != "" && resp.ClientID != "" && resp.ClientSecret == "****"
	return resp, nil
}

// SaveEntraIDConfig persists the Entra ID config, encrypting the client secret.
func (s *Service) SaveEntraIDConfig(ctx context.Context, orgID string, in SaveEntraIDConfigInput) error {
	existing, _ := s.getDecryptedEntraIDConfig(ctx, orgID)

	secret := in.ClientSecret
	if secret == "****" && existing != nil {
		secret = existing.ClientSecret
	}

	encSecret := ""
	if secret != "" {
		ct, err := sharedcrypto.Encrypt(s.masterKey, []byte(secret))
		if err != nil {
			return fmt.Errorf("encrypt entra id client secret: %w", err)
		}
		encSecret = hex.EncodeToString(ct)
	}

	return s.repo.UpsertConfig(ctx, orgID, ProviderEntraID, map[string]any{
		"tenant_id":     in.TenantID,
		"client_id":     in.ClientID,
		"client_secret": encSecret,
	})
}

func (s *Service) getDecryptedEntraIDConfig(ctx context.Context, orgID string) (*EntraIDConfig, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderEntraID)
	if err != nil || raw == nil {
		return nil, err
	}
	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, err
	}
	cfg := &EntraIDConfig{
		TenantID: stored["tenant_id"],
		ClientID: stored["client_id"],
	}
	if enc := stored["client_secret"]; enc != "" {
		ct, err := hex.DecodeString(enc)
		if err != nil {
			return nil, fmt.Errorf("decode entra id client secret: %w", err)
		}
		plain, err := sharedcrypto.Decrypt(s.masterKey, ct)
		if err != nil {
			return nil, fmt.Errorf("decrypt entra id client secret: %w", err)
		}
		cfg.ClientSecret = string(plain)
	}
	return cfg, nil
}

// SyncEntraID runs the Entra ID collector and updates sync status.
func (s *Service) SyncEntraID(ctx context.Context, orgID string) (int, error) {
	cfg, err := s.getDecryptedEntraIDConfig(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("load entra id config: %w", err)
	}
	if cfg == nil || cfg.TenantID == "" {
		return 0, fmt.Errorf("Entra ID nicht konfiguriert")
	}
	collector := NewEntraIDCollector(s.db, s.evidence)
	count, syncErr := collector.Collect(ctx, orgID, *cfg)
	status := "success"
	if syncErr != nil {
		status = "error"
	}
	if updateErr := s.repo.UpdateSyncResult(ctx, orgID, ProviderEntraID, status, syncErr); updateErr != nil {
		log.Warn().Err(updateErr).Str("org_id", orgID).Msg("cloud: failed to update Entra ID sync result")
	}
	return count, syncErr
}

// GetEntraIDStatus returns sync status for Entra ID.
func (s *Service) GetEntraIDStatus(ctx context.Context, orgID string) (*EntraIDStatus, error) {
	ci, err := s.repo.GetIntegration(ctx, orgID, ProviderEntraID)
	if err != nil {
		return nil, err
	}
	st := &EntraIDStatus{SyncStatus: SyncStatus{Provider: ProviderEntraID, Enabled: true}}
	if ci != nil {
		st.Enabled = ci.Enabled
		st.LastSyncAt = ci.LastSyncAt
		st.LastSyncStatus = ci.LastSyncStatus
		st.LastSyncError = ci.LastSyncError
	}
	count, _ := s.repo.CountEvidence(ctx, orgID, entraidSource)
	st.EvidenceCount = count
	return st, nil
}

// --- Intune (S88-7) ---

const intuneGraphBaseURL = "https://graph.microsoft.com"

func (s *Service) GetIntuneConfig(ctx context.Context, orgID string) (*IntuneConfigResponse, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderIntune)
	if err != nil {
		return nil, err
	}
	resp := &IntuneConfigResponse{}
	if raw == nil {
		return resp, nil
	}
	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return resp, nil
	}
	resp.TenantID = stored["tenant_id"]
	resp.ClientID = stored["client_id"]
	if stored["client_secret"] != "" {
		resp.ClientSecret = "****"
	}
	resp.IsConfigured = resp.TenantID != "" && resp.ClientID != "" && resp.ClientSecret == "****"
	return resp, nil
}

// SaveIntuneConfig persists the Intune config, encrypting the client secret.
// The fixed Microsoft Graph endpoint is validated through the same outbound
// SSRF guard every collector uses (S88-7 AC), so a future endpoint override can
// never point at a private/IMDS address.
func (s *Service) SaveIntuneConfig(ctx context.Context, orgID string, in SaveIntuneConfigInput) error {
	if err := httputil.ValidateOutboundURL(intuneGraphBaseURL, false); err != nil {
		return fmt.Errorf("intune graph endpoint rejected by outbound guard: %w", err)
	}
	existing, _ := s.getDecryptedIntuneConfig(ctx, orgID)
	secret := in.ClientSecret
	if secret == "****" && existing != nil {
		secret = existing.ClientSecret
	}
	encSecret := ""
	if secret != "" {
		ct, err := sharedcrypto.Encrypt(s.masterKey, []byte(secret))
		if err != nil {
			return fmt.Errorf("encrypt intune client secret: %w", err)
		}
		encSecret = hex.EncodeToString(ct)
	}
	return s.repo.UpsertConfig(ctx, orgID, ProviderIntune, map[string]any{
		"tenant_id":     in.TenantID,
		"client_id":     in.ClientID,
		"client_secret": encSecret,
	})
}

func (s *Service) getDecryptedIntuneConfig(ctx context.Context, orgID string) (*IntuneConfig, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderIntune)
	if err != nil || raw == nil {
		return nil, err
	}
	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, err
	}
	cfg := &IntuneConfig{TenantID: stored["tenant_id"], ClientID: stored["client_id"]}
	if enc := stored["client_secret"]; enc != "" {
		ct, err := hex.DecodeString(enc)
		if err != nil {
			return nil, fmt.Errorf("decode intune client secret: %w", err)
		}
		plain, err := sharedcrypto.Decrypt(s.masterKey, ct)
		if err != nil {
			return nil, fmt.Errorf("decrypt intune client secret: %w", err)
		}
		cfg.ClientSecret = string(plain)
	}
	return cfg, nil
}

// SyncIntune runs the Intune collector and updates sync status.
func (s *Service) SyncIntune(ctx context.Context, orgID string) (int, error) {
	cfg, err := s.getDecryptedIntuneConfig(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("load intune config: %w", err)
	}
	if cfg == nil || cfg.TenantID == "" {
		return 0, fmt.Errorf("Intune nicht konfiguriert")
	}
	collector := NewIntuneCollector(s.db, s.evidence)
	count, syncErr := collector.Collect(ctx, orgID, *cfg)
	status := "success"
	if syncErr != nil {
		status = "error"
	}
	if updateErr := s.repo.UpdateSyncResult(ctx, orgID, ProviderIntune, status, syncErr); updateErr != nil {
		log.Warn().Err(updateErr).Str("org_id", orgID).Msg("cloud: failed to update Intune sync result")
	}
	return count, syncErr
}

// GetIntuneStatus returns sync status for Intune.
func (s *Service) GetIntuneStatus(ctx context.Context, orgID string) (*IntuneStatus, error) {
	ci, err := s.repo.GetIntegration(ctx, orgID, ProviderIntune)
	if err != nil {
		return nil, err
	}
	st := &IntuneStatus{SyncStatus: SyncStatus{Provider: ProviderIntune, Enabled: true}}
	if ci != nil {
		st.Enabled = ci.Enabled
		st.LastSyncAt = ci.LastSyncAt
		st.LastSyncStatus = ci.LastSyncStatus
		st.LastSyncError = ci.LastSyncError
	}
	count, _ := s.repo.CountEvidence(ctx, orgID, intuneSource)
	st.EvidenceCount = count
	return st, nil
}

// --- Keycloak ---

// GetKeycloakConfig returns the Keycloak config for an org with secrets masked.
func (s *Service) GetKeycloakConfig(ctx context.Context, orgID string) (*KeycloakConfigResponse, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderKeycloak)
	if err != nil {
		return nil, err
	}
	resp := &KeycloakConfigResponse{}
	if raw == nil {
		return resp, nil
	}
	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return resp, nil
	}
	resp.KeycloakURL = stored["keycloak_url"]
	resp.Realm = stored["realm"]
	resp.ClientID = stored["client_id"]
	if stored["client_secret"] != "" {
		resp.ClientSecret = "****"
	}
	resp.IsConfigured = resp.KeycloakURL != "" && resp.Realm != "" && resp.ClientID != "" && resp.ClientSecret == "****"
	return resp, nil
}

// SaveKeycloakConfig persists the Keycloak config, encrypting the client secret.
func (s *Service) SaveKeycloakConfig(ctx context.Context, orgID string, in SaveKeycloakConfigInput) error {
	if err := httputil.ValidateOutboundURL(in.KeycloakURL, in.AllowPrivateTarget); err != nil {
		return fmt.Errorf("keycloak_url: %w", err)
	}
	existing, _ := s.getDecryptedKeycloakConfig(ctx, orgID)

	secret := in.ClientSecret
	if secret == "****" && existing != nil {
		secret = existing.ClientSecret
	}

	encSecret := ""
	if secret != "" {
		ct, err := sharedcrypto.Encrypt(s.masterKey, []byte(secret))
		if err != nil {
			return fmt.Errorf("encrypt keycloak client secret: %w", err)
		}
		encSecret = hex.EncodeToString(ct)
	}

	return s.repo.UpsertConfig(ctx, orgID, ProviderKeycloak, map[string]any{
		"keycloak_url":  in.KeycloakURL,
		"realm":         in.Realm,
		"client_id":     in.ClientID,
		"client_secret": encSecret,
	})
}

func (s *Service) getDecryptedKeycloakConfig(ctx context.Context, orgID string) (*KeycloakConfig, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderKeycloak)
	if err != nil || raw == nil {
		return nil, err
	}
	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, err
	}
	cfg := &KeycloakConfig{
		KeycloakURL: stored["keycloak_url"],
		Realm:       stored["realm"],
		ClientID:    stored["client_id"],
	}
	if enc := stored["client_secret"]; enc != "" {
		ct, err := hex.DecodeString(enc)
		if err != nil {
			return nil, fmt.Errorf("decode keycloak client secret: %w", err)
		}
		plain, err := sharedcrypto.Decrypt(s.masterKey, ct)
		if err != nil {
			return nil, fmt.Errorf("decrypt keycloak client secret: %w", err)
		}
		cfg.ClientSecret = string(plain)
	}
	return cfg, nil
}

// SyncKeycloak runs the Keycloak collector and updates sync status.
func (s *Service) SyncKeycloak(ctx context.Context, orgID string) (int, error) {
	cfg, err := s.getDecryptedKeycloakConfig(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("load keycloak config: %w", err)
	}
	if cfg == nil || cfg.KeycloakURL == "" {
		return 0, fmt.Errorf("Keycloak nicht konfiguriert")
	}
	collector := NewKeycloakCollector(s.db, s.evidence)
	count, syncErr := collector.Collect(ctx, orgID, *cfg)
	status := "success"
	if syncErr != nil {
		status = "error"
	}
	if updateErr := s.repo.UpdateSyncResult(ctx, orgID, ProviderKeycloak, status, syncErr); updateErr != nil {
		log.Warn().Err(updateErr).Str("org_id", orgID).Msg("cloud: failed to update Keycloak sync result")
	}
	return count, syncErr
}

// GetKeycloakStatus returns sync status for Keycloak.
func (s *Service) GetKeycloakStatus(ctx context.Context, orgID string) (*KeycloakStatus, error) {
	ci, err := s.repo.GetIntegration(ctx, orgID, ProviderKeycloak)
	if err != nil {
		return nil, err
	}
	st := &KeycloakStatus{SyncStatus: SyncStatus{Provider: ProviderKeycloak, Enabled: true}}
	if ci != nil {
		st.Enabled = ci.Enabled
		st.LastSyncAt = ci.LastSyncAt
		st.LastSyncStatus = ci.LastSyncStatus
		st.LastSyncError = ci.LastSyncError
	}
	count, _ := s.repo.CountEvidence(ctx, orgID, keycloakSource)
	st.EvidenceCount = count
	return st, nil
}

// --- LDAP ---

// GetLDAPConfig returns the LDAP config for an org with secrets masked.
func (s *Service) GetLDAPConfig(ctx context.Context, orgID string) (*LDAPConfigResponse, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderLDAP)
	if err != nil {
		return nil, err
	}
	resp := &LDAPConfigResponse{}
	if raw == nil {
		return resp, nil
	}
	var stored map[string]any
	if err := json.Unmarshal(raw, &stored); err != nil {
		return resp, nil
	}
	if v, ok := stored["host"].(string); ok {
		resp.Host = v
	}
	if v, ok := stored["port"].(float64); ok {
		resp.Port = int(v)
	}
	if v, ok := stored["bind_dn"].(string); ok {
		resp.BindDN = v
	}
	if v, ok := stored["base_dn"].(string); ok {
		resp.BaseDN = v
	}
	if v, ok := stored["use_tls"].(bool); ok {
		resp.UseTLS = v
	}
	if v, ok := stored["is_active_directory"].(bool); ok {
		resp.IsActiveDirectory = v
	}
	if v, ok := stored["privileged_groups"].([]any); ok {
		for _, g := range v {
			if gs, ok := g.(string); ok {
				resp.PrivilegedGroups = append(resp.PrivilegedGroups, gs)
			}
		}
	}
	if enc, ok := stored["bind_password"].(string); ok && enc != "" {
		resp.BindPassword = "****"
	}
	resp.IsConfigured = resp.Host != "" && resp.BindDN != "" && resp.BaseDN != "" && resp.BindPassword == "****"
	return resp, nil
}

// SaveLDAPConfig persists the LDAP config, encrypting the bind password.
func (s *Service) SaveLDAPConfig(ctx context.Context, orgID string, in SaveLDAPConfigInput) error {
	existing, _ := s.getDecryptedLDAPConfig(ctx, orgID)

	pw := in.BindPassword
	if pw == "****" && existing != nil {
		pw = existing.BindPassword
	}

	encPW := ""
	if pw != "" {
		ct, err := sharedcrypto.Encrypt(s.masterKey, []byte(pw))
		if err != nil {
			return fmt.Errorf("encrypt ldap bind password: %w", err)
		}
		encPW = hex.EncodeToString(ct)
	}

	groups := in.PrivilegedGroups
	if len(groups) == 0 {
		groups = []string{"Domain Admins", "Administrators"}
	}

	return s.repo.UpsertConfig(ctx, orgID, ProviderLDAP, map[string]any{
		"host":                in.Host,
		"port":                in.Port,
		"bind_dn":             in.BindDN,
		"bind_password":       encPW,
		"base_dn":             in.BaseDN,
		"use_tls":             in.UseTLS,
		"is_active_directory": in.IsActiveDirectory,
		"privileged_groups":   groups,
	})
}

func (s *Service) getDecryptedLDAPConfig(ctx context.Context, orgID string) (*LDAPConfig, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderLDAP)
	if err != nil || raw == nil {
		return nil, err
	}
	var stored map[string]any
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, err
	}
	cfg := &LDAPConfig{}
	if v, ok := stored["host"].(string); ok {
		cfg.Host = v
	}
	if v, ok := stored["port"].(float64); ok {
		cfg.Port = int(v)
	}
	if v, ok := stored["bind_dn"].(string); ok {
		cfg.BindDN = v
	}
	if v, ok := stored["base_dn"].(string); ok {
		cfg.BaseDN = v
	}
	if v, ok := stored["use_tls"].(bool); ok {
		cfg.UseTLS = v
	}
	if v, ok := stored["is_active_directory"].(bool); ok {
		cfg.IsActiveDirectory = v
	}
	if v, ok := stored["privileged_groups"].([]any); ok {
		for _, g := range v {
			if gs, ok := g.(string); ok {
				cfg.PrivilegedGroups = append(cfg.PrivilegedGroups, gs)
			}
		}
	}
	if enc, ok := stored["bind_password"].(string); ok && enc != "" {
		ct, decErr := hex.DecodeString(enc)
		if decErr != nil {
			return nil, fmt.Errorf("decode ldap bind password: %w", decErr)
		}
		plain, decErr := sharedcrypto.Decrypt(s.masterKey, ct)
		if decErr != nil {
			return nil, fmt.Errorf("decrypt ldap bind password: %w", decErr)
		}
		cfg.BindPassword = string(plain)
	}
	return cfg, nil
}

// SyncLDAP runs the LDAP collector and updates sync status.
func (s *Service) SyncLDAP(ctx context.Context, orgID string) (int, error) {
	cfg, err := s.getDecryptedLDAPConfig(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("load ldap config: %w", err)
	}
	if cfg == nil || cfg.Host == "" {
		return 0, fmt.Errorf("LDAP nicht konfiguriert")
	}
	collector := NewLDAPEvidenceCollector(s.db, s.evidence)
	count, syncErr := collector.Collect(ctx, orgID, *cfg)
	status := "success"
	if syncErr != nil {
		status = "error"
	}
	if updateErr := s.repo.UpdateSyncResult(ctx, orgID, ProviderLDAP, status, syncErr); updateErr != nil {
		log.Warn().Err(updateErr).Str("org_id", orgID).Msg("cloud: failed to update LDAP sync result")
	}
	return count, syncErr
}

// GetLDAPStatus returns sync status for LDAP.
func (s *Service) GetLDAPStatus(ctx context.Context, orgID string) (*LDAPStatus, error) {
	ci, err := s.repo.GetIntegration(ctx, orgID, ProviderLDAP)
	if err != nil {
		return nil, err
	}
	st := &LDAPStatus{SyncStatus: SyncStatus{Provider: ProviderLDAP, Enabled: true}}
	if ci != nil {
		st.Enabled = ci.Enabled
		st.LastSyncAt = ci.LastSyncAt
		st.LastSyncStatus = ci.LastSyncStatus
		st.LastSyncError = ci.LastSyncError
	}
	count, _ := s.repo.CountEvidence(ctx, orgID, ldapSource)
	st.EvidenceCount = count
	return st, nil
}

// --- GitLab ---

// GetGitLabConfig returns the GitLab config for an org with secrets masked.
func (s *Service) GetGitLabConfig(ctx context.Context, orgID string) (*GitLabConfigResponse, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderGitLab)
	if err != nil {
		return nil, err
	}
	resp := &GitLabConfigResponse{}
	if raw == nil {
		return resp, nil
	}
	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return resp, nil
	}
	resp.GitLabURL = stored["gitlab_url"]
	resp.GroupID = stored["group_id"]
	if stored["access_token"] != "" {
		resp.AccessToken = "****"
	}
	resp.IsConfigured = resp.GitLabURL != "" && resp.AccessToken == "****"
	return resp, nil
}

// SaveGitLabConfig persists the GitLab config, encrypting the access token.
func (s *Service) SaveGitLabConfig(ctx context.Context, orgID string, in SaveGitLabConfigInput) error {
	if err := httputil.ValidateOutboundURL(in.GitLabURL, in.AllowPrivateTarget); err != nil {
		return fmt.Errorf("gitlab_url: %w", err)
	}
	existing, _ := s.getDecryptedGitLabConfig(ctx, orgID)

	token := in.AccessToken
	if token == "****" && existing != nil {
		token = existing.AccessToken
	}

	encToken := ""
	if token != "" {
		ct, err := sharedcrypto.Encrypt(s.masterKey, []byte(token))
		if err != nil {
			return fmt.Errorf("encrypt gitlab token: %w", err)
		}
		encToken = hex.EncodeToString(ct)
	}

	return s.repo.UpsertConfig(ctx, orgID, ProviderGitLab, map[string]any{
		"gitlab_url":   in.GitLabURL,
		"access_token": encToken,
		"group_id":     in.GroupID,
	})
}

func (s *Service) getDecryptedGitLabConfig(ctx context.Context, orgID string) (*GitLabConfig, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderGitLab)
	if err != nil || raw == nil {
		return nil, err
	}
	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, err
	}
	cfg := &GitLabConfig{
		GitLabURL: stored["gitlab_url"],
		GroupID:   stored["group_id"],
	}
	if stored["access_token"] != "" {
		ct, err := hex.DecodeString(stored["access_token"])
		if err != nil {
			return nil, fmt.Errorf("decode gitlab token: %w", err)
		}
		plain, err := sharedcrypto.Decrypt(s.masterKey, ct)
		if err != nil {
			return nil, fmt.Errorf("decrypt gitlab token: %w", err)
		}
		cfg.AccessToken = string(plain)
	}
	return cfg, nil
}

// SyncGitLab runs the GitLab collector and updates sync status.
func (s *Service) SyncGitLab(ctx context.Context, orgID string) (int, error) {
	cfg, err := s.getDecryptedGitLabConfig(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("load gitlab config: %w", err)
	}
	if cfg == nil || cfg.GitLabURL == "" {
		return 0, fmt.Errorf("GitLab nicht konfiguriert")
	}
	collector := NewGitLabCollector(s.db, s.evidence)
	count, syncErr := collector.Collect(ctx, orgID, *cfg)
	status := "success"
	if syncErr != nil {
		status = "error"
	}
	if updateErr := s.repo.UpdateSyncResult(ctx, orgID, ProviderGitLab, status, syncErr); updateErr != nil {
		log.Warn().Err(updateErr).Str("org_id", orgID).Msg("cloud: failed to update GitLab sync result")
	}
	return count, syncErr
}

// GetGitLabStatus returns sync status and project metrics for GitLab.
func (s *Service) GetGitLabStatus(ctx context.Context, orgID string) (*GitLabStatus, error) {
	ci, err := s.repo.GetIntegration(ctx, orgID, ProviderGitLab)
	if err != nil {
		return nil, err
	}
	st := &GitLabStatus{SyncStatus: SyncStatus{Provider: ProviderGitLab, Enabled: true}}
	if ci != nil {
		st.Enabled = ci.Enabled
		st.LastSyncAt = ci.LastSyncAt
		st.LastSyncStatus = ci.LastSyncStatus
		st.LastSyncError = ci.LastSyncError
	}
	count, _ := s.repo.CountEvidence(ctx, orgID, gitlabSource)
	st.EvidenceCount = count
	return st, nil
}

// --- SonarQube ---

// GetSonarQubeConfig returns the SonarQube config for an org with secrets masked.
func (s *Service) GetSonarQubeConfig(ctx context.Context, orgID string) (*SonarQubeConfigResponse, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderSonarQube)
	if err != nil {
		return nil, err
	}
	resp := &SonarQubeConfigResponse{}
	if raw == nil {
		return resp, nil
	}
	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return resp, nil
	}
	resp.BaseURL = stored["base_url"]
	if stored["token"] != "" {
		resp.Token = "****"
	}
	resp.IsConfigured = resp.BaseURL != "" && resp.Token == "****"
	return resp, nil
}

// SaveSonarQubeConfig persists the SonarQube config, encrypting the token.
func (s *Service) SaveSonarQubeConfig(ctx context.Context, orgID string, in SaveSonarQubeConfigInput) error {
	if err := httputil.ValidateOutboundURL(in.BaseURL, in.AllowPrivateTarget); err != nil {
		return fmt.Errorf("sonarqube base_url: %w", err)
	}
	existing, _ := s.getDecryptedSonarQubeConfig(ctx, orgID)

	token := in.Token
	if token == "****" && existing != nil {
		token = existing.Token
	}

	encToken := ""
	if token != "" {
		ct, err := sharedcrypto.Encrypt(s.masterKey, []byte(token))
		if err != nil {
			return fmt.Errorf("encrypt sonarqube token: %w", err)
		}
		encToken = hex.EncodeToString(ct)
	}

	return s.repo.UpsertConfig(ctx, orgID, ProviderSonarQube, map[string]any{
		"base_url": in.BaseURL,
		"token":    encToken,
	})
}

func (s *Service) getDecryptedSonarQubeConfig(ctx context.Context, orgID string) (*SonarQubeConfig, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderSonarQube)
	if err != nil || raw == nil {
		return nil, err
	}
	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, err
	}
	cfg := &SonarQubeConfig{BaseURL: stored["base_url"]}
	if stored["token"] != "" {
		ct, err := hex.DecodeString(stored["token"])
		if err != nil {
			return nil, fmt.Errorf("decode sonarqube token: %w", err)
		}
		plain, err := sharedcrypto.Decrypt(s.masterKey, ct)
		if err != nil {
			return nil, fmt.Errorf("decrypt sonarqube token: %w", err)
		}
		cfg.Token = string(plain)
	}
	return cfg, nil
}

// SyncSonarQube runs the SonarQube collector and updates sync status.
func (s *Service) SyncSonarQube(ctx context.Context, orgID string) (int, error) {
	cfg, err := s.getDecryptedSonarQubeConfig(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("load sonarqube config: %w", err)
	}
	if cfg == nil || cfg.BaseURL == "" {
		return 0, fmt.Errorf("SonarQube nicht konfiguriert")
	}
	collector := NewSonarQubeCollector(s.db, s.evidence)
	count, syncErr := collector.Collect(ctx, orgID, *cfg)
	status := "success"
	if syncErr != nil {
		status = "error"
	}
	if updateErr := s.repo.UpdateSyncResult(ctx, orgID, ProviderSonarQube, status, syncErr); updateErr != nil {
		log.Warn().Err(updateErr).Str("org_id", orgID).Msg("cloud: failed to update SonarQube sync result")
	}
	return count, syncErr
}

// GetSonarQubeStatus returns sync status and quality gate metrics for SonarQube.
func (s *Service) GetSonarQubeStatus(ctx context.Context, orgID string) (*SonarQubeStatus, error) {
	ci, err := s.repo.GetIntegration(ctx, orgID, ProviderSonarQube)
	if err != nil {
		return nil, err
	}
	st := &SonarQubeStatus{SyncStatus: SyncStatus{Provider: ProviderSonarQube, Enabled: true}}
	if ci != nil {
		st.Enabled = ci.Enabled
		st.LastSyncAt = ci.LastSyncAt
		st.LastSyncStatus = ci.LastSyncStatus
		st.LastSyncError = ci.LastSyncError
	}
	count, _ := s.repo.CountEvidence(ctx, orgID, sonarqubeSource)
	st.EvidenceCount = count
	return st, nil
}

// --- Personio ---

// GetPersonioConfig returns the Personio webhook config with secret masked.
func (s *Service) GetPersonioConfig(ctx context.Context, orgID string) (*PersonioConfigResponse, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderPersonio)
	if err != nil {
		return nil, err
	}
	resp := &PersonioConfigResponse{}
	if raw == nil {
		return resp, nil
	}
	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return resp, nil
	}
	if stored["webhook_secret"] != "" {
		resp.WebhookSecret = "****"
	}
	resp.IsConfigured = resp.WebhookSecret == "****"
	return resp, nil
}

// SavePersonioConfig persists the Personio webhook secret, encrypted.
func (s *Service) SavePersonioConfig(ctx context.Context, orgID string, in SavePersonioConfigInput) error {
	existing, _ := s.getDecryptedPersonioConfig(ctx, orgID)

	secret := in.WebhookSecret
	if secret == "****" && existing != nil {
		secret = existing.WebhookSecret
	}

	encSecret := ""
	if secret != "" {
		ct, err := sharedcrypto.Encrypt(s.masterKey, []byte(secret))
		if err != nil {
			return fmt.Errorf("encrypt personio webhook secret: %w", err)
		}
		encSecret = hex.EncodeToString(ct)
	}

	return s.repo.UpsertConfig(ctx, orgID, ProviderPersonio, map[string]any{
		"webhook_secret": encSecret,
	})
}

// GetDecryptedPersonioSecret returns the plaintext webhook secret for HMAC verification.
// Exported so the vakthr webhook handler can verify Personio signatures.
func (s *Service) GetDecryptedPersonioSecret(ctx context.Context, orgID string) (string, error) {
	cfg, err := s.getDecryptedPersonioConfig(ctx, orgID)
	if err != nil {
		return "", err
	}
	if cfg == nil {
		return "", nil
	}
	return cfg.WebhookSecret, nil
}

func (s *Service) getDecryptedPersonioConfig(ctx context.Context, orgID string) (*PersonioConfig, error) {
	raw, err := s.repo.GetConfig(ctx, orgID, ProviderPersonio)
	if err != nil || raw == nil {
		return nil, err
	}
	var stored map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, err
	}
	cfg := &PersonioConfig{}
	if stored["webhook_secret"] != "" {
		ct, err := hex.DecodeString(stored["webhook_secret"])
		if err != nil {
			return nil, fmt.Errorf("decode personio webhook secret: %w", err)
		}
		plain, err := sharedcrypto.Decrypt(s.masterKey, ct)
		if err != nil {
			return nil, fmt.Errorf("decrypt personio webhook secret: %w", err)
		}
		cfg.WebhookSecret = string(plain)
	}
	return cfg, nil
}

// GetPersonioStatus returns status for the Personio integration.
func (s *Service) GetPersonioStatus(ctx context.Context, orgID string) (*PersonioStatus, error) {
	ci, err := s.repo.GetIntegration(ctx, orgID, ProviderPersonio)
	if err != nil {
		return nil, err
	}
	st := &PersonioStatus{SyncStatus: SyncStatus{Provider: ProviderPersonio, Enabled: true}}
	if ci != nil {
		st.Enabled = ci.Enabled
		st.LastSyncAt = ci.LastSyncAt
		st.LastSyncStatus = ci.LastSyncStatus
	}

	cfg, _ := s.GetPersonioConfig(ctx, orgID)
	st.WebhookConfigured = cfg != nil && cfg.IsConfigured

	// Count employees with personio_employee_id as proxy for triggered offboardings
	var triggered int
	_ = s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM hr_employees
		WHERE org_id = $1::uuid AND personio_employee_id IS NOT NULL`, orgID,
	).Scan(&triggered)
	st.OffboardingsTriggered = triggered

	return st, nil
}

// RecordPersonioWebhook updates last_sync_at for the personio integration (push-event timestamp).
func (s *Service) RecordPersonioWebhook(ctx context.Context, orgID string) error {
	return s.repo.UpdateSyncResult(ctx, orgID, ProviderPersonio, "success", nil)
}

// --- Scheduled sync (all enabled integrations) ---

// SyncAllEnabled runs evidence collection for all enabled cloud integrations.
// Used by the daily Asynq job.
func (s *Service) SyncAllEnabled(ctx context.Context) error {
	integrations, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return fmt.Errorf("list enabled cloud integrations: %w", err)
	}

	for _, ci := range integrations {
		switch ci.Provider {
		case ProviderAWS:
			if _, syncErr := s.SyncAWS(ctx, ci.OrgID); syncErr != nil {
				log.Error().Err(syncErr).Str("org_id", ci.OrgID).Msg("cloud sync: aws failed")
			} else {
				log.Info().Str("org_id", ci.OrgID).Msg("cloud sync: aws completed")
			}
		case ProviderAzure:
			if _, syncErr := s.SyncAzure(ctx, ci.OrgID); syncErr != nil {
				log.Error().Err(syncErr).Str("org_id", ci.OrgID).Msg("cloud sync: azure failed")
			} else {
				log.Info().Str("org_id", ci.OrgID).Msg("cloud sync: azure completed")
			}
		case ProviderHetzner:
			if _, syncErr := s.SyncHetzner(ctx, ci.OrgID); syncErr != nil {
				log.Error().Err(syncErr).Str("org_id", ci.OrgID).Msg("cloud sync: hetzner failed")
			} else {
				log.Info().Str("org_id", ci.OrgID).Msg("cloud sync: hetzner completed")
			}
		case ProviderIONOS:
			if _, syncErr := s.SyncIONOS(ctx, ci.OrgID); syncErr != nil {
				log.Error().Err(syncErr).Str("org_id", ci.OrgID).Msg("cloud sync: ionos failed")
			} else {
				log.Info().Str("org_id", ci.OrgID).Msg("cloud sync: ionos completed")
			}
		case ProviderWazuh:
			if _, syncErr := s.SyncWazuh(ctx, ci.OrgID); syncErr != nil {
				log.Error().Err(syncErr).Str("org_id", ci.OrgID).Msg("cloud sync: wazuh failed")
			} else {
				log.Info().Str("org_id", ci.OrgID).Msg("cloud sync: wazuh completed")
			}
		case ProviderPrometheus:
			if _, syncErr := s.SyncPrometheus(ctx, ci.OrgID); syncErr != nil {
				log.Error().Err(syncErr).Str("org_id", ci.OrgID).Msg("cloud sync: prometheus failed")
			} else {
				log.Info().Str("org_id", ci.OrgID).Msg("cloud sync: prometheus completed")
			}
		case ProviderEntraID:
			if _, syncErr := s.SyncEntraID(ctx, ci.OrgID); syncErr != nil {
				log.Error().Err(syncErr).Str("org_id", ci.OrgID).Msg("cloud sync: entra id failed")
			} else {
				log.Info().Str("org_id", ci.OrgID).Msg("cloud sync: entra id completed")
			}
		case ProviderIntune:
			if _, syncErr := s.SyncIntune(ctx, ci.OrgID); syncErr != nil {
				log.Error().Err(syncErr).Str("org_id", ci.OrgID).Msg("cloud sync: intune failed")
			} else {
				log.Info().Str("org_id", ci.OrgID).Msg("cloud sync: intune completed")
			}
		case ProviderKeycloak:
			if _, syncErr := s.SyncKeycloak(ctx, ci.OrgID); syncErr != nil {
				log.Error().Err(syncErr).Str("org_id", ci.OrgID).Msg("cloud sync: keycloak failed")
			} else {
				log.Info().Str("org_id", ci.OrgID).Msg("cloud sync: keycloak completed")
			}
		case ProviderLDAP:
			if _, syncErr := s.SyncLDAP(ctx, ci.OrgID); syncErr != nil {
				log.Error().Err(syncErr).Str("org_id", ci.OrgID).Msg("cloud sync: ldap failed")
			} else {
				log.Info().Str("org_id", ci.OrgID).Msg("cloud sync: ldap completed")
			}
		case ProviderGitLab:
			if _, syncErr := s.SyncGitLab(ctx, ci.OrgID); syncErr != nil {
				log.Error().Err(syncErr).Str("org_id", ci.OrgID).Msg("cloud sync: gitlab failed")
			} else {
				log.Info().Str("org_id", ci.OrgID).Msg("cloud sync: gitlab completed")
			}
		case ProviderSonarQube:
			if _, syncErr := s.SyncSonarQube(ctx, ci.OrgID); syncErr != nil {
				log.Error().Err(syncErr).Str("org_id", ci.OrgID).Msg("cloud sync: sonarqube failed")
			} else {
				log.Info().Str("org_id", ci.OrgID).Msg("cloud sync: sonarqube completed")
			}
			// Personio is push-only — no daily pull sync
		}
	}
	return nil
}

// RecentEvidence returns up to 5 recent evidence items for a provider.
func (s *Service) RecentEvidence(ctx context.Context, orgID, provider string) ([]EvidenceItem, error) {
	sourceMap := map[string]string{
		ProviderAWS:        awsSource,
		ProviderAzure:      azureSource,
		ProviderHetzner:    hetznerSource,
		ProviderIONOS:      ionosSource,
		ProviderWazuh:      wazuhSource,
		ProviderPrometheus: prometheusSource,
		ProviderEntraID:    entraidSource,
		ProviderIntune:     intuneSource,
		ProviderKeycloak:   keycloakSource,
		ProviderLDAP:       ldapSource,
		ProviderGitLab:     gitlabSource,
		ProviderSonarQube:  sonarqubeSource,
	}
	source, ok := sourceMap[provider]
	if !ok {
		source = awsSource
	}
	return s.repo.RecentEvidence(ctx, orgID, source, 5)
}
