// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

//go:build integration

package lexware

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// newFreeSub creates a free subscription that has already been issued (status paid, one
// granted period on record) — the state a conversion actually starts from.
func newFreeSub(t *testing.T, ctx context.Context, db *pgxpool.Pool, h *Handler) (string, time.Time) {
	t.Helper()
	subID, err := h.CreateSubscription(ctx, NewSubscription{
		CompanyName: "Partner GmbH", Email: "p@test.invalid",
		Product: "pro", Interval: "year", Quantity: 1, IsFree: true,
	}, "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(ctx, `DELETE FROM billing_invoices WHERE subscription_id = $1`, subID)
		_, _ = db.Exec(ctx, `DELETE FROM billing_licenses WHERE subscription_id = $1`, subID)
		_, _ = db.Exec(ctx, `DELETE FROM billing_quote_requests WHERE id = $1`, subID)
	})

	plan, _ := PlanFor("pro", "year")
	from, to := plan.Period(time.Now())
	charge, _ := plan.Charge(1, 0)

	if _, err := db.Exec(ctx, `
		INSERT INTO billing_licenses (subscription_id, org_name, license_key, expires_at, kind, note)
		VALUES ($1, 'Partner GmbH', 'vakt_k', $2, 'full', 'Freilizenz')`,
		subID, to.AddDate(0, 0, plan.GraceDays)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `
		UPDATE billing_quote_requests SET status='paid', approved_at=NOW(), paid_at=NOW(),
		       next_invoice_at=$2 WHERE id=$1`, subID, plan.NextInvoiceAt(to)); err != nil {
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
	return subID, to
}

// TestConvertRefusesBeforeTouchingLexware checks the guards that run BEFORE the contact
// is created — the only moment at which a bad conversion is still free to reject.
//
// Once CreateContact has run, something exists in Lexware; once the invoice is finalised,
// it cannot be withdrawn at all. Everything that can be caught must be caught here.
func TestConvertRefusesBeforeTouchingLexware(t *testing.T) {
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

	// A client that reports itself configured but is never reached: every case below must
	// fail before the first HTTP call. If one of them ever does reach Lexware, this test
	// will try to talk to the live API with a bogus key — and fail loudly, which is
	// exactly the alarm we want.
	h := &Handler{db: db, client: New("never-used-guards-must-fire-first")}

	good := ConvertInput{Street: "Industriestr. 12", Zip: "12345", City: "Musterstadt"}

	t.Run("Adresse fehlt", func(t *testing.T) {
		subID, _ := newFreeSub(t, ctx, db, h)
		_, err := h.ConvertToPaid(ctx, subID, ConvertInput{}, "test")
		if err == nil || !strings.Contains(err.Error(), "Adresse") {
			t.Fatalf("eine Rechnung ohne Adresse muss abgelehnt werden, bekam: %v", err)
		}
	})

	t.Run("bereits zahlend", func(t *testing.T) {
		subID, _ := newFreeSub(t, ctx, db, h)
		if _, err := db.Exec(ctx,
			`UPDATE billing_quote_requests SET is_free = false WHERE id = $1`, subID); err != nil {
			t.Fatal(err)
		}
		_, err := h.ConvertToPaid(ctx, subID, good, "test")
		if err == nil || !strings.Contains(err.Error(), "bereits ein zahlendes Abo") {
			t.Fatalf("ein zahlendes Abo darf nicht erneut umgewandelt werden — das legte einen "+
				"zweiten Lexware-Kontakt an. Bekam: %v", err)
		}
	})

	t.Run("gekündigt", func(t *testing.T) {
		subID, _ := newFreeSub(t, ctx, db, h)
		if _, err := db.Exec(ctx,
			`UPDATE billing_quote_requests SET cancelled_at = NOW() WHERE id = $1`, subID); err != nil {
			t.Fatal(err)
		}
		_, err := h.ConvertToPaid(ctx, subID, good, "test")
		if err == nil || !strings.Contains(err.Error(), "gekündigt") {
			t.Fatalf("ein gekündigtes Abo umzuwandeln würde es wiederbeleben, bekam: %v", err)
		}
	})

	t.Run("Rabatt über 90", func(t *testing.T) {
		subID, _ := newFreeSub(t, ctx, db, h)
		in := good
		in.DiscountPercent = 100
		_, err := h.ConvertToPaid(ctx, subID, in, "test")
		if err == nil || !strings.Contains(err.Error(), "90") {
			t.Fatalf("100 %% Rabatt muss abgelehnt werden, bekam: %v", err)
		}
	})
}

