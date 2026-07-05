package vaktcomply

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/matharnica/vakt/internal/db"
)

// --- Resilience Tests (DORA Art. 24-27) ---

func resilienceTestFromCkResilienceTests(r db.CkResilienceTests) ResilienceTest {
	t := ResilienceTest{
		ID:                r.ID,
		OrgID:             r.OrgID,
		Type:              r.Type,
		Scope:             r.Scope.String,
		Provider:          r.Provider.String,
		Summary:           r.Summary.String,
		RemediationStatus: r.RemediationStatus,
		AttachmentURL:     r.AttachmentUrl.String,
		CreatedAt:         ckTsToTime(r.CreatedAt),
		UpdatedAt:         ckTsToTime(r.UpdatedAt),
	}
	if r.TestDate.Valid {
		t.TestDate = r.TestDate.Time
	}
	return t
}

// ListResilienceTests returns all resilience tests for an organisation, sorted by test_date DESC.
func (r *Repository) ListResilienceTests(ctx context.Context, orgID string) ([]ResilienceTest, error) {
	rows, err := r.q.ListCKResilienceTests(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list resilience tests: %w", err)
	}
	out := make([]ResilienceTest, 0, len(rows))
	for _, row := range rows {
		out = append(out, resilienceTestFromCkResilienceTests(row))
	}
	return out, nil
}

// GetResilienceTest returns a single resilience test by ID within an organisation.
// Returns an error containing "not found" if the test does not exist.
func (r *Repository) GetResilienceTest(ctx context.Context, orgID, id string) (*ResilienceTest, error) {
	row, err := r.q.GetCKResilienceTest(ctx, db.GetCKResilienceTestParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("resilience test not found: %w", err)
	}
	t := resilienceTestFromCkResilienceTests(row)
	return &t, nil
}

// CreateResilienceTest inserts a new resilience test entry and returns it.
func (r *Repository) CreateResilienceTest(ctx context.Context, orgID string, in CreateResilienceTestInput) (*ResilienceTest, error) {
	remStatus := in.RemediationStatus
	if remStatus == "" {
		remStatus = "open"
	}
	row, err := r.q.CreateCKResilienceTest(ctx, db.CreateCKResilienceTestParams{
		OrgID:             orgID,
		Type:              in.Type,
		Scope:             in.Scope,
		Provider:          in.Provider,
		TestDate:          pgtype.Date{Time: in.TestDate, Valid: true},
		Summary:           in.Summary,
		RemediationStatus: remStatus,
	})
	if err != nil {
		return nil, fmt.Errorf("create resilience test: %w", err)
	}
	t := resilienceTestFromCkResilienceTests(row)
	return &t, nil
}

// UpdateResilienceTest updates an existing resilience test entry and returns it.
func (r *Repository) UpdateResilienceTest(ctx context.Context, orgID, id string, in UpdateResilienceTestInput) (*ResilienceTest, error) {
	row, err := r.q.UpdateCKResilienceTest(ctx, db.UpdateCKResilienceTestParams{
		ID:                id,
		OrgID:             orgID,
		Type:              in.Type,
		Scope:             in.Scope,
		Provider:          in.Provider,
		TestDate:          pgtype.Date{Time: in.TestDate, Valid: true},
		Summary:           in.Summary,
		RemediationStatus: in.RemediationStatus,
	})
	if err != nil {
		return nil, fmt.Errorf("update resilience test: %w", err)
	}
	t := resilienceTestFromCkResilienceTests(row)
	return &t, nil
}

// DeleteResilienceTest removes a resilience test entry.
func (r *Repository) DeleteResilienceTest(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteCKResilienceTest(ctx, db.DeleteCKResilienceTestParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete resilience test: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("resilience test not found")
	}
	return nil
}

// UpdateResilienceTestAttachment sets the attachment_url on a resilience test entry.
func (r *Repository) UpdateResilienceTestAttachment(ctx context.Context, orgID, id, url string) error {
	n, err := r.q.UpdateCKResilienceTestAttachment(ctx, db.UpdateCKResilienceTestAttachmentParams{
		ID:            id,
		OrgID:         orgID,
		AttachmentUrl: ckOptText(url),
	})
	if err != nil {
		return fmt.Errorf("update resilience test attachment: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("resilience test not found")
	}
	return nil
}
