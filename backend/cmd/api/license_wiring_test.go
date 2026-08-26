// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/license"
)

// The regression tests for R1-17-L04/L05 and R1-07-B02 live in internal/license
// and rebuild the mounting from cmd/api instead of using it — that package has to
// replace publicKeyPEM to have a valid key at all (activation_reach_test.go:23-27).
// The gap this leaves was measured: with e.Use(licInst.Middleware()) reverted to
// the old value-capturing closure and license.AdoptActivatedKey removed from
// registerRoutes, 632 tests ran, 0 skipped, 0 red. The fix worked but nothing
// held it in place.
//
// The two tests below close that gap from inside package main.

// licenceTestConfig is a fixed config instead of testConfig(): applyMiddleware
// calls log.Fatal() on a wildcard CORS origin in non-demo mode, and testConfig()
// takes its origins from the environment.
func licenceTestConfig() *config.Config {
	return &config.Config{
		Version:     "test",
		APIPort:     "8080",
		CORSOrigins: []string{"https://vakt.test"},
	}
}

// TestActivatedLicenceReachesRoutesThroughTheRealGlobalChain drives the actual
// applyMiddleware from cmd/api/middleware.go — not a rebuild of it.
//
// The defect was that the chain captured the startup licence BY VALUE, so a key
// activated while the process was running stayed invisible to every route that is
// not behind license.DBMiddleware: the SSO entry points, SCIM, and GET /license
// itself. Replacing the licence in the holder is exactly what the activation
// handler does (license/handler.go: h.inst.Set(lic)); that the handler does it is
// covered by internal/license/activation_reach_test.go. What is covered HERE is
// the other half: that the chain cmd/api mounts reads the holder per request.
func TestActivatedLicenceReachesRoutesThroughTheRealGlobalChain(t *testing.T) {
	e := echo.New()
	// A UI-licensed instance: no VAKT_LICENSE_KEY, so startup is Community.
	inst := license.NewInstance(license.Load("", false))
	applyMiddleware(e, licenceTestConfig(), zerolog.New(io.Discard), inst)

	// Two probes on the bare tree — the class /auth/oidc/*, /auth/saml/* and
	// /scim/v2/* belong to: no DBMiddleware, no org context.
	e.GET("/probe/tier", func(c echo.Context) error {
		lic, ok := c.Get("license").(*license.License)
		if !ok || lic == nil {
			return c.String(http.StatusInternalServerError, "no licence on the request context")
		}
		return c.String(http.StatusOK, lic.Tier)
	})
	e.GET("/probe/sso", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	}, license.Require(license.FeatureSSO))

	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	// Before activation — this side stayed green even with the defect present,
	// so on its own it proves nothing. It is here to show the probe measures a
	// change and not a constant.
	require.Equal(t, "community", get("/probe/tier").Body.String(),
		"an instance without a key must report community before activation")
	require.Equal(t, http.StatusPaymentRequired, get("/probe/sso").Code,
		"the feature gate must be closed before activation")

	// POST /license/activate, minus the signature check the key would need: the
	// handler ends in exactly this call on exactly this holder.
	inst.Set(&license.License{
		Tier:     "pro",
		Features: []string{license.FeatureSSO},
		OrgName:  "Kunde GmbH",
		IssuedAt: time.Now(),
	})

	assert.Equal(t, "pro", get("/probe/tier").Body.String(),
		"a licence activated while the process runs does not reach the routes outside `protected` — "+
			"the global chain in cmd/api/middleware.go captured the startup licence by value instead of "+
			"mounting licInst.Middleware() (R1-17-L04/L05)")
	assert.Equal(t, http.StatusOK, get("/probe/sso").Code,
		"the SSO gate still answers 402 after a successful activation — same cause: the global chain "+
			"is not reading the instance holder")
}

