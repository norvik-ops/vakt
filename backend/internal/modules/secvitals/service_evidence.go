package secvitals

import (
	"context"

	"github.com/rs/zerolog/log"
)

// GetEvidenceHistory returns the audit trail for a single evidence item.
func (s *Service) GetEvidenceHistory(ctx context.Context, orgID, evidenceID string) ([]EvidenceHistoryEntry, error) {
	return s.repo.ListEvidenceHistory(ctx, orgID, evidenceID)
}

// recordEvidenceHistory inserts a history row for an evidence change.
// Errors are logged but not returned — history recording is best-effort.
func (s *Service) recordEvidenceHistory(ctx context.Context, orgID, evidenceID, changedByUserID string, e Evidence, note string) {
	var changedBy *string
	if changedByUserID != "" {
		changedBy = &changedByUserID
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO ck_evidence_history (evidence_id, org_id, changed_by, title, description, status, file_url, change_note)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8)`,
		evidenceID, orgID, changedBy, e.Title, e.Description, e.Status, e.FilePath, note,
	)
	if err != nil {
		log.Error().Err(err).Str("evidence_id", evidenceID).Msg("evidence history: record failed")
	}
}
