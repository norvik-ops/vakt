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
	FeatureTISAX   = "tisax"
	FeatureDORA    = "dora"
	FeatureEUAIAct = "eu_ai_act"
	FeatureCRA     = "cra"
	// FeatureAIAdvisor war vor v0.6.x ein Pro-Gate für die AI-Copilot-Endpunkte.
	// Seit v0.6.x ist AI Community: qwen2.5:3b läuft CPU-lokal in jeder Instanz,
	// das frühere Gate war Marketing-Limitierung ohne echten Schutz. Die Konstante
	// bleibt erhalten, weil ausgegebene Lizenzen sie noch im features-Array führen
	// — Lizenz-Validierung soll weiterhin erfolgreich sein.
	FeatureAIAdvisor = "ai_advisor"
	FeatureAuditPDF  = "audit_pdf"
	FeatureSSO       = "sso"
	FeatureAPI       = "api_access"
	// FeatureSecReflex gates advanced phishing campaign management, detailed analytics,
	// template management, and target group management. Basic training assignment is Community.
	FeatureSecReflex = "vaktaware_advanced"
	// FeatureSecPulse gates SBOM scanning, EOL tracking, and report generation/export.
	// Basic findings listing and the vulnerability dashboard remain Community.
	FeatureSecPulse = "vaktscan_advanced"
	// FeatureGranularPermissions gates the per-user module permission management API.
	// Admins on Community may still see permissions; only writing (PUT) is gated.
	FeatureGranularPermissions = "granular_permissions"
	// FeatureSupplierPortal gates the full supplier register, assessments, and external portal.
	// Reading a shared portal link (/supplier/:token GET) remains public.
	FeatureSupplierPortal = "supplier_portal"
	// FeatureNIS2Reporting gates NIS2/BSI incident reportability assessment, deadline tracking,
	// and the structured notification form generator.
	FeatureNIS2Reporting = "nis2_reporting"
	// FeatureSecVault gates advanced vault workflows: secret rotation, git leak scans,
	// and access reviews. Basic secret storage (projects, envs, secret CRUD, sharing,
	// import/export) remains Community.
	FeatureSecVault = "vaktvault_advanced"
	// FeatureSecPrivacy gates advanced privacy workflows: DPIA management, transfer
	// impact assessments (TIA/Schrems II), deletion reminders, and privacy-by-design
	// assessments. VVT, AVV register, breach register, and DSR handling remain Community.
	FeatureSecPrivacy = "vaktprivacy_advanced"
	// FeatureBSIGrundschutz gates the BSI IT-Grundschutz workflow: enabling the BSI
	// framework, Baustein modelling, target objects (Strukturanalyse), Grundschutz-Check,
	// 200-3 risk assessments, cockpit/GAP report, and reference reports.
	FeatureBSIGrundschutz = "bsi_grundschutz"
	// FeatureISO42001 gates enabling the ISO/IEC 42001 AI-management framework.
	// Not sold: see features.UnsoldFeatures.
	FeatureISO42001 = "iso_42001"
	// The following features were introduced in the platform feature registry
	// (shared/platform/features/flags.go) and are mirrored here so issued license
	// keys can carry them. flags.go aliases these constants.
	FeatureSAMLAuth         = "saml_auth"
	FeatureSCIMProvisioning = "scim_provisioning"
	FeatureSIEM             = "siem_export"
	FeatureAgentWriteTools  = "agent_write_tools"
	FeatureMultiFramework   = "multi_framework"
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
	FeatureSecVault,
	FeatureSecPrivacy,
	FeatureBSIGrundschutz,
	FeatureISO42001,
	FeatureGranularPermissions,
	FeatureSupplierPortal,
	FeatureNIS2Reporting,
	FeatureSAMLAuth,
	FeatureSCIMProvisioning,
	FeatureSIEM,
	FeatureAgentWriteTools,
	FeatureMultiFramework,
}

