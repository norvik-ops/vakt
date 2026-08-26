package vaktcomply

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktcomply/reporting"
)

// approvalRouteHarness mountiert die ECHTE Routenregistrierung auf einem echten
// Echo und legt nur die Identitaet unter, die sonst die AuthMiddleware setzt —
// mit demselben Schluessel und demselben Typ (`roles`, []string, middleware.go).
// Genau diese Naht war der Defekt: Der Service las `role` (Singular, string),
// den niemand setzt. Ein Test, der die Rollenpruefung direkt auf dem Service
// aufruft, haette das nie gesehen, weil er den Wert selbst mitgebracht haette.
//
// Der Service ist absichtlich nil: Fuer die Ablehnung wird er nie erreicht.
// middleware.Recover() faengt den Panic ab, falls doch — dann ist der Status
// 500 und eben NICHT 403, was fuer die Durchlass-Faelle die Aussage ist.
func approvalRouteHarness(t *testing.T, roles []string) *echo.Echo {
	t.Helper()
	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("org_id", "11111111-1111-1111-1111-111111111111")
			c.Set("user_id", "22222222-2222-2222-2222-222222222222")
			c.Set("roles", roles)
			return next(c)
		}
	})
	registerRoutes(e.Group(""), &Handler{})
	return e
}

// TestISMSScopeApprovalIsAdminOnly haelt die eine verbliebene Antwort auf die
// Berechtigungsfrage fest: die Route. Vor R1-W7C-N1 gab es zwei — die Route
// liess Admin und SecurityAnalyst durch, der Service verlangte "admin" aus
// einem ungesetzten Kontextschluessel und lehnte damit jeden ab.
func TestISMSScopeApprovalIsAdminOnly(t *testing.T) {
	denied := map[string][]string{
		"SecurityAnalyst": {"SecurityAnalyst"},
		"InternalAuditor": {"InternalAuditor"},
		"Viewer":          {"Viewer"},
		"ohne Rolle":      nil,
	}
	for name, roles := range denied {
		t.Run("abgelehnt/"+name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/isms-scope/approve",
				strings.NewReader(`{"id":"33333333-3333-3333-3333-333333333333"}`))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			approvalRouteHarness(t, roles).ServeHTTP(rec, req)
			assert.Equal(t, http.StatusForbidden, rec.Code)
			assert.Contains(t, rec.Body.String(), "AUTH_INSUFFICIENT_ROLE")
		})
	}

	// Der Admin kommt durch die Rollenpruefung. Belegt wird das ohne Datenbank
	// ueber die Pflichtfeldpruefung des Handlers: Ein leerer Body erzeugt 400
	// "id is required" — eine Antwort, die es nur GIBT, wenn der Request den
	// Handler ueberhaupt erreicht hat. Vor dem Fix war hier 403.
	t.Run("durchgelassen/Admin", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/isms-scope/approve", strings.NewReader(`{}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		approvalRouteHarness(t, []string{"Admin"}).ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, "Admin muss den Handler erreichen, nicht 403 bekommen")
		assert.Contains(t, rec.Body.String(), "id is required")
	})
}

// TestManagementReviewApprovalIsInternalAuditorOnly haelt die Rollenmatrix aus
// ADR-0055 fest: Admin ❌, SecurityAnalyst ❌, InternalAuditor ✅. Der geloeschte
// Service-Check verlangte "admin" und widersprach damit dem ADR — er war nur
// deshalb nie aufgefallen, weil er ohnehin jeden ablehnte.
func TestManagementReviewApprovalIsInternalAuditorOnly(t *testing.T) {
	const path = "/management-reviews/33333333-3333-3333-3333-333333333333/approve"

	denied := map[string][]string{
		"Admin":           {"Admin"},
		"SecurityAnalyst": {"SecurityAnalyst"},
		"Viewer":          {"Viewer"},
		"ohne Rolle":      nil,
	}
	for name, roles := range denied {
		t.Run("abgelehnt/"+name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, path, nil)
			approvalRouteHarness(t, roles).ServeHTTP(rec, req)
			assert.Equal(t, http.StatusForbidden, rec.Code)
			assert.Contains(t, rec.Body.String(), "AUTH_INSUFFICIENT_ROLE")
		})
	}

	// InternalAuditor kommt durch die Rollenpruefung und laeuft ohne Service in
	// den abgefangenen Panic (500). Die Aussage ist "nicht 403" — den Beweis,
	// dass danach wirklich freigegeben wird, fuehrt der Integrationstest gegen
	// echtes Postgres.
	t.Run("durchgelassen/InternalAuditor", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		approvalRouteHarness(t, []string{"InternalAuditor"}).ServeHTTP(rec, req)
		assert.NotEqual(t, http.StatusForbidden, rec.Code, "InternalAuditor darf nicht an der Rollenpruefung scheitern")
	})
}

