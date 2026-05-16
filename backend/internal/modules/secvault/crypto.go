// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package secvault re-exports the shared crypto primitives so that internal
// secvault code can call Encrypt/Decrypt/DeriveProjectKey without importing
// the shared package by a different path.
package secvault

import sharedcrypto "github.com/sechealth-app/sechealth/internal/shared/crypto"

// Re-exports from shared/crypto — keeps the secvault-internal API stable
// while the canonical implementation lives in shared/crypto.
var (
	Encrypt          = sharedcrypto.Encrypt
	Decrypt          = sharedcrypto.Decrypt
	DeriveProjectKey = sharedcrypto.DeriveProjectKey
)
