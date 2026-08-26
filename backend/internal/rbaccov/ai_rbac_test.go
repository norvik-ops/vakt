// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package rbaccov_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/services/ai"
)

// aiViewerWriteAllow lists the AI write routes a Viewer MAY reach, each with a
// reason. Everything else under /ai must answer 403 for a Viewer.
//
// R1-W5A-N2 (Nacharbeit): die Begründung hier lautete bis zur Nachprüfung, die
// sechs Routen „persist NOTHING", und die einzigen Schreibvorgänge in
// internal/services/ai seien `UPDATE ck_risks SET ai_narrative` und das
// Verwerfen eines Insights. Beides ist so nicht richtig. Vollständige Liste
// aller Schreibvorgänge im Paket (grep über INSERT/UPDATE/DELETE, ohne Tests):
//
//	usage.go:214       INSERT INTO ai_usage           ← von ALLEN sechs erreicht
//	agent.go:476/495   ai_pending_approvals           ← hinter aiWrite
//	handler.go:508     UPDATE ck_risks (Narrative)    ← hinter aiWrite
//	handler.go:605     UPDATE ck_ai_insights          ← hinter aiWrite
//	agent_tools.go:182 UPDATE ck_controls             ← hinter aiWrite
//
// Die sechs Routen persistieren also sehr wohl etwas: über UsageTracker.Record
// je Aufruf eine Zeile in `ai_usage` (Modell, Tokens, geschätzte Kosten). Was
// stimmt, ist die sicherheitsrelevante Hälfte der Aussage — keine dieser Routen
// schreibt Fachdaten, jeder Fachdaten-Schreibpfad des Pakets hängt hinter
// aiWrite. `ai_usage` ist Kosten-Buchführung, und die Kostenseite ist bewusst
// über RequireAILimit und die Lizenz begrenzt statt über die Rolle (stehende
// MA-04-Entscheidung). Ein Viewer kann damit Budget verbrauchen, aber keine
// fremden Daten verändern und nichts lesen, was er nicht ohnehin lesen darf.
//
// Entschieden wurde deshalb für die korrigierte Begründung, nicht für ein Gate:
// die Ausnahme trägt eine bestehende Produktentscheidung, und ein Gate hier
// würde Nur-Lese-Rollen (u. a. externe Prüfer) die KI-Erklärungen entziehen.
// Was NICHT bleiben durfte, ist der Satz „persist NOTHING" — er lädt dazu ein,
// beim nächsten Durchsehen nicht mehr nachzuschauen.
//
// Prüfregel für die Zukunft: schreibt eine dieser Routen etwas anderes als eine
// `ai_usage`-Zeile, gehört sie hinter aiWrite und ihr Eintrag hier muss weg.
var aiViewerWriteAllow = map[string]bool{
	"POST /vaktcomply/ai/report":               true,
	"POST /vaktcomply/ai/advice":               true,
	"POST /vaktcomply/ai/draft-policy":         true,
	"POST /vaktcomply/ai/incident-guide":       true,
	"POST /vaktcomply/ai/chat/stream":          true,
	"POST /vaktcomply/ai/controls/:id/explain": true,
}

// buildAIRouter mounts the AI package the way cmd/api/routes.go does — on a
// /vaktcomply group behind Paseto auth — with nil backing stores. Role gates run
// before the handler, so a gated route short-circuits without touching them.
//
// The licence middleware is deliberately NOT mounted: c.Get("license") is then
// nil and license.Require answers 402. That is what makes this test sharp — 402
// is not 403, so a route whose ONLY protection is the licence gate cannot
// masquerade as role-protected here.
func buildAIRouter(t *testing.T) *echo.Echo {
	t.Helper()
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)
	paseto := auth.PasetoMiddleware(key, nil)

	e := echo.New()
	e.Use(echomw.Recover())
	ai.RegisterWithOptions(e.Group("/vaktcomply", paseto), nil, "ollama", "", "", "test-model", ai.RegisterOptions{})
	return e
}

// TestAISurfaceDenyByDefault is the R1-SA13-01 class gate. The specific defect
// (POST /ai/agent/run, .../approve and .../reject reachable by a Viewer) is
// pinned end-to-end by cmd/api/ai_agent_rbac_test.go against the real router.
// This test guards the SHAPE instead: it enumerates every route the AI package
// actually registers via echo.Routes() and requires a Viewer 403 on each
// mutating method unless it is in aiViewerWriteAllow with a reason.
//
// That matters because the agent routes were not forgotten once — the package
// grew write routes over three sprints (S18 agent, S32-2 approve/reject, S124-8
// narrative/insights) and the role gate arrived late or never. A curated list of
// known routes would have stayed green through exactly that. This one is red by
// construction for the next ungated write route, without anyone remembering to
// extend it.
func TestAISurfaceDenyByDefault(t *testing.T) {
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)
	viewerTok := issueTok(t, key, "Viewer")

	e := buildAIRouter(t)

	checked := 0
	for _, route := range e.Routes() {
		if !writeMethods[route.Method] {
			continue
		}
		key := route.Method + " " + route.Path
		if aiViewerWriteAllow[key] {
			continue
		}
		checked++
		t.Run(key, func(t *testing.T) {
			reqPath := strings.NewReplacer(":id", "00000000-0000-0000-0000-000000000000",
				":run_id", "00000000-0000-0000-0000-000000000000").Replace(route.Path)
			req := httptest.NewRequest(route.Method, reqPath, nil)
			req.Header.Set("Authorization", "Bearer "+viewerTok)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusForbidden, rec.Code,
				"Viewer must be 403 on AI write route %s (add to aiViewerWriteAllow with a "+
					"reason if it genuinely persists nothing). Body: %s", key, rec.Body.String())
		})
	}

	// Name the denominator. A sweep that silently enumerated nothing — provider
	// "disabled", a renamed group, a Register() that returned early — would
	// otherwise report success for work it never did.
	require.GreaterOrEqual(t, checked, 5,
		"expected at least the 5 gated AI write routes (3 agent + narrative + insight "+
			"dismiss), got %d — the AI package did not register as expected", checked)
	t.Logf("AI write routes checked: %d (allowlisted: %d)", checked, len(aiViewerWriteAllow))
}
