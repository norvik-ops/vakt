// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package license

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
	"time"
)

// Sign creates a signed license key from a PEM-encoded ECDSA private key.
// The PEM may use literal \n escapes (as stored in env vars) or real newlines.
// expires is optional — pass nil for a perpetual license.
func Sign(privateKeyPEM, tier, org string, features []string, expires *time.Time) (string, error) {
	privateKeyPEM = strings.ReplaceAll(privateKeyPEM, `\n`, "\n")
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return "", fmt.Errorf("license: no PEM block found in private key")
	}
	privKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("license: parse private key: %w", err)
	}
	return signWith(privKey, tier, org, features, expires)
}

func signWith(privKey *ecdsa.PrivateKey, tier, org string, features []string, expires *time.Time) (string, error) {
	p := payload{
		Tier:     tier,
		Features: features,
		Org:      org,
		IssuedAt: time.Now().UTC().Unix(),
	}
	if expires != nil {
		exp := expires.UTC().Unix()
		p.Exp = &exp
	}

	payloadJSON, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("license: marshal payload: %w", err)
	}

	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)
	hash := sha256.Sum256([]byte(payloadB64))

	r, s, err := ecdsa.Sign(rand.Reader, privKey, hash[:])
	if err != nil {
		return "", fmt.Errorf("license: sign: %w", err)
	}

	sig := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):64], sBytes)

	return payloadB64 + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}
