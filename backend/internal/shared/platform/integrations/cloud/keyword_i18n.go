// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import "strings"

// germanKeywordSynonyms maps a lowercase English collector keyword to German
// substrings that actually occur in the shipped ISO 27001 / BSI control titles
// and domains. Those catalogues are German (e.g. "Zugangskontrolle",
// "Identitätsmanagement", "Verschlüsselung"), while the collectors below pass
// English keywords ("access", "identity", "encryption"). FindCKControlsByKeywords
// matches lower(title)/lower(domain) with ILIKE '%keyword%', so an English-only
// keyword list never matched a single German control and the collected evidence
// was attached to nothing (R1-M1-N1 / R1-19-W06).
//
// The map is deliberately NOT exhaustive: it covers the keywords the collectors
// in this package actually use and only lists German terms that appear as a
// substring in a real shipped control. Synonyms that match nothing are harmless
// (they simply never hit), so erring toward more entries is safe. Loanwords that
// are identical in both catalogues (firewall, patch, audit, mfa, server, cloud,
// asset) need no entry — the English keyword already matches.
//
// What this does NOT do: it does not touch the SQL query, does not change how
// existing English keywords match (they are always preserved), and does not add
// German for terms that never appear in the German catalogue.
var germanKeywordSynonyms = map[string][]string{
	"access":         {"zugang", "zugriff"},
	"identity":       {"identität"},
	"encryption":     {"verschlüsselung", "schlüssel"},
	"password":       {"passwort"},
	"authentication": {"authentifizierung"},
	"privileged":     {"privilegiert"},
	"rights":         {"rechte"},
	"account":        {"konto"},
	"credential":     {"anmeldedaten", "zugangsdaten"},
	"key":            {"schlüssel"},
	"audit":          {"prüfung", "überprüfung"},
	"log":            {"protokoll"},
	"logging":        {"protokollierung"},
	"monitoring":     {"überwachung"},
	"incident":       {"vorfall"},
	"vulnerability":  {"schwachstelle"},
	"inventory":      {"inventar"},
	"network":        {"netz", "netzwerk"},
	"configuration":  {"konfiguration"},
	"hardening":      {"härtung"},
	"approval":       {"genehmigung", "freigabe"},
	"review":         {"überprüfung", "prüfung"},
	"policy":         {"richtlinie"},
	"compliance":     {"konformität", "einhaltung"},
	"risk":           {"risiko"},
	"security":       {"sicherheit"},
	"storage":        {"speicher"},
	"availability":   {"verfügbarkeit"},
	"capacity":       {"kapazität"},
	"integrity":      {"integrität"},
	"endpoint":       {"endpunkt"},
	"device":         {"gerät"},
	"recovery":       {"wiederherstellung"},
	"backup":         {"sicherung", "datensicherung"},
}

// withGerman returns the given English collector keywords plus the German
// substrings that match the shipped German control catalogue. The original
// keywords are always preserved (backward compatible), German synonyms are
// appended, and the result is de-duplicated while keeping first-seen order.
// A keyword with no German synonym passes through unchanged.
func withGerman(keywords ...string) []string {
	out := make([]string, 0, len(keywords)*2)
	seen := make(map[string]struct{}, len(keywords)*2)
	add := func(kw string) {
		if _, ok := seen[kw]; ok {
			return
		}
		seen[kw] = struct{}{}
		out = append(out, kw)
	}
	for _, kw := range keywords {
		add(kw)
		for _, de := range germanKeywordSynonyms[strings.ToLower(kw)] {
			add(de)
		}
	}
	return out
}
