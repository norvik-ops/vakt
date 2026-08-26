// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNothingPaid means the customer has not paid for a single period yet, so there is
// no entitlement to sign a key against.
var ErrNothingPaid = fmt.Errorf("billing: no paid period for this subscription")

// Entitlement is the latest moment a licence for this subscription may be valid: the
// end of the last period the customer ACTUALLY PAID FOR, plus the plan's grace.
//
// This one rule is what makes licence control both effective and safe, and it has to
// cut both ways:
//
//   - A key may never be signed or renewed beyond it. Someone who stops paying is out
//     shortly after the period they paid for ends — no code path can extend them,
//     because there is nothing to extend them TO.
//
//   - A key is ALREADY valid through it. A customer who paid a year in advance does
//     not depend on us staying up. If our billing service or our mail is down for
//     three months, their key carries them anyway; their ISMS does not go dark —
//     possibly mid-audit — because of an outage on our side.
//
// The tempting alternative was a short-lived key (90 days) that must be continuously
// renewed. It delivers the same control and quietly moves the risk of OUR failure
// onto the customer who did everything right. For a compliance product that is worse
// than backwards: what they bought is precisely that it keeps working.
//
// Control and robustness fall out of the same rule. They are not a trade-off.
func Entitlement(ctx context.Context, db *pgxpool.Pool, subID string) (time.Time, error) {
	limit, live, err := paidThrough(ctx, db, subID)

	// Order matters. A real failure — the query could not run, the subscription does
	// not exist, the plan is gone — is reported as itself. Turning that into "no
	// entitlement" would make a dropped database connection indistinguishable from a
	// customer who stopped paying, and the callers respond very differently to the two.
	if err != nil && err != ErrNothingPaid {
		return time.Time{}, err
	}
	if !live {
		// Unpaid or cancelled: no entitlement at all. The sentinel stays pgx.ErrNoRows,
		// exactly what the gated WHERE clause produced before, because callers
		// distinguish ErrNothingPaid ("nothing yet") from "cannot answer" and every
		// signing path has to land in the second bucket.
		return time.Time{}, pgx.ErrNoRows
	}
	if err != nil {
		return time.Time{}, err // ErrNothingPaid
	}
	return limit, nil
}

// paidThrough answers only "how far does their money reach?", without the gate that
// makes Entitlement the signing limit.
//
// It is split out because the gate belongs to signing and NOT to display, and
// conflating the two made a state unreachable: LicenceStatus called Entitlement,
// which filters cancelled subscriptions away, so it got pgx.ErrNoRows for every
// cancelled subscription and returned before its own `case cancelled != nil` could
// fire. StatusCancelled — "gekündigt" — was dead code, both consumers swallowed the
// error, and the templates rendered an EMPTY red pill in /licences, the subscription
// detail page and the MSP portal. StatusRevoked was unreachable for a revoked key on
// a cancelled subscription for the same reason.
//
// The `live` flag, not a second query: one place computes the paid-through date, so
// the gate cannot drift away from the number it gates.
func paidThrough(ctx context.Context, db *pgxpool.Pool, subID string) (limit time.Time, live bool, err error) {
	var product, interval, status string
	var cancelled, end *time.Time
	if err := db.QueryRow(ctx, `
		SELECT s.product, s.interval, s.status, s.cancelled_at,
		       (SELECT MAX(bi.period_end) FROM billing_invoices bi
		         WHERE bi.subscription_id = s.id AND bi.status = 'paid')
		  FROM billing_quote_requests s
		 WHERE s.id = $1`, subID).Scan(&product, &interval, &status, &cancelled, &end); err != nil {
		return time.Time{}, false, err
	}
	live = status == "paid" && cancelled == nil
	if end == nil {
		return time.Time{}, live, ErrNothingPaid
	}
	plan, err := PlanFor(product, interval)
	if err != nil {
		return time.Time{}, live, err
	}
	return end.AddDate(0, 0, plan.GraceDays), live, nil
}

// EntitlementByToken is the same, keyed by a licence's renewal token — the renewal
// path holds the token, not the subscription id.
func EntitlementByToken(ctx context.Context, db *pgxpool.Pool, token string) (time.Time, error) {
	var subID string
	if err := db.QueryRow(ctx,
		`SELECT subscription_id FROM billing_licenses WHERE renewal_token = $1::uuid`, token).
		Scan(&subID); err != nil {
		return time.Time{}, err
	}
	return Entitlement(ctx, db, subID)
}

