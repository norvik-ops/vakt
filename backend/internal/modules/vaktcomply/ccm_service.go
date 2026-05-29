package vaktcomply

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
)

// ListCCMChecks returns all CCM checks for the given organisation.
func (s *Service) ListCCMChecks(ctx context.Context, orgID string) ([]CCMCheck, error) {
	checks, err := s.repo.ListCCMChecks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list ccm checks: %w", err)
	}
	return checks, nil
}

// CreateCCMCheck creates a new CCM check for the given organisation.
func (s *Service) CreateCCMCheck(ctx context.Context, orgID string, in CreateCCMCheckInput) (*CCMCheck, error) {
	check, err := s.repo.CreateCCMCheck(ctx, orgID, in)
	if err != nil {
		return nil, fmt.Errorf("create ccm check: %w", err)
	}
	return check, nil
}

// DeleteCCMCheck removes a CCM check by ID scoped to org.
func (s *Service) DeleteCCMCheck(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteCCMCheck(ctx, orgID, id)
}

// ToggleCCMCheck enables or disables a CCM check.
func (s *Service) ToggleCCMCheck(ctx context.Context, orgID, id string, enabled bool) error {
	// Verify the check belongs to the org before modifying.
	if _, err := s.repo.GetCCMCheck(ctx, orgID, id); err != nil {
		return fmt.Errorf("ccm check not found: %w", err)
	}
	return s.repo.UpdateCCMCheckEnabled(ctx, id, enabled)
}

// RunCCMCheck executes a check immediately and persists the result.
func (s *Service) RunCCMCheck(ctx context.Context, orgID, id string) (*CCMResult, error) {
	check, err := s.repo.GetCCMCheck(ctx, orgID, id)
	if err != nil {
		return nil, fmt.Errorf("get ccm check: %w", err)
	}

	status, output, runErr := RunCheck(ctx, s.db, *check)
	if runErr != nil {
		// Execution itself failed (should rarely happen — runners handle errors internally).
		status = "unknown"
		output = runErr.Error()
	}

	if err := s.repo.SaveCCMResult(ctx, id, status, output); err != nil {
		return nil, fmt.Errorf("save ccm result: %w", err)
	}

	if err := s.repo.UpdateCCMCheckLastRun(ctx, id, status, output); err != nil {
		return nil, fmt.Errorf("update ccm check last run: %w", err)
	}

	log.Info().
		Str("check_id", id).
		Str("org_id", orgID).
		Str("status", status).
		Msg("ccm: check executed")

	results, err := s.repo.ListCCMResults(ctx, id, 1)
	if err != nil || len(results) == 0 {
		// Return a synthetic result if listing just-written result fails.
		return &CCMResult{CheckID: id, Status: status, Output: output}, nil
	}
	return &results[0], nil
}

// ListCCMResults returns the last 10 results for a given check.
func (s *Service) ListCCMResults(ctx context.Context, orgID, checkID string) ([]CCMResult, error) {
	// Verify ownership.
	if _, err := s.repo.GetCCMCheck(ctx, orgID, checkID); err != nil {
		return nil, fmt.Errorf("ccm check not found: %w", err)
	}
	results, err := s.repo.ListCCMResults(ctx, checkID, 10)
	if err != nil {
		return nil, fmt.Errorf("list ccm results: %w", err)
	}
	return results, nil
}

// RunDueCCMChecks is called by the background worker to run all overdue checks.
func (s *Service) RunDueCCMChecks(ctx context.Context) error {
	checks, err := s.repo.ListDueCCMChecks(ctx)
	if err != nil {
		return fmt.Errorf("list due ccm checks: %w", err)
	}

	log.Info().Int("count", len(checks)).Msg("ccm: running due checks")

	for _, check := range checks {
		status, output, runErr := RunCheck(ctx, s.db, check)
		if runErr != nil {
			status = "unknown"
			output = runErr.Error()
		}

		if saveErr := s.repo.SaveCCMResult(ctx, check.ID, status, output); saveErr != nil {
			log.Error().Err(saveErr).Str("check_id", check.ID).Msg("ccm: save result failed")
			continue
		}

		if updateErr := s.repo.UpdateCCMCheckLastRun(ctx, check.ID, status, output); updateErr != nil {
			log.Error().Err(updateErr).Str("check_id", check.ID).Msg("ccm: update last run failed")
		}

		log.Info().
			Str("check_id", check.ID).
			Str("org_id", check.OrgID).
			Str("status", status).
			Msg("ccm: due check executed")
	}

	return nil
}
