// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Prüfung einer ausländischen USt-IdNr. gegen VIES (EU-Kommission).
//
// Warum das eine Haftungsfrage ist und kein Formatcheck: Bei Reverse Charge geht die
// Steuerschuld auf den Kunden über — aber nur, wenn dessen Unternehmereigenschaft
// nachgewiesen ist. Ist seine Nummer ungültig, schulden WIR die Umsatzsteuer, die wir
// nie berechnet haben. Ein Feld, in das jemand "ABC" tippen kann, trägt das nicht.
//
// ── Datenabfluss (Skill `data-egress`) ──────────────────────────────────────────
// Dieser Aufruf verlässt die eigene Infrastruktur. Das ist zulässig und kein Bruch der
// Invariante: Er läuft auf dem BILLING-Server (api.norvikops.de), nicht in einer
// Kundeninstanz, und übermittelt ausschließlich die USt-IdNr. eines Geschäftspartners
// an eine Behörde der EU-Kommission — kein Vakt-Nutzdatum, keine ISMS-Daten. Die
// Prüfung ist gesetzlich veranlasst, nicht analytisch.
//
// ── Was hier NOCH NICHT geht ────────────────────────────────────────────────────
// VIES kennt zwei Auskunftstiefen:
//
//	EINFACH      "ist die Nummer gültig?" — offen, ohne Registrierung nutzbar.
//	QUALIFIZIERT zusätzlich Name und Anschrift des Inhabers, abgeglichen mit den
//	             übermittelten Daten. NUR das ist der Nachweis, der bei einer
//	             Betriebsprüfung trägt, und er verlangt die EIGENE USt-IdNr. als
//	             Anfragenden.
//
// NorvikOps hat noch keine eigene USt-IdNr. (S130, offener Punkt 5). Deshalb ist hier
// heute nur die EINFACHE Prüfung implementiert. Die Struktur trägt die qualifizierte
// bereits (RequesterVATID, Qualified, TraderName/-Address), und Qualified bleibt false,
// solange keine eigene Nummer konfiguriert ist.
//
// Das ist bewusst so freigegeben: Die einfache Prüfung ist deutlich besser als gar keine
// — sie fängt Tippfehler, erfundene und erloschene Nummern —, sie ist aber KEIN
// vollwertiger Nachweis. Wer VIESResult.Qualified == false sieht, weiß das.
//
// Siehe docs/stories/s130-umsatzsteuer-dach.md (AP2) und ADR-0074.

const viesBaseURL = "https://ec.europa.eu/taxation_customs/vies/rest-api"

// VIESResult ist das, was gespeichert wird — der Nachweis, nicht nur die Antwort.
type VIESResult struct {
	CountryCode string
	VATNumber   string
	Valid       bool
	Qualified   bool // mit Name/Anschrift geprüft — heute immer false, siehe oben
	TraderName  string
	TraderAddr  string
	CheckedAt   time.Time
	// RawStatus trägt, was VIES gemeldet hat, auch wenn es kein klares ja/nein war.
	// Ein Dienstausfall darf nicht wie ein "ungültig" aussehen und erst recht nicht
	// wie ein "gültig".
	RawStatus string
	// RequestIdentifier vergibt VIES nur bei qualifizierter Anfrage. Sie ist DER Beleg
	// gegenüber dem Finanzamt; leer heißt "geprüft, aber nicht nachweisbar geprüft".
	RequestIdentifier string
}

// VIESClient fragt die EU-Kommission.
type VIESClient struct {
	http *http.Client
	// RequesterVATID ist die EIGENE USt-IdNr. Leer = nur einfache Prüfung möglich.
	RequesterVATID string
}

func NewVIESClient(requesterVATID string) *VIESClient {
	return &VIESClient{
		// Kurzes Timeout mit Absicht: Der Aufruf hängt an einem Freigabeklick, den ein
		// Mensch gerade macht. Lieber ein sauberes "nicht prüfbar" als eine Seite, die
		// eine halbe Minute steht.
		http:           &http.Client{Timeout: 10 * time.Second},
		RequesterVATID: strings.ToUpper(strings.TrimSpace(requesterVATID)),
	}
}

// ErrVIESUnavailable trennt "Dienst antwortet nicht" von "Nummer ist ungültig".
//
// Diese Unterscheidung ist der Kern: Beides führt dazu, dass NICHT mit Reverse Charge
// abgerechnet wird — aber aus verschiedenen Gründen, und nur einer davon ist ein Problem
// des Kunden. Wer beides zu "ungültig" zusammenzieht, schickt einen Kunden mit korrekter
// Nummer weg, weil ein Server der EU-Kommission gerade wartet.
var ErrVIESUnavailable = fmt.Errorf("vies: Prüfdienst nicht erreichbar")

// Check prüft eine USt-IdNr.
//
// FAIL-CLOSED: Jeder Fehler führt dazu, dass Valid false bleibt. Es gibt keinen Pfad,
// auf dem ein Ausfall des Dienstes eine Nummer als gültig durchgehen lässt — die
// Steuerschuld läge sonst bei uns.
func (c *VIESClient) Check(ctx context.Context, vatID string) (VIESResult, error) {
	cc, num, err := splitVATID(vatID)
	if err != nil {
		return VIESResult{
			CountryCode: cc, VATNumber: num,
			CheckedAt: time.Now().UTC(), RawStatus: "malformed",
		}, err
	}
	return c.checkAt(ctx, viesBaseURL, cc, num)
}

