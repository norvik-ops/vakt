package secvitals

import "context"

// --- Control Exceptions ---

func (s *Service) ListAllControlExceptions(ctx context.Context, orgID string) ([]ControlException, error) {
	return s.repo.ListAllControlExceptions(ctx, orgID)
}

func (s *Service) ListControlExceptions(ctx context.Context, orgID, controlID string) ([]ControlException, error) {
	return s.repo.ListControlExceptions(ctx, orgID, controlID)
}

func (s *Service) CreateControlException(ctx context.Context, orgID, controlID string, in CreateControlExceptionInput, createdBy string) (*ControlException, error) {
	return s.repo.CreateControlException(ctx, orgID, controlID, in, createdBy)
}

func (s *Service) UpdateControlException(ctx context.Context, orgID, id string, in UpdateControlExceptionInput) (*ControlException, error) {
	return s.repo.UpdateControlException(ctx, orgID, id, in)
}

func (s *Service) DeleteControlException(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteControlException(ctx, orgID, id)
}
