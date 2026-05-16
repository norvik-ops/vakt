package secvault

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeriveProjectKey_Deterministic(t *testing.T) {
	masterKey := bytes.Repeat([]byte{0xAB}, 32)
	projectID := "test-project-uuid"

	key1, err := DeriveProjectKey(masterKey, projectID)
	require.NoError(t, err)
	require.Len(t, key1, 32)

	key2, err := DeriveProjectKey(masterKey, projectID)
	require.NoError(t, err)

	require.Equal(t, key1, key2, "DeriveProjectKey must be deterministic")
}

func TestDeriveProjectKey_DifferentProjects(t *testing.T) {
	masterKey := bytes.Repeat([]byte{0xAB}, 32)

	key1, err := DeriveProjectKey(masterKey, "project-a")
	require.NoError(t, err)

	key2, err := DeriveProjectKey(masterKey, "project-b")
	require.NoError(t, err)

	require.NotEqual(t, key1, key2, "different project IDs must yield different keys")
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	plaintext := []byte("super-secret-value")

	ciphertext, err := Encrypt(key, plaintext)
	require.NoError(t, err)
	require.NotEqual(t, plaintext, ciphertext)

	decrypted, err := Decrypt(key, ciphertext)
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted)
}

func TestEncryptDecrypt_EmptyPlaintext(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, 32)

	ciphertext, err := Encrypt(key, []byte{})
	require.NoError(t, err)

	decrypted, err := Decrypt(key, ciphertext)
	require.NoError(t, err)
	// AES-GCM Open returns nil for empty plaintext; treat nil and empty as equal.
	require.Empty(t, decrypted)
}

func TestEncrypt_ProducesUniqueNonces(t *testing.T) {
	key := bytes.Repeat([]byte{0x77}, 32)
	plaintext := []byte("same plaintext")

	ct1, err := Encrypt(key, plaintext)
	require.NoError(t, err)

	ct2, err := Encrypt(key, plaintext)
	require.NoError(t, err)

	require.NotEqual(t, ct1, ct2, "repeated encryption must produce different ciphertexts")
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := bytes.Repeat([]byte{0x11}, 32)
	key2 := bytes.Repeat([]byte{0x22}, 32)

	ct, err := Encrypt(key1, []byte("secret"))
	require.NoError(t, err)

	_, err = Decrypt(key2, ct)
	require.Error(t, err)
}

func TestDecrypt_TruncatedCiphertext(t *testing.T) {
	key := bytes.Repeat([]byte{0x33}, 32)
	_, err := Decrypt(key, []byte{0x01, 0x02})
	require.Error(t, err)
}
