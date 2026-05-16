package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

const (
	totpIssuer      = "Vakt"
	backupCodeCount = 8
)

// GenerateTOTPSecret creates a new TOTP secret for a user.
// Returns the base32-encoded secret, the otpauth provisioning URI (for QR-code), and any error.
func GenerateTOTPSecret(email, issuer string) (secret, uri string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: email,
	})
	if err != nil {
		return "", "", fmt.Errorf("generate totp secret: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

// ValidateTOTP checks a 6-digit TOTP code against the base32-encoded secret.
// Uses a ±1 window to tolerate small clock skews.
func ValidateTOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}

// GenerateBackupCodes creates 8 single-use backup codes in the format XXXX-XXXX.
// Returns the plaintext codes (for display) and their bcrypt hashes (for storage).
func GenerateBackupCodes() (plainCodes []string, hashedCodes []string, err error) {
	plainCodes = make([]string, backupCodeCount)
	hashedCodes = make([]string, backupCodeCount)

	for i := range plainCodes {
		raw := make([]byte, 4)
		if _, e := rand.Read(raw); e != nil {
			return nil, nil, fmt.Errorf("generate backup code random bytes: %w", e)
		}
		hex1 := strings.ToUpper(hex.EncodeToString(raw[:2]))
		hex2 := strings.ToUpper(hex.EncodeToString(raw[2:]))
		plain := hex1 + "-" + hex2
		plainCodes[i] = plain

		hash, e := bcrypt.GenerateFromPassword([]byte(plain), 12)
		if e != nil {
			return nil, nil, fmt.Errorf("hash backup code: %w", e)
		}
		hashedCodes[i] = string(hash)
	}
	return plainCodes, hashedCodes, nil
}

// CheckBackupCode tests a candidate code against a slice of bcrypt-hashed backup codes.
// Returns the index of the matching code (for removal) or -1 if none match.
func CheckBackupCode(candidate string, hashedCodes []string) int {
	for i, h := range hashedCodes {
		if bcrypt.CompareHashAndPassword([]byte(h), []byte(candidate)) == nil {
			return i
		}
	}
	return -1
}
