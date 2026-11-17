// Package pdftest liest erzeugte fpdf-Dokumente für Tests aus: welcher Text auf
// der Seite steht und in welcher Füllfarbe er gesetzt ist.
//
// Warum die Farbe: Text, der weiß auf weiß gesetzt ist, steht vollständig in der
// PDF und wird von jeder Textextraktion gefunden — lesbar ist er trotzdem nicht.
// Ein Test, der nur Text extrahiert, ist bei diesem Fehler grün.
//
// Was das Paket NICHT prüft: Hintergrundfarben unter dem Text (es meldet nur die
// Textfarbe), Text außerhalb des sichtbaren Bereichs, Schriftgröße, andere
// Textoperatoren als Tj (fpdf schreibt Zellen und MultiCells mit Tj).
package pdftest

import (
	"bytes"
	"compress/zlib"
	"io"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"
)

// TextRun ist ein Textstück mit der Füllfarbe, mit der es gezeichnet wurde.
// Color ist der PDF-Operand der Nicht-Strich-Farbe, normalisiert auf fpdfs
// Schreibweise: "0.000 g" (Grau) oder "0.118 0.118 0.118 rg" (RGB).
type TextRun struct {
	Page  int
	Text  string
	Color string
}

// White ist die Füllfarbe, die fpdf für SetTextColor(255, 255, 255) schreibt.
const White = "1.000 g"

var streamRe = regexp.MustCompile(`(?s)stream\r?\n(.*?)endstream`)

// TextRuns gibt alle Textstücke des Dokuments in Ausgabereihenfolge zurück.
// Jeder komprimierte Inhaltsstrom gilt als eine Seite; der Grafikzustand
// beginnt je Seite mit Schwarz, wie im PDF-Standard.
func TextRuns(t *testing.T, pdf []byte) []TextRun {
	t.Helper()
	var out []TextRun
	page := 0
	for _, m := range streamRe.FindAllSubmatch(pdf, -1) {
		zr, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			continue // nicht jeder Strom ist ein komprimierter Inhaltsstrom
		}
		raw, err := io.ReadAll(zr)
		_ = zr.Close()
		if err != nil || !bytes.Contains(raw, []byte("BT")) {
			continue // Schrift- oder Bilddaten, kein Seiteninhalt
		}
		page++
		out = append(out, pageRuns(page, raw)...)
	}
	if len(out) == 0 {
		t.Fatal("aus der PDF ließ sich kein Text lesen — der Test wäre sonst leer grün")
	}
	return out
}

// pageRuns wertet einen Seiteninhalt aus. Ein kleiner Tokenizer statt Regex,
// weil Literal-Strings Leerzeichen, Klammern und Operatornamen enthalten können.
func pageRuns(page int, raw []byte) []TextRun {
	var out []TextRun
	color := "0.000 g"
	var stack []string
	var operands []string
	var lastLiteral []byte
	for i := 0; i < len(raw); {
		switch raw[i] {
		case ' ', '\n', '\r', '\t':
			i++
			continue
		case '(':
			j, depth := i+1, 1
			for ; j < len(raw) && depth > 0; j++ {
				switch raw[j] {
				case '\\':
					j++
				case '(':
					depth++
				case ')':
					depth--
				}
			}
			lastLiteral = raw[i+1 : j-1]
			i = j
			continue
		}
		j := i
		for j < len(raw) && raw[j] != ' ' && raw[j] != '\n' && raw[j] != '\r' && raw[j] != '\t' && raw[j] != '(' {
			j++
		}
		tok := string(raw[i:j])
		i = j
		switch tok {
		case "g":
			if n := len(operands); n >= 1 {
				color = operands[n-1] + " g"
			}
		case "rg":
			if n := len(operands); n >= 3 {
				color = strings.Join(operands[n-3:], " ") + " rg"
			}
		case "q":
			stack = append(stack, color)
		case "Q":
			if n := len(stack); n > 0 {
				color, stack = stack[n-1], stack[:n-1]
			}
		case "Tj":
			if s := strings.TrimSpace(decodeUTF16(lastLiteral)); s != "" {
				out = append(out, TextRun{Page: page, Text: s, Color: color})
			}
		default:
			operands = append(operands, tok)
			continue
		}
		operands = operands[:0]
	}
	return out
}

// decodeUTF16 löst die Escapes eines PDF-Literalstrings auf und dekodiert das
// Ergebnis als UTF-16BE (so schreibt fpdf Text in eingebetteten UTF-8-Schriften).
func decodeUTF16(b []byte) string {
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
