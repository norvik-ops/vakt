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
	EncryptWithAAD   = sharedcrypto.EncryptWithAAD
	DecryptWithAAD   = sharedcrypto.DecryptWithAAD
	DeriveProjectKey = sharedcrypto.DeriveProjectKey
)

// SecretAAD builds the Associated Data that binds a stored secret's ciphertext
// to its (org, secret-row) context (S90-3, ADR-0059). A ciphertext copied to a
// different org or a different secret row fails the GCM tag check on decrypt.
//
// Exported because cmd/rotate-key must reconstruct exactly this AAD to re-encrypt
// a Vault secret under a new master key. That tool carries its own copy of the
// formula — importing this package would pull the whole HTTP module into the
// binary — and pins the copy against this function in a test. The two drifting
// apart would silently break key rotation for every Vault secret.
func SecretAAD(orgID, secretID string) []byte {
	return []byte(orgID + ":" + secretID)
}

// secretAAD is the package-internal spelling used by the service paths.
func secretAAD(orgID, secretID string) []byte {
	return SecretAAD(orgID, secretID)
}
