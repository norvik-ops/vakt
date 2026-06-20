// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S100-7 / ARCH-L01: Benchmarks and property-based invariant tests for the
// critical crypto path.
//
// Benchmarks: go test -bench=. -benchmem ./internal/shared/crypto/
//
// Full property-based testing with pgregory.net/rapid is planned once the
// dependency is approved (ARCH-L01 follow-up). The table-driven tests below
// cover the same invariant space without an external generator library.

package crypto

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func mustRandomKey(b *testing.B) []byte {
	b.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(b, err)
	return key
}

func mustRandomBytes(b *testing.B, n int) []byte {
	b.Helper()
	buf := make([]byte, n)
	_, err := rand.Read(buf)
	require.NoError(b, err)
	return buf
}

func randomKeyT(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	return key
}

func randomBytesT(t *testing.T, n int) []byte {
	t.Helper()
	buf := make([]byte, n)
	_, err := rand.Read(buf)
	require.NoError(t, err)
	return buf
}

// ── Benchmarks: Encrypt / Decrypt (no AAD) ───────────────────────────────────

func BenchmarkEncrypt_1KB(b *testing.B) {
	key := mustRandomKey(b)
	pt := mustRandomBytes(b, 1024)
	b.ResetTimer()
	b.SetBytes(int64(len(pt)))
	for range b.N {
		ct, err := Encrypt(key, pt)
		if err != nil || ct == nil {
			b.Fatalf("Encrypt failed: %v", err)
		}
	}
}

func BenchmarkDecrypt_1KB(b *testing.B) {
	key := mustRandomKey(b)
	ct, err := Encrypt(key, mustRandomBytes(b, 1024))
	require.NoError(b, err)
	b.ResetTimer()
	b.SetBytes(int64(len(ct)))
	for range b.N {
		pt, decErr := Decrypt(key, ct)
		if decErr != nil || pt == nil {
			b.Fatalf("Decrypt failed: %v", decErr)
		}
	}
}

func BenchmarkEncryptDecrypt_RoundTrip_1KB(b *testing.B) {
	key := mustRandomKey(b)
	plain := mustRandomBytes(b, 1024)
	b.ResetTimer()
	b.SetBytes(int64(len(plain)))
	for range b.N {
		ct, err := Encrypt(key, plain)
		if err != nil {
			b.Fatalf("Encrypt failed: %v", err)
		}
		if _, err = Decrypt(key, ct); err != nil {
			b.Fatalf("Decrypt failed: %v", err)
		}
	}
}

// BenchmarkEncrypt_Sizes sweeps payload sizes to show throughput scaling.
func BenchmarkEncrypt_Sizes(b *testing.B) {
	key := mustRandomKey(b)
	for _, size := range []int{64, 512, 4096, 65536} {
		pt := mustRandomBytes(b, size)
		b.Run(fmt.Sprintf("%dB", size), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ResetTimer()
			for range b.N {
				_, _ = Encrypt(key, pt)
			}
		})
	}
}

// ── Benchmarks: EncryptWithAAD / DecryptWithAAD (v2) ─────────────────────────

func BenchmarkEncryptWithAAD_1KB(b *testing.B) {
	key := mustRandomKey(b)
	pt := mustRandomBytes(b, 1024)
	aad := []byte("org-abc123:secret-def456")
	b.ResetTimer()
	b.SetBytes(int64(len(pt)))
	for range b.N {
		ct, err := EncryptWithAAD(key, pt, aad)
		if err != nil || ct == nil {
			b.Fatalf("EncryptWithAAD failed: %v", err)
		}
	}
}

func BenchmarkDecryptWithAAD_1KB(b *testing.B) {
	key := mustRandomKey(b)
	aad := []byte("org-abc123:secret-def456")
	ct, err := EncryptWithAAD(key, mustRandomBytes(b, 1024), aad)
	require.NoError(b, err)
	b.ResetTimer()
	b.SetBytes(int64(len(ct)))
	for range b.N {
		pt, decErr := DecryptWithAAD(key, ct, aad)
		if decErr != nil || pt == nil {
			b.Fatalf("DecryptWithAAD failed: %v", decErr)
		}
	}
}

// ── Benchmarks: HKDF key derivation ──────────────────────────────────────────

