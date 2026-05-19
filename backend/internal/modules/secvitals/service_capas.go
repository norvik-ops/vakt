package secvitals

import "context"

// BulkUpdateCAPAStatus sets status for multiple CAPAs in a single query.
func (s *Service) BulkUpdateCAPAStatus(ctx context.Context, orgID string, ids []string, status string) error {
	return s.repo.BulkUpdateCAPAStatus(ctx, orgID, ids, status)
}
