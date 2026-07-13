// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

//go:build integration

package lexware

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestDiscountRoundTripsThroughPostgres runs the real queries against the real schema.
//
// Every discount bug this codebase could still have lives below the Go type system: a
// column that does not exist, a CHECK that does not fire, a DEFAULT that quietly makes
// existing customers something other than full-price. `go build` sees none of it, and
// the unit tests price plans in memory without ever touching a table.
//
// VAKT_TEST_DB_URL=postgres://... go test -tags=integration ./internal/billing/lexware/
func TestDiscountRoundTripsThroughPostgres(t *testing.T) {
	url := os.Getenv("VAKT_TEST_DB_URL")
	if url == "" {
		t.Skip("VAKT_TEST_DB_URL not set")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &Handler{db: db}

	// A customer created by hand, with a rebate, exactly as the panel does it.
	subID, err := h.CreateSubscription(ctx, NewSubscription{
		CompanyName: "Rabatt Test GmbH", Email: "rabatt@test.invalid",
		Product: "pro", Interval: "year", Quantity: 1, DiscountPercent: 20,
	}, "test")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(ctx, `DELETE FROM billing_quote_requests WHERE id = $1`, subID)
	})

	// The rebate must be ON THE SUBSCRIPTION — that is what makes it survive renewals.
	var stored int
	if err := db.QueryRow(ctx,
		`SELECT discount_percent FROM billing_quote_requests WHERE id = $1`, subID).
		Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 20 {
		t.Fatalf("discount stored as %d %%, expected 20", stored)
	}

	// And it must price the invoice: 2.990 € − 20 % = 2.392 €.
	plan, _ := PlanFor("pro", "year")
	charge, err := plan.Charge(1, stored)
	if err != nil {
		t.Fatal(err)
	}
	if charge.NetCents != 239200 {
		t.Fatalf("net %d cents, expected 239200", charge.NetCents)
	}

	// Changing it on a LIVE subscription is the case that matters, and the one the
	// template bug would have made impossible.
	msg, err := h.SetDiscount(ctx, subID, 50, "test")
	if err != nil {
		t.Fatalf("SetDiscount: %v", err)
	}
	if msg == "" {
		t.Fatal("SetDiscount said nothing — the operator must be told what the next invoice reads")
	}
	if err := db.QueryRow(ctx,
		`SELECT discount_percent FROM billing_quote_requests WHERE id = $1`, subID).
		Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 50 {
		t.Fatalf("discount is %d %% after SetDiscount(50)", stored)
	}

	// The database must refuse a 100 % rebate even if Go somehow did not — the CHECK is
	// the last line, and a 0 € invoice silently kills the subscription.
	if _, err := db.Exec(ctx,
		`UPDATE billing_quote_requests SET discount_percent = 100 WHERE id = $1`, subID); err == nil {
		t.Fatal("Postgres accepted a 100 % discount — the CHECK constraint is missing. " +
			"A 0 € invoice is never paid, so settle() never runs and the customer never " +
			"gets a full key.")
	}

	// An existing customer with no rebate must still be full-price: the column DEFAULT
	// is what stands between "we added a feature" and "we gave everyone 0 €".
	var plain string
	if err := db.QueryRow(ctx, `
		INSERT INTO billing_quote_requests (company_name, email, approval_token_hash)
		VALUES ('Voll GmbH', 'voll@test.invalid', 'h') RETURNING id`).Scan(&plain); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(ctx, `DELETE FROM billing_quote_requests WHERE id = $1`, plain)
	})
	if err := db.QueryRow(ctx,
		`SELECT discount_percent FROM billing_quote_requests WHERE id = $1`, plain).
		Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 0 {
		t.Fatalf("a subscription created without a discount has %d %% — existing customers "+
			"must default to the list price", stored)
	}
}
