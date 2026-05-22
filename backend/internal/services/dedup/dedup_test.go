package dedup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func key(cveID, assetID string) FindingKey {
	return FindingKey{CVEID: cveID, AssetID: assetID}
}

// TestDeduplicate_MergesTwoScannersForSameCVEAndAsset verifies that when Trivy
// and Nuclei both report the same CVE on the same asset they are merged into a
// single MergedFinding with both scanner names in Sources.
func TestDeduplicate_MergesTwoScannersForSameCVEAndAsset(t *testing.T) {
	findings := []SourcedFinding{
		{ID: "f1", Key: key("CVE-2024-12345", "asset-a"), Scanner: "trivy", Severity: "high", Title: "Some vulnerability"},
		{ID: "f2", Key: key("CVE-2024-12345", "asset-a"), Scanner: "nuclei", Severity: "high", Title: "Some vulnerability (nuclei)"},
	}

	result := Deduplicate(findings)

	require.Len(t, result, 1, "two findings for the same CVE+asset should merge into one")
	merged := result[0]
	assert.Equal(t, "CVE-2024-12345", merged.Key.CVEID)
	assert.Equal(t, "asset-a", merged.Key.AssetID)
	assert.ElementsMatch(t, []string{"trivy", "nuclei"}, merged.Sources)
	assert.Equal(t, "high", merged.Severity)
	// Title comes from the first source.
	assert.Equal(t, "Some vulnerability", merged.Title)
}

// TestDeduplicate_DifferentCVEIDsNotMerged verifies that two findings with
// different CVE IDs on the same asset are NOT merged.
func TestDeduplicate_DifferentCVEIDsNotMerged(t *testing.T) {
	findings := []SourcedFinding{
		{ID: "f1", Key: key("CVE-2024-11111", "asset-a"), Scanner: "trivy", Severity: "medium", Title: "First CVE"},
		{ID: "f2", Key: key("CVE-2024-22222", "asset-a"), Scanner: "nuclei", Severity: "low", Title: "Second CVE"},
	}

	result := Deduplicate(findings)

	require.Len(t, result, 2, "different CVE IDs must not be merged")
	cveIDs := []string{result[0].Key.CVEID, result[1].Key.CVEID}
	assert.Contains(t, cveIDs, "CVE-2024-11111")
	assert.Contains(t, cveIDs, "CVE-2024-22222")
}

// TestDeduplicate_EmptyCVEIDPassthrough verifies that findings without a CVE ID
// are not deduplicated — each is kept as its own separate MergedFinding.
func TestDeduplicate_EmptyCVEIDPassthrough(t *testing.T) {
	findings := []SourcedFinding{
		{ID: "f1", Key: FindingKey{CVEID: "", AssetID: "asset-a"}, Scanner: "trivy", Severity: "low", Title: "No-CVE finding #1"},
		{ID: "f2", Key: FindingKey{CVEID: "", AssetID: "asset-a"}, Scanner: "nuclei", Severity: "low", Title: "No-CVE finding #2"},
	}

	result := Deduplicate(findings)

	assert.Len(t, result, 2, "findings with empty CVEID must not be merged even if AssetID matches")
}

// TestDeduplicate_SeverityMerge_HighWins verifies that when Trivy reports
// medium and Nuclei reports high for the same CVE on the same asset the merged
// finding carries the higher severity (high).
func TestDeduplicate_SeverityMerge_HighWins(t *testing.T) {
	findings := []SourcedFinding{
		{ID: "f1", Key: key("CVE-2024-77777", "asset-b"), Scanner: "trivy", Severity: "medium", Title: "Vuln X"},
		{ID: "f2", Key: key("CVE-2024-77777", "asset-b"), Scanner: "nuclei", Severity: "high", Title: "Vuln X (nuclei)"},
	}

	result := Deduplicate(findings)

	require.Len(t, result, 1)
	assert.Equal(t, "high", result[0].Severity, "merged severity should be the highest across sources")
	assert.ElementsMatch(t, []string{"trivy", "nuclei"}, result[0].Sources)
}

// TestDeduplicate_SeverityMerge_CriticalWins verifies the full severity order:
// critical > high > medium > low.
func TestDeduplicate_SeverityMerge_CriticalWins(t *testing.T) {
	findings := []SourcedFinding{
		{ID: "f1", Key: key("CVE-2024-55555", "asset-c"), Scanner: "trivy", Severity: "low", Title: "T"},
		{ID: "f2", Key: key("CVE-2024-55555", "asset-c"), Scanner: "nuclei", Severity: "medium", Title: "T"},
		{ID: "f3", Key: key("CVE-2024-55555", "asset-c"), Scanner: "openvas", Severity: "critical", Title: "T"},
	}

	result := Deduplicate(findings)

	require.Len(t, result, 1)
	assert.Equal(t, "critical", result[0].Severity)
	assert.ElementsMatch(t, []string{"trivy", "nuclei", "openvas"}, result[0].Sources)
}

// TestDeduplicate_SameScannerNotDuplicated verifies that duplicate reports from
// the same scanner for the same CVE are merged into one entry but the scanner
// name appears only once in Sources.
func TestDeduplicate_SameScannerNotDuplicated(t *testing.T) {
	findings := []SourcedFinding{
		{ID: "f1", Key: key("CVE-2024-33333", "asset-d"), Scanner: "trivy", Severity: "high", Title: "Dup"},
		{ID: "f2", Key: key("CVE-2024-33333", "asset-d"), Scanner: "trivy", Severity: "high", Title: "Dup again"},
	}

	result := Deduplicate(findings)

	require.Len(t, result, 1)
	assert.Equal(t, []string{"trivy"}, result[0].Sources, "same scanner should appear only once")
}

// TestDeduplicate_EmptyInput returns an empty slice without panicking.
func TestDeduplicate_EmptyInput(t *testing.T) {
	result := Deduplicate(nil)
	assert.Empty(t, result)

	result2 := Deduplicate([]SourcedFinding{})
	assert.Empty(t, result2)
}

// TestDeduplicate_TitleFromFirstSource verifies that the title of the merged
// finding is taken from the first source that reported it.
func TestDeduplicate_TitleFromFirstSource(t *testing.T) {
	findings := []SourcedFinding{
		{ID: "f1", Key: key("CVE-2024-44444", "asset-e"), Scanner: "trivy", Severity: "medium", Title: "Title from Trivy"},
		{ID: "f2", Key: key("CVE-2024-44444", "asset-e"), Scanner: "nuclei", Severity: "low", Title: "Title from Nuclei"},
	}

	result := Deduplicate(findings)

	require.Len(t, result, 1)
	assert.Equal(t, "Title from Trivy", result[0].Title)
}

// TestDeduplicate_DifferentAssetsNotMerged verifies the same CVE on different
// assets produces separate merged findings.
func TestDeduplicate_DifferentAssetsNotMerged(t *testing.T) {
	findings := []SourcedFinding{
		{ID: "f1", Key: key("CVE-2024-66666", "asset-x"), Scanner: "trivy", Severity: "high", Title: "V"},
		{ID: "f2", Key: key("CVE-2024-66666", "asset-y"), Scanner: "trivy", Severity: "high", Title: "V"},
	}

	result := Deduplicate(findings)

	assert.Len(t, result, 2, "same CVE on different assets must not be merged")
}
