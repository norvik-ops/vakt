// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktaware

import (
	"html/template"
	"strings"
	"testing"
)

// TestEveryPresetRenders is the regression guard for R1-27-V01.
//
// The predecessor guard (TestNormaliseMailPlaceholders) checked hand-written
// strings and was green while all 50 shipped presets were unsendable — it even
// asserted "unbekannt {{foo}} bleibt", which is the failure mode itself. A test
// over invented input cannot see a gap between the normaliser's token map and
// the token set the product actually ships.
//
// This one takes the shipped library as its input and asserts the whole
// send-path contract per preset: body, subject, sender name and sender address
// must parse, must render, and must leave no unresolved {{token}} behind. A new
// preset with a new token fails here instead of on a customer's SMTP session.
func TestEveryPresetRenders(t *testing.T) {
	presets := presetTemplates()
	if len(presets) == 0 {
		t.Fatal("no presets — presetTemplates() is broken, not the renderer")
	}

	data := mailRenderData{
		FirstName:   "Ada",
		LastName:    "Lovelace",
		Email:       "ada@kunde.test",
		Department:  "IT",
		Company:     "Muster GmbH",
		TrackingURL: "https://vakt.kunde.test/api/v1/vaktaware/t/tok",
		OpenPixel:   template.HTML(openPixelHTML("https://vakt.kunde.test", "tok")),
	}
	addrData := data
	addrData.Company = companySlug(data.Company)
	addrData.OpenPixel = ""
	headerData := data
	headerData.OpenPixel = ""

	for _, p := range presets {
		t.Run(p.ID, func(t *testing.T) {
			bodyTmpl, err := template.New("body").Parse(normaliseMailPlaceholders(p.HTMLBody))
			if err != nil {
				t.Fatalf("parse body: %v", err)
			}
			var body strings.Builder
			if err := bodyTmpl.Execute(&body, data); err != nil {
				t.Fatalf("render body: %v", err)
			}
			assertResolved(t, "html_body", body.String())

			subject, err := renderHeader("subject", p.Subject, headerData)
			if err != nil {
				t.Fatalf("render subject: %v", err)
			}
			assertResolved(t, "subject", subject)

			fromName, err := renderHeader("from_name", p.FromName, headerData)
			if err != nil {
				t.Fatalf("render from_name: %v", err)
			}
			assertResolved(t, "from_name", fromName)

			fromEmail, err := renderHeader("from_email", p.FromEmail, addrData)
			if err != nil {
				t.Fatalf("render from_email: %v", err)
			}
			assertResolved(t, "from_email", fromEmail)
			if !addressSafe.MatchString(fromEmail) {
				t.Errorf("from_email %q is not an address the mail server will accept", fromEmail)
			}
		})
	}
}

// assertResolved fails when a placeholder survived rendering. An unresolved
// {{company}} in a From line is not a cosmetic defect: it is what the recipient
// sees, and in an address it is what makes the SMTP server reject the session.
func assertResolved(t *testing.T, field, rendered string) {
	t.Helper()
	if strings.Contains(rendered, "{{") || strings.Contains(rendered, "}}") {
		t.Errorf("%s still carries an unresolved placeholder: %q", field, rendered)
	}
}

// TestSeedTemplateSpellingRenders covers the other token set that reaches the
// renderer: cmd/seed writes {{TRACK_URL}} into sr_templates, so every instance
// that ran the seed has two templates in that spelling in its database.
func TestSeedTemplateSpellingRenders(t *testing.T) {
	body := normaliseMailPlaceholders(`<p>Klicken Sie <a href="{{TRACK_URL}}">hier</a>.</p>`)
	tmpl, err := template.New("body").Parse(body)
	if err != nil {
		t.Fatalf("parse seeded body: %v", err)
	}
	var sb strings.Builder
	if err := tmpl.Execute(&sb, mailRenderData{TrackingURL: "https://x/t/tok"}); err != nil {
		t.Fatalf("render seeded body: %v", err)
	}
	if !strings.Contains(sb.String(), "https://x/t/tok") {
		t.Errorf("tracking link missing from rendered seed template: %q", sb.String())
	}
}

// TestOpenPixelNotDuplicated: the preset positions the pixel itself via
// {{open_pixel}}; buildMIMEMessage must then not append a second one.
func TestOpenPixelNotDuplicated(t *testing.T) {
	const appURL, token = "https://vakt.kunde.test", "tok"
	body := `<html><body><p>Hallo</p>` + openPixelHTML(appURL, token) + `</body></html>`
	msg := string(buildMIMEMessage("Absender", "a@b.test", "c@d.test", "Betreff", body, token, appURL, true))
	if n := strings.Count(msg, openPixelURL(appURL, token)); n != 1 {
		t.Errorf("open pixel appears %d times, want exactly 1", n)
	}

	// Templates without the placeholder still get one appended.
	plain := `<html><body><p>Hallo</p></body></html>`
	msg = string(buildMIMEMessage("Absender", "a@b.test", "c@d.test", "Betreff", plain, token, appURL, true))
	if n := strings.Count(msg, openPixelURL(appURL, token)); n != 1 {
		t.Errorf("open pixel appears %d times in a template without the placeholder, want exactly 1", n)
	}
}

// TestCompanySlug pins the address-safe rendering of the org name: 21 presets
// build their spoofed sender out of it.
func TestCompanySlug(t *testing.T) {
	cases := map[string]string{
		"Muster GmbH":       "muster-gmbh",
		"Müller & Partner":  "m-ller-partner",
		"ACME":              "acme",
		"  Rand  Spaces   ": "rand-spaces",
	}
	for in, want := range cases {
		if got := companySlug(in); got != want {
			t.Errorf("companySlug(%q) = %q, want %q", in, got, want)
		}
	}
}
