// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S87-3 (F-05, CWE-208): Login must perform constant bcrypt work regardless of
// whether the e-mail exists, so response latency cannot be used to enumerate
// users. These white-box tests cover the dummy-hash mechanism that backs the
// unknown-e-mail branch.
//
// R1-INT-01: a cost-12 bcrypt op costs ~2.4s under -race, and this file used to
// pay for 8 of them (~21s of a 38s package) — enough that the `Backend (Go)`
// job blew its 120s timeout on a slow runner. The cost is now a var
// (dummyBcryptCost, lowered to bcrypt.MinCost by TestMain), and exactly ONE
// production-cost hash is generated for the whole package: the two assertions
// that genuinely depend on cost 12 share it via productionCostDummyHash().
// Everything else asserts cost-independent properties and runs at MinCost.
package auth

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// productionCostDummyHash builds one real, production-cost (12) dummy hash for
// the whole test binary. It drives the REAL newDummyBcryptHash — not an inlined
// copy — with dummyBcryptCost temporarily restored, so a regression in that
// function (wrong cost, hardcoded constant, empty return) still shows up here.
//
// Safe without a mutex around the global: no test in this package calls
// t.Parallel(), so Go runs them sequentially.
var productionCostDummyHash = sync.OnceValue(func() []byte {
	saved := dummyBcryptCost
	dummyBcryptCost = defaultDummyBcryptCost
	defer func() { dummyBcryptCost = saved }()
	return newDummyBcryptHash()
})

// TestNewDummyBcryptHash_IsCost12 pins the production timing parameter. The
// dummy compare only masks the existence of an account if it costs the same as
// a real one — cost 12, matching the real password hashes (service.go).
//
// Both halves are load-bearing: the constant check catches "someone lowered the
// default", the generated-hash check catches "the default is no longer what
// newDummyBcryptHash actually uses".
func TestNewDummyBcryptHash_IsCost12(t *testing.T) {
	assert.Equal(t, 12, defaultDummyBcryptCost,
		"production dummy-hash cost must stay at 12 (OWASP 2025), matching real password hashes")

	h := productionCostDummyHash()
	require.NotEmpty(t, h)
	cost, err := bcrypt.Cost(h)
	require.NoError(t, err)
	assert.Equal(t, 12, cost, "dummy hash must use the same cost as real hashes so timing matches")
}

// TestNewService_PopulatesDummyHash verifies the hash is precomputed at
// construction time (not lazily per login) and that newDummyBcryptHash honours
// the configured cost rather than hardcoding one.
//
// Runs at the test cost: "is it populated, and does it use the configured
// cost?" is independent of what that cost happens to be. The production VALUE
// of the cost is pinned by TestNewDummyBcryptHash_IsCost12 above.
func TestNewService_PopulatesDummyHash(t *testing.T) {
	key, err := GenerateSymmetricKey("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	require.NoError(t, err)
	svc := NewService(nil, nil, key)
	require.NotEmpty(t, svc.dummyBcryptHash, "NewService must precompute the dummy hash")
	cost, err := bcrypt.Cost(svc.dummyBcryptHash)
	require.NoError(t, err)
	assert.Equal(t, dummyBcryptCost, cost,
		"NewService must build the dummy hash at the configured cost, not a hardcoded one")
}

// TestDummyHash_NeverMatches guarantees the unknown-e-mail compare always fails
// (so it can never be mistaken for a successful login).
//
// This property comes from the hash being built over a crypto/rand secret, not
// from the cost factor — so it is asserted at the test cost. Verifying it at
// cost 12 bought nothing and cost ~11.9s (5 bcrypt ops) of the package budget.
func TestDummyHash_NeverMatches(t *testing.T) {
	h := newDummyBcryptHash()
	require.NotEmpty(t, h)
	for _, pw := range []string{"", "password", "Sup3r$ecret!!", "any-random-guess"} {
		assert.Error(t, bcrypt.CompareHashAndPassword(h, []byte(pw)),
			"dummy hash must not match arbitrary passwords")
	}
}

// dummyCompareMinDuration is the floor for a cost-12 bcrypt compare.
//
// The old value was 2ms, and it was VACUOUS: a bcrypt.MinCost compare clears it
// too, so the test passed no matter what cost the dummy hash used. Caught by
// deliberately building the hash at MinCost — the test stayed green, which is
// the only way this kind of hole shows up.
//
// Measured on the dev machine (2026-07-29), compare only:
//
//	cost 4  (MinCost):    0.78ms plain / 10.5ms plain-idle -race, MAX 12.2ms over 40 samples
//	cost 12 (production):  188ms plain / 2395ms -race
//
// 30ms is therefore ~2.46x above the worst observed MinCost reading (12.2ms),
// not the ~3x an average would suggest — use the MAX, a floor is only as good
// as its worst case.
//
// AND THE MARGIN ERODES UNDER LOAD. With 32 spinners on 8 cores, a MinCost
// compare reached p90 30.6ms / MAX 50.9ms — above this floor. A loaded runner
// pushes BOTH sides up, not just the cost-12 side, so the timing assertion
// alone degrades toward vacuity exactly when CI is busiest. (A slower runner
// only pushes cost-12 up, which is the safe direction; contention is the case
// that hurts.)
//
// That is why the timing check is NOT the only guard in the test below: the
// deterministic bcrypt.Cost assertion carries the cost claim, and the timing
// assertion adds "bcrypt actually ran". The floor may go fuzzy under load
// without the test's statement collapsing.
const dummyCompareMinDuration = 30 * time.Millisecond

// TestDummyHashCompare_DoesRealWork asserts the unknown-user compare path is not
// a no-op AND that it runs at production cost: a cost-12 bcrypt compare takes
// meaningful time, and that time is exactly the work that masks the existence
// of the account (CWE-208). A compare that got silently cheapened — short-
// circuited, or built at a lower cost — stops masking anything.
//
// Two independent guards, deliberately:
//
//	bcrypt.Cost   — deterministic, immune to CPU contention, carries the
//	                "runs at cost 12" claim on its own.
//	elapsed time  — catches a short-circuited/no-op compare, which no cost
//	                assertion can see. Degrades under load (see the const
//	                above), hence never the sole guard.
//
// This one genuinely needs the production cost. It reuses the shared
// production-cost hash, so the package pays for the cost-12 generate once.
func TestDummyHashCompare_DoesRealWork(t *testing.T) {
	if testing.Short() {
		t.Skip("timing assertion skipped in -short mode")
	}
	h := productionCostDummyHash()

	cost, err := bcrypt.Cost(h)
	require.NoError(t, err)
	require.Equal(t, defaultDummyBcryptCost, cost,
		"the compare under test must run against a production-cost hash — "+
			"a cheaper hash would make the timing assertion below vacuous")

	start := time.Now()
	_ = bcrypt.CompareHashAndPassword(h, []byte("any-password"))
	elapsed := time.Since(start)
	assert.Greater(t, elapsed, dummyCompareMinDuration,
		"a production-cost bcrypt compare must take non-trivial time (constant-work timing defense); "+
			"anything this fast means the compare was short-circuited")
}
