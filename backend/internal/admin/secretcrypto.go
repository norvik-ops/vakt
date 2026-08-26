// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package admin

import (
	"bytes"
	"encoding/base64"
	"fmt"

	sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"
)

// Admin-managed secrets used to be sealed with the RAW master key, while every
// other encrypted column in the product moved to an HKDF sub-key years ago
// (ADR-0058). Six columns were left behind — org_oidc_configs.client_secret_enc
// and organizations.{smtp_pass_enc, ldap_bind_pass_enc, backup_passphrase_enc,
// backup_notify_webhook_enc, backup_dest_config_enc}. Because cmd/rotate-key
// derives a sub-key per purpose and has no stage that knows the raw master, a
// key swap left all six PERMANENTLY undecryptable: nothing could open them
// afterwards, and nothing said so.
//
// The fix follows the route already taken for SAML (ADR-0038): derive a
// dedicated sub-key, mark the new format, and keep reading the legacy format
// until the stock has been carried across.

// SecretKeyPurpose is the HKDF purpose for admin-managed secrets.
//
// Exported because cmd/rotate-key must agree on both values byte for byte, and
// a wire format that two binaries share should be stated once, in the package
// that owns it, rather than guessed twice. rotate-key still does not IMPORT
// this package outside of tests (that would drag the Echo handlers into the
// tool); it duplicates the literals and pins them here.
const SecretKeyPurpose = "vakt-admin-v1"

// SecretMarker marks a value sealed with the derived admin sub-key.
//
// The marker is what makes re-keying idempotent and the storage format
// self-describing: a value either carries it (derived key) or it does not
// (legacy raw master). There is deliberately no second source of truth — no
// version column, no state table — because a marker on the row itself cannot
// drift away from the row it describes.
//
// It must NOT collide with crypto.aadMarkerV2 ("enc:v2:"), which DecryptWithAAD
// parses, nor with the legacy OIDC wrapper ("enc:v1:") handled below.
// Duplicated in cmd/rotate-key (adminsecrets.go) on purpose — importing this
// package from a cmd/* tree would drag the Echo handlers in — and pinned there
// by TestAdminSecretConstantsMatchRuntime.
const SecretMarker = "enc:adm1:"

var adminSecretPrefix = []byte(SecretMarker)

// legacyOIDCPrefix is the wrapper UpsertOIDCConfig wrote before this change:
// the ASCII string "enc:v1:" followed by base64(ciphertext), stored in a BYTEA
// column. The other five columns stored raw ciphertext bytes with no wrapper.
var legacyOIDCPrefix = []byte("enc:v1:")

// sealSecret encrypts an admin-managed secret under the derived sub-key and
// prefixes the marker. Every writer of the six columns goes through here.
func (s *Service) sealSecret(plain []byte) ([]byte, error) {
	key, err := s.adminSecretKey()
	if err != nil {
		return nil, err
	}
	ct, err := sharedcrypto.Encrypt(key, plain)
	if err != nil {
		return nil, fmt.Errorf("seal admin secret: %w", err)
	}
	return append(append([]byte{}, adminSecretPrefix...), ct...), nil
}

// openSecret opens a stored admin secret in whichever of the three formats it
// is in. The legacy paths are what makes the change safe to deploy BEFORE the
// stock has been re-keyed: an instance that upgrades and never runs the re-key
// keeps working, and each write lazily carries its own row across.
//
//	enc:adm1:<ct>   — derived sub-key (current)
//	enc:v1:<b64ct>  — legacy raw master, OIDC client secret only
//	<ct>            — legacy raw master, the other five columns
func (s *Service) openSecret(stored []byte) ([]byte, error) {
	if len(stored) == 0 {
		return nil, fmt.Errorf("open admin secret: empty value")
	}
	if bytes.HasPrefix(stored, adminSecretPrefix) {
		key, err := s.adminSecretKey()
		if err != nil {
			return nil, err
		}
		plain, err := sharedcrypto.Decrypt(key, stored[len(adminSecretPrefix):])
		if err != nil {
			return nil, fmt.Errorf("open admin secret (derived key): %w", err)
		}
		return plain, nil
	}
	if len(s.masterKey) == 0 {
		return nil, fmt.Errorf("open admin secret: master key not configured")
	}
	if bytes.HasPrefix(stored, legacyOIDCPrefix) {
		ct, err := base64.URLEncoding.DecodeString(string(stored[len(legacyOIDCPrefix):]))
		if err != nil {
			return nil, fmt.Errorf("open admin secret (legacy oidc base64): %w", err)
		}
		plain, err := sharedcrypto.Decrypt(s.masterKey, ct)
		if err != nil {
			return nil, fmt.Errorf("open admin secret (legacy oidc): %w", err)
		}
		return plain, nil
	}
	plain, err := sharedcrypto.Decrypt(s.masterKey, stored)
	if err != nil {
		return nil, fmt.Errorf("open admin secret (legacy raw master): %w", err)
	}
	return plain, nil
}

// adminSecretKey returns the HKDF-derived sub-key, deriving it on first use.
// Kept separate from the raw master, which stays on the Service because the
// internal backup-config endpoint uses hex(masterKey) as its shared bearer
// token — swapping that value out would break the backup sidecar's auth.
func (s *Service) adminSecretKey() ([]byte, error) {
	if len(s.secretKey) > 0 {
		return s.secretKey, nil
	}
	if len(s.masterKey) == 0 {
		return nil, fmt.Errorf("admin secret key: master key not configured")
	}
	key, err := sharedcrypto.DeriveServiceKey(s.masterKey, SecretKeyPurpose)
	if err != nil {
		return nil, fmt.Errorf("admin secret key: %w", err)
	}
	s.secretKey = key
	return s.secretKey, nil
}
