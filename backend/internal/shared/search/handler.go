package search

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/shared/safego"
)

// SearchResult is a single cross-module search hit.
type SearchResult struct {
	ID         string `json:"id"`
	EntityType string `json:"entity_type"` // control | risk | policy | incident | capa | asset | finding | dsr | breach
	Title      string `json:"title"`
	Subtitle   string `json:"subtitle"` // framework name / status / severity / short description
	URL        string `json:"url"`      // frontend navigation path
}

// SearchResponse is the top-level search API response.
type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Total   int            `json:"total"`
}

// Handler holds the DB pool for search queries.
type Handler struct{ db *pgxpool.Pool }

// NewHandler creates a new search Handler.
func NewHandler(db *pgxpool.Pool) *Handler { return &Handler{db: db} }

// Search handles GET /api/v1/search?q=<term>&limit=<n>
func (h *Handler) Search(c echo.Context) error {
	orgID, ok := c.Get("org_id").(string)
	if !ok || orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	q := strings.TrimSpace(c.QueryParam("q"))
	if len(q) < 2 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "query too short"})
	}
	if len(q) > 100 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "query too long"})
	}

	// Escape SQL LIKE wildcards to prevent expensive full-table scans and
	// unintended pattern matching. Escape \ first so it is not double-escaped.
	safe := strings.ReplaceAll(q, `\`, `\\`)
	safe = strings.ReplaceAll(safe, `%`, `\%`)
	safe = strings.ReplaceAll(safe, `_`, `\_`)
	pattern := "%" + strings.ToLower(safe) + "%"

	ctx := c.Request().Context()
	const perSource = 5

	type fetcher func(context.Context, *pgxpool.Pool, string, string, int) []SearchResult

	sources := []fetcher{
		searchControls,
		searchRisks,
		searchPolicies,
		searchIncidents,
		searchCAPAs,
		searchAssets,
		searchFindings,
		searchDSRs,
		searchBreaches,
	}

	var (
		mu  sync.Mutex
		all []SearchResult
		wg  sync.WaitGroup
	)

	// ADR-0018: Cross-Module-Search-Fanout läuft über safego.Run mit dem
	// Request-Context. Eine Panic in einem Source-Fetcher (z.B. fehlerhaftes
	// SQL-Mapping) lässt den Search-Endpoint nicht crashen — der Panic-Hook
	// loggt + tracet, die anderen Sources fertig.
	for _, fn := range sources {
		wg.Add(1)
		f := fn
		safego.Run(ctx, "search.fanout.fetch", func(c context.Context) error {
			defer wg.Done()
			res := f(c, h.db, orgID, pattern, perSource)
			if len(res) == 0 {
				return nil
			}
			mu.Lock()
			all = append(all, res...)
			mu.Unlock()
			return nil
		})
	}
	wg.Wait()

	// Stable ordering: entity_type first so grouped display is easy on the frontend.
	// Within each type results are already ordered by the DB (LIMIT 5).
	order := map[string]int{
		"control":  0,
		"risk":     1,
		"policy":   2,
		"incident": 3,
		"capa":     4,
		"asset":    5,
		"finding":  6,
		"dsr":      7,
		"breach":   8,
	}
	sortResults(all, order)

	limit := 20
	if len(all) > limit {
		all = all[:limit]
	}

	return c.JSON(http.StatusOK, SearchResponse{Results: all, Total: len(all)})
}

// sortResults does a stable in-place sort by entity_type priority.
func sortResults(results []SearchResult, order map[string]int) {
	// Insertion sort — list is small (≤ 9*5 = 45 items).
	for i := 1; i < len(results); i++ {
		key := results[i]
		j := i - 1
		for j >= 0 && order[results[j].EntityType] > order[key.EntityType] {
			results[j+1] = results[j]
			j--
		}
		results[j+1] = key
	}
}

// --- per-entity search helpers ---

func searchControls(ctx context.Context, db *pgxpool.Pool, orgID, pattern string, limit int) []SearchResult {
	rows, err := db.Query(ctx, `
		SELECT c.id::text, c.title,
		       COALESCE((SELECT name FROM ck_frameworks WHERE id = c.framework_id LIMIT 1), '') AS subtitle,
		       c.framework_id::text
		FROM ck_controls c
		WHERE c.org_id = $1::uuid
		  AND (lower(c.title) LIKE $2 ESCAPE '\' OR lower(c.control_id) LIKE $2 ESCAPE '\' OR lower(COALESCE(c.description,'')) LIKE $2 ESCAPE '\')
		LIMIT $3`,
		orgID, pattern, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var frameworkID string
		if err := rows.Scan(&r.ID, &r.Title, &r.Subtitle, &frameworkID); err != nil {
			continue
		}
		r.EntityType = "control"
		r.URL = "/vaktcomply/frameworks/" + frameworkID
		results = append(results, r)
	}
	return results
}

func searchRisks(ctx context.Context, db *pgxpool.Pool, orgID, pattern string, limit int) []SearchResult {
	rows, err := db.Query(ctx,
		`SELECT id::text, title, COALESCE(status,'') FROM ck_risks WHERE org_id=$1::uuid AND lower(title) LIKE $2 ESCAPE '\' LIMIT $3`,
		orgID, pattern, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.Title, &r.Subtitle); err != nil {
			continue
		}
		r.EntityType = "risk"
		r.URL = "/vaktcomply/risks"
		results = append(results, r)
	}
	return results
}

