// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

// Package lexware talks to the Lexware Office (formerly lexoffice) public API
// to create contacts and invoices for direct/invoice sales, and to read back
// payment status.
//
// Why this exists: Vakt is sold to DACH B2B buyers who purchase by invoice and
// bank transfer, not by card. Selling through a US merchant-of-record also sat
// badly with a product whose whole promise is that data stays out of US clouds.
// Lexware Office hosts in Germany, has an Art. 28 DPA, and is where the books
// live anyway.
//
// Only ever active on the billing instance (api.norvikops.de) — a customer's
// self-hosted Vakt never has VAKT_LEXWARE_API_KEY set, so this package stays dark.
package lexware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

// BaseURL is the Lexware Office API gateway. The old lexoffice.io host was
// retired in December 2025.
const BaseURL = "https://api.lexware.io"

// Lexware enforces 2 requests/second across ALL endpoints, token-bucket, and
// warns that sustained overruns lead to a permanent block. We stay under it by
// construction rather than by reacting to 429s.
const rateLimitPerSecond = 2

// lexwareTime is the only date format Lexware accepts on a voucher.
//
// time.RFC3339 produces "2026-07-12T09:17:13+00:00" — and Lexware rejects it
// with a flat 400: "The date value ... cannot be parsed." It insists on
// milliseconds ("2026-07-12T09:17:13.000+00:00"), which RFC3339 does not emit.
// Found the hard way: the very first invoice this code tried to create failed,
// and the reason was invisible because the error body was not surfaced.
const lexwareTime = "2006-01-02T15:04:05.000-07:00"

// Client is a Lexware Office API client. Safe for concurrent use.
type Client struct {
	apiKey string
	http   *http.Client
	lim    *rate.Limiter

	// smallBusiness spiegelt § 19 UStG. Default true = Zustand vor S130: jede Rechnung
	// geht als "vatfree" raus, ohne Fallunterscheidung nach Land.
	//
	// Bewusst Konfiguration und nicht aus Lexwares Profile() gelesen, obwohl das Profil
	// den Wert kennt: Die Steuerbehandlung einer Rechnung darf nicht davon abhängen, ob
	// ein API-Aufruf gerade durchkommt. Ein Aussetzer würde sonst still das Steuerregime
	// wechseln. Stattdessen prüft VerifyTaxStatus() beim Start gegen das Profil und
	// meldet eine Abweichung laut — siehe dort.
	smallBusiness bool
}

func New(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		http:   &http.Client{Timeout: 20 * time.Second},
		lim:    rate.NewLimiter(rate.Limit(rateLimitPerSecond), 1),
		// Default = heutiges Verhalten. Wer die Regelbesteuerung will, muss sie
		// ausdrücklich einschalten; kein stiller Wechsel durch ein fehlendes Flag.
		smallBusiness: true,
	}
}

// WithSmallBusiness schaltet zwischen § 19 UStG und Regelbesteuerung um.
//
// false bedeutet: Ab jetzt entscheidet das Land des Kunden über die Steuerbehandlung
// (siehe tax.go). Das ist der Schalter aus S130 — die Umstellung ist damit eine
// Konfigurations- und keine Code-Änderung.
func (c *Client) WithSmallBusiness(b bool) *Client {
	c.smallBusiness = b
	return c
}

// VerifyTaxStatus vergleicht unsere Konfiguration mit dem, was Lexware über den
// Mandanten weiß, und gibt eine Abweichung als Fehler zurück.
//
// Warum das nötig ist: Die beiden Zustände können unabhängig voneinander wechseln.
// Wird der Mandant in Lexware auf Regelbesteuerung umgestellt, während der Dienst noch
// smallBusiness=true fährt, gehen weiter "vatfree"-Rechnungen raus — Umsatzsteuer wird
// geschuldet, aber nicht ausgewiesen (§ 14c UStG). Umgekehrt schickte der Dienst
// "net"/19 an einen Mandanten, der noch als Kleinunternehmer geführt wird.
//
// Beides ist still. Deshalb wird es beim Start einmal aktiv geprüft, statt darauf zu
// hoffen, dass jemand daran denkt.
func (c *Client) VerifyTaxStatus(ctx context.Context) error {
	prof, err := c.Profile(ctx)
	if err != nil {
		return fmt.Errorf("lexware: Steuerstatus nicht prüfbar: %w", err)
	}
	if prof.SmallBusiness != c.smallBusiness {
		return fmt.Errorf(
			"lexware: Steuerstatus weicht ab — Lexware führt den Mandanten mit smallBusiness=%v, "+
				"dieser Dienst rechnet mit smallBusiness=%v. Bis das übereinstimmt, ist jede "+
				"Rechnung steuerlich falsch (§ 14c UStG). Konfiguration oder Mandant angleichen",
			prof.SmallBusiness, c.smallBusiness)
	}
	return nil
}

