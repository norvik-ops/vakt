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
	UserID    string   `json:"user_id"`
	OrgID     string   `json:"org_id"`
	Roles     []string `json:"roles"`
	PwVersion int64    `json:"pw_version"`
	// MFA is true when the session proved a second factor (TOTP/backup/recovery)
	// in THIS login, not merely that the user has TOTP enrolled (S124-1/SA14-01).
	// MFAEnforceMiddleware requires MFA==true when the org mandates MFA.
	MFA bool `json:"mfa"`
}

// MFAPendingTTL bounds the window between a correct password and the second
// factor. Short so a captured password + pending token is not a durable foothold.
const MFAPendingTTL = 5 * time.Minute

// IssueMFAPendingToken mints a short-lived token that proves ONLY that the
// password step succeeded. It carries no roles and is accepted exclusively by the
// MFA login-verify endpoint (ParseMFAPendingToken) — never by AuthMiddleware,
// which rejects every token bearing mfa_pending=true. This is the first leg of the
// two-stage login (S124-1).
func IssueMFAPendingToken(key paseto.V4SymmetricKey, userID, orgID string) (string, error) {
	token := paseto.NewToken()
	now := time.Now()
	token.SetIssuedAt(now)
	token.SetExpiration(now.Add(MFAPendingTTL))
	token.SetString("user_id", userID)
	token.SetString("org_id", orgID)
	if err := token.Set("mfa_pending", true); err != nil {
		return "", fmt.Errorf("set mfa_pending claim: %w", err)
	}
	return token.V4Encrypt(key, nil), nil
}

// ParseMFAPendingToken validates an mfa_pending token and returns its subject.
// It errors if the token is not a pending token (mfa_pending != true), is
// expired, or is a full access token — so a full token can never be replayed
// here and a pending token can never be used as a full token.
func ParseMFAPendingToken(key paseto.V4SymmetricKey, tokenStr string) (userID, orgID string, err error) {
	parser := paseto.NewParser()
	parsed, err := parser.ParseV4Local(key, tokenStr, nil)
	if err != nil {
		return "", "", fmt.Errorf("parse mfa pending token: %w", err)
	}
	var pending bool
	if err := parsed.Get("mfa_pending", &pending); err != nil || !pending {
		return "", "", fmt.Errorf("not an mfa pending token")
	}
	userID, err = parsed.GetString("user_id")
	if err != nil || userID == "" {
		return "", "", fmt.Errorf("mfa pending token missing user_id")
	}
	orgID, _ = parsed.GetString("org_id")
	return userID, orgID, nil
}

// GenerateSymmetricKey creates a Paseto v4 symmetric key from a 32-byte hex-encoded secret.
// Prefer GenerateSymmetricKeyFromBytes when a pre-derived key is already available.
func GenerateSymmetricKey(hexSecret string) (paseto.V4SymmetricKey, error) {
	raw, err := hex.DecodeString(hexSecret)
	if err != nil {
		return paseto.NewV4SymmetricKey(), fmt.Errorf("decode hex secret: %w", err)
	}
	return GenerateSymmetricKeyFromBytes(raw)
}

// GenerateSymmetricKeyFromBytes creates a Paseto v4 symmetric key from 32 raw bytes.
// Use this together with crypto.DeriveServiceKey("vakt-paseto-v1") so the PASETO
// signing key is domain-separated from the AES-256-GCM encryption keys.
func GenerateSymmetricKeyFromBytes(raw []byte) (paseto.V4SymmetricKey, error) {
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
	if err := token.Set("pw_version", claims.PwVersion); err != nil {
		return "", fmt.Errorf("set pw_version claim: %w", err)
	}
	if err := token.Set("mfa", claims.MFA); err != nil {
		return "", fmt.Errorf("set mfa claim: %w", err)
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

	// S124-1: an mfa_pending token is NOT a full access token — reject it here so
	// it can only ever be used at the login-verify endpoint, never to reach a
	// protected route directly.
	var mfaPending bool
	_ = token.Get("mfa_pending", &mfaPending)
	if mfaPending {
		return nil, fmt.Errorf("mfa pending token is not a valid access token")
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

	// pw_version may be absent in tokens minted before this feature was added;
	// treat a missing claim as version 0.
	var pwVersion int64
	_ = token.Get("pw_version", &pwVersion)

	// mfa is absent in tokens minted before S124-1; treat missing as false.
	var mfa bool
	_ = token.Get("mfa", &mfa)

	return &Claims{
		UserID:    userID,
		OrgID:     orgID,
		Roles:     roles,
		PwVersion: pwVersion,
		MFA:       mfa,
	}, nil
}
