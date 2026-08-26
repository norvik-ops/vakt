// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// REV-ESK12 B1/B2 — die BCP-Plan-Schreibpfade ueber HTTP, nicht ueber den
// Service.
//
// WARUM DIESE EBENE
// -----------------
// Beide nachgebesserten Befunde leben zwischen Requestbody und Antwort, nicht im
// Service:
//
//	B1: ein `*int` kann "Feld fehlt" und "Feld ist null" nicht unterscheiden —
//	    der Unterschied entsteht beim Dekodieren und ist in einem
//	    Struct-Literal gar nicht ausdrueckbar. Ein Servicetest haette den
//	    Datenverlust deshalb nicht sehen koennen.
//	B2: der 422-GRUND entsteht im Handler. Die Bereichspruefung lag doppelt vor
//	    (validate-Tags UND Service-Sentinels); die Tags liefen zuerst und
//	    antworteten "Ungültige Eingabe", womit drei der vier benannten Gruende
//	    ueber HTTP unerreichbar waren. Nur ein Test auf Antwortkoerper-Ebene
//	    unterscheidet "422 mit Grund" von "422 ohne".
package vaktcomply

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type bcpHTTPFixture struct {
	e     *echo.Echo
	h     *Handler
	orgID string
}

func newBCPHTTPFixture(t *testing.T) *bcpHTTPFixture {
	t.Helper()
	url := os.Getenv("VAKT_DB_URL")
	if url == "" {
		t.Skip("VAKT_DB_URL not set — dieser Test braucht eine migrierte Postgres (CI setzt sie)")
	}
	pool, err := pgxpool.New(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var orgID string
	slug := fmt.Sprintf("esk12-http-%s", t.Name())
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO organizations (name, slug) VALUES ($1, $1) RETURNING id::text`, slug).Scan(&orgID))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1::uuid`, orgID)
	})

	return &bcpHTTPFixture{e: echo.New(), h: NewHandler(NewService(pool)), orgID: orgID}
}

// call fuehrt einen Request durch den echten Handler und gibt Status und
// dekodierten Body zurueck.
func (f *bcpHTTPFixture) call(t *testing.T, method, path, id, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := f.e.NewContext(req, rec)
	c.Set("org_id", f.orgID)

	var err error
	switch method {
	case http.MethodPost:
		err = f.h.CreateBCPPlan(c)
	case http.MethodPatch:
		c.SetParamNames("id")
		c.SetParamValues(id)
		err = f.h.UpdateBCPPlan(c)
	default:
		t.Fatalf("unbekannte Methode %s", method)
	}
	require.NoError(t, err)

	out := map[string]any{}
	if rec.Body.Len() > 0 {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out), "Antwortkoerper: %s", rec.Body.String())
	}
	return rec.Code, out
}

