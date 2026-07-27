// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	sharedevents "github.com/matharnica/vakt/internal/shared/events"
)

// AssetProtectionLinker writes the protection_need_id reverse soft-link on
// vb_assets. It lets vaktcomply link a protection-need assessment to an asset
// without writing the vb_ prefix from its own package (module isolation,
// ADR-0079): vaktscan owns vb_assets and also reads this column back
// (GetAssetProtectionNeedID), so the write belongs here.
type AssetProtectionLinker struct {
	db *pgxpool.Pool
}

// NewAssetProtectionLinker returns a linker backed by the given pool, typed as
// the shared interface so cmd/api can inject it into vaktcomply.
func NewAssetProtectionLinker(pool *pgxpool.Pool) sharedevents.AssetProtectionLinker {
	return &AssetProtectionLinker{db: pool}
}

// SetAssetProtectionNeed points vb_assets.protection_need_id at pnaID. It is
// best-effort by design (mirrors the historic reverse-link write in vaktcomply):
// a zero-row result is not an error, so the forward-link response never blocks
// on a missing asset.
func (l *AssetProtectionLinker) SetAssetProtectionNeed(ctx context.Context, orgID, assetID, pnaID string) error {
	_, err := l.db.Exec(ctx,
		`UPDATE vb_assets SET protection_need_id = $1::uuid, updated_at = NOW()
		 WHERE id = $2::uuid AND org_id = $3::uuid`,
		pnaID, assetID, orgID,
	)
	return err
}
