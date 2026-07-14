// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Der Kern von Vakt Scan ist die Übersetzung von Scanner-Ausgabe in Findings, und
// sie war bis hierher von keinem Test erreichbar — sie steckte in einer Funktion,
// die einen Unterprozess startet und in die Datenbank schreibt.
//
// Das ist nicht bloß eine Coverage-Lücke. Ein Scan, der die Ausgabe nicht mehr
// versteht, meldet **null Schwachstellen**, und null Schwachstellen ist von einem
// sauberen System nicht zu unterscheiden. Dieselbe Klasse wie die 0-%-Klickrate
// von Vakt Aware: kein fehlender Wert, sondern ein plausibel falscher, der als
// Nachweis in einen Compliance-Bericht wandert.

// trivyRealOutput ist eine gekürzte, aber strukturell unveränderte Trivy-Ausgabe
// (`trivy image --format json`). Die Feldnamen sind Trivys, nicht unsere — genau
// darum geht es: Wenn jemand `VulnerabilityID` in `CveID` umbenennt, weil es
// „schöner" aussieht, bricht dieser Test und nicht erst die nächste Kundenanlage.
const trivyRealOutput = `{
  "SchemaVersion": 2,
  "ArtifactName": "nginx:1.21",
  "Results": [
    {
      "Target": "nginx:1.21 (debian 11.2)",
      "Class": "os-pkgs",
      "Type": "debian",
      "Vulnerabilities": [
        {
          "VulnerabilityID": "CVE-2021-3711",
          "PkgName": "libssl1.1",
          "Title": "openssl: SM2 Decryption Buffer Overflow",
          "Description": "In order to decrypt SM2 encrypted data an application is expected to call the API function EVP_PKEY_decrypt().",
          "Severity": "CRITICAL",
          "CVSS": { "nvd": { "V3Score": 9.8 } }
        },
        {
          "VulnerabilityID": "CVE-2021-3712",
          "PkgName": "libssl1.1",
          "Title": "openssl: Read buffer overruns processing ASN.1 strings",
          "Severity": "MEDIUM",
          "CVSS": { "nvd": { "V3Score": 7.4 } }
        }
      ]
    },
    {
      "Target": "usr/local/bin/app",
      "Class": "lang-pkgs",
      "Vulnerabilities": [
        {
          "VulnerabilityID": "GHSA-xxxx-yyyy-zzzz",
          "Title": "Ein Fund ganz ohne CVSS und ohne Severity",
          "CVSS": {}
        }
      ]
    },
    {
      "Target": "leeres Ergebnis — Trivy liefert das für saubere Layer",
      "Vulnerabilities": []
    }
  ]
}`

func TestFindingsFromTrivy(t *testing.T) {
	payload := ScanPayload{
		ScanID:  "11111111-1111-1111-1111-111111111111",
		OrgID:   "22222222-2222-2222-2222-222222222222",
		AssetID: "33333333-3333-3333-3333-333333333333",
	}

	findings, err := findingsFromTrivy([]byte(trivyRealOutput), payload)
	require.NoError(t, err)
	require.Len(t, findings, 3, "die Funde ALLER Results werden zu einer flachen Liste — pro Result-Block zu gruppieren würde die Hälfte verlieren")

	// Erster Fund: vollständig.
	f := findings[0]
	assert.Equal(t, payload.OrgID, f.OrgID)
	assert.Equal(t, payload.AssetID, f.AssetID)
	require.NotNil(t, f.ScanID)
	assert.Equal(t, payload.ScanID, *f.ScanID)
	require.NotNil(t, f.CVEID)
	assert.Equal(t, "CVE-2021-3711", *f.CVEID)
	assert.Equal(t, "critical", f.Severity, "Trivy schreibt CRITICAL, wir speichern klein")
	require.NotNil(t, f.CVSSScore)
	assert.InDelta(t, 9.8, *f.CVSSScore, 0.001)
	assert.Equal(t, "trivy", f.Scanner)
	assert.Equal(t, []string{"trivy"}, f.Sources)
	assert.Equal(t, "open", f.Status)
	require.NotNil(t, f.RiskScore, "ohne Risikowert lässt sich nichts priorisieren — das ist der halbe Produktnutzen")

	// Dritter Fund: kein CVSS, keine Severity.
	f = findings[2]
	assert.Equal(t, "info", f.Severity, "ohne Severity ist ein Fund `info`, nicht leer")
	assert.Nil(t, f.CVSSScore,
		"ohne CVSS-Wert bleibt der Wert NIL — eine 0 wäre die Behauptung „harmlos“, und das ist eine andere Aussage als „unbekannt“")
	require.NotNil(t, f.CVEID)
	assert.Equal(t, "GHSA-xxxx-yyyy-zzzz", *f.CVEID, "auch ein Advisory ohne CVE-Nummer ist ein Fund")
}

