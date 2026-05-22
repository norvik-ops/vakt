// Package features provides the centralised feature-flag API for Vakt.
// All Pro-feature gates should call IsEnabled (handler/service layer)
// or use Require as Echo route middleware.
//
// Adding a new Pro-feature requires:
//  1. One Feature constant here
//  2. One string constant in the license package (mirrored from license.Feature*)
//
// No other guard code is needed. See ADR-0023 and the license package.
package features

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/license"
)

// Feature is a typed identifier for a Pro-tier feature flag.
type Feature = string

// Feature constants mirror license.Feature* so callers only need to import
// this package, not the license package.
const (
	FeatureTISAX              Feature = license.FeatureTISAX
	FeatureDORA               Feature = license.FeatureDORA
	FeatureEUAIAct            Feature = license.FeatureEUAIAct
	FeatureCRA                Feature = license.FeatureCRA
	FeatureAIAdvisor          Feature = license.FeatureAIAdvisor
	FeatureAuditPDF           Feature = license.FeatureAuditPDF
	FeatureSSO                Feature = license.FeatureSSO
	FeatureAPI                Feature = license.FeatureAPI
	FeatureSecReflex          Feature = license.FeatureSecReflex
	FeatureSecPulse           Feature = license.FeatureSecPulse
	FeatureGranularPermissions Feature = license.FeatureGranularPermissions
	FeatureSupplierPortal     Feature = license.FeatureSupplierPortal
	FeatureNIS2Reporting      Feature = license.FeatureNIS2Reporting

	// New Pro-features (not yet in license.go constants — added here first,
	// then mirrored when issuing new license keys):
	FeatureSAMLAuth       Feature = "saml_auth"
	FeatureSCIMProvisioning Feature = "scim_provisioning"
	FeatureSIEM           Feature = "siem_export"
	FeatureAgentWriteTools Feature = "agent_write_tools"
	FeatureMultiFramework  Feature = "multi_framework"
)

// IsEnabled reports whether the feature is available for the current request.
// It reads the *license.License from the Echo context (set by license.DBMiddleware).
// Returns false when the license is missing or the feature is not included.
func IsEnabled(c echo.Context, feature Feature) bool {
	lic, _ := c.Get("license").(*license.License)
	if lic == nil {
		return false
	}
	return lic.Has(feature)
}

// Require returns an Echo middleware that rejects the request with HTTP 402
// when the current license does not include the given feature.
// It is a thin wrapper around license.Require, keeping all gates in this package.
func Require(feature Feature) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !IsEnabled(c, feature) {
				return c.JSON(http.StatusPaymentRequired, map[string]string{
					"error":   "feature_not_available",
					"message": "This feature requires Vakt Pro. Visit https://vakt.io for details.",
					"feature": feature,
				})
			}
			return next(c)
		}
	}
}
