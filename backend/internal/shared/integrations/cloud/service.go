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
		case "aws":
			if _, syncErr := s.SyncAWS(ctx, ci.OrgID); syncErr != nil {
				log.Error().Err(syncErr).Str("org_id", ci.OrgID).Msg("cloud sync: aws failed")
			} else {
				log.Info().Str("org_id", ci.OrgID).Msg("cloud sync: aws completed")
			}
		case "azure":
			if _, syncErr := s.SyncAzure(ctx, ci.OrgID); syncErr != nil {
				log.Error().Err(syncErr).Str("org_id", ci.OrgID).Msg("cloud sync: azure failed")
			} else {
				log.Info().Str("org_id", ci.OrgID).Msg("cloud sync: azure completed")
			}
		}
	}
	return nil
}

// RecentEvidence returns up to 5 recent evidence items for a provider.
func (s *Service) RecentEvidence(ctx context.Context, orgID, provider string) ([]EvidenceItem, error) {
	source := awsSource
	if provider == "azure" {
		source = azureSource
	}
	return s.repo.RecentEvidence(ctx, orgID, source, 5)
}
