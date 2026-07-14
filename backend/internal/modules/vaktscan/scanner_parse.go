// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Die Übersetzung von Scanner-Ausgabe in Findings — und nichts sonst.
//
// Warum das ein eigener Ort ist: Ein Scan, der die Ausgabe nicht mehr versteht,
// meldet **null Schwachstellen** — und null Schwachstellen sieht exakt so aus wie
// ein sauberes System. Das ist dieselbe Klasse wie die 0-%-Klickrate von Vakt
// Aware: kein fehlender Wert, sondern ein falscher, und zwar ein plausibel
// falscher, der in einem Compliance-Bericht als Nachweis landet.
//
// Solange das Parsen mitten in einer Funktion steckte, die einen Unterprozess
// startet und in die Datenbank schreibt, konnte es kein Test anfassen. Hier ist es
// eine reine Funktion: Bytes rein, Findings raus, keine Datenbank, kein Prozess.

// runScanner ist die Naht zum externen Scanner-Binary.
//
// Produktion startet trivy/nuclei als Unterprozess; ein Test hängt hier eine
// Funktion ein, die eine aufgezeichnete Ausgabe zurückgibt. Damit ist der ganze
// Weg — Ziel-Prüfung, Ausführung, Fehlerbehandlung, Parsen, Upsert, Status —
// fahrbar, ohne dass trivy installiert sein muss.
//
// Eine Paket-Variable statt eines Interfaces, weil RunTrivyScan/RunNucleiScan
// freie Funktionen sind (der Worker ruft sie so auf) und es genau eine
// Implementierung gibt. Tests setzen sie zurück, was sie seriell macht — das ist
// der Preis und er ist hier klein.
var runScanner = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

// SetScanRunnerForTest hängt das Scanner-Binary aus und gibt eine Funktion zurück,
// die es zurückhängt. Nur für Tests.
//
// Der Name sagt, was er ist, damit ihn niemand versehentlich für eine
// Konfigurationsschnittstelle hält: Produktion ruft ihn nie auf. Die Alternative
// wäre ein exportiertes, beschreibbares `ScanRunner` gewesen — eine veränderliche
// öffentliche Variable, die jeder jederzeit umbiegen kann. Ein benannter Haken mit
// Rückgabe der Wiederherstellung ist ehrlicher und lässt sich nicht vergessen.
//
// Die Cross-Package-Integrationstests liegen in einem eigenen Paket und kommen an
// die unexportierte Variable sonst nicht heran.
func SetScanRunnerForTest(f func(ctx context.Context, name string, args ...string) ([]byte, error)) (restore func()) {
	prev := runScanner
	runScanner = f
	return func() { runScanner = prev }
}

// findingsFromTrivy übersetzt eine Trivy-JSON-Ausgabe in Findings.
//
// Trivy meldet Schwachstellen gruppiert nach Ziel (Image-Layer, Dateisystem-Pfad);
// uns interessiert die flache Liste. Fehlende Felder sind bei Trivy normal und
// kein Fehler: Ein Eintrag ohne Severity ist `info`, einer ohne CVSS-Wert hat
// keinen (nicht 0 — das wäre die Behauptung „harmlos"), einer ohne CVE-ID ist ein
// Fund ohne CVE, kein Fund ohne Namen.
func findingsFromTrivy(out []byte, payload ScanPayload) ([]Finding, error) {
	var parsed trivyOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("parse trivy output: %w", err)
	}

	scanID := payload.ScanID
	var findings []Finding
	for _, result := range parsed.Results {
		for _, vuln := range result.Vulnerabilities {
			severity := strings.ToLower(vuln.Severity)
			if severity == "" {
				severity = "info"
			}

			var cvss *float64
			if vuln.CVSS.NVD.V3Score > 0 {
				v := vuln.CVSS.NVD.V3Score
				cvss = &v
			}

			var cveID *string
			if vuln.VulnerabilityID != "" {
				id := vuln.VulnerabilityID
				cveID = &id
			}

			f := Finding{
				OrgID:       payload.OrgID,
				AssetID:     payload.AssetID,
				ScanID:      &scanID,
				CVEID:       cveID,
				Title:       vuln.Title,
				Description: vuln.Description,
				Severity:    severity,
				CVSSScore:   cvss,
				Scanner:     "trivy",
				Sources:     []string{"trivy"},
				Status:      "open",
				LastSeenAt:  time.Now(),
			}
			ComputeRiskScore(&f)
			findings = append(findings, f)
		}
	}
	return findings, nil
}

// findingsFromNuclei übersetzt eine Nuclei-Ausgabe (JSONL — ein JSON-Objekt pro
// Zeile) in Findings.
//
// Eine kaputte Zeile wird übersprungen, nicht der ganze Lauf verworfen: Nuclei
// schreibt seine Treffer im Strom, und ein einzelner unlesbarer Eintrag darf die
// hundert davor und danach nicht mitreißen.
//
// ── Warum zeilenweise und nicht mit json.Decoder ──────────────────────────────
//
// Vorher stand hier ein `json.Decoder` in einer `for decoder.More()`-Schleife, die
// bei einem Decode-Fehler `continue` machte. Das ist eine ENDLOSSCHLEIFE: `More()`
// liefert nach einem gescheiterten `Decode` weiterhin `true`, weil der Decoder auf
// dem kaputten Byte stehen bleibt — er kommt von selbst nicht darüber hinweg. Eine
// einzige unvollständige Zeile (abgebrochener Prozess, volle Platte, halb
// geschriebener Puffer) hätte den Worker-Goroutine für immer bei 100 % CPU
// festgenagelt. Gefunden von dem Test, der genau eine solche Zeile enthält — er
// lief 600 Sekunden ins Timeout, statt fehlzuschlagen (2026-07-14).
//
// JSONL ist zeilenorientiert, also wird es zeilenorientiert gelesen. Damit ist das
// Überspringen echt: Die Zeile danach wird wieder gelesen.
func findingsFromNuclei(out []byte, payload ScanPayload) []Finding {
	scanID := payload.ScanID
	var findings []Finding

	scanner := bufio.NewScanner(bytes.NewReader(out))
	// Nuclei-Treffer tragen den gematchten Request; eine Zeile kann lang werden.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var r nucleiResult
		if err := json.Unmarshal(line, &r); err != nil {
			log.Warn().Err(err).Str("scan_id", payload.ScanID).
				Msg("nuclei: unlesbare Ausgabezeile übersprungen")
			continue
		}

		severity := strings.ToLower(r.Info.Severity)
		if severity == "" {
			severity = "info"
		}

		f := Finding{
			OrgID:      payload.OrgID,
			AssetID:    payload.AssetID,
			ScanID:     &scanID,
			Title:      r.Info.Name,
			Severity:   severity,
			Scanner:    "nuclei",
			TemplateID: r.TemplateID,
			Sources:    []string{"nuclei"},
			Status:     "open",
			LastSeenAt: time.Now(),
		}
		ComputeRiskScore(&f)
		findings = append(findings, f)
	}
	return findings
}
