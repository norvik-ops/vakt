// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/billing/licensing"
)

// Seats is the one place that hands out a licence key against a paid subscription.
//
// Two callers need this: the admin panel (Stefan issues a key by hand) and the MSP
// self-service portal (the MSP issues one for a client it just onboarded). Two
// implementations would drift, and the way they would drift is: one forgets the seat
// cap, or forgets to write the ledger, and the count silently stops being true.
type Seats struct {
	db     *pgxpool.Pool
	issuer *licensing.Issuer
}

func NewSeats(db *pgxpool.Pool, iss *licensing.Issuer) *Seats {
	return &Seats{db: db, issuer: iss}
}

// Licence is one issued key.
type Licence struct {
	OrgName      string
	Key          string
	Kind         string
	ExpiresAt    time.Time
	RenewalToken string
	LastSeen     *time.Time
	Revoked      bool
	Note         string

	// Status is derived from the invoices, never stored: "bezahlt" flips to
	// "läuft aus" the moment an invoice goes out unpaid, and back again by itself the
	// moment it is settled. Nothing to reset by hand, nothing that can drift.
	Status Status
}

// SeatState is what both the panel and the portal show.
type SeatState struct {
	SubscriptionID string
	Company        string
	Email          string
	Plan           string
	Quantity       int
	Used           int // distinct organisations holding a key
	Free           int
	Cancelled      bool
	Licences       []Licence
}

var (
	ErrNoSuchSubscription = fmt.Errorf("billing: no such subscription, or it is not paid")
	ErrCancelled          = fmt.Errorf("billing: the subscription is cancelled")
	ErrNoSeatsLeft        = fmt.Errorf("billing: every seat is taken")
	ErrOrgNameRequired    = fmt.Errorf("billing: the organisation name is required — it is signed into the key")
)