// AllFeatures returns every feature string a signed key can carry.
//
// It is the denominator for the tier-coverage test in
// internal/shared/platform/features: a feature that exists here but sits in no
// sellable tier cannot be bought by anyone, so every route gated on it answers
// 402 for every customer (R1-17-L01). The slice is copied so a caller cannot
// mutate the package-level list.
func AllFeatures() []string {
	out := make([]string, len(allFeatures))
	copy(out, allFeatures)
	return out
}

// License describes the capabilities granted to this Vakt instance.
type License struct {
	Tier      string
	Features  []string
	OrgName   string
	IssuedAt  time.Time
	ExpiresAt *time.Time
	Demo      bool

	// RenewalToken is carried inside the signed key, so the instance can fetch its
	// own next key when this one runs low without the customer configuring anything.
	// Empty on keys signed before this existed, and on CLI-issued keys.
	RenewalToken string

	// Expired is true when the license key has passed its ExpiresAt timestamp.
	// Unlike a community license, an expired Pro license retains read
	// access to Pro-module data (GET routes succeed); write operations return 402.
	// This prevents data lock-out after subscription lapse.
	Expired bool
}

// payload is the JSON structure embedded in a license key.
type payload struct {
	Tier     string   `json:"tier"`
	Features []string `json:"features"`
	Org      string   `json:"org"`
	IssuedAt int64    `json:"iat"`
	Exp      *int64   `json:"exp,omitempty"`

	// RenewalToken lets the instance fetch its OWN next key when this one is about
	// to run out — without the customer having to configure anything.
	//
	// It lives inside the signed key on purpose. The alternative was a second env
	// var (VAKT_LICENSE_TOKEN), which meant renewal only worked for the customers who
	// happened to read that part of the mail. Everyone else had to paste a new key by
	// hand, forever. One paste, and it just works.
	//
	// This is signed along with everything else, so it cannot be swapped for someone
	// else's token. Empty on old keys and on keys signed by the admin CLI — the
	// instance then simply never calls, exactly as before.
	RenewalToken string `json:"rt,omitempty"`
}

// sellableTiers is the complete set of tiers this platform issues.
//
// There are two, and there is no third. "enterprise" was removed on 2026-08-08:
// no signing path ever set it (billing/licensing/service.go hard-codes "pro" on
// both paths, the admin CLI has no --tier at all), so no customer key in the
// field can carry it — while the price list sold three frameworks behind it
// (R1-17-02). A tier nobody can buy is not a tier.
//
// The set is the gate for parse(): a validly SIGNED key whose tier is not in
// here is rejected and Load falls back to community. Fail closed on purpose —
// the alternative (keep the tier string, ignore it) would leave the key's own
// Features array in force, so a hand-signed "enterprise" key would still unlock
// everything it listed. Signature validity says who minted a key, not what the
// platform still sells.
var sellableTiers = map[string]bool{
	"community": true,
	"pro":       true,
}

// KnownTier reports whether tier is one this platform issues and honours.
// Exported so the signing side can refuse to mint what the loading side will
// throw away — a key that verifies but never grants anything is the worst of
// both worlds: it looks bought and behaves free.
func KnownTier(tier string) bool { return sellableTiers[tier] }

// Load parses a license key and returns the resulting License.
// If isDemo is true a full-feature demo license is returned regardless of the key.
// An empty key or any parse/verification error yields a community license — and
// so does a correctly signed key carrying a tier this platform no longer sells.
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

// legacyProFeatures lists features that every Pro license grants implicitly,
// even when the key was issued before these feature strings were introduced.
// A Pro key issued via Polar.sh before a new feature string was added to
// proFeatures (e.g. bsi_grundschutz, vaktvault_advanced, vaktprivacy_advanced)
// would otherwise receive HTTP 402 on previously-free routes.
//
// Add a feature here when it is included in the current proFeatures list
// (polar/handler.go) but was NOT present in all historically issued Pro keys.
// Remove entries once auto-renewal has had enough time to reissue all active keys
// (typically 90 days after adding to proFeatures).
var legacyProFeatures = []string{
	FeatureBSIGrundschutz,  // added S74; old Pro keys lack this string
	FeatureSecVault,        // added S70; old Pro keys lack this string
	FeatureSecPrivacy,      // added S70; old Pro keys lack this string
	FeatureAgentWriteTools, // added S79-4; old Pro keys lack this string
}

