// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package license

import (
	"context"
	"sync/atomic"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// Instance holds the licence this installation is running under, and it is
// mutable on purpose.
//
// Before this type existed, cmd/api loaded the licence once at startup and put
// that value on every request context (cmd/api/middleware.go). A key pasted
// into VAKT_LICENSE_KEY therefore reached every route, while the SAME key
// activated through POST /license/activate reached only the `protected` group —
// the one place DBMiddleware is mounted. Everything else kept seeing the frozen
// community licence from startup: the SSO login entry points
// (/api/v1/auth/oidc/*, /api/v1/auth/saml/*), the SCIM endpoints, and
// GET /api/v1/license itself, which is why a customer who had just activated
// successfully was told "community" for as long as the process lived
// (R1-17-L04, R1-17-L05, R1-07-B02).
//
// Scope of an instance licence: it answers the routes that have no organisation
// yet. A login-initiating SSO request carries no org_id, and SCIM authenticates
// with its own bearer token — for those, "which licence" can only mean the
// installation's.
//
// Behind auth, DBMiddleware overrides it with the key stored for the requesting
// org — but only when that org HAS a row in license_keys. An org without its own
// key keeps whatever the instance licence currently is (middleware.go:187-190).
// DBMiddleware is therefore not strictly finer than the instance licence: it is
// finer where a per-org key exists and a pass-through everywhere else. On an
// install where several orgs activated different keys, the instance licence is
// the most recently activated one, and an org without a key of its own inherits
// exactly that one — on per-org routes included.
//
// That pass-through is the intended behaviour here, not an oversight: Vakt is
// self-hosted, one customer per server, no multi-tenancy (CLAUDE.md:28 — "MSPs
// deploy per-customer, never centrally. […] no multi-tenancy"). The licence
// belongs to the installation the customer paid for, not to a row in
// license_keys, so an org on that installation inheriting it is what the
// customer bought. A second org on one instance is a demo artefact or the
// symptom of a separate defect — and then THAT defect is the thing to fix.
type Instance struct {
	v atomic.Pointer[License]
}

// NewInstance wraps the licence loaded at startup.
func NewInstance(l *License) *Instance {
	i := &Instance{}
	i.Set(l)
	return i
}

// Get returns the current instance licence, never nil.
func (i *Instance) Get() *License {
	if i == nil {
		return communityLicense()
	}
	if l := i.v.Load(); l != nil {
		return l
	}
	return communityLicense()
}

// Set replaces the instance licence. Called after a successful activation and
// after auto-renewal fetched a newer key.
func (i *Instance) Set(l *License) {
	if i == nil {
		return
	}
	if l == nil {
		l = communityLicense()
	}
	i.v.Store(l)
}

// Middleware puts the CURRENT instance licence on every request context.
// It replaces the closure in cmd/api/middleware.go that captured the startup
// licence by value: that capture is what made an activation invisible to every
// route outside `protected`.
func (i *Instance) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("license", i.Get())
			return next(c)
		}
	}
}

// AdoptActivatedKey loads the most recently activated key from license_keys and
// makes it the instance licence.
//
// Without it the fix would only survive until the next restart: the activated
// key lives in the database, VAKT_LICENSE_KEY is empty on an instance that was
// licensed through the UI, and startup would go back to community — which is
// exactly the "permanently community" symptom, one restart later.
//
// It is a no-op when the instance already carries an UNEXPIRED Pro/Enterprise
// licence or a demo licence: a configured, still-valid key wins over the
// database. An expired env key does not win — IsPro() is false once Expired is
// set (license.go:268-277), so the activated row replaces it.
//
// Two limits worth knowing before relying on this:
//   - The query has no WHERE org_id: it takes the most recently activated key on
//     the installation. On a Vakt install that is the customer's own key (one
//     customer per server, CLAUDE.md:28); autorefresh.go:219 resolves the org the
//     same way.
//   - The loaded key is not checked for expiry either. parse() only sets
//     Expired, so an expired row can become the instance licence, and Require()
//     still lets read-only requests through for an expired Pro licence
//     (middleware.go:34-37). The log line below reports the tier, not the expiry.
//
// A parse failure is logged and otherwise ignored — a broken row must not stop
// the server from starting.
func AdoptActivatedKey(ctx context.Context, db *pgxpool.Pool, inst *Instance) {
	if db == nil || inst == nil {
		return
	}
	if cur := inst.Get(); cur.IsPro() || cur.Demo {
		return
	}
	// The instance licence is by definition not one org's — this looks for the key
	// THIS INSTALLATION was licensed with. See the first bullet above for why that
	// is sound on a Vakt install, and autorefresh.go:219 for the same query shape.
	var keyValue string
	// orgid-lint: global — instance-wide on purpose (one customer per server)
	err := db.QueryRow(ctx,
		`SELECT key_value FROM license_keys ORDER BY activated_at DESC LIMIT 1`,
	).Scan(&keyValue)
	if err != nil || keyValue == "" {
		return
	}
	lic, parseErr := parse(keyValue)
	if parseErr != nil {
		log.Warn().Err(parseErr).Msg("license: activated key in DB is invalid — staying on the startup licence")
		return
	}
	inst.Set(lic)
	log.Info().
		Str("tier", lic.Tier).
		Str("org", lic.OrgName).
		Msg("license: adopted the activated key from the database as the instance licence")
}
