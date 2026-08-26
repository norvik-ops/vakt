package vaktaware

import (
	"bytes"
	"testing"
	"time"
)

// TestTrainingCertificatePDFEncodesGermanCleanly is the byte-level regression guard
// for R1-20-A1: German umlauts / typographic characters must render through an
// embedded UTF-8 font, never as raw UTF-8 bytes into a WinAnsi core font (which
// produces the Ã¼ / ÃŸ mojibake seen live in 32 of 35 deliverables).
//
// Discriminator: a WinAnsi core-font PDF (the garbled path) embeds no font program —
// it has neither /FontFile2 (the TrueType program) nor /CIDFontType2 (the composite
// descendant font). Their presence proves the AddUTF8Font path is live. A raw-byte
// search for 0xC3 0xBC is deliberately NOT used: the fixed content stream is CID-encoded
// (2-byte glyph indices), so such a search would false-positive on coincidental index
// pairs. The embedded-font markers are the reliable, non-coincidental signal.
func TestTrainingCertificatePDFEncodesGermanCleanly(t *testing.T) {
	score := 92
	// Deliverable text that a German compliance product actually emits:
	// umlauts, ß, em-dash, typographic quotes and the euro sign.
	pdf, err := GenerateTrainingCertificatePDF(
		"Phishing-Grundlagen: „Gefährliche Anhänge“ – Prämie 5 €",
		"mitarbeiter@größe.de",
		&score,
		true,
		time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
		"Größe & Söhne GmbH",
	)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(pdf) == 0 {
		t.Fatal("empty pdf")
	}
	if !bytes.Contains(pdf, []byte("/FontFile2")) {
		t.Error("no embedded TrueType font (/FontFile2) — generator still uses a WinAnsi core font")
	}
	if !bytes.Contains(pdf, []byte("/CIDFontType2")) {
		t.Error("no composite UTF-8 font (/CIDFontType2) — generator still uses a WinAnsi core font")
	}
}
