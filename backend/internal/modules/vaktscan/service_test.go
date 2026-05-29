package vaktscan

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateFinding_RequiresJustificationForAcceptedRisk(t *testing.T) {
	// Service validates that status=accepted_risk requires a non-empty justification.
	// We use a nil repo/client — the validation is pure logic before any DB call.
	svc := &Service{}
	_, err := svc.UpdateFinding(context.TODO(), "org1", "finding1", UpdateFindingInput{
		Status: ptr("accepted_risk"),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "justification")
}

func TestCalculateRiskScore(t *testing.T) {
	tests := []struct {
		name        string
		cvss        *float64
		epssPercent *float64
		criticality string
		wantMin     float64
	}{
		{
			name:        "high cvss and epss with high criticality",
			cvss:        ptr(7.5),
			epssPercent: ptr(0.5),
			criticality: "high",
			wantMin:     5.0,
		},
		{
			name:        "nil cvss and epss with medium criticality",
			cvss:        nil,
			epssPercent: nil,
			criticality: "medium",
			wantMin:     0.0,
		},
		{
			name:        "critical cvss and epss with critical criticality",
			cvss:        ptr(9.0),
			epssPercent: ptr(0.8),
			criticality: "critical",
			wantMin:     10.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calculateRiskScore(tt.cvss, tt.epssPercent, tt.criticality)
			assert.GreaterOrEqual(t, score, tt.wantMin)
		})
	}
}

func ptr[T any](v T) *T { return &v }
