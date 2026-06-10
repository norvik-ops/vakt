package vaktcomply

import "context"

// BulkUpdateCAPAStatus sets status for multiple CAPAs in a single query.
func (s *Service) BulkUpdateCAPAStatus(ctx context.Context, orgID string, ids []string, status string) error {
	return s.repo.BulkUpdateCAPAStatus(ctx, orgID, ids, status)
}

// UpdateCAPANCFields updates the NC root-cause and effectiveness-planning fields of a CAPA.
func (s *Service) UpdateCAPANCFields(ctx context.Context, orgID, id string, fields CAPANCFields) error {
	return s.repo.UpdateCAPANCFields(ctx, orgID, id, fields)
}

// CompleteEffectivenessCheck records the result of a CAPA effectiveness check.
func (s *Service) CompleteEffectivenessCheck(ctx context.Context, orgID, id, userID string, in EffectivenessCheckInput) error {
	return s.repo.CompleteEffectivenessCheck(ctx, orgID, id, userID, in)
}