// checkAt ist Check mit einstellbarem Endpunkt — damit die Fehlerpfade gegen einen
// httptest-Server prüfbar sind, ohne die EU-Kommission zu befragen.
//
// Die Fehlerpfade sind hier das Wesentliche: Ein Test, der nur den Erfolgsfall abdeckt,
// belegt nicht, dass ein Dienstausfall nicht als "gültig" durchgeht — und genau das ist
// die Eigenschaft, an der die Steuerhaftung hängt.
func (c *VIESClient) checkAt(ctx context.Context, baseURL, cc, num string) (VIESResult, error) {
	res := VIESResult{CountryCode: cc, VATNumber: num, CheckedAt: time.Now().UTC()}

	url := fmt.Sprintf("%s/ms/%s/vat/%s", baseURL, cc, num)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return res, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		res.RawStatus = "unreachable"
		return res, fmt.Errorf("%w: %v", ErrVIESUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		res.RawStatus = "unreadable"
		return res, fmt.Errorf("%w: %v", ErrVIESUnavailable, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		res.RawStatus = fmt.Sprintf("http_%d", resp.StatusCode)
		return res, fmt.Errorf("%w: status %d", ErrVIESUnavailable, resp.StatusCode)
	}

	// Die Feldnamen sind gegen die ECHTE Antwort der EU-Kommission belegt (2026-07-19),
	// nicht aus der Doku abgeleitet. Das ist kein Detail: Eine frühere Fassung las
	// "valid" statt "isValid" — der Wert blieb damit IMMER false, und jede gültige
	// USt-IdNr. wäre abgewiesen worden. Die Unit-Tests bemerkten es nicht, weil sie mit
	// derselben erfundenen JSON-Form gefüttert waren.
	//
	// Echte Antwort (gekürzt):
	//   {"isValid": false, "userError": "INVALID", "name": "", "address": "",
	//    "requestIdentifier": "", "vatNumber": "315037332", "requestDate": "..."}
	var body struct {
		IsValid           bool   `json:"isValid"`
		UserError         string `json:"userError"`
		VATNumber         string `json:"vatNumber"`
		Name              string `json:"name"`
		Address           string `json:"address"`
		RequestIdentifier string `json:"requestIdentifier"`
		RequestDate       string `json:"requestDate"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		res.RawStatus = "undecodable"
		return res, fmt.Errorf("%w: %v", ErrVIESUnavailable, err)
	}

	res.Valid = body.IsValid
	res.TraderName = body.Name
	res.TraderAddr = body.Address

	// requestIdentifier vergibt VIES nur bei einer QUALIFIZIERTEN Anfrage (also mit
	// eigener USt-IdNr. als Anfragendem). Genau diese Kennung ist der Beleg, den eine
	// Betriebsprüfung sehen will — bei der einfachen Abfrage bleibt sie leer, und das
	// ist der sichtbare Unterschied zwischen "geprüft" und "nachweisbar geprüft".
	res.RequestIdentifier = body.RequestIdentifier

	// userError trägt den Grund, wenn die Nummer nicht gilt (z. B. "INVALID"). Ohne ihn
	// sähe eine erloschene Nummer aus wie ein Tippfehler.
	if body.IsValid {
		res.RawStatus = "ok"
	} else if body.UserError != "" {
		res.RawStatus = "invalid:" + body.UserError
	} else {
		res.RawStatus = "invalid"
	}

	// Qualified bleibt false, bis eine eigene USt-IdNr. konfiguriert ist UND der
	// qualifizierte Endpunkt genutzt wird. Name und Anschrift, die die einfache
	// Auskunft evtl. mitliefert, machen sie NICHT qualifiziert — qualifiziert heißt
	// abgeglichen mit den von uns übermittelten Daten, und das haben wir nicht getan.
	res.Qualified = false

	return res, nil
}

// splitVATID zerlegt "ATU12345678" in Land und Nummer.
//
// Der Ländercode der USt-IdNr. wird bewusst NICHT gegen das Land des Kunden geprüft:
// Beides kann legitim auseinanderfallen (Niederlassung, Organschaft). Ob die Kombination
// steuerlich trägt, ist eine Frage an den Steuerberater, keine, die ein Parser
// beantwortet. Hier geht es nur darum, VIES korrekt anzufragen.
func splitVATID(vatID string) (country, number string, err error) {
	s := strings.ToUpper(strings.TrimSpace(vatID))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, "-", "")

	if len(s) < 3 {
		return "", "", fmt.Errorf("vies: USt-IdNr. zu kurz: %q", vatID)
	}
	country, number = s[:2], s[2:]
	if !isAlpha2(country) {
		return "", "", fmt.Errorf("vies: kein gültiger Länderpräfix in %q", vatID)
	}
	if !euCountries[country] {
		// Griechenland meldet sich umsatzsteuerlich als EL, nicht GR — ISO und
		// USt-IdNr. weichen genau hier voneinander ab. Ohne diesen Sonderfall wäre
		// jede griechische Nummer als "nicht EU" abgewiesen worden.
		if country != "EL" {
			return "", "", fmt.Errorf("vies: %s ist kein EU-Mitgliedstaat — VIES kennt es nicht", country)
		}
	}
	if number == "" {
		return "", "", fmt.Errorf("vies: USt-IdNr. ohne Nummernteil: %q", vatID)
	}
	return country, number, nil
}
