package ai

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestReportsWithoutScan covers the R1-KW-15 follow-up: Vakt Scan is archived,
// so a report must not tell management "0 open vulnerabilities" — nothing was
// measured — and the agent offers no findings tool.
func TestReportsWithoutScan(t *testing.T) {
	cc := &ComplianceContext{OrgName: "Acme"}
	for name, build := range map[string]func(*ComplianceContext) string{
		"gap":       buildGapAnalysisPrompt,
		"risk":      buildRiskSummaryPrompt,
		"executive": buildExecutiveSummaryPrompt,
	} {
		assert.NotContains(t, build(cc), "Sicherheitslücken", name)
		assert.NotContains(t, build(cc), "%!", name+": format verbs must stay balanced")
	}
}

func TestAgentToolsWithoutFindings(t *testing.T) {
	var names []string
	for _, tl := range DefaultAgentTools(nil) {
		names = append(names, tl.Name())
	}
	joined := strings.Join(names, ",")
	assert.NotContains(t, joined, "list_open_findings")
	assert.Contains(t, joined, "list_stale_evidence")
}
