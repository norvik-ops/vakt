// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"fmt"
	"os"
	"time"
)

// --- Resilience Tests (DORA Art. 24-27) ---

// isTLPTOverdue returns true when no TLPT test exists in the last 3 years.
func isTLPTOverdue(tests []ResilienceTest, now time.Time) bool {
	threshold := now.AddDate(-3, 0, 0)
	for _, t := range tests {
		if t.Type == "tlpt" && t.TestDate.After(threshold) {
			return false
		}
	}
	return true
}

// ListResilienceTests returns all resilience tests for the organisation, with computed OverdueWarning per entry.
// It also returns whether there is a global TLPT overdue warning.
func (s *Service) ListResilienceTests(ctx context.Context, orgID string) ([]ResilienceTest, bool, error) {
	tests, err := s.repo.ListResilienceTests(ctx, orgID)
	if err != nil {
		return nil, false, fmt.Errorf("list resilience tests: %w", err)
	}
	if tests == nil {
		tests = []ResilienceTest{}
	}
	now := time.Now().UTC()
	threshold := now.AddDate(-3, 0, 0)
	for i := range tests {
		if tests[i].Type == "tlpt" && tests[i].TestDate.Before(threshold) {
			tests[i].OverdueWarning = true
		}
	}
	tlptOverdue := isTLPTOverdue(tests, now)
	return tests, tlptOverdue, nil
}

// GetResilienceTest returns a single resilience test with computed OverdueWarning.
func (s *Service) GetResilienceTest(ctx context.Context, orgID, id string) (*ResilienceTest, error) {
	t, err := s.repo.GetResilienceTest(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if t.Type == "tlpt" && t.TestDate.Before(time.Now().UTC().AddDate(-3, 0, 0)) {
		t.OverdueWarning = true
	}
	return t, nil
}

// CreateResilienceTest creates a new resilience test entry.
func (s *Service) CreateResilienceTest(ctx context.Context, orgID string, in CreateResilienceTestInput) (*ResilienceTest, error) {
	return s.repo.CreateResilienceTest(ctx, orgID, in)
}

// UpdateResilienceTest updates an existing resilience test entry.
func (s *Service) UpdateResilienceTest(ctx context.Context, orgID, id string, in UpdateResilienceTestInput) (*ResilienceTest, error) {
	t, err := s.repo.UpdateResilienceTest(ctx, orgID, id, in)
	if err != nil {
		return nil, err
	}
	if t.Type == "tlpt" && t.TestDate.Before(time.Now().UTC().AddDate(-3, 0, 0)) {
		t.OverdueWarning = true
	}
	return t, nil
}

// DeleteResilienceTest removes a resilience test entry.
func (s *Service) DeleteResilienceTest(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteResilienceTest(ctx, orgID, id)
}

// AttachResilienceTestFile saves an uploaded file to disk and updates the attachment_url.
// Files are stored at uploadDir/resilience-tests/{id}/{filename}.
func (s *Service) AttachResilienceTestFile(ctx context.Context, orgID, id, uploadDir string, fileBytes []byte, filename string) (*ResilienceTest, error) {
	dir := fmt.Sprintf("%s/resilience-tests/%s", uploadDir, id)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create upload dir: %w", err)
	}
	destPath := fmt.Sprintf("%s/%s", dir, filename)
	if err := os.WriteFile(destPath, fileBytes, 0o640); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}
	if err := s.repo.UpdateResilienceTestAttachment(ctx, orgID, id, destPath); err != nil {
		return nil, fmt.Errorf("update attachment: %w", err)
	}
	return s.GetResilienceTest(ctx, orgID, id)
}
