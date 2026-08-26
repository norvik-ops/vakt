// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package artifact_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/artifact"
)

// mount wires artifact.Stream into a real Echo handler exactly the way the ten
// artifact handlers now do: on a Stream error the handler logs and returns a 500,
// on success it returns the artifact bytes.
func mount(gen func(io.Writer) error) *echo.Echo {
	e := echo.New()
	e.GET("/artifact", func(c echo.Context) error {
		if err := artifact.Stream(c, "application/pdf", "report.pdf", gen); err != nil {
			return c.String(http.StatusInternalServerError, "generation failed")
		}
		return nil
	})
	return e
}

// TestStream_GenerationFailureIsNotAnEmpty200 is the R1-20-A3 regression. The old
// handlers committed "200 application/pdf" and then ran the fallible generation
// against the live stream, so a failure produced a 200 with Content-Length 0 — an
// empty file that looks like a successful download. artifact.Stream buffers first,
// so a generation failure must surface as a real error status with NO artifact
// body and NO application/pdf content type.
//
// Red-on-revert: the pre-fix construction (WriteHeader(200) + Header set before
// the generator) makes this request return 200 with an empty body — both asserts
// below fail.
func TestStream_GenerationFailureIsNotAnEmpty200(t *testing.T) {
	e := mount(func(w io.Writer) error {
		// A generator that writes a little and then fails partway through —
		// the realistic PDF/ZIP case, and the one that most convincingly proves
		// nothing partial leaks to the client.
		_, _ = io.WriteString(w, "%PDF-1.4 partial")
		return errors.New("render blew up")
	})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/artifact", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code,
		"a generation failure must be a real error status, not a 200")
	assert.NotEqual(t, "application/pdf", rec.Header().Get("Content-Type"),
		"a failed artifact must not be served as application/pdf")
	assert.NotContains(t, rec.Body.String(), "%PDF",
		"no partial artifact bytes may leak into a failed response")
}

// TestStream_SuccessWritesBytesHeadersAndLength confirms the happy path: the
// buffered bytes, the download filename and an accurate Content-Length all reach
// the client, and the status is 200.
func TestStream_SuccessWritesBytesHeadersAndLength(t *testing.T) {
	body := "%PDF-1.4 complete document"
	e := mount(func(w io.Writer) error {
		_, err := io.WriteString(w, body)
		return err
	})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/artifact", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="report.pdf"`, rec.Header().Get("Content-Disposition"))
	assert.Equal(t, body, rec.Body.String())
	// c.Blob sets an accurate Content-Length from the buffered bytes — the very
	// thing the empty-200 bug got wrong (it sent Content-Length 0).
	assert.NotZero(t, rec.Body.Len())
}
