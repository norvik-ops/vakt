// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import "testing"

// S124-1 (SA14-01) token-layer guarantees for the two-stage login.

func TestMFAClaim_RoundTrips(t *testing.T) {
	key, err := GenerateSymmetricKey(testTokenHexKey)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	tok, err := IssueAccessToken(key, Claims{UserID: "u1", OrgID: "o1", Roles: []string{"Admin"}, MFA: true})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	claims, err := ParseAccessToken(key, tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !claims.MFA {
		t.Error("MFA claim did not round-trip as true")
	}

	// A token minted without MFA parses as MFA=false.
	tok2, _ := IssueAccessToken(key, Claims{UserID: "u1", OrgID: "o1", Roles: []string{"Admin"}})
	claims2, _ := ParseAccessToken(key, tok2)
	if claims2.MFA {
		t.Error("expected MFA=false for a token issued without MFA")
	}
}

func TestMFAPendingToken_NotAcceptedAsAccessToken(t *testing.T) {
	key, err := GenerateSymmetricKey(testTokenHexKey)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	pending, err := IssueMFAPendingToken(key, "u1", "o1")
	if err != nil {
		t.Fatalf("issue pending: %v", err)
	}

	// The pending token must parse at the pending endpoint...
	uid, oid, err := ParseMFAPendingToken(key, pending)
	if err != nil || uid != "u1" || oid != "o1" {
		t.Fatalf("ParseMFAPendingToken: uid=%q oid=%q err=%v", uid, oid, err)
	}

	// ...but MUST be rejected by ParseAccessToken so it can never reach a
	// protected route as if it were a full session.
	if _, err := ParseAccessToken(key, pending); err == nil {
		t.Error("ParseAccessToken accepted an mfa_pending token — it must not")
	}
}

func TestFullAccessToken_NotAcceptedAsPending(t *testing.T) {
	key, err := GenerateSymmetricKey(testTokenHexKey)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	full, _ := IssueAccessToken(key, Claims{UserID: "u1", OrgID: "o1", Roles: []string{"Admin"}, MFA: true})
	if _, _, err := ParseMFAPendingToken(key, full); err == nil {
		t.Error("ParseMFAPendingToken accepted a full access token — it must not")
	}
}

const testTokenHexKey = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
