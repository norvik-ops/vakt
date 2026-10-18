package ai

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestReportsWithoutScan covers the R1-KW-15 follow-up: with Vakt Scan off a
// report must not tell management "0 open vulnerabilities" — nothing was measured.
func TestReportsWithoutScan(t *testing.T) {
	off := &ComplianceContext{OrgName: "Acme", OpenFindings: 0, FindingsTracked: false}
	on := &ComplianceContext{OrgName: "Acme", OpenFindings: 7, FindingsTracked: true}

	for name, build := range map[string]func(*ComplianceContext) string{
		"gap":       buildGapAnalysisPrompt,
		"risk":      buildRiskSummaryPrompt,
		"executive": buildExecutiveSummaryPrompt,
	} {
		assert.NotContains(t, build(off), "Sicherheitslücken", name)
		assert.Contains(t, build(on), "Offene Sicherheitslücken: 7", name)
		assert.NotContains(t, build(off), "%!", name+": format verbs must stay balanced")
		assert.NotContains(t, build(on), "%!", name+": format verbs must stay balanced")
	}
}

func TestAgentToolsWithoutScan(t *testing.T) {
	names := func(tools []AgentTool) string {
		var n []string
		for _, tl := range tools {
			n = append(n, tl.Name())
		}
		return strings.Join(n, ",")
	}
	assert.Contains(t, names(agentTools(nil, false)), "list_open_findings")
	assert.NotContains(t, names(agentTools(nil, true)), "list_open_findings")
	assert.Contains(t, names(agentTools(nil, true)), "list_stale_evidence")
}
