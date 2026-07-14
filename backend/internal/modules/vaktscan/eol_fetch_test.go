// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fetchEOL entscheidet, ob eine Komponente als „end-of-life" gilt — und damit, ob
// im Bericht eine Warnung steht oder nicht. Die Funktion sprach bisher nur mit
// endoflife.date und war von keinem Test erreichbar.
//
// Der heikle Teil ist nicht der Happy Path, sondern die Frage, was passiert, wenn
// die Antwort NICHT kommt oder nicht verstanden wird. Ein Fehler, der still zu
// „nicht end-of-life" wird, ist eine Entwarnung, die niemand gegeben hat.

func eolTestChecker(t *testing.T, h http.Handler) *EOLChecker {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &EOLChecker{
		httpClient: srv.Client(),
		baseURL:    srv.URL,
	}
}

func TestFetchEOL_ErkenntAbgelaufenenZyklus(t *testing.T) {
	c := eolTestChecker(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/nginx.json", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		// Echte Form der endoflife.date-Antwort: `eol` ist mal ein Datum, mal ein
		// Boolean — genau dafür gibt es eolValue.UnmarshalJSON.
		_, _ = w.Write([]byte(`[
			{"cycle":"1.25","eol":false,"latest":"1.25.3"},
			{"cycle":"1.21","eol":"2022-05-24","latest":"1.21.6"},
			{"cycle":"1.20","eol":true,"latest":"1.20.2"}
		]`))
	}))

	status, date, raw, err := c.fetchEOL(context.Background(), "nginx", "1.21")
	require.NoError(t, err)
	assert.Equal(t, "eol", status, "ein Zyklus mit vergangenem EOL-Datum ist end-of-life")
	require.NotNil(t, date)
	assert.Equal(t, "2022-05-24", *date)
	assert.NotEmpty(t, raw, "die Rohantwort wird zwischengespeichert — sonst fragt jede Prüfung erneut nach")
}

func TestFetchEOL_UnterstuetzterZyklus(t *testing.T) {
	c := eolTestChecker(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"cycle":"1.25","eol":false,"latest":"1.25.3"}]`))
	}))

	status, date, _, err := c.fetchEOL(context.Background(), "nginx", "1.25")
	require.NoError(t, err)
	assert.Equal(t, "supported", status)
	assert.Nil(t, date, "ein unterstützter Zyklus hat kein EOL-Datum")
}

func TestFetchEOL_UnbekanntBleibtUnbekannt(t *testing.T) {
	// 404: endoflife.date kennt das Produkt nicht. Das ist KEIN Fehler (die meisten
	// Komponenten einer SBOM stehen dort nicht), aber es ist auch keine Entwarnung.
	c := eolTestChecker(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	status, date, raw, err := c.fetchEOL(context.Background(), "gibts-nicht", "1.0")
	require.NoError(t, err, "ein unbekanntes Produkt ist ein normaler Fall, kein Fehler")
	assert.Equal(t, "unknown", status,
		"unbekannt ist NICHT dasselbe wie unterstützt — eine stille Entwarnung wäre die gefährlichere Lüge")
	assert.Nil(t, date)
	assert.Nil(t, raw)
}

func TestFetchEOL_ServerFehlerIstFehler(t *testing.T) {
	c := eolTestChecker(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	status, _, _, err := c.fetchEOL(context.Background(), "nginx", "1.21")
	require.Error(t, err, "ein 500 der EOL-API darf nicht als „unterstützt“ durchgehen")
	assert.Equal(t, "unknown", status)
}

func TestFetchEOL_ZyklusNichtInDerAntwort(t *testing.T) {
	// Das Produkt ist bekannt, unser Zyklus aber nicht gelistet (z. B. ein
	// Eigenbau-Fork). Auch hier: unbekannt, nicht entwarnt.
	c := eolTestChecker(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"cycle":"1.25","eol":false}]`))
	}))

	status, _, _, err := c.fetchEOL(context.Background(), "nginx", "0.9-eigenbau")
	require.NoError(t, err)
	assert.Equal(t, "unknown", status)
}

// TestEOLValue_RoundTrip nagelt die Falle fest, die das EOL-Tracking jahrelang
// stillgelegt hat: Ein Typ mit eigenem UnmarshalJSON und ohne MarshalJSON ist nur
// in eine Richtung ein Typ. Beim Zurückschreiben entstand aus dem API-Wert
// "2022-05-24" die interne Struktur {"IsEOL":true,"Date":"..."} — die das eigene
// UnmarshalJSON nicht mehr lesen konnte. Kein Compiler-Fehler, kein Test, nur ein
// log.Warn und ein Feature, das nichts meldet.
func TestEOLValue_RoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"konkretes EOL-Datum", `"2022-05-24"`},
		{"boolean true", `true`},
		{"boolean false", `false`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var v eolValue
			require.NoError(t, json.Unmarshal([]byte(tc.raw), &v))

			out, err := json.Marshal(v)
			require.NoError(t, err)
			assert.JSONEq(t, tc.raw, string(out),
				"was hereinkam, muss auch wieder herauskommen — sonst zerstört jeder Round-Trip den Wert lautlos")

			// Und das Ergebnis muss wieder lesbar sein: Genau daran ist es gescheitert.
			var back eolValue
			require.NoError(t, json.Unmarshal(out, &back),
				"das eigene Format muss vom eigenen Parser lesbar sein")
			assert.Equal(t, v, back)
		})
	}
}

// TestEOLCycle_RoundTrip prüft dasselbe für den ganzen Zyklus — das ist die Form,
// die tatsächlich zwischengespeichert wird.
func TestEOLCycle_RoundTrip(t *testing.T) {
	const apiForm = `{"cycle":"1.21","eol":"2022-05-24"}`

	var c eolCycle
	require.NoError(t, json.Unmarshal([]byte(apiForm), &c))

	out, err := json.Marshal(c)
	require.NoError(t, err)

	status, date, err := parseEOLPayload(out)
	require.NoError(t, err, "ein zwischengespeicherter Zyklus muss wieder lesbar sein — sonst fällt die Komponente beim Cache-Treffer heraus")
	assert.Equal(t, "eol", status)
	require.NotNil(t, date)
	assert.Equal(t, "2022-05-24", *date)
}