// TestConvertedSubscriptionIsInvoicedAndKeepsItsToken is the claim the whole feature
// rests on, and it is a claim about SQL.
//
// Converting must leave the subscription in a state where (a) the renewal sweep picks it
// up and raises a real invoice, and (b) the customer's licence — and therefore their
// renewal token — is untouched. Get (a) wrong and the customer never pays and silently
// expires; get (b) wrong and their VAKT_LICENSE_TOKEN dies at the moment they agree to
// buy.
//
// The Lexware call itself cannot run here, so this performs exactly the database step
// ConvertToPaid performs (a contact id, is_free=false, next_invoice_at chained to the end
// of the granted period) and then checks the sweep's own query.
func TestConvertedSubscriptionIsInvoicedAndKeepsItsToken(t *testing.T) {
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

	subID, freeEnd := newFreeSub(t, ctx, db, h)

	var tokenBefore string
	if err := db.QueryRow(ctx,
		`SELECT renewal_token FROM billing_licenses WHERE subscription_id = $1`, subID).
		Scan(&tokenBefore); err != nil {
		t.Fatal(err)
	}

	plan, _ := PlanFor("pro", "year")
	if _, err := db.Exec(ctx, `
		UPDATE billing_quote_requests
		   SET is_free = false, lexware_contact_id = 'lx-contact-1', discount_percent = 20,
		       street = 'Industriestr. 12', zip = '12345', city = 'Musterstadt',
		       next_invoice_at = $2
		 WHERE id = $1`, subID, plan.NextInvoiceAt(freeEnd)); err != nil {
		t.Fatal(err)
	}

	// (b) The token — the thing the whole in-place design exists to protect.
	var tokenAfter, key string
	if err := db.QueryRow(ctx,
		`SELECT renewal_token, license_key FROM billing_licenses WHERE subscription_id = $1`, subID).
		Scan(&tokenAfter, &key); err != nil {
		t.Fatal(err)
	}
	if tokenAfter != tokenBefore {
		t.Fatal("der Renewal-Token hat sich geändert — der Kunde müsste seine .env anfassen, " +
			"genau in dem Moment, in dem er zusagt zu zahlen")
	}
	if key == "" {
		t.Fatal("der Schlüssel des Kunden ist weg")
	}

	// (a) The sweep must now see it. Force it due and run the sweep's own query.
	if _, err := db.Exec(ctx,
		`UPDATE billing_quote_requests SET next_invoice_at = NOW() - INTERVAL '1 day' WHERE id = $1`,
		subID); err != nil {
		t.Fatal(err)
	}
	var seen bool
	var isFree bool
	var discount int
	if err := db.QueryRow(ctx, `
		SELECT true, s.is_free, s.discount_percent
		  FROM billing_quote_requests s
		 WHERE s.id = $1
		   AND s.status = 'paid' AND s.cancelled_at IS NULL
		   AND s.next_invoice_at IS NOT NULL AND s.next_invoice_at <= NOW()
		   AND (s.is_free OR s.lexware_contact_id IS NOT NULL)
		   AND NOT EXISTS (SELECT 1 FROM billing_invoices bi
		                    WHERE bi.subscription_id = s.id AND bi.status = 'open')`, subID).
		Scan(&seen, &isFree, &discount); err != nil {
		t.Fatalf("der Verlängerungs-Sweep sieht das umgewandelte Abo NICHT (%v) — es würde nie "+
			"abgerechnet und lautlos auslaufen", err)
	}
	if isFree {
		t.Error("das Abo ist noch als gratis markiert")
	}
	if discount != 20 {
		t.Errorf("der bei der Umwandlung vereinbarte Rabatt ist weg (%d %%)", discount)
	}
}

// TestOrphanedConversionIsDetected covers the trap that made this feature necessary in
// the first place.
//
// Someone flips is_free = false in psql. The subscription now has neither is_free NOR a
// Lexware contact, so the sweep's "(is_free OR contact IS NOT NULL)" condition silently
// drops it: never invoiced, never renewed, key eventually expires, no error anywhere.
//
// A condition that silently excludes rows has to count what it excludes.
func TestOrphanedConversionIsDetected(t *testing.T) {
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

	subID, _ := newFreeSub(t, ctx, db, h)

	// The hand-edit: free flag off, no contact created.
	if _, err := db.Exec(ctx, `
		UPDATE billing_quote_requests
		   SET is_free = false, next_invoice_at = NOW() - INTERVAL '1 day'
		 WHERE id = $1`, subID); err != nil {
		t.Fatal(err)
	}

	// It is invisible to the sweep …
	var due int
	if err := db.QueryRow(ctx, `
		SELECT count(*) FROM billing_quote_requests s
		 WHERE s.id = $1
		   AND s.status = 'paid' AND s.cancelled_at IS NULL
		   AND s.next_invoice_at IS NOT NULL AND s.next_invoice_at <= NOW()
		   AND (s.is_free OR s.lexware_contact_id IS NOT NULL)`, subID).Scan(&due); err != nil {
		t.Fatal(err)
	}
	if due != 0 {
		t.Fatal("Annahme falsch: der Sweep sieht es doch")
	}

	// … and therefore the orphan detector MUST see it.
	var orphans int
	if err := db.QueryRow(ctx, `
		SELECT count(*) FROM billing_quote_requests s
		 WHERE s.status = 'paid' AND s.cancelled_at IS NULL
		   AND s.next_invoice_at IS NOT NULL AND s.next_invoice_at <= NOW()
		   AND NOT s.is_free AND s.lexware_contact_id IS NULL`).Scan(&orphans); err != nil {
		t.Fatal(err)
	}
	if orphans < 1 {
		t.Fatal("ein Abo, das aus dem Sweep gefallen ist, wird NICHT gemeldet — es läuft " +
			"lautlos aus, und niemand erfährt es")
	}
}