// Enabled reports whether an API key is configured. Every caller must check
// this — on a customer's self-hosted instance the key is empty and every
// Lexware call must be skipped rather than attempted and failed.
func (c *Client) Enabled() bool { return c != nil && c.apiKey != "" }

// errNotApplicable marks a 406, which Lexware uses for "this voucher is still a
// draft". Their docs state plainly: "This is not an error condition."
var errNotApplicable = fmt.Errorf("lexware: resource not applicable (draft)")

func (c *Client) do(ctx context.Context, method, path string, body []byte, accept string) ([]byte, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("lexware: no API key configured")
	}
	if err := c.lim.Wait(ctx); err != nil {
		return nil, err
	}

	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, BaseURL+path, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if accept == "" {
		accept = "application/json"
	}
	req.Header.Set("Accept", accept)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lexware: %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("lexware: read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotAcceptable {
		return nil, errNotApplicable
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// Surface Lexware's own explanation. Without it, a 400 is unactionable —
		// the first real invoice failed on a date format and the log said only
		// "status 400", which cost a debugging round against the live API.
		//
		// Only the `message` field is taken, never the whole body: Lexware echoes
		// submitted fields back in validation errors, and those carry customer data.
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &e)
		if e.Message != "" {
			return nil, fmt.Errorf("lexware: %s %s: status %d: %s", method, path, resp.StatusCode, e.Message)
		}
		return nil, fmt.Errorf("lexware: %s %s: status %d", method, path, resp.StatusCode)
	}
	return raw, nil
}

// ── Profile ──────────────────────────────────────────────────────────────────

type Profile struct {
	CompanyName   string `json:"companyName"`
	SmallBusiness bool   `json:"smallBusiness"` // Kleinunternehmer nach § 19 UStG
	TaxType       string `json:"taxType"`       // net | gross | vatfree
}

func (c *Client) Profile(ctx context.Context) (*Profile, error) {
	raw, err := c.do(ctx, http.MethodGet, "/v1/profile", nil, "")
	if err != nil {
		return nil, err
	}
	var p Profile
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("lexware: decode profile: %w", err)
	}
	return &p, nil
}

// ── Contacts ─────────────────────────────────────────────────────────────────

// ContactInput is the subset of a Lexware contact we need for a B2B sale.
type ContactInput struct {
	CompanyName string
	VATID       string // USt-IdNr., may be empty
	ContactName string
	Email       string
	Street      string
	Zip         string
	City        string
	CountryCode string // ISO-2, e.g. "DE"
}

// CreateContact creates a customer contact and returns its ID.
//
// allowTaxFreeInvoices is set unconditionally: as a §19 small business every
// invoice we issue is tax-free, and Lexware rejects vat-free invoices for a
// referenced contact that does not permit them.
func (c *Client) CreateContact(ctx context.Context, in ContactInput) (string, error) {
	req := contactRequest{
		Version: 0,
		Company: contactCompany{
			Name:                 in.CompanyName,
			AllowTaxFreeInvoices: true,
			VatRegistrationID:    in.VATID,
		},
		Addresses: contactAddresses{Billing: []contactAddress{{
			Street: in.Street, Zip: in.Zip, City: in.City, CountryCode: in.CountryCode,
		}}},
		Emails: contactEmails{Business: []string{in.Email}},
	}
	if in.ContactName != "" {
		req.Company.ContactPersons = []contactPerson{{
			LastName: in.ContactName, Primary: true, EmailAddress: in.Email,
		}}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("lexware: marshal contact: %w", err)
	}
	raw, err := c.do(ctx, http.MethodPost, "/v1/contacts", body, "")
	if err != nil {
		return "", err
	}
	var out idResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("lexware: decode contact: %w", err)
	}
	return out.ID, nil
}

// ── Invoices ─────────────────────────────────────────────────────────────────

