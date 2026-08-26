// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package artifact provides a single, safe way to return a generated download
// (PDF, CSV, ZIP, …) from an HTTP handler.
package artifact

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Stream renders a downloadable artifact by first writing it completely into an
// in-memory buffer and only then committing status, headers and body.
//
// This closes R1-20-A3. The artifact handlers previously called
// Response().WriteHeader(200) and set Content-Type BEFORE running the (fallible)
// generation directly against the response stream. When generation then failed,
// the status line was already sent, so the client received a
// "200 application/pdf" with Content-Length 0 — an empty file that looks exactly
// like success and carries no error. Buffering first means a generation failure
// leaves the response untouched (no status, no headers written), so the caller
// can return a real 4xx/5xx instead.
//
// gen must write the complete artifact to w and return a non-nil error on any
// failure. On error, Stream writes NOTHING to the response and returns that
// error unchanged, so the caller maps it to the appropriate status. On success,
// Stream sets an attachment Content-Disposition (when filename != "") and emits
// the buffered bytes via c.Blob, which also sets an accurate Content-Length.
//
// The whole artifact is held in memory; use this only for bounded artifacts
// (reports, metadata bundles, exports of a single organisation's records), not
// for streaming arbitrarily large binary payloads.
func Stream(c echo.Context, contentType, filename string, gen func(w io.Writer) error) error {
	var buf bytes.Buffer
	if err := gen(&buf); err != nil {
		return err
	}
	if filename != "" {
		c.Response().Header().Set(
			"Content-Disposition",
			fmt.Sprintf("attachment; filename=%q", filename),
		)
	}
	return c.Blob(http.StatusOK, contentType, buf.Bytes())
}