func TestFindingsFromTrivy_LeereUndKaputteAusgabe(t *testing.T) {
	payload := ScanPayload{ScanID: "s", OrgID: "o", AssetID: "a"}

	// Ein sauberes Image: gültiges JSON, keine Funde. Das ist ein ERGEBNIS, kein Fehler.
	findings, err := findingsFromTrivy([]byte(`{"Results":[]}`), payload)
	require.NoError(t, err)
	assert.Empty(t, findings)

	// Kaputte Ausgabe MUSS ein Fehler sein und darf nicht als „keine Funde“
	// durchgehen — sonst meldet ein abgestürzter Scanner ein sauberes System.
	_, err = findingsFromTrivy([]byte(`{"Results": [ das ist kein JSON`), payload)
	require.Error(t, err, "unlesbare Scanner-Ausgabe darf nicht wie ein sauberes Ergebnis aussehen")

	// Auch eine leere Ausgabe (Scanner lief, schrieb nichts) ist ein Fehler.
	_, err = findingsFromTrivy(nil, payload)
	require.Error(t, err)
}

// nucleiRealOutput ist echte Nuclei-Ausgabe: JSONL, ein Objekt pro Zeile.
// Die dritte Zeile ist absichtlich Müll — Nuclei schreibt im Strom, und eine
// unlesbare Zeile darf die Funde davor und danach nicht mitreißen.
const nucleiRealOutput = `{"template-id":"CVE-2021-44228","info":{"name":"Apache Log4j RCE","severity":"critical"},"matched-at":"https://example.test:443"}
{"template-id":"tech-detect","info":{"name":"Nginx erkannt","severity":"info"},"matched-at":"https://example.test"}
{ kaputte zeile
{"template-id":"missing-severity","info":{"name":"Fund ohne Severity"},"matched-at":"https://example.test/x"}`

func TestFindingsFromNuclei(t *testing.T) {
	payload := ScanPayload{
		ScanID:  "11111111-1111-1111-1111-111111111111",
		OrgID:   "22222222-2222-2222-2222-222222222222",
		AssetID: "33333333-3333-3333-3333-333333333333",
	}

	findings := findingsFromNuclei([]byte(nucleiRealOutput), payload)

	// Drei Funde: die zwei vor der kaputten Zeile UND der eine dahinter.
	//
	// Dieser Test hat den Grund geliefert, warum das überhaupt geht. Mit dem alten
	// json.Decoder war es keine „übersprungene Zeile", sondern eine ENDLOSSCHLEIFE:
	// `More()` bleibt nach einem gescheiterten `Decode` auf `true` stehen, weil der
	// Decoder das kaputte Byte nicht überwindet. Der Test lief 600 Sekunden ins
	// Timeout — in Produktion hätte eine halb geschriebene Nuclei-Zeile den Worker
	// dauerhaft bei 100 % CPU festgenagelt.
	require.Len(t, findings, 3,
		"eine kaputte Zeile darf genau eine Zeile kosten — nicht den Rest des Laufs und nicht den Worker")

	f := findings[0]
	assert.Equal(t, "CVE-2021-44228", f.TemplateID, "die Template-ID ist der Dedup-Schlüssel für Nicht-CVE-Funde — geht sie verloren, wird jeder Scan zum neuen Fund")
	assert.Equal(t, "Apache Log4j RCE", f.Title)
	assert.Equal(t, "critical", f.Severity)
	assert.Equal(t, "nuclei", f.Scanner)
	assert.Nil(t, f.CVEID, "Nuclei liefert keine CVE-ID — die Template-ID ist der Schlüssel")
	require.NotNil(t, f.ScanID)
	assert.Equal(t, payload.ScanID, *f.ScanID)

	assert.Equal(t, "info", findings[1].Severity)

	// Der Fund NACH der kaputten Zeile — der Beweis, dass wirklich nur die eine
	// Zeile verloren geht.
	assert.Equal(t, "missing-severity", findings[2].TemplateID)
	assert.Equal(t, "info", findings[2].Severity, "ohne Severity ist ein Fund `info`")
}

func TestFindingsFromNuclei_LeereAusgabe(t *testing.T) {
	// Nuclei ohne Treffer schreibt gar nichts. Das ist ein Ergebnis, kein Fehler —
	// und darf keine Panik auslösen.
	findings := findingsFromNuclei(nil, ScanPayload{ScanID: "s"})
	assert.Empty(t, findings)

	findings = findingsFromNuclei([]byte("\n\n"), ScanPayload{ScanID: "s"})
	assert.Empty(t, findings)
}
