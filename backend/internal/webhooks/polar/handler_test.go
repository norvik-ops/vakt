// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package polar

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

// ── test helpers ──────────────────────────────────────────────────────────────

const (
	testWebhookSecret = "test-secret-polar"
	testWebhookID     = "msg_test_00000000"
)

// testWebhookTS is a fresh Unix timestamp (set at package init) so signed test
// events pass the ±5 min replay-freshness window; the suite runs well within it.
var testWebhookTS = strconv.FormatInt(time.Now().Unix(), 10)

// makeSignature builds a Standard Webhooks signature the way Polar does:
// "v1," + base64(HMAC-SHA256(secret, "{id}.{timestamp}.{body}")). The secret is
// used as raw UTF-8 (Polar's convention), not base64-decoded.
func makeSignature(secret, id, ts string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(id + "." + ts + "." + string(body)))
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// testPrivKeyPEM generates a throwaway ECDSA P-256 private key PEM.
func testPrivKeyPEM(t *testing.T) string {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))
}

// smtpRecorder is a minimal SMTP server that accepts any message and records
// raw DATA payloads so tests can inspect what was sent.
type smtpRecorder struct {
	addr     string
	mu       sync.Mutex
	messages []string
}

func startSMTPRecorder(t *testing.T) *smtpRecorder {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &smtpRecorder{addr: ln.Addr().String()}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go srv.serveConn(conn)
		}
	}()
	return srv
}

func (s *smtpRecorder) serveConn(conn net.Conn) {
	defer conn.Close()
	w := bufio.NewWriter(conn)
	r := bufio.NewReader(conn)
	fmt.Fprintf(w, "220 test ready\r\n")
	w.Flush()

	var dataMode bool
	var body strings.Builder
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		if dataMode {
			if strings.TrimSpace(line) == "." {
				s.mu.Lock()
				s.messages = append(s.messages, body.String())
				s.mu.Unlock()
				body.Reset()
				dataMode = false
				fmt.Fprintf(w, "250 OK\r\n")
			} else {
				body.WriteString(line)
			}
			w.Flush()
			continue
		}
		upper := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			fmt.Fprintf(w, "250-test\r\n250 OK\r\n")
		case strings.HasPrefix(upper, "MAIL"), strings.HasPrefix(upper, "RCPT"):
			fmt.Fprintf(w, "250 OK\r\n")
		case upper == "DATA":
			dataMode = true
			fmt.Fprintf(w, "354 send data, end with .\r\n")
		case strings.HasPrefix(upper, "QUIT"):
			fmt.Fprintf(w, "221 bye\r\n")
			w.Flush()
			return
		default:
			fmt.Fprintf(w, "250 OK\r\n")
		}
		w.Flush()
	}
}

func (s *smtpRecorder) messageCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.messages)
}

func (s *smtpRecorder) subject(t *testing.T, idx int) string {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if idx >= len(s.messages) {
		t.Fatalf("smtpRecorder: want message[%d], only %d received", idx, len(s.messages))
	}
	for _, line := range strings.Split(s.messages[idx], "\n") {
		if strings.HasPrefix(line, "Subject:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Subject:"))
		}
	}
	t.Fatalf("smtpRecorder: no Subject header in message[%d]", idx)
	return ""
}

// extractLicenseKey finds the license key token in a raw SMTP DATA payload.
// The email body places the key after a blank line following "Dein License Key:".
func extractLicenseKey(raw string) string {
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "Dein License Key:" {
			for j := i + 1; j < len(lines); j++ {
				c := strings.TrimSpace(lines[j])
				if c != "" && strings.Contains(c, ".") {
					return c
				}
			}
		}
	}
	return ""
}

func newHandler(t *testing.T, srv *smtpRecorder) *Handler {
	t.Helper()
	host, port, _ := net.SplitHostPort(srv.addr)
	return NewHandler(testWebhookSecret, testPrivKeyPEM(t), SMTPConfig{
		Host: host, Port: port, From: "test@norvikops.de",
	})
}

func buildRequest(t *testing.T, event polarEvent) *http.Request {
	t.Helper()
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/billing/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("webhook-id", testWebhookID)
	req.Header.Set("webhook-timestamp", testWebhookTS)
	req.Header.Set("webhook-signature", makeSignature(testWebhookSecret, testWebhookID, testWebhookTS, body))
	return req
}

func doHandle(t *testing.T, h *Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	e := echo.New()
	c := e.NewContext(req, rec)
	_ = h.Handle(c)
	return rec
}

