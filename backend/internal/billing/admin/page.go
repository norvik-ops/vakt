// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package admin

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/labstack/echo/v4"
)

// page is what every template gets. It carries the chrome — title, which nav item
// is active, the sidebar counts, who is looking — so no handler has to remember to
// fill it in and no page can silently render without navigation.
type page struct {
	Title string
	Nav   string // overview | subs | invoices | licences | lexware | tax
	Who   string
	CSRF  string
	Flash string
	N     counts
}

// counts feed the little badges in the sidebar. They are the one thing on every
// screen, so they are computed in one place — a per-page COUNT(*) that drifts from
// the list below it is worse than no badge at all.
type counts struct {
	Subs         int // paid, not cancelled
	Pending      int // waiting for approval
	OpenInvoices int // billed, not paid — the number that costs money if ignored
	Licences     int // issued keys still alive
}

func (h *Handler) chrome(c echo.Context, title, nav string) page {
	p := page{
		Title: title,
		Nav:   nav,
		Who:   requestEmail(c),
		CSRF:  csrfOf(c),
		Flash: c.QueryParam("flash"),
	}
	if n, err := h.counts(c.Request().Context()); err == nil {
		p.N = n
	}
	return p
}

func (h *Handler) counts(ctx context.Context) (counts, error) {
	var n counts
	err := h.db.QueryRow(ctx, `
		SELECT
		  (SELECT count(*) FROM billing_quote_requests
		    WHERE status = 'paid' AND cancelled_at IS NULL),
		  (SELECT count(*) FROM billing_quote_requests WHERE status = 'requested'),
		  (SELECT count(*) FROM billing_invoices WHERE status = 'open'),
		  (SELECT count(*) FROM billing_licenses
		    WHERE revoked_at IS NULL AND expires_at > NOW())`).
		Scan(&n.Subs, &n.Pending, &n.OpenInvoices, &n.Licences)
	return n, err
}

// eur formats cents the German way: 2.990,00 €. Money is read by a human here, and
// "299000" is not a number anybody can check at a glance.
func eur(cents int64) string {
	neg := cents < 0
	if neg {
		cents = -cents
	}
	whole, frac := cents/100, cents%100

	// Thousands separator, by hand — no locale package, no dependency.
	s := fmt.Sprint(whole)
	var out []byte
	for i, d := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, d)
	}
	sign := ""
	if neg {
		sign = "−"
	}
	return fmt.Sprintf("%s%s,%02d €", sign, out, frac)
}

func day(t time.Time) string { return t.Format("02.01.2006") }
func dayp(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return day(*t)
}

func urlq(s string) string { return url.QueryEscape(s) }