// Status is what a human needs to know about a licence at a glance. It is DERIVED,
// never stored — a stored flag would be one more thing that can drift away from the
// invoices, and the invoices are the truth.
type Status string

const (
	// StatusPaid — the current period is settled. The key renews.
	StatusPaid Status = "bezahlt"

	// StatusExpiring — an invoice is out and unpaid. The key still works, but it will
	// NOT be renewed until the money lands. This is the state Stefan needs to see: it
	// is the last moment to pick up the phone. It flips back to "bezahlt" by itself
	// the second the payment is booked — nothing to reset by hand.
	StatusExpiring Status = "läuft aus"

	// StatusLapsed — nothing paid reaches into the future any more and no working key
	// with an open invoice behind it is left: the paid period plus grace is over, or it
	// never began and the pre-payment key ran out. No further key is issued; the last
	// one runs out on its own.
	StatusLapsed Status = "abgelaufen"

	// StatusCancelled — the customer cancelled. Runs out at the end of what they paid.
	StatusCancelled Status = "gekündigt"

	// StatusRevoked — we stopped renewing this one key. Not a kill switch: it stays
	// valid until it expires.
	StatusRevoked Status = "gesperrt"
)

// LicenceStatus computes the state of one licence from the invoices behind it.
//
// The returned time is the paid-through date and nothing else: it is zero before the
// first payment, because there is no date up to which money reaches yet. Callers that
// want to show "gültig bis" take billing_licenses.expires_at — the key's own expiry —
// which is what the two panels do.
func LicenceStatus(ctx context.Context, db *pgxpool.Pool, renewalToken string) (Status, time.Time, error) {
	var subID string
	var revoked, cancelled *time.Time
	var keyExpires time.Time
	if err := db.QueryRow(ctx, `
		SELECT bl.subscription_id, bl.revoked_at, bl.expires_at, s.cancelled_at
		  FROM billing_licenses bl
		  JOIN billing_quote_requests s ON s.id = bl.subscription_id
		 WHERE bl.renewal_token = $1::uuid`, renewalToken).
		Scan(&subID, &revoked, &keyExpires, &cancelled); err != nil {
		return "", time.Time{}, err
	}

	// paidThrough, NOT Entitlement: the status of a cancelled or unpaid subscription is
	// exactly what a human needs to see here, and Entitlement refuses to answer for
	// those on purpose. Asking it anyway is what made "gekündigt" unreachable.
	limit, _, err := paidThrough(ctx, db, subID)
	if err != nil && err != ErrNothingPaid {
		return "", time.Time{}, err
	}

	switch {
	case revoked != nil:
		return StatusRevoked, limit, nil
	case cancelled != nil:
		return StatusCancelled, limit, nil
	}

	// An open invoice means: we billed them and they have not paid. Asked for BOTH
	// halves below, because "unpaid" means something different on either side of the
	// paid-through date.
	var open bool
	if err := db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM billing_invoices
			 WHERE subscription_id = $1 AND status = 'open')`, subID).Scan(&open); err != nil {
		return "", limit, err
	}

	now := time.Now()
	switch {
	case limit.After(now):
		// Their money reaches into the future. An open invoice on top of that means the
		// key will not be renewed past `limit` until it is paid.
		if open {
			return StatusExpiring, limit, nil
		}
		return StatusPaid, limit, nil

	case open && keyExpires.After(now):
		// BEFORE THE FIRST PAYMENT — the state every new customer passes through, and
		// the one StatusExpiring was written for: the invoice is out, nothing is paid
		// (limit is zero), and the customer holds a 45-day key that WORKS. Deciding this
		// by `limit` alone called it "abgelaufen" while the panel showed a valid date 45
		// days out, right next to it: plausible and wrong, which is worse than the empty
		// pill it replaced.
		//
		// The same branch is right after a lapse that was re-invoiced while the old key
		// still runs: an invoice is out, the key works, someone should pick up the phone.
		return StatusExpiring, limit, nil
	}

	// Nothing paid reaches into the future and no working key with an invoice behind it:
	// the paid period plus grace is over, or it never began and the trial key ran out.
	return StatusLapsed, limit, nil
}