// InvoiceInput describes one Vakt Pro subscription invoice.
//
// The amount is a Charge, not a bare number, and that is deliberate: the caller must
// have priced the invoice through Plan.Charge(), which is the only place a discount
// is applied. Passing a float here would let a call site quietly invoice a list price
// to a customer who has a rebate — which is precisely the bug that would never be
// noticed, because a too-HIGH invoice does not fail, it just gets paid.
type InvoiceInput struct {
	ContactID   string
	Title       string
	Intro       string
	Description string // line item description
	Charge      Charge
	DueInDays   int

	// CountryCode und VATIDVerified entscheiden über die Steuerbehandlung, sobald die
	// Regelbesteuerung greift (siehe tax.go). Unter § 19 UStG werden sie ignoriert.
	//
	// VATIDVerified heißt QUALIFIZIERT GEPRÜFT (VIES, mit Name und Anschrift), nicht
	// "der Kunde hat etwas ins Feld getippt". Wer hier true einsetzt, ohne geprüft zu
	// haben, verlagert die Steuerschuld auf einen Kunden, der sie womöglich nicht trägt —
	// und die Nachforderung landet bei uns.
	CountryCode   string
	VATIDVerified bool
}

// CreateInvoice creates a FINALIZED invoice (status "open") and returns its ID.
//
// Three things are load-bearing and were verified against the live account:
//
//  1. `?finalize=true` — without it Lexware creates a draft, and a draft has no
//     PDF, no invoice number, and cannot be paid. The status of an invoice
//     cannot be changed through the API afterwards, so getting this wrong means
//     the invoice must be finished by hand in the web UI.
//
//  2. taxType "vatfree" — the account is flagged smallBusiness (§ 19 UStG) and
//     Lexware then permits *only* vat-free invoices. `net` or `gross` are
//     rejected. taxTypeNote is deliberately omitted so Lexware inserts the
//     organisation's stored § 19 wording; keeping that text in one place means
//     it cannot drift out of sync with the legally required phrasing.
//
//  3. The discount goes on the TOTAL, not on the line. Lexware has no per-line
//     rebate and rejects a negative unitPrice, so the line carries the LIST price
//     and totalDiscountPercentage takes the rebate off the bottom. The customer
//     therefore reads "2.990 € − 20 % = 2.392 €" instead of an unexplained 2.392 €,
//     which is both the honest presentation and the one their bookkeeping expects.
func (c *Client) CreateInvoice(ctx context.Context, in InvoiceInput) (string, error) {
	if in.DueInDays <= 0 {
		in.DueInDays = 14
	}

	// Einordnung VOR dem Request-Bau und vor allem vor dem finalisierenden POST: Ein
	// EU-Auslandsverkauf ohne geprüfte USt-IdNr. muss hier scheitern, nicht auf dem
	// Papier landen. Eine finalisierte Lexware-Rechnung ist über die API nicht
	// zurückzunehmen — der einzige sichere Moment zum Abbrechen ist dieser hier.
	tax, err := taxTreatmentFor(TaxContext{
		CountryCode:   in.CountryCode,
		VATIDVerified: in.VATIDVerified,
		SmallBusiness: c.smallBusiness,
	})
	if err != nil {
		return "", err
	}

	now := time.Now().Format(lexwareTime)

	req := invoiceRequest{
		VoucherDate: now,
		Address:     invoiceAddress{ContactID: in.ContactID},
		LineItems: []lineItem{{
			Type:        "custom",
			Name:        in.Title,
			Description: in.Description,
			Quantity:    1,
			UnitName:    "Stück",
			UnitPrice: unitPrice{
				Currency: "EUR",
				// The LIST price. The rebate is applied below, by Lexware, so that it is
				// visible on the paper rather than baked into a smaller number.
				NetAmount:         in.Charge.ListEUR(),
				TaxRatePercentage: tax.RatePct,
			},
		}},
		TotalPrice: totalPrice{
			Currency:                "EUR",
			TotalDiscountPercentage: float64(in.Charge.Percent),
		},
		TaxConditions: taxConditions{TaxType: tax.Type, TaxTypeNote: tax.Note},
		PaymentConditions: paymentConditions{
			PaymentTermLabel:    fmt.Sprintf("Zahlbar innerhalb von %d Tagen ohne Abzug.", in.DueInDays),
			PaymentTermDuration: in.DueInDays,
		},
		ShippingConditions: shippingConditions{ShippingType: "service", ShippingDate: now},
		Title:              "Rechnung",
		Introduction:       in.Intro,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("lexware: marshal invoice: %w", err)
	}
	raw, err := c.do(ctx, http.MethodPost, "/v1/invoices?finalize=true", body, "")
	if err != nil {
		return "", err
	}
	var out idResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("lexware: decode invoice: %w", err)
	}
	return out.ID, nil
}

