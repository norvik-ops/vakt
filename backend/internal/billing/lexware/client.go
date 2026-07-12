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

// Client is a Lexware Office API client. Safe for concurrent use.
type Client struct {
	apiKey string
	http   *http.Client
	lim    *rate.Limiter
}

func New(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		http:   &http.Client{Timeout: 20 * time.Second},
		lim:    rate.NewLimiter(rate.Limit(rateLimitPerSecond), 1),
	}
}

// Enabled reports whether an API key is configured. Every caller must check
// this — on a customer's self-hosted instance the key is empty and every
// Lexware call must be skipped rather than attempted and failed.
func (c *Client) Enabled() bool { return c != nil && c.apiKey != "" }

// errNotApplicable marks a 406, which Lexware uses for "this voucher is still a
// draft". Their docs state plainly: "This is not an error condition."
var errNotApplicable = fmt.Errorf("lexware: resource not applicable (draft)")

func (c *Client) do(ctx context.Context, method, path string, body any, accept string) ([]byte, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("lexware: no API key configured")
	}
	if err := c.lim.Wait(ctx); err != nil {
		return nil, err
	}

	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("lexware: marshal request: %w", err)
		}
		rdr = bytes.NewReader(b)
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
		// Never log `raw` at call sites without care: Lexware echoes request
		// fields back in validation errors, which can include customer data.
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
	company := map[string]any{
		"name":                 in.CompanyName,
		"allowTaxFreeInvoices": true,
	}
	if in.VATID != "" {
		company["vatRegistrationId"] = in.VATID
	}
	if in.ContactName != "" {
		company["contactPersons"] = []map[string]any{{
			"lastName":     in.ContactName,
			"primary":      true,
			"emailAddress": in.Email,
		}}
	}

	body := map[string]any{
		"version": 0,
		"roles":   map[string]any{"customer": map[string]any{}},
		"company": company,
		"addresses": map[string]any{
			"billing": []map[string]any{{
				"street":      in.Street,
				"zip":         in.Zip,
				"city":        in.City,
				"countryCode": in.CountryCode,
			}},
		},
		"emailAddresses": map[string]any{"business": []string{in.Email}},
	}

	raw, err := c.do(ctx, http.MethodPost, "/v1/contacts", body, "")
	if err != nil {
		return "", err
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("lexware: decode contact: %w", err)
	}
	return out.ID, nil
}

// ── Invoices ─────────────────────────────────────────────────────────────────

// InvoiceInput describes one Vakt Pro subscription invoice.
type InvoiceInput struct {
	ContactID   string
	Title       string
	Intro       string
	Description string // line item description
	NetAmount   float64
	DueInDays   int
}

// CreateInvoice creates a FINALIZED invoice (status "open") and returns its ID.
//
// Two things are load-bearing and were verified against the live account:
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
func (c *Client) CreateInvoice(ctx context.Context, in InvoiceInput) (string, error) {
	if in.DueInDays <= 0 {
		in.DueInDays = 14
	}
	body := map[string]any{
		"voucherDate": time.Now().Format(time.RFC3339),
		"address":     map[string]any{"contactId": in.ContactID},
		"lineItems": []map[string]any{{
			"type":        "custom",
			"name":        in.Title,
			"description": in.Description,
			"quantity":    1,
			"unitName":    "Stück",
			"unitPrice": map[string]any{
				"currency":          "EUR",
				"netAmount":         in.NetAmount,
				"taxRatePercentage": 0,
			},
		}},
		"totalPrice":    map[string]any{"currency": "EUR"},
		"taxConditions": map[string]any{"taxType": "vatfree"},
		"paymentConditions": map[string]any{
			"paymentTermLabel":    fmt.Sprintf("Zahlbar innerhalb von %d Tagen ohne Abzug.", in.DueInDays),
			"paymentTermDuration": in.DueInDays,
		},
		"shippingConditions": map[string]any{
			"shippingType": "service",
			"shippingDate": time.Now().Format(time.RFC3339),
		},
		"title":        "Rechnung",
		"introduction": in.Intro,
	}

	raw, err := c.do(ctx, http.MethodPost, "/v1/invoices?finalize=true", body, "")
	if err != nil {
		return "", err
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("lexware: decode invoice: %w", err)
	}
	return out.ID, nil
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
	var list struct {
		Content []subscription `json:"content"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return fmt.Errorf("decode event subscriptions: %w", err)
	}
	for _, s := range list.Content {
		if s.EventType == EventPaymentChanged && s.CallbackURL == callbackURL {
			return nil // already registered
		}
	}

	_, err = c.do(ctx, http.MethodPost, "/v1/event-subscriptions", map[string]any{
		"eventType":   EventPaymentChanged,
		"callbackUrl": callbackURL,
	}, "")
	if err != nil {
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
