// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

//go:build integration

package lexware

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestFreeLicenceIsEntitledAndRenews is the test that matters for free licences, and it
// can only be written against a real database.
//
// The whole design rests on one claim: a free period, recorded as a paid 0 € invoice,
// makes the EXISTING machinery work unchanged — entitlement, the renewal sweep, the seat
// count. That claim is about SQL, not about Go. Every one of those places is a query,
// and a query either sees the row or it does not.
//
// The failure this guards against is the quiet one: a free customer whose entitlement is
// empty gets no key renewal, no warning, and no error in any log. They simply stop
// working, and the first person to find out is them.
func TestFreeLicenceIsEntitledAndRenews(t *testing.T) {
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

	subID, err := h.CreateSubscription(ctx, NewSubscription{
		CompanyName: "Design Partner GmbH", Email: "partner@test.invalid",
		Product: "pro", Interval: "year", Quantity: 1, IsFree: true,
	}, "test")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(ctx, `DELETE FROM billing_invoices WHERE subscription_id = $1`, subID)
		_, _ = db.Exec(ctx, `DELETE FROM billing_licenses WHERE subscription_id = $1`, subID)
		_, _ = db.Exec(ctx, `DELETE FROM billing_quote_requests WHERE id = $1`, subID)
	})

	// approveFree needs a signing key, which a unit environment has not got. Do what it
	// does to the DATABASE — that is the part under test — and leave the signing to the
	// unit tests.
	plan, _ := PlanFor("pro", "year")
	from, to := plan.Period(time.Now())
	charge, _ := plan.Charge(1, 0)

	var renewalToken string
	if err := db.QueryRow(ctx, `
		INSERT INTO billing_licenses (subscription_id, org_name, license_key, expires_at, kind, note)
		VALUES ($1, 'Design Partner GmbH', 'vakt_testkey', $2, 'full', 'Freilizenz')
		RETURNING renewal_token`, subID, to.AddDate(0, 0, plan.GraceDays)).Scan(&renewalToken); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `
		UPDATE billing_quote_requests
		   SET status = 'paid', approved_at = NOW(), paid_at = NOW(), next_invoice_at = $2
		 WHERE id = $1`, subID, plan.NextInvoiceAt(to)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO billing_invoices
			(subscription_id, lexware_invoice_id, period_start, period_end,
			 net_amount_cents, list_amount_cents, discount_percent, status, paid_at)
		VALUES ($1, $2, $3, $4, 0, $5, 100, 'paid', NOW())`,
		subID, freeInvoiceRef(subID, from), from, to, charge.ListCents); err != nil {
		t.Fatal(err)
	}

	// 1. Entitlement. This is the one that decides whether the customer keeps working:
	//    GetLicense and MailExpiringKeys both refuse to hand out a key past it.
	limit, err := Entitlement(ctx, db, subID)
	if err != nil {
		t.Fatalf("a free licence has no entitlement (%v) — the customer would get no key "+
			"renewal and go dark, silently", err)
	}
	if !limit.After(time.Now()) {
		t.Fatalf("entitlement is in the past (%s) — the free customer is already cut off",
			limit.Format("2006-01-02"))
	}

	// 2. Seats. "Platz vergeben" requires status='paid'; a free licence must qualify,
	//    or an MSP-style free partner could never be given a key at all.
	seats := NewSeats(db, nil)
	st, err := seats.State(ctx, subID)
	if err != nil {
		t.Fatalf("Seats.State refuses a free subscription: %v", err)
	}
	if st.Used != 1 {
		t.Errorf("seat count is %d, expected 1", st.Used)
	}

	// 3. It must be counted as 0 € of MRR. Reporting revenue that nobody transfers is
	//    how you end up trusting a number that is not true.
	var mrrRows int
	if err := db.QueryRow(ctx, `
		SELECT count(*) FROM billing_quote_requests
		 WHERE id = $1 AND status = 'paid' AND cancelled_at IS NULL AND NOT is_free`,
		subID).Scan(&mrrRows); err != nil {
		t.Fatal(err)
	}
	if mrrRows != 0 {
		t.Error("the free licence is counted in MRR — that is revenue nobody pays")
	}

	// 4. Reconcile must NOT see it. Its synthetic reference has no Lexware counterpart,
	//    so without the exclusion it would be reported as an invented invoice — the most
	//    severe finding there is, on every single free customer, every single sweep.
	var reconcileRows int
	if err := db.QueryRow(ctx, `
		SELECT count(*) FROM billing_invoices i
		  JOIN billing_quote_requests s ON s.id = i.subscription_id
		 WHERE i.subscription_id = $1 AND NOT s.is_free`, subID).Scan(&reconcileRows); err != nil {
		t.Fatal(err)
	}
	if reconcileRows != 0 {
		t.Error("Reconcile would report the free licence as 'nur in Vakt' — a false alarm " +
			"on every free customer, which trains you to ignore the real ones")
	}

	// 5. THE renewal. A free subscription has no Lexware contact, and the sweep used to
	//    demand one — which would have dropped every free licence out of the cycle and
	//    let it expire after one period, silently. Force it due and check the sweep picks
	//    it up and extends it WITHOUT raising an invoice.
	if _, err := db.Exec(ctx,
		`UPDATE billing_quote_requests SET next_invoice_at = NOW() - INTERVAL '1 day' WHERE id = $1`,
		subID); err != nil {
		t.Fatal(err)
	}

	var due dueSubscription
	var end *time.Time
	err = db.QueryRow(ctx, `
		SELECT s.id, s.company_name, s.email, s.product, s.interval, s.quantity,
		       s.discount_percent, s.is_free, COALESCE(s.lexware_contact_id, ''),
		       (SELECT MAX(bi.period_end) FROM billing_invoices bi WHERE bi.subscription_id = s.id)
		  FROM billing_quote_requests s
		 WHERE s.id = $1
		   AND s.status = 'paid' AND s.cancelled_at IS NULL
		   AND s.next_invoice_at IS NOT NULL AND s.next_invoice_at <= NOW()
		   AND (s.is_free OR s.lexware_contact_id IS NOT NULL)
		   AND NOT EXISTS (SELECT 1 FROM billing_invoices bi
		                    WHERE bi.subscription_id = s.id AND bi.status = 'open')`, subID).
		Scan(&due.id, &due.company, &due.email, &due.product, &due.interval, &due.quantity,
			&due.discount, &due.isFree, &due.contactID, &end)
	if err != nil {
		t.Fatalf("the renewal sweep does not see the free licence (%v) — it would expire "+
			"after one period and nobody would be told", err)
	}
	due.periodEnd = *end

	if err := h.renewFree(ctx, due); err != nil {
		t.Fatalf("renewFree: %v", err)
	}

	// The entitlement must now reach further than before, and it must have cost nothing.
	extended, err := Entitlement(ctx, db, subID)
	if err != nil {
		t.Fatal(err)
	}
	if !extended.After(limit) {
		t.Fatalf("entitlement did not move (%s → %s) — the renewal did nothing",
			limit.Format("2006-01-02"), extended.Format("2006-01-02"))
	}

	var invoices, charged int
	if err := db.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE net_amount_cents > 0)
		  FROM billing_invoices WHERE subscription_id = $1`, subID).
		Scan(&invoices, &charged); err != nil {
		t.Fatal(err)
	}
	if invoices != 2 {
		t.Errorf("expected 2 recorded periods after one renewal, got %d", invoices)
	}
	if charged != 0 {
		t.Fatalf("%d invoice(s) with a non-zero amount — a free customer was charged", charged)
	}
}