// InvoiceAmountsFor sagt dem Aufrufer, welche Betraege eine Rechnung mit diesen Daten
// tragen wird — bevor sie erstellt wird.
//
// Getrennt von CreateInvoice, statt dessen Signatur zu aendern: Die Betraege werden an
// zwei Stellen gebraucht (Erstverkauf und Verlaengerung), und beide muessen sie SPEICHERN,
// nachdem Lexware bestaetigt hat. Ein zweiter Rueckgabewert haette die Aufrufer gezwungen,
// ihn auch dort entgegenzunehmen, wo sie ihn wegwerfen.
//
// Wichtig: Es ist DIESELBE Funktion, die CreateInvoice intern verwendet. Eine zweite
// Rechnung an anderer Stelle waere genau der Weg, auf dem Beleg und Buchhaltung
// auseinanderlaufen — das Muster steckt schon einmal in diesem Paket (TotalEUR/TotalCents,
// siehe CLAUDE.md).
func (c *Client) InvoiceAmountsFor(netCents int64, countryCode string, vatIDVerified bool) (InvoiceAmounts, error) {
	tax, err := taxTreatmentFor(TaxContext{
		CountryCode:   countryCode,
		VATIDVerified: vatIDVerified,
		SmallBusiness: c.smallBusiness,
	})
	if err != nil {
		return InvoiceAmounts{}, err
	}
	return amountsFor(netCents, tax), nil
}

// InvoicePDF fetches the rendered invoice.
//
// Lexware has no endpoint that mails an invoice to the customer — the only ways
// out are this PDF or a deeplink. We fetch the PDF and send it over our own SMTP
// so the invoice and the license key that follows it arrive from the same sender.
func (c *Client) InvoicePDF(ctx context.Context, invoiceID string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/v1/invoices/"+invoiceID+"/file", nil, "application/pdf")
}

// ── Payments ─────────────────────────────────────────────────────────────────

type Payment struct {
	OpenAmount    json.Number `json:"openAmount"`
	Currency      string      `json:"currency"`
	PaymentStatus string      `json:"paymentStatus"` // balanced | openRevenue | openExpense
	VoucherStatus string      `json:"voucherStatus"` // open | paid | paidoff | voided | ...
}

// Paid reports whether the invoice is settled in full.
//
// Checking `paymentStatus == "balanced"` rather than trusting the webhook is the
// whole point: payment.changed also fires on PARTIAL payments. Issuing a
// 2.990 € license key because someone transferred 100 € would be a nasty way to
// find that out.
func (p *Payment) Paid() bool { return p != nil && p.PaymentStatus == "balanced" }

// PaymentStatus reads the payment state of a voucher.
//
// A 406 means the voucher is still a draft — Lexware documents this as "not an
// error condition", and payment.changed even fires when an invoice is reset from
// open back to draft. Callers get (nil, nil) for that case and should do nothing.
func (c *Client) PaymentStatus(ctx context.Context, voucherID string) (*Payment, error) {
	raw, err := c.do(ctx, http.MethodGet, "/v1/payments/"+voucherID, nil, "")
	if err == errNotApplicable {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p Payment
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("lexware: decode payment: %w", err)
	}
	return &p, nil
}

// ── Event subscriptions ──────────────────────────────────────────────────────

type subscription struct {
	ID          string `json:"subscriptionId"`
	EventType   string `json:"eventType"`
	CallbackURL string `json:"callbackUrl"`
}