func searchPolicies(ctx context.Context, db *pgxpool.Pool, orgID, pattern string, limit int) []SearchResult {
	rows, err := db.Query(ctx,
		`SELECT id::text, title, COALESCE(status,'') FROM ck_policies WHERE org_id=$1::uuid AND lower(title) LIKE $2 ESCAPE '\' LIMIT $3`,
		orgID, pattern, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.Title, &r.Subtitle); err != nil {
			continue
		}
		r.EntityType = "policy"
		r.URL = "/vaktcomply/policies"
		results = append(results, r)
	}
	return results
}

func searchIncidents(ctx context.Context, db *pgxpool.Pool, orgID, pattern string, limit int) []SearchResult {
	rows, err := db.Query(ctx,
		`SELECT id::text, title, COALESCE(severity,'') FROM ck_incidents WHERE org_id=$1::uuid AND lower(title) LIKE $2 ESCAPE '\' LIMIT $3`,
		orgID, pattern, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.Title, &r.Subtitle); err != nil {
			continue
		}
		r.EntityType = "incident"
		r.URL = "/vaktcomply/incidents"
		results = append(results, r)
	}
	return results
}

func searchCAPAs(ctx context.Context, db *pgxpool.Pool, orgID, pattern string, limit int) []SearchResult {
	rows, err := db.Query(ctx,
		`SELECT id::text, title, COALESCE(status,'') FROM ck_capas WHERE org_id=$1::uuid AND lower(title) LIKE $2 ESCAPE '\' LIMIT $3`,
		orgID, pattern, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.Title, &r.Subtitle); err != nil {
			continue
		}
		r.EntityType = "capa"
		r.URL = "/vaktcomply/capas"
		results = append(results, r)
	}
	return results
}

func searchAssets(ctx context.Context, db *pgxpool.Pool, orgID, pattern string, limit int) []SearchResult {
	rows, err := db.Query(ctx,
		`SELECT id::text, name FROM vb_assets WHERE org_id=$1::uuid AND lower(name) LIKE $2 ESCAPE '\' LIMIT $3`,
		orgID, pattern, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.Title); err != nil {
			continue
		}
		r.EntityType = "asset"
		r.Subtitle = "Asset"
		r.URL = "/vaktscan/assets/" + r.ID
		results = append(results, r)
	}
	return results
}

func searchFindings(ctx context.Context, db *pgxpool.Pool, orgID, pattern string, limit int) []SearchResult {
	rows, err := db.Query(ctx,
		`SELECT id::text, title FROM vb_findings WHERE org_id=$1::uuid AND lower(title) LIKE $2 ESCAPE '\' LIMIT $3`,
		orgID, pattern, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.Title); err != nil {
			continue
		}
		r.EntityType = "finding"
		r.Subtitle = "Finding"
		r.URL = "/vaktscan/findings/" + r.ID
		results = append(results, r)
	}
	return results
}

func searchDSRs(ctx context.Context, db *pgxpool.Pool, orgID, pattern string, limit int) []SearchResult {
	rows, err := db.Query(ctx,
		`SELECT id::text, requester_name FROM po_dsr WHERE org_id=$1::uuid AND lower(requester_name) LIKE $2 ESCAPE '\' LIMIT $3`,
		orgID, pattern, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.Title); err != nil {
			continue
		}
		r.EntityType = "dsr"
		r.Subtitle = "DSR"
		r.URL = "/vaktprivacy/dsr"
		results = append(results, r)
	}
	return results
}

func searchBreaches(ctx context.Context, db *pgxpool.Pool, orgID, pattern string, limit int) []SearchResult {
	rows, err := db.Query(ctx,
		`SELECT id::text, title FROM po_breaches WHERE org_id=$1::uuid AND lower(title) LIKE $2 ESCAPE '\' LIMIT $3`,
		orgID, pattern, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.Title); err != nil {
			continue
		}
		r.EntityType = "breach"
		r.Subtitle = "Datenpanne"
		r.URL = "/vaktprivacy/breach"
		results = append(results, r)
	}
	return results
}