func TestMapAssetType(t *testing.T) {
	cases := map[string]string{
		"asset":             "it_system",
		"network_asset":     "network",
		"application_asset": "application",
		"raum":              "room",
		"geschaeftsprozess": "process",
		"server_device":     "it_system",
	}
	for in, want := range cases {
		if got := mapAssetType(in); got != want {
			t.Errorf("mapAssetType(%q)=%q want %q", in, got, want)
		}
	}
}

func TestTruncateStr(t *testing.T) {
	if got := truncateStr("hello", 10); got != "hello" {
		t.Errorf("no-trunc got %q", got)
	}
	if got := truncateStr("hello world", 5); got != "hello" {
		t.Errorf("trunc got %q", got)
	}
}

func TestNIS2ReportabilityCheck_IsReportable(t *testing.T) {
	tests := []struct {
		name string
		c    NIS2ReportabilityCheck
		want bool
	}{
		{
			name: "all false → not reportable",
			c:    NIS2ReportabilityCheck{CausesSignificantDisruption: false, AffectsThirdParties: false, CausesFinancialDamage: false},
			want: false,
		},
		{
			name: "causes significant disruption → reportable",
			c:    NIS2ReportabilityCheck{CausesSignificantDisruption: true},
			want: true,
		},
		{
			name: "affects third parties → reportable",
			c:    NIS2ReportabilityCheck{AffectsThirdParties: true},
			want: true,
		},
		{
			name: "causes financial damage → reportable",
			c:    NIS2ReportabilityCheck{CausesFinancialDamage: true},
			want: true,
		},
		{
			name: "all true → reportable",
			c:    NIS2ReportabilityCheck{CausesSignificantDisruption: true, AffectsThirdParties: true, CausesFinancialDamage: true},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.c.IsReportable())
		})
	}
}

// TestNIS2Deadlines_CalendarMonth pins the three Art. 23(4) deadlines against the
// real function. The previous version of this test recomputed the arithmetic
// inline and asserted it against itself — a tautology that stayed green no
// matter what the service did, and that also cemented the wrong 30-day month.
func TestNIS2Deadlines_CalendarMonth(t *testing.T) {
	cases := []struct {
		name      string
		from      time.Time
		wantFinal time.Time
	}{
		{
			// The case where 30 days and one calendar month diverge, and where
			// time.AddDate would normalise 31 February into 3 March instead of
			// clamping to 28 February.
			name:      "Monatsende klemmt auf den letzten Tag des Zielmonats",
			from:      time.Date(2026, 1, 31, 9, 0, 0, 0, time.UTC),
			wantFinal: time.Date(2026, 2, 28, 9, 0, 0, 0, time.UTC),
		},
		{
			name:      "Schaltjahr klemmt auf den 29. Februar",
			from:      time.Date(2028, 1, 31, 9, 0, 0, 0, time.UTC),
			wantFinal: time.Date(2028, 2, 29, 9, 0, 0, 0, time.UTC),
		},
		{
			name:      "Jahreswechsel",
			from:      time.Date(2025, 12, 31, 23, 30, 0, 0, time.UTC),
			wantFinal: time.Date(2026, 1, 31, 23, 30, 0, 0, time.UTC),
		},
		{
			name:      "Monatsmitte bleibt auf demselben Tag",
			from:      time.Date(2026, 6, 10, 10, 0, 0, 0, time.UTC),
			wantFinal: time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ew, full, final := reporting.NIS2Deadlines24_72_1M(tc.from)
			assert.Equal(t, tc.from.Add(24*time.Hour), ew, "Fruehwarnung = T+24h")
			assert.Equal(t, tc.from.Add(72*time.Hour), full, "Meldung = T+72h")
			assert.Equal(t, tc.wantFinal, final,
				"ein Monat ist ein Kalenderzeitraum, keine 30 Tage — und time.AddDate "+
					"normalisiert einen Monatsueberlauf, statt ihn zu klemmen")
		})
	}
}

func TestNIS2DeadlineCheck_StageFiltering(t *testing.T) {
	now := time.Now().UTC()
	warn := now.Add(2 * time.Hour)

	// Deadline within warn window and not yet submitted → should notify
	deadline := now.Add(1 * time.Hour)
	assert.True(t, deadline.Before(warn), "deadline in < 2h should trigger notification")

	// Deadline already past → also within warn window
	pastDeadline := now.Add(-1 * time.Hour)
	assert.True(t, pastDeadline.Before(warn), "overdue deadline should also trigger notification")

	// Deadline far in future → should not notify
	futureDeadline := now.Add(3 * time.Hour)
	assert.False(t, futureDeadline.Before(warn), "deadline in > 2h should not trigger notification")
}

