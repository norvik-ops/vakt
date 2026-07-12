package license

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
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

func TestLoad_EnterpriseKey(t *testing.T) {
	priv, restore := setupTestKeys(t)
	defer restore()

	exp := int64(1830211200)
	p := payload{Tier: "enterprise", Org: "Big Corp", IssuedAt: 1778923388, Exp: &exp}
	key := makeTestKey(t, priv, p)

	lic := Load(key, false)
	if !lic.IsEnterprise() {
		t.Fatal("want IsEnterprise()=true")
	}
	if !lic.IsPro() {
		t.Fatal("enterprise must also be pro")
	}
}

func TestIsEnterprise_NilLicense(t *testing.T) {
	var l *License
	if l.IsEnterprise() {
		t.Fatal("nil license must not be enterprise")
	}
}

func TestIsEnterprise_Demo(t *testing.T) {
	lic := Load("", true)
	if !lic.IsEnterprise() {
		t.Fatal("demo license must be enterprise")
	}
}

func TestSign_RoundTrip(t *testing.T) {
	priv, restore := setupTestKeys(t)
	defer restore()

	privDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER}))

	exp := time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC)
	key, err := Sign(privPEM, "pro", "Test Org", []string{FeatureSSO}, &exp)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	lic := Load(key, false)
	if lic.Tier != "pro" {
		t.Fatalf("want tier=pro, got %s", lic.Tier)
	}
	if lic.OrgName != "Test Org" {
		t.Fatalf("want org=Test Org, got %s", lic.OrgName)
	}
	if !lic.Has(FeatureSSO) {
		t.Error("want has(sso)=true")
	}
}

func TestSign_InvalidPEM(t *testing.T) {
	_, err := Sign("not-a-pem", "pro", "Test", nil, nil)
	if err == nil {
		t.Fatal("want error for invalid PEM")
	}
}

func TestIsPro_NilLicense(t *testing.T) {
	var l *License
	if l.IsPro() {
		t.Fatal("nil license must not be pro")
	}
}

func TestHas_NilLicense(t *testing.T) {
	var l *License
	if l.Has(FeatureTISAX) {
		t.Fatal("nil license must not have any feature")
	}
}

func TestLicenseError_Error(t *testing.T) {
	e := licenseError("test error")
	if e.Error() != "test error" {
		t.Fatalf("unexpected error string: %s", e.Error())
	}
}

func TestLoad_ExpiredKey(t *testing.T) {
	priv, restore := setupTestKeys(t)
	defer restore()

	past := int64(1000000000) // 2001 — safely in the past
	p := payload{Tier: "pro", Org: "Old Corp", IssuedAt: 999999999, Exp: &past}
	key := makeTestKey(t, priv, p)

	lic := Load(key, false)
	// S79-3: expired keys are preserved as Expired=true (not downgraded to community)
	// so that read-only access to Pro data is maintained after lapse.
	if lic.Tier != "pro" {
		t.Fatalf("expired key must keep tier=pro (read-only), got %s", lic.Tier)
	}
	if !lic.Expired {
		t.Fatal("expired key must set Expired=true")
	}
	if lic.IsPro() {
		t.Fatal("expired key must not report IsPro()=true")
	}
	if lic.Has(FeatureAuditPDF) {
		t.Fatal("expired key must not grant features via Has()")
	}
	if !lic.HasReadOnly(FeatureAuditPDF) {
		t.Fatal("expired Pro key must grant read-only access via HasReadOnly()")
	}
}

func TestLoad_BadPublicKeyPEM(t *testing.T) {
	old := publicKeyPEM
	publicKeyPEM = "not-a-pem"
	defer func() { publicKeyPEM = old }()

	// Valid-format key (payload.sig) but the public key slot is garbage.
	lic := Load("eyJ0aWVyIjoicHJvIn0.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", false)
	if lic.Tier != "community" {
		t.Fatalf("bad public key PEM must fall back to community, got %s", lic.Tier)
	}
}

func TestSign_Perpetual(t *testing.T) {
	priv, restore := setupTestKeys(t)
	defer restore()

	privDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER}))

	key, err := Sign(privPEM, "pro", "No Expiry", nil, nil)
	if err != nil {
		t.Fatalf("Sign perpetual failed: %v", err)
	}
	lic := Load(key, false)
	if lic.Tier != "pro" {
		t.Fatalf("want pro, got %s", lic.Tier)
	}
}

