// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package assets

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/db"
	"github.com/matharnica/vakt/internal/shared/apperr"
)

// Repository handles asset inventory data access. The sqlc queries keep their
// historic SP names (db/queries/vaktscan.sql, hand-maintained per ADR-0078)
// until the table rename in S5.
type Repository struct {
	db *pgxpool.Pool
	q  *db.Queries
}

// NewRepository creates a new asset repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{db: pool, q: db.New(pool)}
}

// assetFields is the union of fields returned by every Asset-returning sqlc
// query. The Row-types diverge in column order (ADR-0013), so we centralise
// the mapping here.
type assetFields struct {
	ID, OrgID, Name, Type, Criticality, Environment string
	Classification                                  string // S124-3 (DB-01): was written but never selected
	Tags                                            []string
	OwnerID                                         pgtype.UUID
	ExternalUrl                                     pgtype.Text
	CreatedAt, UpdatedAt                            pgtype.Timestamptz
}

func assetFromFields(f assetFields) Asset {
	env := f.Environment
	if env == "" {
		env = "prod"
	}
	return Asset{
		ID:             f.ID,
		OrgID:          f.OrgID,
		Name:           f.Name,
		Type:           f.Type,
		Criticality:    f.Criticality,
		Classification: f.Classification,
		Environment:    env,
		Tags:           f.Tags,
		OwnerID:        uuidPtr(f.OwnerID),
		ExternalURL:    textPtr(f.ExternalUrl),
		CreatedAt:      tsToTime(f.CreatedAt),
		UpdatedAt:      tsToTime(f.UpdatedAt),
	}
}

func optText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

func uuidPtr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := u.String()
	return &s
}

func optUUID(s *string) pgtype.UUID {
	if s == nil || *s == "" {
		return pgtype.UUID{}
	}
	var u pgtype.UUID
	_ = u.Scan(*s)
	return u
}

func tsToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// derefStrPtr returns "" for a nil *string.
func derefStrPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// enrichEnvironments fetches the environment column for a slice of assets via a
// single IN-query and patches the slice in place. Called after every sqlc-based
// asset read because the environment column was added after sqlc generation.
func (r *Repository) enrichEnvironments(ctx context.Context, assets []Asset) {
	if len(assets) == 0 {
		return
	}
	ids := make([]string, len(assets))
	for i, a := range assets {
		ids[i] = a.ID
	}
	// orgid-lint: global — IDs come from a prior org-scoped query; update by PK is safe
	rows, err := r.db.Query(ctx,
		`SELECT id, environment FROM vb_assets WHERE id = ANY($1)`, ids)
	if err != nil {
		return
	}
	defer rows.Close()
	envMap := make(map[string]string, len(assets))
	for rows.Next() {
		var id, env string
		if rows.Scan(&id, &env) == nil {
			envMap[id] = env
		}
	}
	for i := range assets {
		if env, ok := envMap[assets[i].ID]; ok {
			assets[i].Environment = env
		}
	}
}

