// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import (
	"context"
	"fmt"
	"time"

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
	var product, interval string
	var end *time.Time
	if err := db.QueryRow(ctx, `
		SELECT s.product, s.interval,
		       (SELECT MAX(bi.period_end) FROM billing_invoices bi
		         WHERE bi.subscription_id = s.id AND bi.status = 'paid')
		  FROM billing_quote_requests s
		 WHERE s.id = $1
		   AND s.status = 'paid'
		   AND s.cancelled_at IS NULL`, subID).Scan(&product, &interval, &end); err != nil {
		return time.Time{}, err
	}
	if end == nil {
		return time.Time{}, ErrNothingPaid
	}
	plan, err := PlanFor(product, interval)
	if err != nil {
		return time.Time{}, err
	}
	return end.AddDate(0, 0, plan.GraceDays), nil
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

	// StatusLapsed — the paid period plus grace is over. No further key is issued; the
	// last one runs out on its own.
	StatusLapsed Status = "abgelaufen"

	// StatusCancelled — the customer cancelled. Runs out at the end of what they paid.
	StatusCancelled Status = "gekündigt"

	// StatusRevoked — we stopped renewing this one key. Not a kill switch: it stays
	// valid until it expires.
	StatusRevoked Status = "gesperrt"
)

// LicenceStatus computes the state of one licence from the invoices behind it.
func LicenceStatus(ctx context.Context, db *pgxpool.Pool, renewalToken string) (Status, time.Time, error) {
	var subID string
	var revoked, cancelled *time.Time
	if err := db.QueryRow(ctx, `
		SELECT bl.subscription_id, bl.revoked_at, s.cancelled_at
		  FROM billing_licenses bl
		  JOIN billing_quote_requests s ON s.id = bl.subscription_id
		 WHERE bl.renewal_token = $1::uuid`, renewalToken).Scan(&subID, &revoked, &cancelled); err != nil {
		return "", time.Time{}, err
	}

	limit, err := Entitlement(ctx, db, subID)
	if err != nil && err != ErrNothingPaid {
		return "", time.Time{}, err
	}

	switch {
	case revoked != nil:
		return StatusRevoked, limit, nil
	case cancelled != nil:
		return StatusCancelled, limit, nil
	case limit.IsZero() || !limit.After(time.Now()):
		return StatusLapsed, limit, nil
	}

	// An open invoice whose period has already begun means: we billed them, they have
	// not paid, and the key will not be renewed past `limit` until they do.
	var open bool
	if err := db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM billing_invoices
			 WHERE subscription_id = $1 AND status = 'open')`, subID).Scan(&open); err != nil {
		return "", limit, err
	}
	if open {
		return StatusExpiring, limit, nil
	}
	return StatusPaid, limit, nil
}
