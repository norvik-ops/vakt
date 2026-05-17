package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"aidanwoods.dev/go-paseto"
)

const (
	AccessTokenTTL  = 1 * time.Hour
	RefreshTokenTTL = 30 * 24 * time.Hour
)

// Claims holds the user-identifying data embedded in access tokens.
type Claims struct {
	UserID string   `json:"user_id"`
	OrgID  string   `json:"org_id"`
	Roles  []string `json:"roles"`
}

// GenerateSymmetricKey creates a Paseto v4 symmetric key from a 32-byte hex-encoded secret.
func GenerateSymmetricKey(hexSecret string) (paseto.V4SymmetricKey, error) {
	raw, err := hex.DecodeString(hexSecret)
	if err != nil {
		return paseto.NewV4SymmetricKey(), fmt.Errorf("decode hex secret: %w", err)
	}
	key, err := paseto.V4SymmetricKeyFromBytes(raw)
	if err != nil {
		return paseto.NewV4SymmetricKey(), fmt.Errorf("create symmetric key: %w", err)
	}
	return key, nil
}

// IssueAccessToken creates a Paseto v4 local token containing the given Claims.
// The token expires after AccessTokenTTL.
func IssueAccessToken(key paseto.V4SymmetricKey, claims Claims) (string, error) {
	return IssueAccessTokenWithTTL(key, claims, AccessTokenTTL)
}

// IssueAccessTokenWithTTL creates a Paseto v4 local token with a custom TTL.
// Exposed so tests can mint already-expired tokens.
func IssueAccessTokenWithTTL(key paseto.V4SymmetricKey, claims Claims, ttl time.Duration) (string, error) {
	token := paseto.NewToken()
	now := time.Now()
	token.SetIssuedAt(now)
	token.SetExpiration(now.Add(ttl))
	token.SetString("user_id", claims.UserID)
	token.SetString("org_id", claims.OrgID)
	if err := token.Set("roles", claims.Roles); err != nil {
		return "", fmt.Errorf("set roles claim: %w", err)
	}
	return token.V4Encrypt(key, nil), nil
}

// IssueRefreshToken returns a cryptographically random 32-byte hex string.
// It is not a Paseto token; its SHA-256 hash is stored in Redis.
func IssueRefreshToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// ParseAccessToken validates the Paseto v4 local token and returns the embedded Claims.
// Returns an error if the token is malformed, tampered, or expired.
func ParseAccessToken(key paseto.V4SymmetricKey, tokenStr string) (*Claims, error) {
	parser := paseto.NewParser() // already includes NotExpired() rule
	token, err := parser.ParseV4Local(key, tokenStr, nil)
	if err != nil {
		return nil, fmt.Errorf("parse access token: %w", err)
	}

	userID, err := token.GetString("user_id")
	if err != nil {
		return nil, fmt.Errorf("get user_id claim: %w", err)
	}
	orgID, err := token.GetString("org_id")
	if err != nil {
		return nil, fmt.Errorf("get org_id claim: %w", err)
	}

	var roles []string
	if err := token.Get("roles", &roles); err != nil {
		return nil, fmt.Errorf("get roles claim: %w", err)
	}

	return &Claims{
		UserID: userID,
		OrgID:  orgID,
		Roles:  roles,
	}, nil
}
