// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package events

import "context"

// AssetProtectionLinker writes the protection_need_id reverse soft-link on an
// inventory asset (vb_assets). It was introduced when the inventory lived in
// vaktscan and vaktcomply could not write the vb_ prefix ([[ADR-0079]] /
// ADR-0004). Since module-archivieren S1 the implementation lives in
// vaktcomply/assets (see assets.NewAssetProtectionLinker); the interface stays
// so the protection-need code is unchanged.
type AssetProtectionLinker interface {
	// SetAssetProtectionNeed points vb_assets.protection_need_id at pnaID for the
	// given asset. It is best-effort by design (mirrors the historic reverse-link
	// write): a missing asset is not an error, so linking never blocks on it.
	SetAssetProtectionNeed(ctx context.Context, orgID, assetID, pnaID string) error
}

// NoopAssetProtectionLinker satisfies AssetProtectionLinker without doing
// anything. It is the default when no linker is wired (tests, worker
// constructors that never link) — the forward link on the ck_ side still writes.
type NoopAssetProtectionLinker struct{}

// SetAssetProtectionNeed does nothing and never fails.
func (NoopAssetProtectionLinker) SetAssetProtectionNeed(context.Context, string, string, string) error {
	return nil
}