// TestStartupWiringForTheInstanceLicence_SourceGuard reads the source of
// server.go and routes.go.
//
// What it asserts: setupEcho builds ONE license.Instance and hands that same
// variable to both applyMiddleware and registerRoutes, and registerRoutes calls
// license.AdoptActivatedKey with it.
//
// What it does NOT assert: that AdoptActivatedKey loads the right key. That is
// behaviour against a real Postgres and is measured in
// internal/license/activation_db_integration_test.go (build tag `integration`).
//
// Why source and not behaviour: registerRoutes returns early without a reachable
// database, so a behavioural test would need Docker. `make check` runs in the
// pre-push hook and deliberately excludes everything Docker-bound (Makefile,
// admission rule for `gates`), and a file behind `-tags=integration` in this
// package would land outside INTEGRATION_PKGS, where no CI job runs it
// (ci.yml:838). A guard that runs everywhere beats a better test that runs nowhere.
//
// Two separate reverts each make this red: dropping the AdoptActivatedKey call
// (the instance falls back to Community after every restart — the "permanently"
// in R1-07-B02), and building a second Instance for one of the two callers (the
// activation would then reach one half of the tree only).
func TestStartupWiringForTheInstanceLicence_SourceGuard(t *testing.T) {
	fset := token.NewFileSet()

	server, err := parser.ParseFile(fset, "server.go", nil, parser.SkipObjectResolution)
	require.NoError(t, err, "parse server.go")
	routes, err := parser.ParseFile(fset, "routes.go", nil, parser.SkipObjectResolution)
	require.NoError(t, err, "parse routes.go")

	setupEchoFn := findFunc(server, "setupEcho")
	require.NotNil(t, setupEchoFn, "setupEcho not found in server.go — this guard has lost its subject")
	registerRoutesFn := findFunc(routes, "registerRoutes")
	require.NotNil(t, registerRoutesFn, "registerRoutes not found in routes.go — this guard has lost its subject")

	// (1) setupEcho creates exactly one holder.
	holders := assignedFrom(setupEchoFn, "license", "NewInstance")
	require.Len(t, holders, 1,
		"setupEcho must build exactly one license.Instance; found %v. Two holders mean an activation "+
			"reaches only the half of the tree whose holder was written to", holders)
	holder := holders[0]

	// (2) Both consumers get that same holder.
	assert.Contains(t, identArgs(setupEchoFn, "applyMiddleware"), holder,
		"applyMiddleware does not receive the instance licence holder %q — the global middleware chain "+
			"then cannot see an activation (R1-17-L04/L05)", holder)
	assert.Contains(t, identArgs(setupEchoFn, "registerRoutes"), holder,
		"registerRoutes does not receive the instance licence holder %q — DBMiddleware and the license "+
			"routes would hang off a different licence than the global chain", holder)

	// (3) registerRoutes adopts the activated key at startup, with the holder it
	// was handed.
	inst := paramNamed(registerRoutesFn, "license", "Instance")
	require.NotEmpty(t, inst, "registerRoutes has no *license.Instance parameter")
	assert.Contains(t, selectorCallArgs(registerRoutesFn, "license", "AdoptActivatedKey"), inst,
		"registerRoutes does not call license.AdoptActivatedKey(%s) — an instance licensed through the "+
			"UI then comes back as Community after every restart, because the activated key lives in "+
			"license_keys and not in VAKT_LICENSE_KEY (R1-07-B02, the \"permanently\" part)", inst)
}

// findFunc returns the top-level function declaration named name.
func findFunc(file *ast.File, name string) *ast.FuncDecl {
	for _, d := range file.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

// assignedFrom returns the names of the variables assigned from pkg.fn(...)
// inside fn's body.
func assignedFrom(fn *ast.FuncDecl, pkg, name string) []string {
	var out []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, rhs := range as.Rhs {
			if !isSelectorCall(rhs, pkg, name) || i >= len(as.Lhs) {
				continue
			}
			if id, ok := as.Lhs[i].(*ast.Ident); ok {
				out = append(out, id.Name)
			}
		}
		return true
	})
	return out
}

// identArgs returns the identifier arguments of every direct call to fnName.
func identArgs(fn *ast.FuncDecl, fnName string) []string {
	var out []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := call.Fun.(*ast.Ident); !ok || id.Name != fnName {
			return true
		}
		out = append(out, identNames(call.Args)...)
		return true
	})
	return out
}

// selectorCallArgs returns the identifier arguments of every call to pkg.name.
func selectorCallArgs(fn *ast.FuncDecl, pkg, name string) []string {
	var out []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isSelectorCall(call, pkg, name) {
			return true
		}
		out = append(out, identNames(call.Args)...)
		return true
	})
	return out
}

// paramNamed returns the name of fn's first parameter of type *pkg.typ.
func paramNamed(fn *ast.FuncDecl, pkg, typ string) string {
	if fn.Type.Params == nil {
		return ""
	}
	for _, field := range fn.Type.Params.List {
		star, ok := field.Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		sel, ok := star.X.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != typ {
			continue
		}
		if id, ok := sel.X.(*ast.Ident); !ok || id.Name != pkg {
			continue
		}
		if len(field.Names) > 0 {
			return field.Names[0].Name
		}
	}
	return ""
}

func isSelectorCall(e ast.Expr, pkg, name string) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg
}

func identNames(args []ast.Expr) []string {
	var out []string
	for _, a := range args {
		if id, ok := a.(*ast.Ident); ok {
			out = append(out, id.Name)
		}
	}
	return out
}