func TestLicenseToCache_RoundTrip(t *testing.T) {
	priv, restore := setupTestKeys(t)
	defer restore()

	exp := int64(1830211200)
	p := payload{Tier: "pro", Features: []string{FeatureSSO}, Org: "Cache Corp", IssuedAt: 1778923388, Exp: &exp}
	key := makeTestKey(t, priv, p)
	lic := Load(key, false)

	cached := licenseToCache(lic, false)
	restored := cacheToLicense(cached)

	if restored.Tier != lic.Tier {
		t.Fatalf("tier mismatch: %s != %s", restored.Tier, lic.Tier)
	}
	if restored.OrgName != lic.OrgName {
		t.Fatalf("org mismatch: %s != %s", restored.OrgName, lic.OrgName)
	}
	if len(restored.Features) != len(lic.Features) {
		t.Fatalf("features mismatch")
	}
}

func TestNewAutoRefresher_DefaultURL(t *testing.T) {
	r := NewAutoRefresher("token", "", true, nil, nil, nil)
	if r.baseURL != defaultRefreshURL {
		t.Fatalf("want %s, got %s", defaultRefreshURL, r.baseURL)
	}
	if r.token != "token" {
		t.Fatalf("want token=token, got %s", r.token)
	}
}

func TestNewAutoRefresher_CustomURL(t *testing.T) {
	r := NewAutoRefresher("", "http://custom", true, nil, nil, nil)
	if r.baseURL != "http://custom" {
		t.Fatalf("want http://custom, got %s", r.baseURL)
	}
}

func TestAutoRefresher_Start_Disabled(t *testing.T) {
	h := NewHandler(communityLicense())
	r := NewAutoRefresher("", "", false, h, nil, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	r.Start(ctx) // VAKT_LICENSE_AUTORENEW=false — must return immediately and never call out
}

// TestAutoRefresher_CommunityNeverCalls: the Community Edition has no key, so there is
// nothing to renew and no reason to contact anyone. If this ever regresses, a free
// self-hosted instance would start phoning home — the single worst thing this product
// could do to its own promise.
func TestAutoRefresher_CommunityNeverCalls(t *testing.T) {
	h := NewHandler(communityLicense())
	r := NewAutoRefresher("token", "", true, h, nil, nil)
	if r.due() {
		t.Fatal("a community licence has no expiry — the instance must never ask for a renewal")
	}
}

func TestNewHandler_WithOptions(t *testing.T) {
	lic := communityLicense()
	h := NewHandler(lic)
	if h.lic != lic {
		t.Fatal("handler lic mismatch")
	}
	h2 := h.WithDB(nil).WithRedis(nil).WithAutoRenewal()
	if h2 != h {
		t.Fatal("With* must return the same handler")
	}
	if !h.autoRenewalEnabled {
		t.Fatal("autoRenewalEnabled must be true after WithAutoRenewal")
	}
}

func TestHandler_Get_InMemory(t *testing.T) {
	h := NewHandler(communityLicense())

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/license", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Get(c); err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var resp licenseResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Tier != "community" {
		t.Fatalf("want community, got %s", resp.Tier)
	}
}

func TestHandler_Get_FromContext(t *testing.T) {
	h := NewHandler(nil)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/license", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("license", &License{Tier: "pro", Features: []string{FeatureSSO}})

	if err := h.Get(c); err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	var resp licenseResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Tier != "pro" {
		t.Fatalf("want pro, got %s", resp.Tier)
	}
}

func TestInvalidateLicenseCache_NilRDB(t *testing.T) {
	// nil rdb → early return, must not panic
	InvalidateLicenseCache(context.Background(), nil, "org-123")
}

func TestInvalidateLicenseCache_EmptyOrgID(t *testing.T) {
	// empty orgID → early return even with nil rdb
	InvalidateLicenseCache(context.Background(), nil, "")
}

// TestLegacyProKeyFallback verifies that a Pro key without the new feature strings
// (e.g. issued before bsi_grundschutz was added to proFeatures) still grants access
// to those features via the legacyProFeatures implicit grant.
func TestLegacyProKeyFallback(t *testing.T) {
	priv, restore := setupTestKeys(t)
	defer restore()

	exp := int64(1830211200) // 2028
	// Old-style Pro key: only has the original Pro features, none of the new strings.
	p := payload{
		Tier:     "pro",
		Features: []string{"eu_ai_act", "cra", "audit_pdf", "sso", "api_access"},
		Org:      "Legacy Corp",
		IssuedAt: 1700000000,
		Exp:      &exp,
	}
	key := makeTestKey(t, priv, p)
	lic := Load(key, false)

	if !lic.IsPro() {
		t.Fatal("want IsPro()=true")
	}
	// These were added after the key was issued — must still be granted.
	for _, f := range legacyProFeatures {
		if !lic.Has(f) {
			t.Errorf("legacy Pro key must implicitly grant %q", f)
		}
	}
	// Enterprise-only features must NOT be granted via the legacy fallback.
	if lic.Has(FeatureTISAX) {
		t.Error("legacy Pro key must not grant enterprise-only feature tisax")
	}
	if lic.Has(FeatureSCIMProvisioning) {
		t.Error("legacy Pro key must not grant enterprise-only feature scim_provisioning")
	}
}

