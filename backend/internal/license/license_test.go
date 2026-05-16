package license

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"testing"
)

// setupTestKeys generates an ephemeral ECDSA P-256 key pair, injects the
// public key into the package-level publicKeyPEM variable, and returns the
// private key plus a restore function to be deferred by the caller.
func setupTestKeys(t *testing.T) (*ecdsa.PrivateKey, func()) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))
	old := publicKeyPEM
	publicKeyPEM = pubPEM
	return priv, func() { publicKeyPEM = old }
}

// makeTestKey signs a payload with priv and returns a license key string.
func makeTestKey(t *testing.T, priv *ecdsa.PrivateKey, p payload) string {
	t.Helper()
	payloadBytes, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(payloadBytes)
	hash := sha256.Sum256([]byte(encoded))
	r, s, err := ecdsa.Sign(rand.Reader, priv, hash[:])
	if err != nil {
		t.Fatal(err)
	}
	sig := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):], sBytes)
	return encoded + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestLoad_ProKey(t *testing.T) {
	priv, restore := setupTestKeys(t)
	defer restore()

	exp := int64(1830211200) // 2028 — safely in the future
	p := payload{
		Tier:     "pro",
		Features: []string{FeatureTISAX, FeatureDORA, FeatureAuditPDF, FeatureSSO},
		Org:      "Acme GmbH",
		IssuedAt: 1778923388,
		Exp:      &exp,
	}
	key := makeTestKey(t, priv, p)

	lic := Load(key, false)
	if lic.Tier != "pro" {
		t.Fatalf("want tier=pro, got %s", lic.Tier)
	}
	if lic.OrgName != "Acme GmbH" {
		t.Fatalf("want org=Acme GmbH, got %s", lic.OrgName)
	}
	if !lic.IsPro() {
		t.Fatal("want IsPro()=true")
	}
	if !lic.Has(FeatureTISAX) {
		t.Error("want has(tisax)=true")
	}
	if !lic.Has(FeatureSSO) {
		t.Error("want has(sso)=true")
	}
	if lic.Has(FeatureCRA) {
		t.Error("want has(cra)=false (not in key)")
	}
}

func TestLoad_EmptyKey_ReturnsCommunity(t *testing.T) {
	lic := Load("", false)
	if lic.Tier != "community" {
		t.Fatalf("want tier=community, got %s", lic.Tier)
	}
	if lic.IsPro() {
		t.Fatal("community license must not be pro")
	}
	if lic.Has(FeatureTISAX) {
		t.Error("community must not have tisax")
	}
}

func TestLoad_Demo_GrantsAll(t *testing.T) {
	lic := Load("", true)
	if !lic.IsPro() {
		t.Fatal("demo must be pro")
	}
	for _, f := range allFeatures {
		if !lic.Has(f) {
			t.Errorf("demo must have feature %s", f)
		}
	}
}

func TestLoad_BadSignature_ReturnsCommunity(t *testing.T) {
	_, restore := setupTestKeys(t)
	defer restore()

	bad := "eyJ0aWVyIjoicHJvIn0.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	lic := Load(bad, false)
	if lic.Tier != "community" {
		t.Fatalf("bad key must fall back to community, got %s", lic.Tier)
	}
}

func TestLoad_MalformedKey_ReturnsCommunity(t *testing.T) {
	lic := Load("notakey", false)
	if lic.Tier != "community" {
		t.Fatalf("malformed key must fall back to community, got %s", lic.Tier)
	}
}