// EnsureEventSubscription registers the payment.changed webhook if it is not
// already registered for this callback URL.
//
// This runs on every boot on purpose. Lexware deletes ALL subscriptions created
// with an API key when that key is rotated — and the key expires after 24
// months. Without a boot-time re-registration the webhooks would silently stop
// firing on the day the key is renewed, and license keys would quietly stop
// being issued after payment. Nobody would notice until a customer complained.
func (c *Client) EnsureEventSubscription(ctx context.Context, callbackURL string) error {
	if !c.Enabled() || callbackURL == "" {
		return nil
	}

	raw, err := c.do(ctx, http.MethodGet, "/v1/event-subscriptions", nil, "")
	if err != nil {
		return fmt.Errorf("list event subscriptions: %w", err)
	}
	var list subscriptionList
	if err := json.Unmarshal(raw, &list); err != nil {
		return fmt.Errorf("decode event subscriptions: %w", err)
	}
	for _, s := range list.Content {
		if s.EventType == EventPaymentChanged && s.CallbackURL == callbackURL {
			return nil // already registered
		}
	}

	sub, err := json.Marshal(eventSubscriptionRequest{
		EventType:   EventPaymentChanged,
		CallbackURL: callbackURL,
	})
	if err != nil {
		return fmt.Errorf("marshal event subscription: %w", err)
	}
	if _, err = c.do(ctx, http.MethodPost, "/v1/event-subscriptions", sub, ""); err != nil {
		return fmt.Errorf("create event subscription: %w", err)
	}
	return nil
}

// EventPaymentChanged fires when a payment is assigned to a voucher.
//
// Note what it does NOT mean: money arriving in the bank account. Lexware's bank
// sync only *suggests* matches — "ohne Ihre aktive Übernahme findet keine
// endgültige Verbuchung im System statt". The event follows the human accepting
// the suggestion. That one click per payment is the last manual step in the
// sale, and it doubles as the moment someone eyeballs the amount and the sender.
const EventPaymentChanged = "payment.changed"

// WebhookEvent is the (deliberately thin) payload Lexware POSTs to us. It never
// carries business data — only an ID to go and fetch.
type WebhookEvent struct {
	OrganizationID string `json:"organizationId"`
	EventType      string `json:"eventType"`
	ResourceID     string `json:"resourceId"`
	EventDate      string `json:"eventDate"`
}

// ── Reconciliation: what does Lexware think? ─────────────────────────────────

// Voucher is one invoice as Lexware sees it.
type Voucher struct {
	ID            string  `json:"id"`
	VoucherNumber string  `json:"voucherNumber"`
	VoucherStatus string  `json:"voucherStatus"` // open | paid | voided | overdue | draft
	TotalAmount   float64 `json:"totalAmount"`
	Currency      string  `json:"currency"`
	ContactName   string  `json:"contactName"`
	VoucherDate   string  `json:"voucherDate"`
}

// Voided reports whether Lexware has cancelled this invoice. A storno never reaches
// us as a payment event — that is the whole reason this call exists.
func (v Voucher) Voided() bool { return v.VoucherStatus == "voided" }

type voucherList struct {
	Content       []Voucher `json:"content"`
	TotalElements int       `json:"totalElements"`
	Last          bool      `json:"last"`
}

// Invoices returns every invoice Lexware knows about.
//
// Vakt's own view is not enough, and the gap is not academic:
//
//   - A STORNO in Lexware produces no payment event. An invoice we recorded as paid,
//     for which we already signed and mailed a licence key, can quietly become void —
//     and nothing in our database would ever notice. (The very first test invoice,
//     RE0003, was exactly this: voided in Lexware, "paid" with a key issued in Vakt.)
//   - Invoices raised BY HAND in Lexware do not exist here at all, so every revenue
//     figure in the panel is a partial truth without them.
//
// Neither is fixable by listening harder. It needs asking.
func (c *Client) Invoices(ctx context.Context) ([]Voucher, error) {
	var out []Voucher
	for page := 0; page < 20; page++ { // 20 × 250 = 5000; a hard stop beats an endless loop
		// NOT "overdue": Lexware rejects it in combination with other states
		// ("voucherStatus filter 'overdue' cannot be used in combination with other
		// states", HTTP 400). It is a sub-state of "open" anyway — an open invoice past
		// its due date — so open,paid,voided is the complete set. Found by running it,
		// not by reading: the call failed silently and the reconciliation page cheerfully
		// reported "no drift" while a voided invoice sat right there.
		path := fmt.Sprintf(
			"/v1/voucherlist?voucherType=invoice&voucherStatus=open,paid,voided&size=250&page=%d", page)
		body, err := c.do(ctx, http.MethodGet, path, nil, "application/json")
		if err != nil {
			return nil, fmt.Errorf("lexware: list invoices: %w", err)
		}
		var vl voucherList
		if err := json.Unmarshal(body, &vl); err != nil {
			return nil, fmt.Errorf("lexware: decode invoice list: %w", err)
		}
		out = append(out, vl.Content...)
		if vl.Last || len(vl.Content) == 0 {
			break
		}
	}
	return out, nil
}
