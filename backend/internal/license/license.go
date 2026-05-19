// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package license

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"strings"
	"time"
)

const (
	FeatureTISAX     = "tisax"
	FeatureDORA      = "dora"
	FeatureEUAIAct   = "eu_ai_act"
	FeatureCRA       = "cra"
	FeatureAIAdvisor = "ai_advisor"
	FeatureAuditPDF  = "audit_pdf"
	FeatureSSO       = "sso"
	FeatureAPI       = "api_access"
	// FeatureSecReflex gates advanced phishing campaign management, detailed analytics,
	// template management, and target group management. Basic training assignment is Community.
	FeatureSecReflex = "secreflex_advanced"
	// FeatureSecPulse gates SBOM scanning, EOL tracking, and report generation/export.
	// Basic findings listing and the vulnerability dashboard remain Community.
	FeatureSecPulse = "secpulse_advanced"
	// FeatureGranularPermissions gates the per-user module permission management API.
	// Admins on Community may still see permissions; only writing (PUT) is gated.
	FeatureGranularPermissions = "granular_permissions"
	// FeatureSupplierPortal gates the full supplier register, assessments, and external portal.
	// Reading a shared portal link (/supplier/:token GET) remains public.
	FeatureSupplierPortal = "supplier_portal"
	// FeatureNIS2Reporting gates NIS2/BSI incident reportability assessment, deadline tracking,
	// and the structured notification form generator.
	FeatureNIS2Reporting = "nis2_reporting"
)

var publicKeyPEM = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE3telTJDYBVT/H7T79l1A6PUGpLyM
4eb/dvUwzB4Ua/HmZCVglQCVq7G3hIV9ToUedzyeiNtO6CZqttDBuv46Ow==
-----END PUBLIC KEY-----`

var allFeatures = []string{
	FeatureTISAX,
	FeatureDORA,
	FeatureEUAIAct,
	FeatureCRA,
	FeatureAIAdvisor,
	FeatureAuditPDF,
	FeatureSSO,
	FeatureAPI,
	FeatureSecReflex,
	FeatureSecPulse,
	FeatureGranularPermissions,
	FeatureSupplierPortal,
	FeatureNIS2Reporting,
}

// License describes the capabilities granted to this Vakt instance.
type License struct {
	Tier      string
	Features  []string
	OrgName   string
	IssuedAt  time.Time
	ExpiresAt *time.Time
	Demo      bool
	// Revoked is true when the org's subscription has been cancelled/refunded and
	// found in ls_revoked_subscriptions. The license is downgraded to community but
	// the frontend can use this flag to show a targeted cancellation message.
	Revoked bool
}

// payload is the JSON structure embedded in a license key.
type payload struct {
	Tier     string   `json:"tier"`
	Features []string `json:"features"`
	Org      string   `json:"org"`
	IssuedAt int64    `json:"iat"`
	Exp      *int64   `json:"exp,omitempty"`
}

// Load parses a license key and returns the resulting License.
// If isDemo is true a full-feature demo license is returned regardless of the key.
// An empty key or any parse/verification error yields a community license.
func Load(licenseKey string, isDemo bool) *License {
	if isDemo {
		return demoLicense()
	}
	if licenseKey == "" {
		return communityLicense()
	}
	lic, err := parse(licenseKey)
	if err != nil {
		return communityLicense()
	}
	return lic
}

// Has reports whether the license grants the named feature.
func (l *License) Has(feature string) bool {
	if l == nil {
		return false
	}
	if l.Demo {
		return true
	}
	for _, f := range l.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// IsPro reports whether the license is a Pro-tier license.
func (l *License) IsPro() bool {
	if l == nil {
		return false
	}
	return l.Demo || l.Tier == "pro"
}

func communityLicense() *License {
	return &License{
		Tier:     "community",
		Features: []string{},
		IssuedAt: time.Now(),
	}
}

func demoLicense() *License {
	return &License{
		Tier:     "pro",
		Features: allFeatures,
		OrgName:  "Demo",
		IssuedAt: time.Now(),
		Demo:     true,
	}
}

func parse(key string) (*License, error) {
	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return nil, errInvalidFormat
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	pub, err := loadPublicKey()
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256([]byte(parts[0]))
	if !verifySignature(pub, hash[:], sigBytes) {
		return nil, errInvalidSignature
	}

	var p payload
	if err := json.Unmarshal(payloadBytes, &p); err != nil {
		return nil, err
	}

	var expiresAt *time.Time
	if p.Exp != nil {
		t := time.Unix(*p.Exp, 0).UTC()
		if time.Now().After(t) {
			return nil, errExpired
		}
		expiresAt = &t
	}

	return &License{
		Tier:      p.Tier,
		Features:  p.Features,
		OrgName:   p.Org,
		IssuedAt:  time.Unix(p.IssuedAt, 0).UTC(),
		ExpiresAt: expiresAt,
	}, nil
}

func loadPublicKey() (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return nil, errInvalidKey
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	ecPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, errInvalidKey
	}
	return ecPub, nil
}

// verifySignature checks an ASN.1 DER-encoded ECDSA signature.
// The signature format produced by the generator is raw r||s (32 bytes each).
func verifySignature(pub *ecdsa.PublicKey, hash, sig []byte) bool {
	if len(sig) != 64 {
		return false
	}
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	return ecdsa.Verify(pub, hash, r, s)
}

var (
	errInvalidFormat    = licenseError("invalid license key format")
	errInvalidSignature = licenseError("license signature verification failed")
	errInvalidKey       = licenseError("invalid public key")
	errExpired          = licenseError("license has expired")
)

type licenseError string

func (e licenseError) Error() string { return string(e) }