func (f *bcpHTTPFixture) get(t *testing.T, id string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/bcp/plans/"+id, nil)
	rec := httptest.NewRecorder()
	c := f.e.NewContext(req, rec)
	c.Set("org_id", f.orgID)
	c.SetParamNames("id")
	c.SetParamValues(id)
	require.NoError(t, f.h.GetBCPPlan(c))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	out := map[string]any{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

// TestBCPPlanHTTP_PatchWithoutFieldsKeepsThem ist der Rundlauf aus REV-ESK12 B1
// auf der Achse, auf der der Befund verortet ist: der HTTP-Flaeche, die Kunden
// integrieren. Der zweite PATCH-Body ist woertlich der, den openapi.yaml vor
// diesem Commit als vollstaendig auswies.
//
// GEMESSEN VOR DER NACHBESSERUNG (07b125f, echte Postgres):
//
//	PATCH {"title":"mit werten","status":"active"}
//	  -> 200  "rto_hours":null,"rpo_hours":null,"schutzbedarfsklasse":null
func TestBCPPlanHTTP_PatchWithoutFieldsKeepsThem(t *testing.T) {
	f := newBCPHTTPFixture(t)

	code, created := f.call(t, http.MethodPost, "/bcp/plans", "",
		`{"title":"mit werten","scope":"RZ Nord","version":"2.1","owner":"Leitung IT",
		  "rto_hours":8,"rpo_hours":2,"schutzbedarfsklasse":3}`)
	require.Equal(t, http.StatusCreated, code, "%v", created)
	id, _ := created["id"].(string)
	require.NotEmpty(t, id)

	code, patched := f.call(t, http.MethodPatch, "/bcp/plans/"+id, id,
		`{"title":"mit werten","status":"active"}`)
	require.Equal(t, http.StatusOK, code, "%v", patched)

	for _, tc := range []struct {
		label string
		body  map[string]any
	}{{"PATCH-Antwort", patched}, {"frischer GET", f.get(t, id)}} {
		assert.EqualValues(t, 8, tc.body["rto_hours"], "%s: rto_hours still geloescht", tc.label)
		assert.EqualValues(t, 2, tc.body["rpo_hours"], "%s: rpo_hours still geloescht", tc.label)
		assert.EqualValues(t, 3, tc.body["schutzbedarfsklasse"], "%s: schutzbedarfsklasse still geloescht", tc.label)
		assert.Equal(t, "RZ Nord", tc.body["scope"], "%s: scope still geleert", tc.label)
		assert.Equal(t, "2.1", tc.body["version"], "%s: version still geleert", tc.label)
		assert.Equal(t, "Leitung IT", tc.body["owner"], "%s: owner still geleert", tc.label)
		assert.Equal(t, "active", tc.body["status"], "%s", tc.label)
	}
}

// TestBCPPlanHTTP_PatchExplicitNullClears: der Weg zum Loeschen bleibt offen und
// ist ueber HTTP genau der, den openapi.yaml zusagt.
func TestBCPPlanHTTP_PatchExplicitNullClears(t *testing.T) {
	f := newBCPHTTPFixture(t)

	code, created := f.call(t, http.MethodPost, "/bcp/plans", "",
		`{"title":"zu leeren","rto_hours":8,"rpo_hours":2,"schutzbedarfsklasse":3}`)
	require.Equal(t, http.StatusCreated, code, "%v", created)
	id, _ := created["id"].(string)

	code, patched := f.call(t, http.MethodPatch, "/bcp/plans/"+id, id,
		`{"title":"zu leeren","status":"draft","rto_hours":null,"schutzbedarfsklasse":null}`)
	require.Equal(t, http.StatusOK, code, "%v", patched)

	got := f.get(t, id)
	assert.Nil(t, got["rto_hours"], "explizites null muss loeschen")
	assert.Nil(t, got["schutzbedarfsklasse"], "explizites null muss loeschen")
	assert.EqualValues(t, 2, got["rpo_hours"], "rpo_hours stand nicht im Body und muss bleiben")
	// Und das leere scope: ein mitgeschicktes "" leert weiterhin.
	code, _ = f.call(t, http.MethodPatch, "/bcp/plans/"+id, id,
		`{"title":"zu leeren","status":"draft","scope":""}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "", f.get(t, id)["scope"])
}

// TestBCPPlanHTTP_ValidationReasonReachesCaller ist REV-ESK12 B2: ALLE VIER
// Ablehnungsgruende muessen beim Aufrufer ankommen. Vor der Nachbesserung kam
// nur ErrRPOExceedsRTO durch — die drei anderen fingen die `validate`-Tags
// vorher ab und antworteten "Ungültige Eingabe", also einen 422 ohne Auskunft
// darueber, welches Feld zu korrigieren ist.
//
// Der Fall schutzbedarfsklasse=4 traegt zusaetzlich die Frage aus dem Auftrag:
// er darf NICHT als SQLSTATE-500 durchschlagen. Der CHECK der Tabelle
// (schutzbedarfsklasse IN (1,2,3), Migration 216) wuerde ihn sonst als
// 23514-Fehler fangen, und der Handler macht daraus einen 500.
func TestBCPPlanHTTP_ValidationReasonReachesCaller(t *testing.T) {
	f := newBCPHTTPFixture(t)

	code, created := f.call(t, http.MethodPost, "/bcp/plans", "", `{"title":"basis"}`)
	require.Equal(t, http.StatusCreated, code, "%v", created)
	id, _ := created["id"].(string)

	for _, tc := range []struct {
		name       string
		body       string
		wantReason string
	}{
		{"rto unter 1", `{"title":"x","rto_hours":0}`, "rto_hours must be between 1 and 8760"},
		{"rto negativ", `{"title":"x","rto_hours":-5}`, "rto_hours must be between 1 and 8760"},
		{"rto ueber einem Jahr", `{"title":"x","rto_hours":8761}`, "rto_hours must be between 1 and 8760"},
		{"rto ueber int32", `{"title":"x","rto_hours":2147483648}`, "rto_hours must be between 1 and 8760"},
		{"rpo unter 1", `{"title":"x","rpo_hours":0}`, "rpo_hours must be between 1 and 8760"},
		{"rpo ueber einem Jahr", `{"title":"x","rpo_hours":99999}`, "rpo_hours must be between 1 and 8760"},
		{"schutzbedarfsklasse 0", `{"title":"x","schutzbedarfsklasse":0}`, "schutzbedarfsklasse must be 1, 2 or 3"},
		{"schutzbedarfsklasse 4", `{"title":"x","schutzbedarfsklasse":4}`, "schutzbedarfsklasse must be 1, 2 or 3"},
		{"schutzbedarfsklasse negativ", `{"title":"x","schutzbedarfsklasse":-1}`, "schutzbedarfsklasse must be 1, 2 or 3"},
		{"rpo groesser als rto", `{"title":"x","rto_hours":4,"rpo_hours":8}`, "rpo_hours must be less than or equal to rto_hours"},
	} {
		t.Run("POST "+tc.name, func(t *testing.T) {
			code, body := f.call(t, http.MethodPost, "/bcp/plans", "", tc.body)
			require.Equal(t, http.StatusUnprocessableEntity, code,
				"ein Eingabefehler darf weder 500 (SQLSTATE durchgeschlagen) noch 201 sein: %v", body)
			assert.Equal(t, "VALIDATION_ERROR", body["code"])
			assert.Contains(t, fmt.Sprint(body["error"]), tc.wantReason,
				"der Grund muss beim Aufrufer ankommen, nicht nur der Statuscode")
		})
		t.Run("PATCH "+tc.name, func(t *testing.T) {
			patchBody := strings.Replace(tc.body, `{"title":"x",`, `{"title":"x","status":"draft",`, 1)
			code, body := f.call(t, http.MethodPatch, "/bcp/plans/"+id, id, patchBody)
			require.Equal(t, http.StatusUnprocessableEntity, code, "%v", body)
			assert.Equal(t, "VALIDATION_ERROR", body["code"])
			assert.Contains(t, fmt.Sprint(body["error"]), tc.wantReason)
		})
	}

	// Der Bestand darf von keiner der Ablehnungen beruehrt worden sein.
	got := f.get(t, id)
	assert.Nil(t, got["rto_hours"])
	assert.Nil(t, got["rpo_hours"])
	assert.Nil(t, got["schutzbedarfsklasse"])
}

// TestBCPPlanHTTP_StatusIsAcceptedOnCreate deckt REV-ESK12 B3 von der
// Laufzeitseite: `status` im POST-Body wirkt tatsaechlich. Die Spezifikation
// fuehrt das Feld seit dieser Nachbesserung; ohne diesen Test waere "steht in
// openapi.yaml" wieder nur eine Behauptung.
func TestBCPPlanHTTP_StatusIsAcceptedOnCreate(t *testing.T) {
	f := newBCPHTTPFixture(t)

	code, created := f.call(t, http.MethodPost, "/bcp/plans", "", `{"title":"aktiv","status":"active"}`)
	require.Equal(t, http.StatusCreated, code, "%v", created)
	assert.Equal(t, "active", created["status"])

	code, dflt := f.call(t, http.MethodPost, "/bcp/plans", "", `{"title":"ohne status"}`)
	require.Equal(t, http.StatusCreated, code, "%v", dflt)
	assert.Equal(t, "draft", dflt["status"], "weggelassen bleibt draft — der dokumentierte Default")

	code, bad := f.call(t, http.MethodPost, "/bcp/plans", "", `{"title":"x","status":"erfunden"}`)
	require.Equal(t, http.StatusUnprocessableEntity, code, "%v", bad)
}
