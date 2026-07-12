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
	return SignWithToken(privateKeyPEM, tier, org, "", features, expires)
}

// SignWithToken embeds the licence's own renewal token, so the instance can fetch
// its next key by itself when this one runs low. See payload.RenewalToken.
func SignWithToken(privateKeyPEM, tier, org, renewalToken string, features []string, expires *time.Time) (string, error) {
	privateKeyPEM = strings.ReplaceAll(privateKeyPEM, `\n`, "\n")
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return "", fmt.Errorf("license: no PEM block found in private key")
	}
	privKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("license: parse private key: %w", err)
	}
	return signWith(privKey, tier, org, renewalToken, features, expires)
}

func signWith(privKey *ecdsa.PrivateKey, tier, org, renewalToken string, features []string, expires *time.Time) (string, error) {
	p := payload{
		Tier:         tier,
		Features:     features,
		Org:          org,
		RenewalToken: renewalToken,
		IssuedAt:     time.Now().UTC().Unix(),
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

// TrialLifetimeDays is the key issued WITH the invoice, before the money lands. A
// B2B buyer should not have to wait for a bank transfer to clear before they can
// start; if they never pay, it runs out on its own.
const TrialLifetimeDays = 45

// RenewBelowDays is the remaining lifetime below which a renewal poll re-signs the
// key, so an instance that checks in always carries a comfortable runway.
//
// It never extends the key beyond what the customer actually paid for — see
// billing.PaidThrough. That cap is the whole safety mechanism.
const RenewBelowDays = 60

// MailBelowDays is the remaining lifetime below which a customer WITHOUT auto-renewal
// gets a fresh key by mail. Deliberately above the 30-day in-app expiry banner, so a
// customer who did nothing wrong never sees a warning at all.
const MailBelowDays = 35

// TrialExpiry is when the pre-payment key runs out.
func TrialExpiry() time.Time {
	return time.Now().Add(TrialLifetimeDays * 24 * time.Hour)
}

// KeyExpiry is the fallback for callers with no subscription behind them — the
// admin CLI signing a key by hand. Real sales do NOT use this: they cap the expiry
// at the period the customer actually paid for (billing.Entitlement).
//
// Why the distinction matters, and why it is not a detail:
//
// The obvious way to keep control over a licence is to make keys short-lived and
// force a renewal. It is also wrong. A short key moves the risk of OUR outage onto
// a customer who paid a year in advance: if our mail or our billing service is down
// for three months, their ISMS goes dark — possibly mid-audit — through no fault of
// theirs. The party that did everything right must not carry our operational risk.
//
// The correct rule is the other one: a key is valid exactly as long as the customer
// paid for, plus grace. It cannot be extended past that, so someone who stops paying
// is out shortly after their period ends — and it does not need to be extended by us
// to keep working, so our outage cannot hurt them. Control and robustness come from
// the same rule, not from a trade-off between them.
func KeyExpiry(interval, status string) time.Time {
	if status == "trialing" {
		return TrialExpiry()
	}
	if interval == "year" {
		return time.Now().AddDate(0, 0, 365+30)
	}
	return time.Now().AddDate(0, 0, 30+5)
}