// ── keyExpiry ─────────────────────────────────────────────────────────────────

func TestKeyExpiry_Month(t *testing.T) {
	exp := keyExpiry("month", "active")
	low := time.Now().Add(34 * 24 * time.Hour)
	high := time.Now().Add(36 * 24 * time.Hour)
	if exp.Before(low) || exp.After(high) {
		t.Fatalf("month expiry %v not in [34d, 36d] window from now", exp)
	}
}

func TestKeyExpiry_Year(t *testing.T) {
	exp := keyExpiry("year", "active")
	low := time.Now().Add(394 * 24 * time.Hour)
	high := time.Now().Add(396 * 24 * time.Hour)
	if exp.Before(low) || exp.After(high) {
		t.Fatalf("year expiry %v not in [394d, 396d] window from now", exp)
	}
}

func TestKeyExpiry_Trialing_ShortRegardlessOfInterval(t *testing.T) {
	exp := keyExpiry("year", "trialing")
	low := time.Now().Add(44 * 24 * time.Hour)
	high := time.Now().Add(46 * 24 * time.Hour)
	if exp.Before(low) || exp.After(high) {
		t.Fatalf("trialing yearly key must be capped to ~45d, got %v", exp)
	}
}

func TestKeyExpiry_UnknownFallsBackToMonth(t *testing.T) {
	exp := keyExpiry("", "active")
	low := time.Now().Add(34 * 24 * time.Hour)
	high := time.Now().Add(36 * 24 * time.Hour)
	if exp.Before(low) || exp.After(high) {
		t.Fatalf("unknown interval should fall back to ~35d, got %v", exp)
	}
}

// ── verifySignature ───────────────────────────────────────────────────────────

func TestVerifySignature_Valid(t *testing.T) {
	h := &Handler{webhookSecret: testWebhookSecret}
	body := []byte(`{"type":"subscription.created"}`)
	sig := makeSignature(testWebhookSecret, testWebhookID, testWebhookTS, body)
	if !h.verifySignature(testWebhookID, testWebhookTS, sig, body) {
		t.Fatal("valid signature must verify")
	}
}

func TestVerifySignature_WrongSecret(t *testing.T) {
	h := &Handler{webhookSecret: testWebhookSecret}
	body := []byte(`{"type":"subscription.created"}`)
	sig := makeSignature("wrong", testWebhookID, testWebhookTS, body)
	if h.verifySignature(testWebhookID, testWebhookTS, sig, body) {
		t.Fatal("wrong secret must fail")
	}
}

func TestVerifySignature_EmptySecret_RejectsAll(t *testing.T) {
	h := &Handler{webhookSecret: ""}
	body := []byte(`{"type":"test"}`)
	sig := makeSignature("anything", testWebhookID, testWebhookTS, body)
	if h.verifySignature(testWebhookID, testWebhookTS, sig, body) {
		t.Fatal("empty webhook secret must reject all signatures")
	}
}

func TestVerifySignature_TamperedBody(t *testing.T) {
	h := &Handler{webhookSecret: testWebhookSecret}
	original := []byte(`{"type":"subscription.created"}`)
	sig := makeSignature(testWebhookSecret, testWebhookID, testWebhookTS, original)
	if h.verifySignature(testWebhookID, testWebhookTS, sig, []byte(`{"type":"subscription.revoked"}`)) {
		t.Fatal("tampered body must fail")
	}
}

func TestVerifySignature_TamperedTimestamp(t *testing.T) {
	h := &Handler{webhookSecret: testWebhookSecret}
	body := []byte(`{"type":"subscription.created"}`)
	sig := makeSignature(testWebhookSecret, testWebhookID, testWebhookTS, body)
	// A different but still-fresh timestamp — must fail on the signature (the
	// timestamp is part of the signed content), not on the freshness check.
	otherTS := strconv.FormatInt(time.Now().Unix()+5, 10)
	if h.verifySignature(testWebhookID, otherTS, sig, body) {
		t.Fatal("tampered timestamp must fail")
	}
}

func TestVerifySignature_StaleTimestamp_ReplayRejected(t *testing.T) {
	h := &Handler{webhookSecret: testWebhookSecret}
	body := []byte(`{"type":"subscription.active"}`)
	old := strconv.FormatInt(time.Now().Unix()-3600, 10) // 1h ago
	sig := makeSignature(testWebhookSecret, testWebhookID, old, body)
	// Signature is valid, but the timestamp is outside the ±5 min window → reject.
	if h.verifySignature(testWebhookID, old, sig, body) {
		t.Fatal("stale timestamp (replay) must reject even with a valid signature")
	}
}

