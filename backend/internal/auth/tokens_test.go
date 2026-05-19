package auth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
)

// testHexKey is a valid 32-byte hex-encoded key for tests.
const testHexKey = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

func TestGenerateSymmetricKey(t *testing.T) {
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)
	// Key should be usable — verify by round-tripping a token.
	claims := auth.Claims{
		UserID: "user-1",
		OrgID:  "org-1",
		Roles:  []string{"Admin"},
	}
	tok, err := auth.IssueAccessToken(key, claims)
	require.NoError(t, err)
	assert.NotEmpty(t, tok)
}

func TestGenerateSymmetricKey_InvalidHex(t *testing.T) {
	_, err := auth.GenerateSymmetricKey("not-hex!")
	assert.Error(t, err)
}

func TestIssueAndParseAccessToken(t *testing.T) {
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)

	claims := auth.Claims{
		UserID: "user-abc",
		OrgID:  "org-xyz",
		Roles:  []string{"SecurityAnalyst", "Viewer"},
	}

	tok, err := auth.IssueAccessToken(key, claims)
	require.NoError(t, err)
	assert.NotEmpty(t, tok)

	parsed, err := auth.ParseAccessToken(key, tok)
	require.NoError(t, err)
	require.NotNil(t, parsed)

	assert.Equal(t, claims.UserID, parsed.UserID)
	assert.Equal(t, claims.OrgID, parsed.OrgID)
	assert.Equal(t, claims.Roles, parsed.Roles)
}

func TestParseAccessToken_Expired(t *testing.T) {
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)

	// Issue a token that is already expired.
	tok, err := auth.IssueAccessTokenWithTTL(key, auth.Claims{
		UserID: "user-expired",
		OrgID:  "org-1",
		Roles:  []string{"Viewer"},
	}, -1*time.Second)
	require.NoError(t, err)

	_, err = auth.ParseAccessToken(key, tok)
	assert.Error(t, err, "expired token should return error")
}

func TestParseAccessToken_InvalidToken(t *testing.T) {
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)

	_, err = auth.ParseAccessToken(key, "not.a.valid.paseto.token")
	assert.Error(t, err)
}

func TestIssueRefreshToken(t *testing.T) {
	tok1, err := auth.IssueRefreshToken()
	require.NoError(t, err)
	assert.NotEmpty(t, tok1)

	tok2, err := auth.IssueRefreshToken()
	require.NoError(t, err)

	assert.NotEqual(t, tok1, tok2, "each refresh token must be unique")
	// Should be 64 hex chars (32 bytes)
	assert.Len(t, tok1, 64)
}
