// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// CryptoKey represents a cryptographic key or certificate in the register.
type CryptoKey struct {
	ID                   string     `json:"id"`
	OrgID                string     `json:"org_id"`
	Name                 string     `json:"name"`
	KeyType              string     `json:"key_type"`
	Algorithm            string     `json:"algorithm"`
	KeyLength            *int       `json:"key_length,omitempty"`
	Purpose              string     `json:"purpose"`
	Location             string     `json:"location,omitempty"`
	RotationIntervalDays *int       `json:"rotation_interval_days,omitempty"`
	LastRotationDate     *string    `json:"last_rotation_date,omitempty"`
	NextRotationDue      *string    `json:"next_rotation_due,omitempty"`
	ExpiryDate           *string    `json:"expiry_date,omitempty"`
	IsWeakAlgorithm      bool       `json:"is_weak_algorithm"`
	RotationStatus       string     `json:"rotation_status"` // ok | due_soon | overdue | none
	Notes                string     `json:"notes,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// CreateCryptoKeyInput is the validated input for creating a crypto key record.
type CreateCryptoKeyInput struct {
	Name                 string  `json:"name"      validate:"required,max=255"`
	KeyType              string  `json:"key_type"  validate:"required,oneof=symmetric asymmetric certificate hmac signing other"`
	Algorithm            string  `json:"algorithm" validate:"required,max=100"`
	KeyLength            *int    `json:"key_length,omitempty"`
	Purpose              string  `json:"purpose"   validate:"required,max=500"`
	Location             string  `json:"location"`
	RotationIntervalDays *int    `json:"rotation_interval_days,omitempty"`
	LastRotationDate     *string `json:"last_rotation_date,omitempty"`
	ExpiryDate           *string `json:"expiry_date,omitempty"`
	Notes                string  `json:"notes"`
}

var weakAlgorithms = []string{"MD5", "SHA-1", "SHA1", "DES", "3DES", "RC4", "RC2"}
var weakKeyLengths = map[string]int{"RSA": 2048, "DSA": 2048}

// IsWeakAlgorithm detects known-insecure algorithms and key lengths.
func IsWeakAlgorithm(algorithm string, keyLength *int) bool {
	upper := strings.ToUpper(algorithm)
	for _, w := range weakAlgorithms {
		if strings.Contains(upper, strings.ToUpper(w)) {
			return true
		}
	}
	if keyLength != nil {
		for prefix, minLen := range weakKeyLengths {
			if strings.HasPrefix(upper, prefix) && *keyLength < minLen {
				return true
			}
		}
	}
	return false
}

// computeRotationStatus returns ok | due_soon | overdue | none based on next_rotation_due.
func computeRotationStatus(nextRotationDue *string) string {
	if nextRotationDue == nil || *nextRotationDue == "" {
		return "none"
	}
	t, err := time.Parse("2006-01-02", *nextRotationDue)
	if err != nil {
		return "none"
	}
	now := time.Now().UTC()
	if t.Before(now) {
		return "overdue"
	}
	if t.Before(now.Add(30 * 24 * time.Hour)) {
		return "due_soon"
	}
	return "ok"
}

// ListCryptoKeys returns all crypto keys for an org.
func (s *Service) ListCryptoKeys(ctx context.Context, orgID string) ([]CryptoKey, error) {
	keys, err := s.repo.ListCryptoKeys(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list crypto keys: %w", err)
	}
	for i := range keys {
		keys[i].RotationStatus = computeRotationStatus(keys[i].NextRotationDue)
	}
	return keys, nil
}

// CreateCryptoKey creates a new crypto key record.
func (s *Service) CreateCryptoKey(ctx context.Context, orgID string, in CreateCryptoKeyInput) (*CryptoKey, error) {
	weak := IsWeakAlgorithm(in.Algorithm, in.KeyLength)

	var nextDue *string
	if in.RotationIntervalDays != nil && in.LastRotationDate != nil {
		t, err := time.Parse("2006-01-02", *in.LastRotationDate)
		if err == nil {
			nd := t.AddDate(0, 0, *in.RotationIntervalDays).Format("2006-01-02")
			nextDue = &nd
		}
	}

	key, err := s.repo.CreateCryptoKey(ctx, orgID, in, weak, nextDue)
	if err != nil {
		return nil, fmt.Errorf("create crypto key: %w", err)
	}
	key.RotationStatus = computeRotationStatus(key.NextRotationDue)
	return key, nil
}

// RecordKeyRotation records a manual key rotation event.
func (s *Service) RecordKeyRotation(ctx context.Context, orgID, keyID string) (*CryptoKey, error) {
	today := time.Now().UTC().Format("2006-01-02")

	key, err := s.repo.GetCryptoKey(ctx, orgID, keyID)
	if err != nil {
		return nil, fmt.Errorf("get crypto key: %w", err)
	}

	var nextDue *string
	if key.RotationIntervalDays != nil {
		t, _ := time.Parse("2006-01-02", today)
		nd := t.AddDate(0, 0, *key.RotationIntervalDays).Format("2006-01-02")
		nextDue = &nd
	}

	updated, err := s.repo.RecordKeyRotation(ctx, orgID, keyID, today, nextDue)
	if err != nil {
		return nil, fmt.Errorf("record key rotation: %w", err)
	}
	updated.RotationStatus = computeRotationStatus(updated.NextRotationDue)
	return updated, nil
}

// DeleteCryptoKey removes a crypto key record.
func (s *Service) DeleteCryptoKey(ctx context.Context, orgID, keyID string) error {
	return s.repo.DeleteCryptoKey(ctx, orgID, keyID)
}