func BenchmarkDeriveProjectKey(b *testing.B) {
	master := mustRandomKey(b)
	b.ResetTimer()
	for i := range b.N {
		if _, err := DeriveProjectKey(master, fmt.Sprintf("project-%d", i)); err != nil {
			b.Fatalf("DeriveProjectKey failed: %v", err)
		}
	}
}

func BenchmarkDeriveServiceKey(b *testing.B) {
	master := mustRandomKey(b)
	b.ResetTimer()
	for range b.N {
		if _, err := DeriveServiceKey(master, "vakt-vault-v1"); err != nil {
			b.Fatalf("DeriveServiceKey failed: %v", err)
		}
	}
}

// ── Property-based invariant tests (table-driven) ────────────────────────────
//
// These encode the fundamental crypto invariants that pgregory.net/rapid would
// generate automatically. The test runner exercises N payload sizes and AAD
// variants to catch edge-cases (zero-length, exactly-nonce-size, etc.).
// Planned upgrade path: replace with rapid generators once approved (ARCH-L01).

func TestProperty_EncryptDecryptRoundTrip_VariousLengths(t *testing.T) {
	key := randomKeyT(t)
	for _, size := range []int{0, 1, 11, 12, 13, 32, 255, 1024, 4097, 65536} {
		t.Run(fmt.Sprintf("size=%d", size), func(t *testing.T) {
			pt := randomBytesT(t, size)
			ct, err := Encrypt(key, pt)
			require.NoError(t, err, "Encrypt must not fail for %d-byte plaintext", size)

			recovered, err := Decrypt(key, ct)
			require.NoError(t, err, "Decrypt must not fail for %d-byte plaintext", size)
			// gcm.Open returns nil (not []byte{}) for zero-length plaintext; treat
			// both as semantically empty to avoid a spurious assertion failure.
			if size == 0 {
				assert.Empty(t, recovered, "round-trip of empty plaintext must return empty result")
			} else {
				assert.Equal(t, pt, recovered, "round-trip must recover the original plaintext")
			}
		})
	}
}

func TestProperty_EncryptWithAAD_RoundTrip_VariousAADs(t *testing.T) {
	key := randomKeyT(t)
	pt := []byte("invariant-test-payload")
	aads := [][]byte{
		nil,
		[]byte(""),
		[]byte("org-1:secret-1"),
		[]byte("org-a:secret-b:extra-context"),
		randomBytesT(t, 256),
	}
	for i, aad := range aads {
		t.Run(fmt.Sprintf("aad_%d", i), func(t *testing.T) {
			ct, err := EncryptWithAAD(key, pt, aad)
			require.NoError(t, err)

			recovered, err := DecryptWithAAD(key, ct, aad)
			require.NoError(t, err)
			assert.Equal(t, pt, recovered)
		})
	}
}

func TestProperty_WrongAAD_AlwaysFails(t *testing.T) {
	key := randomKeyT(t)
	pt := randomBytesT(t, 256)
	aad := []byte("correct-aad")
	ct, err := EncryptWithAAD(key, pt, aad)
	require.NoError(t, err)

	wrongAADs := [][]byte{
		nil,
		[]byte(""),
		[]byte("wrong-aad"),
		[]byte("correct-aa"),   // one byte short
		[]byte("correct-aad "), // one byte long
		randomBytesT(t, 16),
	}
	for i, wrong := range wrongAADs {
		t.Run(fmt.Sprintf("wrong_%d", i), func(t *testing.T) {
			_, decErr := DecryptWithAAD(key, ct, wrong)
			assert.Error(t, decErr,
				"decrypting with wrong AAD must fail (ciphertext is bound to a specific context)")
		})
	}
}

func TestProperty_LegacyCiphertexts_DecryptWithAnyAAD(t *testing.T) {
	key := randomKeyT(t)
	pt := []byte("legacy value — no marker, no AAD binding")
	legacyCT, err := Encrypt(key, pt)
	require.NoError(t, err)

	// Pre-v2 ciphertexts were never bound to an AAD — any AAD must be ignored.
	for i, aad := range [][]byte{nil, []byte("any"), randomBytesT(t, 32)} {
		t.Run(fmt.Sprintf("aad_%d", i), func(t *testing.T) {
			recovered, decErr := DecryptWithAAD(key, legacyCT, aad)
			require.NoError(t, decErr, "legacy ciphertext must decrypt regardless of AAD argument")
			assert.Equal(t, pt, recovered)
		})
	}
}
