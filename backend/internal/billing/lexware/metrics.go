// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// Metrics are the numbers a panel cannot deliver.
//
// The reconciliation page shows a storno for an invoice whose licence key is already
// with the customer. That is the single most expensive thing this system can discover —
// and it only helps if somebody LOOKS. A dashboard nobody opens is a dashboard that
// does not exist.
//
// So the same facts go out as Prometheus metrics, Zabbix scrapes them, and Telegram
// says something. The panel is where you go once you have been told.
type Metrics struct {
	driftSevere    atomic.Int64
	invoicesOpen   atomic.Int64
	invoicesLate   atomic.Int64
	subsActive     atomic.Int64
	mrrCents       atomic.Int64
	licencesLive   atomic.Int64
	lastReconcile  atomic.Int64 // unix seconds; 0 = never
	reconcileError atomic.Int64 // 1 = the last reconciliation failed
}

var billingMetrics Metrics

// WriteMetrics renders the Prometheus exposition format.
//
// No labels, no histograms: these are a handful of gauges a human reads in a Zabbix
// trigger. Cardinality that nobody queries is cost that nobody notices until it is a
// problem.
func WriteMetrics(w io.Writer) {
	m := &billingMetrics
	g := func(name, help string, v int64) {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n%s %d\n", name, help, name, name, v)
	}

	g("vakt_billing_drift_severe",
		"Abweichungen zwischen Vakt und Lexware, die eine Entscheidung brauchen (z. B. Storno mit bereits ausgestelltem Lizenzschluessel). >0 ist immer ein Alarm.",
		m.driftSevere.Load())
	g("vakt_billing_invoices_overdue",
		"Offene Rechnungen, deren Zahlungsziel abgelaufen ist.",
		m.invoicesLate.Load())
	g("vakt_billing_invoices_open",
		"Gestellte, noch nicht bezahlte Rechnungen.",
		m.invoicesOpen.Load())
	g("vakt_billing_subscriptions_active",
		"Bezahlte, nicht gekuendigte Abos.",
		m.subsActive.Load())
	g("vakt_billing_mrr_cents",
		"Monatlich wiederkehrender Umsatz in Cent (Jahresplaene auf den Monat umgerechnet).",
		m.mrrCents.Load())
	g("vakt_billing_licences_live",
		"Ausgestellte Lizenzschluessel, die weder gesperrt noch abgelaufen sind.",
		m.licencesLive.Load())
	g("vakt_billing_reconcile_error",
		"1 = der letzte Lexware-Abgleich ist fehlgeschlagen. Dann sind ALLE anderen Zahlen hier veraltet — ein stiller Abgleich ist gefaehrlicher als gar keiner.",
		m.reconcileError.Load())

	// Age, not a timestamp. A Zabbix trigger asks "is this older than X", and doing
	// that arithmetic on the trigger side means every consumer repeats it — and one of
	// them gets the sign wrong.
	age := int64(0)
	if last := m.lastReconcile.Load(); last > 0 {
		age = time.Now().Unix() - last
	}
	g("vakt_billing_reconcile_age_seconds",
		"Sekunden seit dem letzten erfolgreichen Lexware-Abgleich. Waechst er ueber ein paar Stunden, laeuft der Abgleich nicht mehr — und Stornos bleiben unentdeckt.",
		age)
}

// RefreshMetrics recomputes the gauges. Called after every reconciliation, so the
// numbers a Zabbix trigger sees are never older than the last sweep.
func (h *Handler) RefreshMetrics(ctx context.Context, severe int, reconcileFailed bool) {
	m := &billingMetrics
	m.driftSevere.Store(int64(severe))

	if reconcileFailed {
		m.reconcileError.Store(1)
		// Deliberately do NOT touch lastReconcile: the age keeps growing, and the
		// trigger fires. A failed sweep that quietly kept the old timestamp would look
		// exactly like a healthy one.
		return
	}
	m.reconcileError.Store(0)
	m.lastReconcile.Store(time.Now().Unix())

	var open, late, subs, licences int64
	if err := h.db.QueryRow(ctx, `
		SELECT
		  (SELECT count(*) FROM billing_invoices WHERE status = 'open'),
		  (SELECT count(*) FROM billing_invoices
		    WHERE status = 'open' AND created_at < NOW() - INTERVAL '14 days'),
		  (SELECT count(*) FROM billing_quote_requests
		    WHERE status = 'paid' AND cancelled_at IS NULL),
		  (SELECT count(*) FROM billing_licenses
		    WHERE revoked_at IS NULL AND expires_at > NOW() AND license_key <> '')`).
		Scan(&open, &late, &subs, &licences); err != nil {
		log.Error().Err(err).Msg("billing: metrics query")
		return
	}
	m.invoicesOpen.Store(open)
	m.invoicesLate.Store(late)
	m.subsActive.Store(subs)
	m.licencesLive.Store(licences)

	// MRR from the plan catalogue, not from invoices: an invoice is a moment, a
	// subscription is a rate. Summing invoices would make MRR jump every time somebody
	// pays.
	rows, err := h.db.Query(ctx, `
		SELECT product, interval, quantity FROM billing_quote_requests
		 WHERE status = 'paid' AND cancelled_at IS NULL`)
	if err != nil {
		return
	}
	defer rows.Close()

	var mrr int64
	for rows.Next() {
		var product, interval string
		var qty int
		if err := rows.Scan(&product, &interval, &qty); err != nil {
			continue
		}
		p, err := PlanFor(product, interval)
		if err != nil {
			continue
		}
		cents := p.TotalCents(qty)
		if interval == "year" {
			cents /= 12
		}
		mrr += cents
	}
	m.mrrCents.Store(mrr)
}