func TestVerifySignature_MultipleSignatures(t *testing.T) {
	h := &Handler{webhookSecret: testWebhookSecret}
	body := []byte(`{"type":"subscription.active"}`)
	good := makeSignature(testWebhookSecret, testWebhookID, testWebhookTS, body)
	// Standard Webhooks allows a space-separated list (e.g. during secret rotation);
	// ours is the second entry, an unrelated one is first.
	header := "v1,bm90LXRoZS1yaWdodC1zaWc= " + good
	if !h.verifySignature(testWebhookID, testWebhookTS, header, body) {
		t.Fatal("must verify when a valid signature is present among several")
	}
}

func TestVerifySignature_MissingHeaders(t *testing.T) {
	h := &Handler{webhookSecret: testWebhookSecret}
	body := []byte(`{}`)
	sig := makeSignature(testWebhookSecret, testWebhookID, testWebhookTS, body)
	if h.verifySignature("", testWebhookTS, sig, body) {
		t.Fatal("missing webhook-id must reject")
	}
	if h.verifySignature(testWebhookID, "", sig, body) {
		t.Fatal("missing webhook-timestamp must reject")
	}
	if h.verifySignature(testWebhookID, testWebhookTS, "", body) {
		t.Fatal("missing webhook-signature must reject")
	}
}

// ── HTTP routing ──────────────────────────────────────────────────────────────

