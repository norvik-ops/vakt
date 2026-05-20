// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package cloud provides automated evidence collection from AWS and Azure cloud accounts.
package cloud

import "time"

// AWSConfig holds AWS credentials and region for evidence collection.
// SecretAccessKey is stored encrypted (AES-256-GCM) inside cloud_integrations.config JSONB.
type AWSConfig struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"` // stored encrypted
	Region          string `json:"region"`
	AccountID       string `json:"account_id"`
}

// AzureConfig holds Azure service principal credentials for evidence collection.
// ClientSecret is stored encrypted (AES-256-GCM) inside cloud_integrations.config JSONB.
type AzureConfig struct {
	TenantID       string `json:"tenant_id"`
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"` // stored encrypted
	SubscriptionID string `json:"subscription_id"`
}

// CloudIntegration represents a row from the cloud_integrations table.
type CloudIntegration struct {
	ID             string     `json:"id"`
	OrgID          string     `json:"org_id"`
	Provider       string     `json:"provider"` // "aws" | "azure"
	Enabled        bool       `json:"enabled"`
	LastSyncAt     *time.Time `json:"last_sync_at,omitempty"`
	LastSyncStatus *string    `json:"last_sync_status,omitempty"`
	LastSyncError  *string    `json:"last_sync_error,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// SaveAWSConfigInput is the validated HTTP input for saving AWS config.
type SaveAWSConfigInput struct {
	AccessKeyID     string `json:"access_key_id"     validate:"required"`
	SecretAccessKey string `json:"secret_access_key" validate:"required"`
	Region          string `json:"region"            validate:"required"`
	AccountID       string `json:"account_id"`
}

// SaveAzureConfigInput is the validated HTTP input for saving Azure config.
type SaveAzureConfigInput struct {
	TenantID       string `json:"tenant_id"       validate:"required"`
	ClientID       string `json:"client_id"       validate:"required"`
	ClientSecret   string `json:"client_secret"   validate:"required"`
	SubscriptionID string `json:"subscription_id" validate:"required"`
}

// AWSConfigResponse is returned from GET /config (secrets masked).
type AWSConfigResponse struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"` // "****" if set
	Region          string `json:"region"`
	AccountID       string `json:"account_id"`
	IsConfigured    bool   `json:"is_configured"`
}

// AzureConfigResponse is returned from GET /config (secrets masked).
type AzureConfigResponse struct {
	TenantID       string `json:"tenant_id"`
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"` // "****" if set
	SubscriptionID string `json:"subscription_id"`
	IsConfigured   bool   `json:"is_configured"`
}

// SyncStatus is returned from GET /status.
type SyncStatus struct {
	Provider       string     `json:"provider"`
	Enabled        bool       `json:"enabled"`
	LastSyncAt     *time.Time `json:"last_sync_at,omitempty"`
	LastSyncStatus *string    `json:"last_sync_status,omitempty"`
	LastSyncError  *string    `json:"last_sync_error,omitempty"`
	EvidenceCount  int        `json:"evidence_count"`
}