// State loads the seat picture for one subscription.
func (s *Seats) State(ctx context.Context, subID string) (*SeatState, error) {
	var st SeatState
	var product, interval string
	var cancelled *time.Time
	if err := s.db.QueryRow(ctx, `
		SELECT id, company_name, email, product, interval, quantity, cancelled_at
		  FROM billing_quote_requests
		 WHERE id = $1 AND status = 'paid'`, subID).
		Scan(&st.SubscriptionID, &st.Company, &st.Email, &product, &interval, &st.Quantity, &cancelled); err != nil {
		return nil, ErrNoSuchSubscription
	}
	st.Plan = product + "/" + interval
	st.Cancelled = cancelled != nil

	rows, err := s.db.Query(ctx, `
		SELECT org_name, license_key, kind, expires_at, renewal_token, last_seen_at, revoked_at, note
		  FROM billing_licenses WHERE subscription_id = $1 ORDER BY created_at DESC`, subID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orgs := map[string]bool{}
	for rows.Next() {
		var l Licence
		var revoked *time.Time
		if err := rows.Scan(&l.OrgName, &l.Key, &l.Kind, &l.ExpiresAt, &l.RenewalToken,
			&l.LastSeen, &revoked, &l.Note); err != nil {
			return nil, err
		}
		l.Revoked = revoked != nil
		if status, _, err := LicenceStatus(ctx, s.db, l.RenewalToken); err == nil {
			l.Status = status
		}
		st.Licences = append(st.Licences, l)
		orgs[l.OrgName] = true
	}
	// A seat is an ORGANISATION, not a key: a normal customer holds two keys for one
	// company (the 45-day trial, then the full one). Counting keys would report them
	// as using two of their one seat.
	st.Used = len(orgs)
	st.Free = st.Quantity - st.Used
	if st.Free < 0 {
		st.Free = 0
	}
	return &st, rows.Err()
}

// Issue signs a licence for one end customer and records it.
//
// sendTo may be empty — the key then goes to the subscription's own address.
func (s *Seats) Issue(ctx context.Context, subID, orgName, sendTo, by string) (*Licence, error) {
	if orgName == "" {
		return nil, ErrOrgNameRequired
	}
	st, err := s.State(ctx, subID)
	if err != nil {
		return nil, err
	}
	if st.Cancelled {
		return nil, ErrCancelled
	}

	// Re-issuing for an organisation that already has a key is always allowed (they
	// lost it, it expired, they rebuilt). Only a NEW organisation consumes a seat.
	known := false
	for _, l := range st.Licences {
		if l.OrgName == orgName {
			known = true
			break
		}
	}
	if !known && st.Free <= 0 {
		return nil, ErrNoSeatsLeft
	}

	var product, interval string
	if err := s.db.QueryRow(ctx,
		`SELECT product, interval FROM billing_quote_requests WHERE id = $1`, subID).
		Scan(&product, &interval); err != nil {
		return nil, err
	}

	// An end customer's key expires when the MSP's paid period does. Not later: the
	// MSP bought a year, not a perpetual right to mint keys. Not a fixed 90 days
	// either — the end customer would then depend on our uptime for something their
	// MSP already paid for.
	entitledTo, err := Entitlement(ctx, s.db, subID)
	if err != nil {
		return nil, err
	}

	// The row is created FIRST, so the renewal token exists before the key is signed
	// — the key's mail has to carry that token, and a key mailed without one would
	// leave the customer unable to auto-renew, with no way to fix it after the fact.
	if sendTo == "" {
		sendTo = st.Email
	}
	var token string
	if err := s.db.QueryRow(ctx, `
		INSERT INTO billing_licenses (subscription_id, org_name, license_key, expires_at, kind, note)
		VALUES ($1, $2, '', $3, 'seat', $4)
		RETURNING renewal_token`,
		subID, orgName, entitledTo, "issued by "+by).Scan(&token); err != nil {
		return nil, err
	}

	key, mailErr := s.issuer.IssueUntil(licensing.Request{
		OrgName: orgName, Email: sendTo, Interval: interval, Trial: false,
		RenewalToken: token,
	}, entitledTo, nil, "")
	if key == "" {
		// Nothing was signed: drop the placeholder rather than leave a seat burnt on
		// a licence that does not exist.
		_, _ = s.db.Exec(ctx, `DELETE FROM billing_licenses WHERE renewal_token = $1::uuid`, token)
		return nil, fmt.Errorf("billing: could not sign the licence: %w", mailErr)
	}

	if _, err := s.db.Exec(ctx,
		`UPDATE billing_licenses SET license_key = $2 WHERE renewal_token = $1::uuid`, token, key); err != nil {
		log.Error().Err(err).Str("subscription_id", subID).
			Msg("billing: CRITICAL — licence signed and mailed but not stored")
	}

	log.Info().Str("subscription_id", subID).Str("org", orgName).Str("by", by).
		Msg("billing: seat licence issued")

	l := &Licence{OrgName: orgName, Key: key, Kind: "seat",
		ExpiresAt: entitledTo, RenewalToken: token}
	if mailErr != nil {
		return l, fmt.Errorf("billing: the key was issued but the mail failed: %w", mailErr)
	}
	return l, nil
}

// Revoke stops renewals for one licence.
//
// It is NOT a kill switch, and the UI must not pretend otherwise: the signed key
// stays valid until it expires (35 or 395 days). A self-hosted instance cannot be
// reached — that is the deal, and it is the same deal that makes the product
// sellable to people who will not tolerate a phone-home. What revocation does is
// close the door: the key is not renewed, and it runs out.
func (s *Seats) Revoke(ctx context.Context, renewalToken, by string) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE billing_licenses SET revoked_at = NOW()
		 WHERE renewal_token = $1::uuid AND revoked_at IS NULL`, renewalToken)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("billing: no active licence with that token")
	}
	log.Warn().Str("by", by).Msg("billing: licence revoked — it will not be renewed and expires on its own")
	return nil
}
