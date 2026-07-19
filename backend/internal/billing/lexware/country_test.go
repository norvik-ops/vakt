// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestIsAlpha2(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"DE", true},
		{"AT", true},
		{"CH", true},
		{"", false},
		{"D", false},
		{"DEU", false},
		{"D1", false},
		{"12", false},
		// Kleinschreibung ist hier absichtlich FALSCH: Der Aufrufer normalisiert
		// vorher mit strings.ToUpper. Faellt die Normalisierung weg, schlaegt das
		// hier fehl statt still ein "de" an Lexware zu schicken.
		{"de", false},
		// Das Trennzeichen aus der Auswahlliste im Bestellformular. Es ist dort
		// disabled und damit nicht waehlbar — aber ein direkter API-Aufruf koennte
		// es senden, und 6 Bytes sind keine 2 Buchstaben.
		{"──────────", false},
	}
	for _, tc := range tests {
		if got := isAlpha2(tc.in); got != tc.want {
			t.Errorf("isAlpha2(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestRequestQuoteRejectsInvalidCountry haelt fest, dass Freitext im Landfeld gar
// nicht erst bis zur Rechnung kommt.
//
// h.db ist ABSICHTLICH nil: Ein ungueltiges Land muss abgelehnt werden, BEVOR
// irgendetwas die Datenbank beruehrt. Panikt dieser Test eines Tages mit einem
// nil-Pointer, ist die Pruefung unter das INSERT gerutscht — dann landet Muell in
// billing_quote_requests und faellt erst bei der Rechnungserstellung auf, also
// nach der Freigabe durch einen Menschen. Genau das will die Pruefung verhindern.
func TestRequestQuoteRejectsInvalidCountry(t *testing.T) {
	for _, country := range []string{"Schweiz", "Oesterreich", "D", "DEU", "X!"} {
		e := echo.New()
		body := `{"company_name":"ACME GmbH","email":"einkauf@acme.de","country_code":"` + country + `"}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		h := &Handler{}
		if err := h.RequestQuote(e.NewContext(req, rec)); err != nil {
			t.Fatalf("country %q: handler returned error: %v", country, err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Errorf("country %q: want 400, got %d (%s)", country, rec.Code, strings.TrimSpace(rec.Body.String()))
		}
	}
}
