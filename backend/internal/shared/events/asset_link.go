// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package events

import "context"

// AssetProtectionLinker writes the protection_need_id reverse soft-link on a
// vaktscan asset (vb_assets). vaktcomply owns the forward link
// (ck_protection_need_assessments.vb_asset_id) but must not write the vb_ prefix
// itself — a module may only write its own table prefix (module isolation,
// [[ADR-0079]] / ADR-0004). vaktcomply calls this interface instead; the real
// implementation lives in vaktscan (see vaktscan.NewAssetProtectionLinker).
type AssetProtectionLinker interface {
	// SetAssetProtectionNeed points vb_assets.protection_need_id at pnaID for the
	// given asset. It is best-effort by design (mirrors the historic reverse-link
	// write): a missing asset is not an error, so linking never blocks on it.
	SetAssetProtectionNeed(ctx context.Context, orgID, assetID, pnaID string) error
}

// NoopAssetProtectionLinker satisfies AssetProtectionLinker without doing
// anything. It is the default when vaktscan is not wired (tests, worker
// constructors that never link) — the forward link on the ck_ side still writes.
type NoopAssetProtectionLinker struct{}

// SetAssetProtectionNeed does nothing and never fails.
func (NoopAssetProtectionLinker) SetAssetProtectionNeed(context.Context, string, string, string) error {
	return nil
}
