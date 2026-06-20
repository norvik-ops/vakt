-- Fix: org_saml_configs.key_pem stores raw AES-GCM ciphertext (binary), but was
-- declared TEXT in migration 135. Raw ciphertext contains non-UTF8 bytes, which a
-- TEXT column rejects (SQLSTATE 22021) — so SAML-direct private-key storage was
-- broken for any real key. All code (auth/saml_direct.go, cmd/rotate-key) already
-- reads/writes key_pem as []byte, so aligning the column to BYTEA needs no code change.
-- Surfaced by the re-enabled key-rotation E2E (rotate_key_real_test.go) after the
-- S99-4 gate false-positive was fixed.
--
-- cert_pem stays TEXT: it holds the public PEM cert as plaintext (valid UTF-8).
-- No data migration needed: non-UTF8 ciphertext could never have been stored in
-- the TEXT column, so existing rows (if any) cast cleanly.
ALTER TABLE org_saml_configs
    ALTER COLUMN key_pem TYPE BYTEA USING key_pem::bytea;