func TestHandle_InvalidSignature_Returns401(t *testing.T) {
	h := NewHandler(testWebhookSecret, "", SMTPConfig{})
	body, _ := json.Marshal(polarEvent{Type: "subscription.created"})
	req := httptest.NewRequest(http.MethodPost, "/billing/webhook", bytes.NewReader(body))
	req.Header.Set("webhook-id", testWebhookID)
	req.Header.Set("webhook-timestamp", testWebhookTS)
	req.Header.Set("webhook-signature", "v1,YmFkc2lnbmF0dXJl")
	rec := doHandle(t, h, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestHandle_UnknownEventType_Returns204(t *testing.T) {
	srv := startSMTPRecorder(t)
	h := newHandler(t, srv)
	rec := doHandle(t, h, buildRequest(t, polarEvent{Type: "order.created"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 204 for unknown event, got %d", rec.Code)
	}
}

func TestHandle_SubscriptionCanceled_NilDB_Returns204(t *testing.T) {
	srv := startSMTPRecorder(t)
	h := newHandler(t, srv)
	event := polarEvent{
		Type: "subscription.canceled",
		Data: polarSubscription{ID: "sub_cancel_1"},
	}
	rec := doHandle(t, h, buildRequest(t, event))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 204, got %d", rec.Code)
	}
}

func TestHandle_SubscriptionRevoked_NilDB_Returns204(t *testing.T) {
	srv := startSMTPRecorder(t)
	h := newHandler(t, srv)
	event := polarEvent{
		Type: "subscription.revoked",
		Data: polarSubscription{ID: "sub_revoke_1"},
	}
	rec := doHandle(t, h, buildRequest(t, event))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 204, got %d", rec.Code)
	}
}

func TestHandle_CancellationDoesNotSendEmail(t *testing.T) {
	srv := startSMTPRecorder(t)
	h := newHandler(t, srv)
	event := polarEvent{
		Type: "subscription.canceled",
		Data: polarSubscription{ID: "sub_cancel_2"},
	}
	doHandle(t, h, buildRequest(t, event))
	if srv.messageCount() != 0 {
		t.Fatalf("cancellation must not trigger email, got %d", srv.messageCount())
	}
}

// ── key issuance and expiry ───────────────────────────────────────────────────

func TestHandle_SubscriptionCreated_EmailSent(t *testing.T) {
	srv := startSMTPRecorder(t)
	h := newHandler(t, srv)

	event := polarEvent{
		Type: "subscription.created",
		Data: polarSubscription{
			ID: "sub_new_1", Status: "active",
			Price: struct {
				RecurringInterval string `json:"recurring_interval"`
			}{RecurringInterval: "month"},
			Customer: struct {
				Email string `json:"email"`
				Name  string `json:"name"`
			}{Email: "kunde@beispiel.de", Name: "Acme GmbH"},
		},
	}
	rec := doHandle(t, h, buildRequest(t, event))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 204, got %d — %s", rec.Code, rec.Body.String())
	}
	if srv.messageCount() != 1 {
		t.Fatalf("want 1 email, got %d", srv.messageCount())
	}
	if subj := srv.subject(t, 0); subj != "Dein Vakt Pro License Key" {
		t.Errorf("new: want subject %q, got %q", "Dein Vakt Pro License Key", subj)
	}
}

func TestHandle_SubscriptionCreated_Trialing_IssuesShortKey(t *testing.T) {
	srv := startSMTPRecorder(t)
	h := newHandler(t, srv)

	event := polarEvent{
		Type: "subscription.created",
		Data: polarSubscription{
			ID: "sub_trial_1", Status: "trialing",
			Price: struct {
				RecurringInterval string `json:"recurring_interval"`
			}{RecurringInterval: "year"},
			Customer: struct {
				Email string `json:"email"`
				Name  string `json:"name"`
			}{Email: "trial@beispiel.de", Name: "Trial GmbH"},
		},
	}
	rec := doHandle(t, h, buildRequest(t, event))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — %s", rec.Code, rec.Body.String())
	}
	if srv.messageCount() != 1 {
		t.Fatalf("trialing subscription must issue a key, got %d emails", srv.messageCount())
	}
	if subj := srv.subject(t, 0); subj != "Dein Vakt Pro License Key (Testphase)" {
		t.Errorf("trial: want subject %q, got %q", "Dein Vakt Pro License Key (Testphase)", subj)
	}

	// The key must be capped to the trial window even though the interval is yearly.
	srv.mu.Lock()
	raw := srv.messages[0]
	srv.mu.Unlock()
	licKey := extractLicenseKey(raw)
	parts := strings.SplitN(licKey, ".", 2)
	if len(parts) != 2 {
		t.Fatalf("invalid key format: %q", licKey)
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var p struct {
		Exp *int64 `json:"exp"`
	}
	if err := json.Unmarshal(payloadJSON, &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if p.Exp == nil {
		t.Fatal("trial key must have an expiry")
	}
	if exp := time.Unix(*p.Exp, 0); exp.After(time.Now().Add(46 * 24 * time.Hour)) {
		t.Errorf("trial key must be short (~45d), got expiry %v — yearly interval must not leak a full year of Pro", exp)
	}
}

func TestHandle_SubscriptionUpdated_Active_RenewalSubjectAndEmail(t *testing.T) {
	srv := startSMTPRecorder(t)
	h := newHandler(t, srv)

	event := polarEvent{
		Type: "subscription.updated",
		Data: polarSubscription{
			ID: "sub_renew_1", Status: "active",
			Price: struct {
				RecurringInterval string `json:"recurring_interval"`
			}{RecurringInterval: "year"},
			Customer: struct {
				Email string `json:"email"`
				Name  string `json:"name"`
			}{Email: "renewal@beispiel.de", Name: "Beta GmbH"},
		},
	}
	rec := doHandle(t, h, buildRequest(t, event))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — %s", rec.Code, rec.Body.String())
	}
	if subj := srv.subject(t, 0); subj != "Dein neuer Vakt Pro License Key" {
		t.Errorf("renewal: want subject %q, got %q", "Dein neuer Vakt Pro License Key", subj)
	}
}

func TestHandle_SubscriptionUncanceled_Active_RenewalEmail(t *testing.T) {
	srv := startSMTPRecorder(t)
	h := newHandler(t, srv)

	event := polarEvent{
		Type: "subscription.uncanceled",
		Data: polarSubscription{
			ID: "sub_uncancel_1", Status: "active",
			Price: struct {
				RecurringInterval string `json:"recurring_interval"`
			}{RecurringInterval: "month"},
			Customer: struct {
				Email string `json:"email"`
				Name  string `json:"name"`
			}{Email: "uncancel@beispiel.de", Name: "Gamma GmbH"},
		},
	}
	rec := doHandle(t, h, buildRequest(t, event))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — %s", rec.Code, rec.Body.String())
	}
	if srv.messageCount() != 1 {
		t.Fatalf("want 1 email for uncanceled active subscription, got %d", srv.messageCount())
	}
}

func TestHandle_SubscriptionUpdated_NotActive_NoEmail(t *testing.T) {
	srv := startSMTPRecorder(t)
	h := newHandler(t, srv)

	event := polarEvent{
		Type: "subscription.updated",
		Data: polarSubscription{
			ID: "sub_past_due_1", Status: "past_due",
			Customer: struct {
				Email string `json:"email"`
				Name  string `json:"name"`
			}{Email: "pastdue@beispiel.de"},
		},
	}
	doHandle(t, h, buildRequest(t, event))
	if srv.messageCount() != 0 {
		t.Fatalf("non-active updated event must not trigger email, got %d", srv.messageCount())
	}
}

