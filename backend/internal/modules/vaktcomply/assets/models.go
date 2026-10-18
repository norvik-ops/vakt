// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package assets is the ISMS asset inventory (ISO 27001 A.5.9): protection
// needs, risk↔asset links, BSI modelling and onboarding read it. It lived in
// vaktscan for historical reasons and moved here with S1 of
// docs/stories/module-archivieren.md. The table keeps its historic name
// vb_assets until the rename in S5; ownership is vaktcomply
// (scripts/check_module_isolation.py, TABLE_OWNER).
package assets

import "time"

// Asset is an entry of the asset inventory.
type Asset struct {
	ID               string    `json:"id"`
	OrgID            string    `json:"org_id"`
	Name             string    `json:"name"`
	Type             string    `json:"type"`
	Criticality      string    `json:"criticality"`
	Environment      string    `json:"environment"`    // prod | staging | dev (S44-2)
	Classification   string    `json:"classification"` // public | internal | confidential | restricted (S67-3)
	Tags             []string  `json:"tags"`
	OwnerID          *string   `json:"owner_id,omitempty"`
	ExternalURL      *string   `json:"external_url,omitempty"`
	ProtectionNeedID *string   `json:"protection_need_id,omitempty"` // soft link to ck_protection_need_assessments, no FK
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ClassificationSummary holds counts of assets by classification level.
type ClassificationSummary struct {
	TotalCount        int            `json:"total_count"`
	ClassifiedCount   int            `json:"classified_count"`
	ByLevel           map[string]int `json:"by_level"`
	UnclassifiedCount int            `json:"unclassified_count"`
}

// CreateAssetInput is the validated request body for creating an asset.
type CreateAssetInput struct {
	Name           string   `json:"name"           validate:"required,min=1,max=255"`
	Type           string   `json:"type"           validate:"required,oneof=server container web_app database repo"`
	Criticality    string   `json:"criticality"    validate:"required,oneof=low medium high critical"`
	Environment    string   `json:"environment"    validate:"omitempty,oneof=prod staging dev"`
	Classification string   `json:"classification" validate:"omitempty,oneof=public internal confidential restricted"`
	Tags           []string `json:"tags"`
	OwnerID        *string  `json:"owner_id,omitempty"`
	ExternalURL    string   `json:"external_url"`
}

// UpdateAssetInput is the validated request body for updating an asset.
type UpdateAssetInput struct {
	Name           *string  `json:"name"           validate:"omitempty,min=1,max=255"`
	Type           *string  `json:"type"           validate:"omitempty,oneof=server container web_app database repo"`
	Criticality    *string  `json:"criticality"    validate:"omitempty,oneof=low medium high critical"`
	Environment    *string  `json:"environment"    validate:"omitempty,oneof=prod staging dev"`
	Classification *string  `json:"classification" validate:"omitempty,oneof=public internal confidential restricted"`
	Tags           []string `json:"tags"`
	OwnerID        *string  `json:"owner_id,omitempty"`
	ExternalURL    *string  `json:"external_url,omitempty"`
}

// CSVAssetRow represents one row from a bulk-import CSV file.
type CSVAssetRow struct {
	Name        string
	Type        string
	Criticality string
	Environment string
	Tags        []string
	ExternalURL string
}
