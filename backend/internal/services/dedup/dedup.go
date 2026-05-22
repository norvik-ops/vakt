// Package dedup provides finding deduplication utilities for Vakt Scan.
// It is a pure, stateless package — no DB access, no HTTP.
package dedup

// FindingKey uniquely identifies a vulnerability on a specific asset and port.
// Two findings are considered duplicates when they share the same CVEID + AssetID
// (CVEID must be non-empty).
type FindingKey struct {
	CVEID    string
	AssetID  string
	Port     int
	Protocol string
}

// SourcedFinding is a raw finding as produced by a single scanner.
type SourcedFinding struct {
	ID       string
	Key      FindingKey
	Scanner  string // "trivy" | "nuclei" | "openvas"
	Severity string // "critical" | "high" | "medium" | "low"
	Title    string
}

// MergedFinding is the result of deduplicating one or more SourcedFindings that
// share the same CVE on the same asset.
type MergedFinding struct {
	Key      FindingKey
	Sources  []string // all scanner names that reported this finding
	Severity string   // highest severity from all sources
	Title    string   // title from the first (primary) source
}

// severityRank maps a severity string to an integer so we can compare them.
// Higher is more severe.
var severityRank = map[string]int{
	"critical": 4,
	"high":     3,
	"medium":   2,
	"low":      1,
}

// higherSeverity returns whichever of a or b is more severe.
// Unrecognised strings are treated as rank 0.
func higherSeverity(a, b string) string {
	if severityRank[b] > severityRank[a] {
		return b
	}
	return a
}

// Deduplicate merges findings from multiple scanners.
//
// Two findings are duplicates when CVEID is non-empty AND both CVEID and AssetID
// match. Findings with an empty CVEID are never merged — they are passed through
// as-is, each becoming its own MergedFinding with a single source.
//
// Merge rules:
//   - Severity: the highest severity across all sources wins.
//   - Title: taken from the first source that reported the CVE on that asset.
//   - Sources: all scanner names in the order they were first encountered.
func Deduplicate(findings []SourcedFinding) []MergedFinding {
	type mergeKey struct {
		cveID   string
		assetID string
	}

	// merged accumulates results for CVE-keyed findings.
	merged := make(map[mergeKey]*MergedFinding)
	// order preserves insertion order for the CVE-keyed map.
	var order []mergeKey

	// passthrough collects findings that cannot be deduplicated (empty CVEID).
	var passthrough []MergedFinding

	for _, f := range findings {
		if f.Key.CVEID == "" {
			// Cannot deduplicate without a CVE ID — pass through unchanged.
			passthrough = append(passthrough, MergedFinding{
				Key:      f.Key,
				Sources:  []string{f.Scanner},
				Severity: f.Severity,
				Title:    f.Title,
			})
			continue
		}

		mk := mergeKey{cveID: f.Key.CVEID, assetID: f.Key.AssetID}
		existing, seen := merged[mk]
		if !seen {
			order = append(order, mk)
			merged[mk] = &MergedFinding{
				Key:      f.Key,
				Sources:  []string{f.Scanner},
				Severity: f.Severity,
				Title:    f.Title,
			}
			continue
		}

		// Merge into existing entry.
		existing.Severity = higherSeverity(existing.Severity, f.Severity)
		// Append scanner name only if not already present.
		if !containsString(existing.Sources, f.Scanner) {
			existing.Sources = append(existing.Sources, f.Scanner)
		}
	}

	result := make([]MergedFinding, 0, len(order)+len(passthrough))
	for _, mk := range order {
		result = append(result, *merged[mk])
	}
	result = append(result, passthrough...)
	return result
}

func containsString(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
