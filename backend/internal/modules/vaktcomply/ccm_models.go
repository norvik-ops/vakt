package vaktcomply

import "github.com/matharnica/vakt/internal/modules/vaktcomply/reporting"

// CCM (Continuous Control Monitoring) types and logic live in the reporting
// sub-package (S103-3). These aliases keep the existing handler/route surface
// (`ccm_handler.go`, `routes.go`) and the worker entrypoint stable while the
// implementation has moved out of the root package.
type (
	// CCMCheck represents an automated compliance control check definition.
	CCMCheck = reporting.CCMCheck
	// CreateCCMCheckInput holds validated input for creating a CCM check.
	CreateCCMCheckInput = reporting.CreateCCMCheckInput
	// ToggleCCMCheckInput holds the enabled flag for toggling a CCM check.
	ToggleCCMCheckInput = reporting.ToggleCCMCheckInput
	// CCMResult represents the result of a single CCM check execution.
	CCMResult = reporting.CCMResult
)