// GetClassificationSummary returns asset counts grouped by classification level.
func (r *Repository) GetClassificationSummary(ctx context.Context, orgID string) (*ClassificationSummary, error) {
	rows, err := r.db.Query(ctx,
		`SELECT COALESCE(classification, 'unclassified') AS cls, COUNT(*) AS cnt
		   FROM vb_assets
		  WHERE org_id = $1::uuid AND (is_deleted IS NULL OR is_deleted = false)
		  GROUP BY cls`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("get classification summary: %w", err)
	}
	defer rows.Close()

	levels := map[string]int{"public": 0, "internal": 0, "confidential": 0, "restricted": 0}
	unclassified := 0
	total := 0
	for rows.Next() {
		var cls string
		var cnt int
		if err := rows.Scan(&cls, &cnt); err != nil {
			return nil, fmt.Errorf("scan classification row: %w", err)
		}
		total += cnt
		if cls == "unclassified" || cls == "" {
			unclassified += cnt
		} else {
			levels[cls] += cnt
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("classification summary rows: %w", err)
	}

	classified := total - unclassified
	return &ClassificationSummary{
		TotalCount:        total,
		ClassifiedCount:   classified,
		ByLevel:           levels,
		UnclassifiedCount: unclassified,
	}, nil
}

// CreateAsset inserts a new asset row and returns the created record.
func (r *Repository) CreateAsset(ctx context.Context, orgID string, input CreateAssetInput) (*Asset, error) {
	tags := input.Tags
	if tags == nil {
		tags = []string{}
	}
	row, err := r.q.CreateSPAsset(ctx, db.CreateSPAssetParams{
		OrgID:       orgID,
		Name:        input.Name,
		Type:        input.Type,
		Criticality: input.Criticality,
		Tags:        tags,
		OwnerID:     optUUID(input.OwnerID),
		ExternalUrl: optText(input.ExternalURL),
	})
	if err != nil {
		return nil, fmt.Errorf("insert asset: %w", err)
	}
	env := input.Environment
	if env == "" {
		env = "prod"
	}
	// orgid-lint: global — UPDATE by PK row.ID from the RETURNING clause of the INSERT just above
	if _, execErr := r.db.Exec(ctx,
		`UPDATE vb_assets SET environment=$1 WHERE id=$2`, env, row.ID); execErr != nil {
		log.Warn().Err(execErr).Str("asset_id", row.ID).Msg("could not set environment on new asset")
	}
	cls := input.Classification
	if cls == "" {
		cls = "internal"
	}
	// orgid-lint: global — UPDATE by PK row.ID from the RETURNING clause of the INSERT just above
	if _, execErr := r.db.Exec(ctx,
		`UPDATE vb_assets SET classification=$1 WHERE id=$2`, cls, row.ID); execErr != nil {
		log.Warn().Err(execErr).Str("asset_id", row.ID).Msg("could not set classification on new asset")
	}
	a := assetFromFields(assetFields{
		ID: row.ID, OrgID: row.OrgID, Name: row.Name, Type: row.Type,
		Criticality: row.Criticality, Environment: env, Tags: row.Tags,
		OwnerID: row.OwnerID, ExternalUrl: row.ExternalUrl,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	a.Classification = cls
	return &a, nil
}

// ListAssets returns a paginated list of non-deleted assets for an org.
// An optional tag filter restricts results to assets containing that tag.
func (r *Repository) ListAssets(ctx context.Context, orgID string, page, limit int, tag string) ([]Asset, int, error) {
	var tagParam pgtype.Text
	if tag != "" {
		tagParam = pgtype.Text{String: tag, Valid: true}
	}
	total, err := r.q.CountSPAssets(ctx, db.CountSPAssetsParams{
		OrgID: orgID,
		Tag:   tagParam,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count assets: %w", err)
	}
	offset := (page - 1) * limit
	rows, err := r.q.ListSPAssets(ctx, db.ListSPAssetsParams{
		OrgID:  orgID,
		Tag:    tagParam,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("query assets: %w", err)
	}
	out := make([]Asset, 0, len(rows))
	for _, row := range rows {
		out = append(out, assetFromFields(assetFields{
			ID: row.ID, OrgID: row.OrgID, Name: row.Name, Type: row.Type,
			Criticality: row.Criticality, Classification: row.Classification, Tags: row.Tags,
			OwnerID: row.OwnerID, ExternalUrl: row.ExternalUrl,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	r.enrichEnvironments(ctx, out)
	return out, int(total), nil
}

// GetAsset fetches a single non-deleted asset by ID within the org.
func (r *Repository) GetAsset(ctx context.Context, orgID, assetID string) (*Asset, error) {
	row, err := r.q.GetSPAsset(ctx, db.GetSPAssetParams{ID: assetID, OrgID: orgID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("asset not found: %w", err)
		}
		return nil, fmt.Errorf("get asset: %w", err)
	}
	tmp := []Asset{assetFromFields(assetFields{
		ID: row.ID, OrgID: row.OrgID, Name: row.Name, Type: row.Type,
		Criticality: row.Criticality, Classification: row.Classification, Tags: row.Tags,
		OwnerID: row.OwnerID, ExternalUrl: row.ExternalUrl,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})}
	r.enrichEnvironments(ctx, tmp)
	return &tmp[0], nil
}

// UpdateAsset applies a partial update to an asset.  Only non-nil fields are changed.
// Read-merge-write because sqlc cannot generate dynamic SET clauses (ADR-0005).
func (r *Repository) UpdateAsset(ctx context.Context, orgID, assetID string, input UpdateAssetInput) (*Asset, error) {
	cur, err := r.GetAsset(ctx, orgID, assetID)
	if err != nil {
		return nil, err
	}

	params := db.UpdateSPAssetParams{
		ID:          assetID,
		OrgID:       orgID,
		Name:        cur.Name,
		Type:        cur.Type,
		Criticality: cur.Criticality,
		Tags:        cur.Tags,
		OwnerID:     optUUID(cur.OwnerID),
		ExternalUrl: optText(derefStrPtr(cur.ExternalURL)),
	}
	if input.Name != nil {
		params.Name = *input.Name
	}
	if input.Type != nil {
		params.Type = *input.Type
	}
	if input.Criticality != nil {
		params.Criticality = *input.Criticality
	}
	if input.Tags != nil {
		params.Tags = input.Tags
	}
	if input.OwnerID != nil {
		params.OwnerID = optUUID(input.OwnerID)
	}
	if input.ExternalURL != nil {
		params.ExternalUrl = optText(*input.ExternalURL)
	}

	row, err := r.q.UpdateSPAsset(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("update asset: %w", err)
	}
	newEnv := cur.Environment
	if input.Environment != nil {
		newEnv = *input.Environment
		if _, execErr := r.db.Exec(ctx,
			`UPDATE vb_assets SET environment=$1 WHERE id=$2 AND org_id=$3`,
			newEnv, assetID, orgID); execErr != nil {
			log.Warn().Err(execErr).Str("asset_id", assetID).Msg("could not update environment")
		}
	}
	newCls := cur.Classification
	if input.Classification != nil {
		newCls = *input.Classification
		if _, execErr := r.db.Exec(ctx,
			`UPDATE vb_assets SET classification=$1 WHERE id=$2 AND org_id=$3`,
			newCls, assetID, orgID); execErr != nil {
			log.Warn().Err(execErr).Str("asset_id", assetID).Msg("could not update classification")
		}
	}
	a := assetFromFields(assetFields{
		ID: row.ID, OrgID: row.OrgID, Name: row.Name, Type: row.Type,
		Criticality: row.Criticality, Environment: newEnv, Tags: row.Tags,
		OwnerID: row.OwnerID, ExternalUrl: row.ExternalUrl,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	a.Classification = newCls
	return &a, nil
}

// SoftDeleteAsset marks an asset as deleted (is_deleted = TRUE).
func (r *Repository) SoftDeleteAsset(ctx context.Context, orgID, assetID string) error {
	n, err := r.q.SoftDeleteSPAsset(ctx, db.SoftDeleteSPAssetParams{ID: assetID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("soft delete asset: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("asset %w", apperr.ErrNotFound)
	}
	return nil
}

// GetAssetProtectionNeedID returns the protection_need_id soft-link for an asset, or nil if unlinked.
func (r *Repository) GetAssetProtectionNeedID(ctx context.Context, orgID, assetID string) (*string, error) {
	var pnaID *string
	// S121: vb_assets has no deleted_at column — soft-delete is the is_deleted
	// boolean. The old `deleted_at IS NULL` 500'd for every asset (SQLSTATE 42703),
	// found by the live route sweep.
	err := r.db.QueryRow(ctx,
		`SELECT protection_need_id FROM vb_assets
		 WHERE id = $1::uuid AND org_id = $2::uuid AND is_deleted = FALSE`,
		assetID, orgID,
	).Scan(&pnaID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get asset protection need id: %w", err)
	}
	return pnaID, nil
}

// BulkCreateAssets inserts multiple assets and returns the number inserted, the
// number of rows rejected, and one message per rejected row.
//
// Each row gets its own savepoint (S126). Without one, a single bad row took the
// whole import down: in Postgres the first failing statement aborts the
// transaction, so every subsequent row failed with 25P02 and COMMIT came back as
// "commit unexpectedly resulted in rollback". A CSV with 500 good rows and one
// typo imported nothing, reported all 501 rows as failed, and handed the user a
// single error message about a rollback instead of the row to fix. The intent was
// always partial import — the per-row error collection and the (inserted, errored,
// errs) signature say so — the transaction just made it impossible.
//
// A savepoint keeps that intent and the atomicity that matters: a bad row rolls
// back to just before itself and the good rows around it still commit, while a
// genuine infrastructure failure still rolls the whole thing back.
func (r *Repository) BulkCreateAssets(ctx context.Context, orgID string, rows []CSVAssetRow) (int, int, []string) {
	var inserted, errored int
	var errs []string

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, len(rows), []string{fmt.Sprintf("begin transaction: %s", err)}
	}
	defer func() {
		_ = tx.Rollback(ctx) // no-op when Commit succeeded
	}()

	for i, row := range rows {
		tags := row.Tags
		if tags == nil {
			tags = []string{}
		}

		// A nested Begin on a pgx transaction is a SAVEPOINT.
		sp, spErr := tx.Begin(ctx)
		if spErr != nil {
			return 0, len(rows), []string{fmt.Sprintf("savepoint: %s", spErr)}
		}

		_, rowErr := r.q.WithTx(sp).CreateSPAsset(ctx, db.CreateSPAssetParams{
			OrgID:       orgID,
			Name:        row.Name,
			Type:        row.Type,
			Criticality: row.Criticality,
			Tags:        tags,
			OwnerID:     pgtype.UUID{}, // bulk import has no owner
			ExternalUrl: optText(row.ExternalURL),
		})
		if rowErr != nil {
			_ = sp.Rollback(ctx) // ROLLBACK TO SAVEPOINT — the outer tx stays usable
			errored++
			errs = append(errs, fmt.Sprintf("row %d (%q): %s", i+1, row.Name, rowErr))
			continue
		}
		if spErr := sp.Commit(ctx); spErr != nil {
			errored++
			errs = append(errs, fmt.Sprintf("row %d (%q): %s", i+1, row.Name, spErr))
			continue
		}
		inserted++
	}

	if err = tx.Commit(ctx); err != nil {
		return 0, len(rows), []string{fmt.Sprintf("commit: %s", err)}
	}
	return inserted, errored, errs
}
