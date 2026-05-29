package vaktcomply

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/matharnica/vakt/internal/db"
)

// CountEvidenceByControl returns the number of approved evidence items per control for a framework.
// Result: map[controlUUID]count.
func (r *Repository) CountEvidenceByControl(ctx context.Context, orgID, frameworkID string) (map[string]int, error) {
	rows, err := r.q.CountCKEvidenceByControl(ctx, db.CountCKEvidenceByControlParams{OrgID: orgID, FrameworkID: frameworkID})
	if err != nil {
		return nil, fmt.Errorf("count evidence by control: %w", err)
	}
	counts := make(map[string]int, len(rows))
	for _, row := range rows {
		counts[row.ControlID] = int(row.EvidenceCount)
	}
	return counts, nil
}

// GetExpiringEvidence returns evidence items expiring within the given threshold for a framework.
func (r *Repository) GetExpiringEvidence(ctx context.Context, orgID, frameworkID string, threshold time.Time) ([]Evidence, error) {
	rows, err := r.q.GetCKExpiringEvidence(ctx, db.GetCKExpiringEvidenceParams{
		OrgID:       orgID,
		FrameworkID: frameworkID,
		ExpiresAt:   pgtype.Timestamptz{Time: threshold, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("get expiring evidence: %w", err)
	}
	out := make([]Evidence, 0, len(rows))
	for _, row := range rows {
		out = append(out, evidenceFromFields(evidenceFields{
			ID: row.ID, ControlID: row.ControlID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Source: row.Source, FilePath: row.FilePath,
			FileSize: row.FileSize, Status: row.Status, Version: row.Version,
			ExpiresAt: row.ExpiresAt, ExpiryNotifiedAt: row.ExpiryNotifiedAt,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

// GetExpiringEvidenceAllFrameworks returns evidence expiring within threshold across all frameworks for an org.
func (r *Repository) GetExpiringEvidenceAllFrameworks(ctx context.Context, orgID string, threshold time.Time) ([]Evidence, error) {
	rows, err := r.q.GetCKExpiringEvidenceAllFrameworks(ctx, db.GetCKExpiringEvidenceAllFrameworksParams{
		OrgID:     orgID,
		ExpiresAt: pgtype.Timestamptz{Time: threshold, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("get expiring evidence all frameworks: %w", err)
	}
	out := make([]Evidence, 0, len(rows))
	for _, row := range rows {
		out = append(out, evidenceFromFields(evidenceFields{
			ID: row.ID, ControlID: row.ControlID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Source: row.Source, FilePath: row.FilePath,
			FileSize: row.FileSize, Status: row.Status, Version: row.Version,
			ExpiresAt: row.ExpiresAt, ExpiryNotifiedAt: row.ExpiryNotifiedAt,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

// EvidenceExpiryNotifyRow is a minimal projection used by the expiry notification worker.
type EvidenceExpiryNotifyRow struct {
	ID           string
	OrgID        string
	Title        string
	ControlTitle string
	ExpiresAt    time.Time
}

// GetUnnotifiedExpiringEvidence returns evidence items that expire within the given
// threshold and have not yet had a notification sent (expiry_notified_at IS NULL).
// It joins ck_controls to include the control title in the notification message.
func (r *Repository) GetUnnotifiedExpiringEvidence(ctx context.Context, orgID string, threshold time.Time) ([]EvidenceExpiryNotifyRow, error) {
	rows, err := r.q.GetCKUnnotifiedExpiringEvidence(ctx, db.GetCKUnnotifiedExpiringEvidenceParams{
		OrgID:     orgID,
		ExpiresAt: pgtype.Timestamptz{Time: threshold, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("get unnotified expiring evidence: %w", err)
	}
	out := make([]EvidenceExpiryNotifyRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, EvidenceExpiryNotifyRow{
			ID:           row.ID,
			OrgID:        row.OrgID,
			Title:        row.EvidenceTitle,
			ControlTitle: row.ControlTitle,
			ExpiresAt:    ckTsToTime(row.ExpiresAt),
		})
	}
	return out, nil
}

// MarkEvidenceExpiryNotified sets expiry_notified_at = NOW() for the given evidence IDs.
func (r *Repository) MarkEvidenceExpiryNotified(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := r.q.MarkCKEvidenceExpiryNotified(ctx, ids); err != nil {
		return fmt.Errorf("mark evidence expiry notified: %w", err)
	}
	return nil
}

// --- Evidence ---

// AddEvidence inserts a new evidence item for a control.
func (r *Repository) AddEvidence(ctx context.Context, orgID, controlID, userID string, input AddEvidenceInput) (*Evidence, error) {
	row, err := r.q.AddCKEvidence(ctx, db.AddCKEvidenceParams{
		ControlID:   ckOptUUIDFromStr(controlID),
		OrgID:       orgID,
		Title:       input.Title,
		Description: ckOptText(input.Description),
		Source:      input.Source,
		FilePath:    input.FilePath,
		FileSize:    input.FileSize,
		ExpiresAt:   ckOptTsPtr(input.ExpiresAt),
		UploadedBy:  ckOptUUIDFromStr(userID),
	})
	if err != nil {
		return nil, fmt.Errorf("add evidence: %w", err)
	}
	ev := evidenceFromFields(evidenceFields{
		ID: row.ID, ControlID: row.ControlID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Source: row.Source, FilePath: row.FilePath,
		FileSize: row.FileSize, Status: row.Status, Version: row.Version,
		ExpiresAt: row.ExpiresAt, ExpiryNotifiedAt: row.ExpiryNotifiedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &ev, nil
}

// ListEvidence returns all evidence items for a control within an organisation.
func (r *Repository) ListEvidence(ctx context.Context, orgID, controlID string) ([]Evidence, error) {
	rows, err := r.q.ListCKEvidence(ctx, db.ListCKEvidenceParams{
		ControlID: ckOptUUIDFromStr(controlID),
		OrgID:     orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("list evidence: %w", err)
	}
	out := make([]Evidence, 0, len(rows))
	for _, row := range rows {
		out = append(out, evidenceFromFields(evidenceFields{
			ID: row.ID, ControlID: row.ControlID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Source: row.Source, FilePath: row.FilePath,
			FileSize: row.FileSize, Status: row.Status, Version: row.Version,
			ExpiresAt: row.ExpiresAt, ExpiryNotifiedAt: row.ExpiryNotifiedAt,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

// ListEvidenceByControls fetches all evidence for the given control IDs in a single query.
// Returns a map[controlID][]Evidence. Controls with no evidence are absent from the map.
func (r *Repository) ListEvidenceByControls(ctx context.Context, orgID string, controlIDs []string) (map[string][]Evidence, error) {
	if len(controlIDs) == 0 {
		return map[string][]Evidence{}, nil
	}
	rows, err := r.q.ListCKEvidenceByControls(ctx, db.ListCKEvidenceByControlsParams{
		Column1: controlIDs,
		OrgID:   orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("list evidence by controls: %w", err)
	}
	result := make(map[string][]Evidence, len(controlIDs))
	for _, row := range rows {
		ev := evidenceFromFields(evidenceFields{
			ID: row.ID, ControlID: row.ControlID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Source: row.Source, FilePath: row.FilePath,
			FileSize: row.FileSize, Status: row.Status, Version: row.Version,
			ExpiresAt: row.ExpiresAt, ExpiryNotifiedAt: row.ExpiryNotifiedAt,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
		result[ev.ControlID] = append(result[ev.ControlID], ev)
	}
	return result, nil
}

// GetEvidence returns a single evidence item by ID within an organisation.
func (r *Repository) GetEvidence(ctx context.Context, orgID, evidenceID string) (*Evidence, error) {
	row, err := r.q.GetCKEvidence(ctx, db.GetCKEvidenceParams{ID: evidenceID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get evidence: %w", err)
	}
	ev := evidenceFromFields(evidenceFields{
		ID: row.ID, ControlID: row.ControlID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Source: row.Source, FilePath: row.FilePath,
		FileSize: row.FileSize, Status: row.Status, Version: row.Version,
		ExpiresAt: row.ExpiresAt, ExpiryNotifiedAt: row.ExpiryNotifiedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &ev, nil
}

// ListEvidenceHistory returns the audit history for an evidence item, newest first.
func (r *Repository) ListEvidenceHistory(ctx context.Context, orgID, evidenceID string) ([]EvidenceHistoryEntry, error) {
	rows, err := r.q.ListCKEvidenceHistory(ctx, db.ListCKEvidenceHistoryParams{
		EvidenceID: evidenceID,
		OrgID:      orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("list evidence history: %w", err)
	}
	items := make([]EvidenceHistoryEntry, 0, len(rows))
	for _, row := range rows {
		items = append(items, EvidenceHistoryEntry{
			ID:         row.ID,
			EvidenceID: row.EvidenceID,
			ChangedBy:  uuidPtrFromPgtype(row.ChangedBy),
			ChangedAt:  ckTsToTime(row.ChangedAt),
			Title:      row.Title.String,
			Status:     row.Status.String,
			ChangeNote: row.ChangeNote.String,
		})
	}
	return items, nil
}

// ReviewEvidence updates the status and reviewer information on an evidence item.
func (r *Repository) ReviewEvidence(ctx context.Context, orgID, evidenceID, reviewerID, status string) error {
	n, err := r.q.ReviewCKEvidence(ctx, db.ReviewCKEvidenceParams{
		Status:     status,
		ReviewedBy: ckOptUUIDFromStr(reviewerID),
		ID:         evidenceID,
		OrgID:      orgID,
	})
	if err != nil {
		return fmt.Errorf("review evidence: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("evidence not found")
	}
	return nil
}

// AddCollectorEvidence inserts evidence produced by an automated collector.
func (r *Repository) AddCollectorEvidence(ctx context.Context, orgID, controlID, userID, source, title string, data []byte) (*Evidence, error) {
	row, err := r.q.AddCKCollectorEvidence(ctx, db.AddCKCollectorEvidenceParams{
		ControlID:     ckOptUUIDFromStr(controlID),
		OrgID:         orgID,
		Title:         title,
		Source:        source,
		CollectorData: data,
		UploadedBy:    ckOptUUIDFromStr(userID),
	})
	if err != nil {
		return nil, fmt.Errorf("add collector evidence: %w", err)
	}
	ev := evidenceFromFields(evidenceFields{
		ID: row.ID, ControlID: row.ControlID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Source: row.Source, FilePath: row.FilePath,
		FileSize: row.FileSize, Status: row.Status, Version: row.Version,
		ExpiresAt: row.ExpiresAt, ExpiryNotifiedAt: row.ExpiryNotifiedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &ev, nil
}

// EvidenceForExport is a flattened view of evidence joined with its control,
// used exclusively by the audit-package ZIP generator.
type EvidenceForExport struct {
	ControlID        string
	ControlTitle     string
	ControlDomain    string // e.g. "A.5" from the control code
	EvidenceID       string
	EvidenceTitle    string
	EvidenceSource   string // 'manual', 'github', 'aws', etc.
	EvidenceDesc     string
	EvidenceFilePath string
	CollectedAt      time.Time
}

// ListEvidenceForFramework returns all evidence for all controls of a framework
// joined with control metadata. Controls without evidence are included with
// empty evidence fields so every control appears in the index PDF.
func (r *Repository) ListEvidenceForFramework(ctx context.Context, orgID, frameworkID string) ([]EvidenceForExport, error) {
	rows, err := r.q.ListCKEvidenceForFramework(ctx, db.ListCKEvidenceForFrameworkParams{
		OrgID:       orgID,
		FrameworkID: frameworkID,
	})
	if err != nil {
		return nil, fmt.Errorf("list evidence for framework: %w", err)
	}
	result := make([]EvidenceForExport, 0, len(rows))
	now := time.Now()
	for _, row := range rows {
		evID := ""
		if row.EvidenceID.Valid {
			evID = row.EvidenceID.String()
		}
		collectedAt := now
		if row.EvidenceCreatedAt.Valid {
			collectedAt = row.EvidenceCreatedAt.Time
		}
		result = append(result, EvidenceForExport{
			ControlID:        row.ControlUuid,
			ControlTitle:     row.ControlTitle,
			ControlDomain:    row.ControlCode,
			EvidenceID:       evID,
			EvidenceTitle:    row.EvidenceTitle.String,
			EvidenceSource:   row.EvidenceSource.String,
			EvidenceDesc:     row.EvidenceDesc.String,
			EvidenceFilePath: row.EvidenceFilePath.String,
			CollectedAt:      collectedAt,
		})
	}
	return result, nil
}

// --- Auditor links ---

// CreateAuditorLink inserts a new auditor link record.
func (r *Repository) CreateAuditorLink(ctx context.Context, orgID, frameworkID, userID, tokenHash string, expiresAt time.Time, maxUses *int) (*AuditorLink, error) {
	row, err := r.q.CreateCKAuditorLink(ctx, db.CreateCKAuditorLinkParams{
		OrgID:       orgID,
		FrameworkID: ckOptUUIDFromStr(frameworkID),
		TokenHash:   tokenHash,
		CreatedBy:   userID,
		ExpiresAt:   pgtype.Timestamptz{Time: expiresAt, Valid: true},
		MaxUses:     ckOptIntPtr(maxUses),
	})
	if err != nil {
		return nil, fmt.Errorf("create auditor link: %w", err)
	}
	return &AuditorLink{
		ID:          row.ID,
		OrgID:       row.OrgID,
		FrameworkID: uuidStringFromPgtype(row.FrameworkID),
		CreatedBy:   row.CreatedBy,
		ExpiresAt:   ckTsToTime(row.ExpiresAt),
		UsedCount:   int(row.UsedCount),
		MaxUses:     intPtrFromInt4(row.MaxUses),
		CreatedAt:   ckTsToTime(row.CreatedAt),
	}, nil
}

// GetAuditorLinkByHash looks up an auditor link by its token hash and validates expiry.
// Returns an error if the link has been revoked.
func (r *Repository) GetAuditorLinkByHash(ctx context.Context, tokenHash string) (*AuditorLink, error) {
	row, err := r.q.GetCKAuditorLinkByHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("get auditor link: %w", err)
	}
	if row.RevokedAt.Valid {
		return nil, fmt.Errorf("auditor link revoked")
	}
	return &AuditorLink{
		ID:          row.ID,
		OrgID:       row.OrgID,
		FrameworkID: uuidStringFromPgtype(row.FrameworkID),
		CreatedBy:   row.CreatedBy,
		ExpiresAt:   ckTsToTime(row.ExpiresAt),
		UsedCount:   int(row.UsedCount),
		MaxUses:     intPtrFromInt4(row.MaxUses),
		CreatedAt:   ckTsToTime(row.CreatedAt),
	}, nil
}

// GetAuditorLinkByID returns an auditor link by UUID within an organisation.
func (r *Repository) GetAuditorLinkByID(ctx context.Context, orgID, linkID string) (*AuditorLinkListItem, error) {
	row, err := r.q.GetCKAuditorLinkByID(ctx, db.GetCKAuditorLinkByIDParams{ID: linkID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get auditor link by id: %w", err)
	}
	return &AuditorLinkListItem{
		ID:             row.ID,
		OrgID:          row.OrgID,
		FrameworkID:    uuidStringFromPgtype(row.FrameworkID),
		Label:          row.Label,
		CreatedBy:      row.CreatedBy,
		ExpiresAt:      ckTsToTime(row.ExpiresAt),
		LastAccessedAt: ckTsToTimePtr(row.LastAccessedAt),
		AccessCount:    int(row.AccessCount),
		RevokedAt:      ckTsToTimePtr(row.RevokedAt),
		CreatedAt:      ckTsToTime(row.CreatedAt),
	}, nil
}

// ListAuditorLinks returns all auditor links for an organisation.
func (r *Repository) ListAuditorLinks(ctx context.Context, orgID string) ([]AuditorLinkListItem, error) {
	rows, err := r.q.ListCKAuditorLinks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list auditor links: %w", err)
	}
	out := make([]AuditorLinkListItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, AuditorLinkListItem{
			ID:             row.ID,
			OrgID:          row.OrgID,
			FrameworkID:    uuidStringFromPgtype(row.FrameworkID),
			Label:          row.Label,
			CreatedBy:      row.CreatedBy,
			ExpiresAt:      ckTsToTime(row.ExpiresAt),
			LastAccessedAt: ckTsToTimePtr(row.LastAccessedAt),
			AccessCount:    int(row.AccessCount),
			RevokedAt:      ckTsToTimePtr(row.RevokedAt),
			CreatedAt:      ckTsToTime(row.CreatedAt),
		})
	}
	return out, nil
}

// RevokeAuditorLink sets revoked_at on an auditor link, preventing future access.
func (r *Repository) RevokeAuditorLink(ctx context.Context, orgID, linkID string) error {
	n, err := r.q.RevokeCKAuditorLink(ctx, db.RevokeCKAuditorLinkParams{ID: linkID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("revoke auditor link: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("auditor link not found or already revoked")
	}
	return nil
}

// UpdateAuditorLinkAccess bumps access_count and sets last_accessed_at.
func (r *Repository) UpdateAuditorLinkAccess(ctx context.Context, linkID string) error {
	return r.q.UpdateCKAuditorLinkAccess(ctx, linkID)
}

// IncrementAuditorLinkUsage bumps the used_count on an auditor link.
func (r *Repository) IncrementAuditorLinkUsage(ctx context.Context, linkID string) error {
	return r.q.IncrementCKAuditorLinkUsage(ctx, linkID)
}

// --- Evidence Files (Migration 074) ---

// evidenceFileFromCk maps the sqlc CkEvidenceFiles row to the domain EvidenceFile.
func evidenceFileFromCk(r db.CkEvidenceFiles) EvidenceFile {
	evID := ""
	if r.EvidenceID.Valid {
		evID = r.EvidenceID.String()
	}
	return EvidenceFile{
		ID:           r.ID,
		OrgID:        r.OrgID,
		EvidenceID:   evID,
		ControlID:    r.ControlID,
		OriginalName: r.OriginalName,
		StoredName:   r.StoredName,
		MimeType:     r.MimeType,
		SizeBytes:    r.SizeBytes,
		UploadedBy:   r.UploadedBy,
		CreatedAt:    ckTsToTime(r.CreatedAt),
	}
}

// CreateEvidenceFile inserts a new evidence file record.
func (r *Repository) CreateEvidenceFile(ctx context.Context, f EvidenceFile) (EvidenceFile, error) {
	row, err := r.q.CreateCKEvidenceFile(ctx, db.CreateCKEvidenceFileParams{
		OrgID:        f.OrgID,
		EvidenceID:   ckOptUUIDFromStr(f.EvidenceID),
		ControlID:    f.ControlID,
		OriginalName: f.OriginalName,
		StoredName:   f.StoredName,
		MimeType:     f.MimeType,
		SizeBytes:    f.SizeBytes,
		UploadedBy:   f.UploadedBy,
	})
	if err != nil {
		return EvidenceFile{}, fmt.Errorf("create evidence file: %w", err)
	}
	return evidenceFileFromCk(row), nil
}

// ListEvidenceFiles returns all files attached to an evidence record.
func (r *Repository) ListEvidenceFiles(ctx context.Context, orgID, evidenceID string) ([]EvidenceFile, error) {
	rows, err := r.q.ListCKEvidenceFiles(ctx, db.ListCKEvidenceFilesParams{
		OrgID:      orgID,
		EvidenceID: ckOptUUIDFromStr(evidenceID),
	})
	if err != nil {
		return nil, fmt.Errorf("list evidence files: %w", err)
	}
	items := make([]EvidenceFile, 0, len(rows))
	for _, row := range rows {
		items = append(items, evidenceFileFromCk(row))
	}
	return items, nil
}

// ListEvidenceFilesByControl returns all files attached to any evidence for a given control.
func (r *Repository) ListEvidenceFilesByControl(ctx context.Context, orgID, controlID string) ([]EvidenceFile, error) {
	rows, err := r.q.ListCKEvidenceFilesByControl(ctx, db.ListCKEvidenceFilesByControlParams{
		OrgID:     orgID,
		ControlID: controlID,
	})
	if err != nil {
		return nil, fmt.Errorf("list evidence files by control: %w", err)
	}
	items := make([]EvidenceFile, 0, len(rows))
	for _, row := range rows {
		items = append(items, evidenceFileFromCk(row))
	}
	return items, nil
}

// GetEvidenceFile returns a single evidence file by ID within an organisation.
func (r *Repository) GetEvidenceFile(ctx context.Context, orgID, fileID string) (EvidenceFile, error) {
	row, err := r.q.GetCKEvidenceFile(ctx, db.GetCKEvidenceFileParams{ID: fileID, OrgID: orgID})
	if err != nil {
		return EvidenceFile{}, fmt.Errorf("get evidence file: %w", err)
	}
	return evidenceFileFromCk(row), nil
}

// DeleteEvidenceFile removes an evidence file record and returns its metadata for disk deletion.
func (r *Repository) DeleteEvidenceFile(ctx context.Context, orgID, fileID string) (EvidenceFile, error) {
	row, err := r.q.DeleteCKEvidenceFile(ctx, db.DeleteCKEvidenceFileParams{ID: fileID, OrgID: orgID})
	if err != nil {
		return EvidenceFile{}, fmt.Errorf("delete evidence file: %w", err)
	}
	return evidenceFileFromCk(row), nil
}
