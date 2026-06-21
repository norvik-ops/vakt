package vaktcomply

import "context"

// CCM (Continuous Control Monitoring) business logic lives in the reporting
// sub-package (S103-3). The parent Service keeps these thin delegations so
// existing callers — the HTTP handlers (ccm_handler.go) and the worker
// entrypoint (handleCCMRunDue) — continue to call them on *vaktcomply.Service.

// ListCCMChecks returns all CCM checks for the given organisation.
func (s *Service) ListCCMChecks(ctx context.Context, orgID string) ([]CCMCheck, error) {
	return s.Reporting.ListCCMChecks(ctx, orgID)
}

// CreateCCMCheck creates a new CCM check for the given organisation.
func (s *Service) CreateCCMCheck(ctx context.Context, orgID string, in CreateCCMCheckInput) (*CCMCheck, error) {
	return s.Reporting.CreateCCMCheck(ctx, orgID, in)
}

// DeleteCCMCheck removes a CCM check by ID scoped to org.
func (s *Service) DeleteCCMCheck(ctx context.Context, orgID, id string) error {
	return s.Reporting.DeleteCCMCheck(ctx, orgID, id)
}

// ToggleCCMCheck enables or disables a CCM check.
func (s *Service) ToggleCCMCheck(ctx context.Context, orgID, id string, enabled bool) error {
	return s.Reporting.ToggleCCMCheck(ctx, orgID, id, enabled)
}

// RunCCMCheck executes a check immediately and persists the result.
func (s *Service) RunCCMCheck(ctx context.Context, orgID, id string) (*CCMResult, error) {
	return s.Reporting.RunCCMCheck(ctx, orgID, id)
}

// ListCCMResults returns the last 10 results for a given check.
func (s *Service) ListCCMResults(ctx context.Context, orgID, checkID string) ([]CCMResult, error) {
	return s.Reporting.ListCCMResults(ctx, orgID, checkID)
}

// RunDueCCMChecks is called by the background worker to run all overdue checks.
func (s *Service) RunDueCCMChecks(ctx context.Context) error {
	return s.Reporting.RunDueCCMChecks(ctx)
}
