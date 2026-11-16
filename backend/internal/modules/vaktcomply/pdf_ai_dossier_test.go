package vaktcomply

import (
	"bytes"
	"compress/zlib"
	"io"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dossierPDFText gibt die Textstücke eines fpdf-Dokuments in Ausgabereihenfolge
// zurück. fpdf komprimiert die Inhaltsströme mit zlib und schreibt Text mit
// UTF-8-Schriften als UTF-16BE-Literal "(…) Tj".
func dossierPDFText(t *testing.T, pdf []byte) []string {
	t.Helper()
	var out []string
	streamRe := regexp.MustCompile(`(?s)stream\r?\n(.*?)endstream`)
	textRe := regexp.MustCompile(`\(((?:\\.|[^\\()])*)\)\s*Tj`)
	for _, m := range streamRe.FindAllSubmatch(pdf, -1) {
		zr, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			continue // nicht jeder Strom ist ein komprimierter Inhaltsstrom
		}
		raw, err := io.ReadAll(zr)
		_ = zr.Close()
		if err != nil {
			continue
		}
		for _, tm := range textRe.FindAllSubmatch(raw, -1) {
			if s := strings.TrimSpace(decodeUTF16PDFString(tm[1])); s != "" {
				out = append(out, s)
			}
		}
	}
	require.NotEmpty(t, out, "aus der PDF ließ sich kein Text lesen — der Test wäre sonst leer grün")
	return out
}

// decodeUTF16PDFString löst die Escapes eines PDF-Literalstrings auf und
// dekodiert das Ergebnis als UTF-16BE.
func decodeUTF16PDFString(b []byte) string {
	var buf []byte
	for i := 0; i < len(b); i++ {
		if b[i] != '\\' || i+1 >= len(b) {
			buf = append(buf, b[i])
			continue
		}
		i++
		switch c := b[i]; c {
		case 'n':
			buf = append(buf, '\n')
		case 'r':
			buf = append(buf, '\r')
		case 't':
			buf = append(buf, '\t')
		case 'b':
			buf = append(buf, '\b')
		case 'f':
			buf = append(buf, '\f')
		default:
			if c >= '0' && c <= '7' { // oktale Escape-Sequenz, bis zu drei Ziffern
				v, n := 0, 0
				for n < 3 && i < len(b) && b[i] >= '0' && b[i] <= '7' {
					v = v*8 + int(b[i]-'0')
					i++
					n++
				}
				i--
				buf = append(buf, byte(v))
				continue
			}
			buf = append(buf, c)
		}
	}
	if len(buf)%2 != 0 {
		return strings.ReplaceAll(string(buf), "\x00", "")
	}
	u := make([]uint16, 0, len(buf)/2)
	for i := 0; i+1 < len(buf); i += 2 {
		u = append(u, uint16(buf[i])<<8|uint16(buf[i+1]))
	}
	return strings.TrimPrefix(string(utf16.Decode(u)), string(rune(0xFEFF)))
}

// Das Dossier nennt im Kopf die Risikoklasse mit demselben Label wie das Bericht-PDF.
func TestGenerateAIDocumentationPDF_NenntRisikoklasse(t *testing.T) {
	cases := []struct {
		name      string
		riskClass string
		want      string
	}{
		{"hoch", "high", "Risikoklasse: Hohes Risiko (Annex III)"},
		{"begrenzt", "limited", "Risikoklasse: Begrenztes Risiko (Transparenzpflicht)"},
		{"minimal", "minimal", "Risikoklasse: Minimales Risiko"},
		{"verboten", "unacceptable", "Risikoklasse: Inakzeptables Risiko (Verboten)"},
		{"nicht klassifiziert", "", "Risikoklasse: Noch nicht klassifiziert"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			system := &AISystem{ID: "ai-1", Name: "Bewerber-Screening", RiskClass: tc.riskClass}
			doc := &AIDocumentation{AISystemID: "ai-1", Version: 1, SystemDescription: "Sortiert Bewerbungen vor."}
			got, err := GenerateAIDocumentationPDF(system, doc)
			require.NoError(t, err)
			text := dossierPDFText(t, got)
			assert.Contains(t, text, tc.want, "Dossier-Kopf muss die Risikoklasse nennen (gelesen: %q)", text)
			assert.Contains(t, strings.Join(text, " "), "Bewerber-Screening")
		})
	}
}

// Dossier und Bericht nutzen dieselbe Bezeichnung; ein unbekannter Schlüssel
// wird nie zu einer erfundenen Klasse.
func TestAIRiskClassLabel(t *testing.T) {
	assert.Equal(t, "Hohes Risiko (Annex III)", aiRiskClassLabel("high"))
	assert.Equal(t, "Noch nicht klassifiziert", aiRiskClassLabel(""))
	assert.Equal(t, "Noch nicht klassifiziert", aiRiskClassLabel("unbekannt"))
}