func TestThreatLibrary_LoadsAtLeast60(t *testing.T) {
	root := loadThreatLibrary()
	if len(root.Threats) < 60 {
		t.Fatalf("threat library has %d entries, want >=60", len(root.Threats))
	}
	if root.Version == "" {
		t.Error("threat library version must be set (for link provenance)")
	}
	seen := map[string]bool{}
	for _, it := range root.Threats {
		if it.ID == "" || it.Title == "" {
			t.Errorf("threat %q has empty id/title", it.ID)
		}
		if seen[it.ID] {
			t.Errorf("duplicate threat id %q", it.ID)
		}
		seen[it.ID] = true
		if it.DefaultLikelihood < 1 || it.DefaultLikelihood > 5 {
			t.Errorf("threat %s default_likelihood out of range: %d", it.ID, it.DefaultLikelihood)
		}
		if it.DefaultImpact < 1 || it.DefaultImpact > 5 {
			t.Errorf("threat %s default_impact out of range: %d", it.ID, it.DefaultImpact)
		}
		if it.SuggestedMeasure == "" {
			t.Errorf("threat %s has no suggested measure", it.ID)
		}
	}
}

func TestThreatCatalogFilter(t *testing.T) {
	svc := &Service{}

	all := svc.ListThreatCatalog(ThreatCatalogFilter{})
	if len(all) < 60 {
		t.Fatalf("unfiltered returned %d, want >=60", len(all))
	}

	iso := svc.ListThreatCatalog(ThreatCatalogFilter{Framework: "ISO27001"})
	if len(iso) == 0 || len(iso) >= len(all) {
		t.Errorf("ISO27001 filter returned %d (want >0 and <%d)", len(iso), len(all))
	}
	for _, it := range iso {
		if !sliceContainsFold(it.Frameworks, "ISO27001") {
			t.Errorf("threat %s leaked into ISO27001 filter", it.ID)
		}
	}

	data := svc.ListThreatCatalog(ThreatCatalogFilter{AssetType: "data"})
	for _, it := range data {
		if !sliceContainsFold(it.AssetTypes, "data") {
			t.Errorf("threat %s leaked into data asset filter", it.ID)
		}
	}

	conf := svc.ListThreatCatalog(ThreatCatalogFilter{CIA: "confidentiality"})
	for _, it := range conf {
		if !sliceContainsFold(it.CIA, "confidentiality") {
			t.Errorf("threat %s leaked into confidentiality filter", it.ID)
		}
	}

	// Combined filter must be a subset of each single filter.
	combined := svc.ListThreatCatalog(ThreatCatalogFilter{Framework: "NIS2", CIA: "availability"})
	for _, it := range combined {
		if !sliceContainsFold(it.Frameworks, "NIS2") || !sliceContainsFold(it.CIA, "availability") {
			t.Errorf("combined filter leaked %s", it.ID)
		}
	}
}

func TestFindThreatCatalogItem(t *testing.T) {
	if _, ok := findThreatCatalogItem("T-RANSOMWARE"); !ok {
		t.Error("T-RANSOMWARE must exist in the catalog")
	}
	if _, ok := findThreatCatalogItem("T-NOPE"); ok {
		t.Error("unknown id must not resolve")
	}
}

func TestParseSupplierCSVRows_AllValidCriticalities(t *testing.T) {
	for _, crit := range []string{"low", "medium", "high", "critical", "standard", "important"} {
		csv := "name,criticality\nAcme," + crit + "\n"
		rows, err := parseSupplierCSVRows(csv)
		require.NoError(t, err, "criticality=%s", crit)
		require.Len(t, rows, 1, "criticality=%s", crit)
		assert.Equal(t, crit, rows[0].Criticality)
	}
}

func TestParseSupplierCSVRows_BoolTrueVariants(t *testing.T) {
	for _, val := range []string{"True", "TRUE", "1"} {
		csv := "name,nis2_relevant\nAcme," + val + "\n"
		rows, err := parseSupplierCSVRows(csv)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.True(t, rows[0].NIS2Relevant, "input=%q should parse as true", val)
	}
}

// ── computeStatus — branches not covered in service_test.go ─────────────────

func TestComputeStatus_InProgressAssessment(t *testing.T) {
	supplier := Supplier{ID: "s1"}
	assessments := []Assessment{{ID: "a1", Status: "in_progress"}}
	st := computeStatus(supplier, assessments, nil, time.Now())
	assert.Equal(t, "yellow", st.Status)
	assert.Equal(t, "assessment_pending", st.Details["reason"])
}

func TestComputeStatus_SubmittedAwaitingReview(t *testing.T) {
	supplier := Supplier{ID: "s1"}
	assessments := []Assessment{{ID: "a1", Status: "submitted"}}
	st := computeStatus(supplier, assessments, nil, time.Now())
	assert.Equal(t, "yellow", st.Status)
	assert.Equal(t, "awaiting_review", st.Details["reason"])
}

func TestComputeStatus_ReviewedNoAnswers_Fallback(t *testing.T) {
	supplier := Supplier{ID: "s1"}
	assessments := []Assessment{{ID: "a1", Status: "reviewed"}}
	st := computeStatus(supplier, assessments, nil, time.Now())
	// reviewed but empty answers → falls through to fallback yellow
	assert.Equal(t, "yellow", st.Status)
}
