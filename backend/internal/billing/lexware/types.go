// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

// Wire types for the Lexware Office API.
//
// These began as untyped JSON maps, which the untyped-interface ratchet rightly
// rejected. Typed structs are not just a lint box to tick here:
// the invoice payload has three fields that must be exactly right (`taxType`
// must be "vatfree" for a §19 small business, `taxRatePercentage` must be 0, and
// `finalize` decides whether the invoice can ever be paid). A typo in a map key
// is silently ignored by the server and surfaces as a mysteriously wrong invoice;
// a typo in a struct field does not compile.
//
// Field names and JSON tags follow the Lexware API reference verbatim.

// ── Contacts ─────────────────────────────────────────────────────────────────

type contactPerson struct {
	LastName     string `json:"lastName"`
	Primary      bool   `json:"primary"`
	EmailAddress string `json:"emailAddress,omitempty"`
}

type contactCompany struct {
	Name string `json:"name"`
	// AllowTaxFreeInvoices must be true for every customer we invoice: as a §19
	// small business every invoice is tax-free, and Lexware refuses a vat-free
	// invoice against a contact that does not permit them.
	AllowTaxFreeInvoices bool            `json:"allowTaxFreeInvoices"`
	VatRegistrationID    string          `json:"vatRegistrationId,omitempty"`
	ContactPersons       []contactPerson `json:"contactPersons,omitempty"`
}

type contactAddress struct {
	Street      string `json:"street,omitempty"`
	Zip         string `json:"zip,omitempty"`
	City        string `json:"city,omitempty"`
	CountryCode string `json:"countryCode"`
}

type contactAddresses struct {
	Billing []contactAddress `json:"billing"`
}

type contactEmails struct {
	Business []string `json:"business,omitempty"`
}

// contactRoles: an empty customer object is how Lexware expresses "this contact
// is a customer". There is no flag; the presence of the key is the signal.
type contactRoles struct {
	Customer struct{} `json:"customer"`
}

type contactRequest struct {
	Version   int              `json:"version"`
	Roles     contactRoles     `json:"roles"`
	Company   contactCompany   `json:"company"`
	Addresses contactAddresses `json:"addresses"`
	Emails    contactEmails    `json:"emailAddresses"`
}

// ── Invoices ─────────────────────────────────────────────────────────────────

type unitPrice struct {
	Currency  string  `json:"currency"`
	NetAmount float64 `json:"netAmount"`
	// TaxRatePercentage is 0 and must stay 0 — §19 UStG, no VAT is charged.
	TaxRatePercentage float64 `json:"taxRatePercentage"`
}

type lineItem struct {
	Type        string    `json:"type"` // "custom"
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Quantity    float64   `json:"quantity"`
	UnitName    string    `json:"unitName"`
	UnitPrice   unitPrice `json:"unitPrice"`
}

type totalPrice struct {
	Currency string `json:"currency"`
	// TotalDiscountPercentage is Lexware's own rebate field — a discount on the whole
	// voucher, which under our vatfree tax conditions applies to the net amount.
	// Lexware has no per-line discount and rejects a negative unitPrice, so this is
	// the only way to make a rebate VISIBLE on the paper: the line item carries the
	// list price, and the customer sees what was taken off it.
	//
	// Deliberately NOT omitempty, and that is the whole point of this comment. The
	// Lexware docs say: "A contact-specific default will be set if available and no
	// total discount was send." So OMITTING the field is the dangerous case — a
	// default rebate stored on the contact (in the Lexware web UI, where nothing in
	// this codebase can see it) would silently price the invoice, and the amount we
	// wrote to billing_invoices would be wrong while everything looked fine.
	//
	// Sending an explicit 0 keeps the amount OURS. If Lexware ever rejects a 0 here,
	// it fails loudly at CreateInvoice and nothing is created — which is the failure
	// we want, rather than a customer quietly billed an amount we never computed.
	TotalDiscountPercentage float64 `json:"totalDiscountPercentage"`
}

// taxConditions carries the single most load-bearing field in this package.
//
// The account is flagged smallBusiness (§19 UStG) — verified against the live
// API — and Lexware then accepts *only* "vatfree". "net" or "gross" are
// rejected outright.
//
// TaxTypeNote is deliberately omitted (omitempty): Lexware then inserts the §19
// wording stored on the organisation. Keeping that legally-prescribed sentence
// in one place means it cannot drift out of sync with what the tax office wants.
type taxConditions struct {
	TaxType     string `json:"taxType"`
	TaxTypeNote string `json:"taxTypeNote,omitempty"`
}

type paymentConditions struct {
	PaymentTermLabel    string `json:"paymentTermLabel"`
	PaymentTermDuration int    `json:"paymentTermDuration"`
}

type shippingConditions struct {
	ShippingType string `json:"shippingType"` // "service"
	ShippingDate string `json:"shippingDate"`
}

type invoiceAddress struct {
	ContactID string `json:"contactId"`
}

type invoiceRequest struct {
	VoucherDate        string             `json:"voucherDate"`
	Address            invoiceAddress     `json:"address"`
	LineItems          []lineItem         `json:"lineItems"`
	TotalPrice         totalPrice         `json:"totalPrice"`
	TaxConditions      taxConditions      `json:"taxConditions"`
	PaymentConditions  paymentConditions  `json:"paymentConditions"`
	ShippingConditions shippingConditions `json:"shippingConditions"`
	Title              string             `json:"title,omitempty"`
	Introduction       string             `json:"introduction,omitempty"`
}

// ── Event subscriptions ──────────────────────────────────────────────────────

type eventSubscriptionRequest struct {
	EventType   string `json:"eventType"`
	CallbackURL string `json:"callbackUrl"`
}

// ── Shared response envelopes ────────────────────────────────────────────────

type idResponse struct {
	ID string `json:"id"`
}

type subscriptionList struct {
	Content []subscription `json:"content"`
}