// TestCommunityKeyNoLegacyFallback verifies that a Community license does NOT
// receive the legacyProFeatures grant.
func TestCommunityKeyNoLegacyFallback(t *testing.T) {
	lic := Load("", false)
	for _, f := range legacyProFeatures {
		if lic.Has(f) {
			t.Errorf("community license must not gain %q via legacy fallback", f)
		}
	}
}

// TestRenewWindowScalesWithTheKey guards the one number that decides whether this is a
// licence renewal or a heartbeat with a nicer name.
//
// A FIXED window would be a disaster on the monthly plan. That key lives 35 days; a
// 30-day window would leave the instance inside it almost permanently, and it would
// call every single day — 365 times a year, exactly the thing we told customers we do
// not do. Deriving the window from the key's own lifetime is what keeps a yearly
// instance at roughly ONE call a year and a monthly one at a handful per renewal.
//
// If someone "simplifies" this back to a constant, nothing else fails. This does.
func TestRenewWindowScalesWithTheKey(t *testing.T) {
	mk := func(lifetimeDays int) *License {
		iat := time.Now().Add(-time.Hour)
		exp := iat.Add(time.Duration(lifetimeDays) * 24 * time.Hour)
		return &License{IssuedAt: iat, ExpiresAt: &exp}
	}

	yearly := renewWindow(mk(395))
	if yearly != maxRenewWindow {
		t.Errorf("yearly key: window = %v, want the %v cap", yearly, maxRenewWindow)
	}

	monthly := renewWindow(mk(35))
	if monthly >= 15*24*time.Hour {
		t.Errorf("monthly key (35 days): window = %v. Anything near a month means the "+
			"instance sits inside the window permanently and phones home DAILY — the exact "+
			"thing the design exists to avoid.", monthly)
	}
	if monthly < 3*24*time.Hour {
		t.Errorf("monthly key: window = %v is too tight — a customer who pays a few days "+
			"late would go dark before the instance ever asked for the new key", monthly)
	}

	// A perpetual key has nothing to renew. It must never call.
	if w := renewWindow(&License{IssuedAt: time.Now()}); w != 0 {
		t.Errorf("perpetual key: window = %v, want 0 — there is nothing to fetch", w)
	}
	if w := renewWindow(nil); w != 0 {
		t.Errorf("nil licence: window = %v, want 0", w)
	}
}

// TestRenewalTokenSurvivesTheRoundTrip: the token rides INSIDE the signed key, which is
// what makes auto-renewal work without the customer configuring anything. If it does
// not survive sign->parse, every instance silently falls back to "enter a key by hand
// forever" — and nothing would fail except the customer's patience.
func TestRenewalTokenSurvivesTheRoundTrip(t *testing.T) {
	priv, restore := setupTestKeys(t)
	defer restore()

	exp := time.Now().Add(90 * 24 * time.Hour)
	key, err := signWith(priv, "pro", "Acme GmbH", "tok-12345", []string{"sso"}, &exp)
	if err != nil {
		t.Fatal(err)
	}
	lic, err := parse(key)
	if err != nil {
		t.Fatal(err)
	}
	if lic.RenewalToken != "tok-12345" {
		t.Errorf("renewal token did not survive the key: got %q, want %q", lic.RenewalToken, "tok-12345")
	}

	// And an old key without one must still parse — every existing customer holds one.
	old, err := signWith(priv, "pro", "Acme GmbH", "", []string{"sso"}, &exp)
	if err != nil {
		t.Fatal(err)
	}
	oldLic, err := parse(old)
	if err != nil {
		t.Fatalf("a key signed without a renewal token must still parse: %v", err)
	}
	if oldLic.RenewalToken != "" {
		t.Errorf("want empty renewal token on a legacy key, got %q", oldLic.RenewalToken)
	}
}