// Has reports whether the license grants the named feature.
// For Pro licenses, features in legacyProFeatures are always granted
// even when not explicitly listed in the key — this prevents 402 errors for
// customers who purchased before the feature strings were added to Pro keys.
//
// An expired license returns false for all features so that write operations
// are blocked. Do NOT call Has directly from a gate — call Allows, which layers
// the read-only carve-out for expired keys on top of it. Has answers "is this
// feature live", Allows answers "may this request use it", and only the second
// question is the one a gate is asking.
func (l *License) Has(feature string) bool {
	if l == nil {
		return false
	}
	if l.Demo {
		return true
	}
	if l.Expired {
		return false
	}
	for _, f := range l.Features {
		if f == feature {
			return true
		}
	}
	// Fallback: Pro licenses implicitly include legacy Pro features.
	if l.IsPro() {
		for _, f := range legacyProFeatures {
			if f == feature {
				return true
			}
		}
	}
	return false
}

// HasReadOnly reports whether the license grants read access to the named feature.
// Unlike Has(), this returns true for expired Pro licenses so that
// GET routes on Pro-module data remain accessible after subscription lapse.
//
// For expired Pro keys: read access is granted for any feature that
// the tier would normally cover. If the key has explicit features, those are
// respected; if the features list is empty (old key format), all Pro-tier
// features are granted for read-only access.
func (l *License) HasReadOnly(feature string) bool {
	if l == nil {
		return false
	}
	if l.Has(feature) {
		return true
	}
	if !l.Expired {
		return false
	}
	if l.Tier != "pro" {
		return false
	}
	// Explicit features in the key take precedence.
	for _, f := range l.Features {
		if f == feature {
			return true
		}
	}
	// Legacy Pro features are always granted (even if key predates them).
	for _, f := range legacyProFeatures {
		if f == feature {
			return true
		}
	}
	// Keys issued without an explicit features list (old format): grant all
	// allFeatures read-only so expired customers keep full read access.
	if len(l.Features) == 0 {
		for _, f := range allFeatures {
			if f == feature {
				return true
			}
		}
	}
	return false
}

// IsPro reports whether the license is at least Pro-tier and not expired.
func (l *License) IsPro() bool {
	if l == nil {
		return false
	}
	if l.Expired {
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

// demoLicense is the public demo instance: every feature on, nothing to buy.
// The tier is "pro" because that is the top tier that exists; the Demo flag —
// not the tier string — is what short-circuits every gate (see Has/IsPro).
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

	// A tier this build does not sell is not downgraded, it is discarded: the
	// caller gets a community license, never a partially honoured key. Placed
	// AFTER signature verification so an attacker learns nothing new, and before
	// the License is built so no code below can read a tier nobody vetted.
	if !sellableTiers[p.Tier] {
		return nil, errUnknownTier
	}

	var expiresAt *time.Time
	expired := false
	if p.Exp != nil {
		t := time.Unix(*p.Exp, 0).UTC()
		expiresAt = &t
		if time.Now().After(t) {
			expired = true
		}
	}

	return &License{
		Tier:         p.Tier,
		Features:     p.Features,
		OrgName:      p.Org,
		RenewalToken: p.RenewalToken,
		IssuedAt:     time.Unix(p.IssuedAt, 0).UTC(),
		ExpiresAt:    expiresAt,
		Expired:      expired,
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
	errUnknownTier      = licenseError("license tier is not sold by this build")
	errInvalidKey       = licenseError("invalid public key")
)

type licenseError string

func (e licenseError) Error() string { return string(e) }