func TestHandle_SubscriptionCreated_NotActive_NoEmail(t *testing.T) {
	srv := startSMTPRecorder(t)
	h := newHandler(t, srv)
	event := polarEvent{
		Type: "subscription.created",
		Data: polarSubscription{
			ID: "sub_pending", Status: "incomplete",
			Customer: struct {
				Email string `json:"email"`
				Name  string `json:"name"`
			}{Email: "pending@beispiel.de"},
		},
	}
	doHandle(t, h, buildRequest(t, event))
	if srv.messageCount() != 0 {
		t.Fatalf("incomplete subscription must not trigger email, got %d", srv.messageCount())
	}
}

// TestIssueKey_IssuedKeyHasExpiry is the primary invariant test for the
// revocation model: every Pro key issued via Polar must embed an expiry date.
// For self-hosted instances Norvik cannot access the customer's DB, so key
// expiry is the sole mechanism that enforces subscription end.
func TestIssueKey_IssuedKeyHasExpiry(t *testing.T) {
	for _, tc := range []struct {
		interval    string
		wantMinDays int
		wantMaxDays int
	}{
		{"month", 34, 36},
		{"year", 394, 396},
		{"", 34, 36}, // unknown falls back to month
	} {
		t.Run("interval="+tc.interval, func(t *testing.T) {
			srv := startSMTPRecorder(t)
			h := newHandler(t, srv)

			err := h.issueKey(t.Context(), "test@beispiel.de", "TestOrg", "", tc.interval, "active", false)
			if err != nil {
				t.Fatalf("issueKey: %v", err)
			}

			srv.mu.Lock()
			raw := srv.messages[0]
			srv.mu.Unlock()

			licKey := extractLicenseKey(raw)
			if licKey == "" {
				t.Fatal("no license key found in email body")
			}

			// Decode the payload part (before ".") to verify exp is set.
			// We intentionally skip signature verification here — we're testing
			// that issueKey passes a non-nil expiry to license.Sign, not that
			// the signature algorithm is correct (that's tested in license_test.go).
			parts := strings.SplitN(licKey, ".", 2)
			if len(parts) != 2 {
				t.Fatalf("invalid key format: %q", licKey)
			}
			payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
			if err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			var p struct {
				Tier string `json:"tier"`
				Exp  *int64 `json:"exp"`
			}
			if err := json.Unmarshal(payloadJSON, &p); err != nil {
				t.Fatalf("unmarshal payload: %v", err)
			}

			if p.Exp == nil {
				t.Fatal("issued key must have an expiry — this is the revocation mechanism for self-hosted instances")
			}
			expTime := time.Unix(*p.Exp, 0)
			low := time.Now().Add(time.Duration(tc.wantMinDays) * 24 * time.Hour)
			high := time.Now().Add(time.Duration(tc.wantMaxDays) * 24 * time.Hour)
			if expTime.Before(low) || expTime.After(high) {
				t.Errorf("expiry %v not in [%dd, %dd] window from now", expTime, tc.wantMinDays, tc.wantMaxDays)
			}
			if p.Tier != "pro" {
				t.Errorf("want tier=pro, got %s", p.Tier)
			}
		})
	}
}

// TestProFeaturesTiering locks the Pro feature set to the public pricing page
// (vakt.norvikops.de): TISAX, DORA, ISO 42001, and multi_framework are not offered
// publicly and must never ship with a Polar-issued Pro key.
func TestProFeaturesTiering(t *testing.T) {
	has := func(f string) bool {
		for _, p := range proFeatures {
			if p == f {
				return true
			}
		}
		return false
	}

	notOffered := []string{"tisax", "dora", "iso_42001", "multi_framework"}
	for _, f := range notOffered {
		if has(f) {
			t.Errorf("proFeatures must not include non-public feature %q", f)
		}
	}

	required := []string{
		"eu_ai_act", "cra", "audit_pdf", "sso", "api_access",
		"vaktaware_advanced", "vaktscan_advanced", "vaktvault_advanced",
		"vaktprivacy_advanced", "bsi_grundschutz",
		"granular_permissions", "supplier_portal", "nis2_reporting", "saml_auth",
		"agent_write_tools", "scim_provisioning", "siem_export",
	}
	for _, f := range required {
		if !has(f) {
			t.Errorf("proFeatures must include Pro feature %q", f)
		}
	}
}
