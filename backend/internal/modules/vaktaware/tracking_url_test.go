// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktaware

import (
	"strings"
	"testing"
)

// TestTrackingURL_PointsToClickPath is the S127-2 (D5) guard. The campaign email's
// click link MUST route to /t/ (TrackClick), not /track/ (TrackOpen, the pixel).
// It used to build /track/, so clicks were recorded as opens. Asserting the path
// against the segment the router registers — not a hard-coded literal — so the
// test moves with the route if it is ever renamed.
func TestTrackingURL_PointsToClickPath(t *testing.T) {
	cfg := SMTPConfig{AppURL: "https://vakt.example"}
	got := cfg.trackingURL("TOKEN123")

	// The click link must contain the click segment "/t/" and the token…
	if !strings.Contains(got, "/api/v1/vaktaware/t/TOKEN123") {
		t.Errorf("click link %q must route to the /t/ (TrackClick) path", got)
	}
	// …and must NOT use the open-pixel segment "/track/".
	if strings.Contains(got, "/vaktaware/track/") {
		t.Errorf("click link %q must NOT point to /track/ (that is the open pixel)", got)
	}
}

// TestBuildMIMEMessage_OpenPixelUsesTrackPath keeps the counterpart honest: the
// open pixel must stay on /track/ (TrackOpen), distinct from the click link.
func TestBuildMIMEMessage_OpenPixelUsesTrackPath(t *testing.T) {
	msg := buildMIMEMessage("Sender", "s@x.test", "target@x.test", "Subj",
		"<html><body>hi</body></html>", "PIXTOKEN", "https://vakt.example", true)
	s := string(msg)
	if !strings.Contains(s, "/api/v1/vaktaware/track/PIXTOKEN") {
		t.Errorf("open pixel must use the /track/ path; message did not contain it")
	}
}
