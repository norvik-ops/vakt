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
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/license"
)

// Feature is a typed identifier for a Pro-tier feature flag.
type Feature = string

// Feature constants mirror license.Feature* so callers only need to import
// this package, not the license package.
const (
	FeatureTISAX               Feature = license.FeatureTISAX
	FeatureDORA                Feature = license.FeatureDORA
	FeatureEUAIAct             Feature = license.FeatureEUAIAct
	FeatureCRA                 Feature = license.FeatureCRA
	FeatureAIAdvisor           Feature = license.FeatureAIAdvisor
	FeatureAuditPDF            Feature = license.FeatureAuditPDF
	FeatureSSO                 Feature = license.FeatureSSO
	FeatureAPI                 Feature = license.FeatureAPI
	FeatureSecReflex           Feature = license.FeatureSecReflex
	FeatureSecPulse            Feature = license.FeatureSecPulse
	FeatureSecVault            Feature = license.FeatureSecVault
	FeatureSecPrivacy          Feature = license.FeatureSecPrivacy
	FeatureBSIGrundschutz      Feature = license.FeatureBSIGrundschutz
	FeatureISO42001            Feature = license.FeatureISO42001
	FeatureGranularPermissions Feature = license.FeatureGranularPermissions
	FeatureSupplierPortal      Feature = license.FeatureSupplierPortal
	FeatureNIS2Reporting       Feature = license.FeatureNIS2Reporting
	FeatureSAMLAuth            Feature = license.FeatureSAMLAuth
	FeatureSCIMProvisioning    Feature = license.FeatureSCIMProvisioning
	FeatureSIEM                Feature = license.FeatureSIEM
	FeatureAgentWriteTools     Feature = license.FeatureAgentWriteTools
	FeatureMultiFramework      Feature = license.FeatureMultiFramework
)

// IsEnabled reports whether the feature is available for the current request.
// It reads the *license.License from the Echo context (set by license.DBMiddleware)
// and asks license.Allows — the same decision Require makes.
//
// "For the current request" includes its HTTP method: on an expired Pro key a
// GET is allowed and a POST is not (see license.Allows). Handler-level callers
// must therefore only use IsEnabled to gate work that matches their route's
// method — asking it on a GET route and then writing would bypass the carve-out.
// All current callers are export handlers on GET routes and field guards on
// PUT/POST handlers, which is exactly the intended use.
func IsEnabled(c echo.Context, feature Feature) bool {
	lic, _ := c.Get("license").(*license.License)
	return license.Allows(lic, c.Request().Method, feature)
}

// Require returns an Echo middleware that rejects the request with HTTP 402
// when the current license does not grant the given feature for this request.
//
// It IS license.Require, not a copy of it. It used to be a copy: the doc said
// "a thin wrapper around license.Require" while the body reimplemented the check
// and silently dropped the expired-license read-only carve-out. Since 164 of 167
// route gates call this function and only 3 called license.Require, the copy was
// the de-facto policy and the documented one was decoration. Keep the delegation.
func Require(feature Feature) echo.MiddlewareFunc {
	return license.Require(feature)
}
