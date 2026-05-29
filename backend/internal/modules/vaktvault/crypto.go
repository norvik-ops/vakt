// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package vaktvault re-exports the shared crypto primitives so that internal
// vaktvault code can call Encrypt/Decrypt/DeriveProjectKey without importing
// the shared package by a different path.
package vaktvault

import sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"

// Re-exports from shared/crypto — keeps the vaktvault-internal API stable
// while the canonical implementation lives in shared/crypto.
var (
	Encrypt          = sharedcrypto.Encrypt
	Decrypt          = sharedcrypto.Decrypt
	DeriveProjectKey = sharedcrypto.DeriveProjectKey
)
