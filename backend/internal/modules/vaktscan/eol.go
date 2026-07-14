// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/httputil"
	"github.com/matharnica/vakt/internal/shared/safego"
)

// productMap maps common component names to their endoflife.date product slug.
var productMap = map[string]string{
	"nodejs":     "nodejs",
	"node":       "nodejs",
	"python":     "python",
	"python3":    "python",
	"golang":     "go",
	"go":         "go",
	"java":       "java",
	"openjdk":    "java",
	"postgresql": "postgresql",
	"postgres":   "postgresql",
	"redis":      "redis",
	"nginx":      "nginx",
	"debian":     "debian",
	"ubuntu":     "ubuntu",
	"alpine":     "alpine",
	"php":        "php",
	"ruby":       "ruby",
}

// eolValue ist eine polymorphe Repräsentation des `eol`-Feldes aus der
// endoflife.date-API: das Feld liefert entweder `true`/`false` (Boolean
// markiert nur dass der Cycle EOL ist) oder einen "YYYY-MM-DD"-String mit
// dem konkreten EOL-Datum. S14-7: typisiert statt rohes any.
type eolValue struct {
	IsEOL bool   // true = Cycle wurde als EOL markiert (Boolean oder dateInVergangenheit)
	Date  string // optional: "YYYY-MM-DD" wenn die API ein konkretes Datum liefert
}

// UnmarshalJSON implementiert die polymorphe Decode-Logik. Akzeptiert:
//   - true / false (Boolean)
//   - "true" / "false" (Strings — kommen vor)
//   - "" (Leerstring → kein EOL)
//   - "YYYY-MM-DD" (konkretes EOL-Datum)
func (e *eolValue) UnmarshalJSON(raw []byte) error {
	// Boolean case
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		e.IsEOL = b
		return nil
	}
	// String case
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("eolValue: expected bool or string, got %s", string(raw))
	}
	switch s {
	case "", "false":
		// supported
	case "true":
		e.IsEOL = true
	default:
		e.IsEOL = true
		e.Date = s
	}
	return nil
}

// MarshalJSON schreibt den Wert in der Form der API zurück, nicht in unserer.
//
// Ohne diese Methode serialisiert Go die Struktur — aus `"2022-05-24"` wurde
// `{"IsEOL":true,"Date":"2022-05-24"}`, und das eigene UnmarshalJSON konnte es
// nicht mehr lesen. Genau das ist passiert (fetchEOL, 2026-07-14): Ein Round-Trip
// durch JSON zerstörte den Wert lautlos, und weil der Fehler beim Aufrufer nur ein
// log.Warn war, hat das EOL-Tracking jahrelang nichts gemeldet.
//
// Ein Typ mit eigenem UnmarshalJSON braucht ein passendes MarshalJSON — sonst ist
// er nur in eine Richtung ein Typ, und die andere Richtung ist eine Falle, die
// keinen Compiler-Fehler erzeugt.
func (e eolValue) MarshalJSON() ([]byte, error) {
	if e.Date != "" {
		return json.Marshal(e.Date)
	}
	return json.Marshal(e.IsEOL)
}

// eolCycle is one entry returned by the endoflife.date API.
type eolCycle struct {
	Cycle             string   `json:"cycle"`
	EOL               eolValue `json:"eol"`
	LatestReleaseDate string   `json:"latestReleaseDate,omitempty"`
}

// EOLChecker checks components against the endoflife.date API and stores results.
//
// httpClient ist zugleich die Test-Naht: Ein Test im Paket hängt hier einen Client
// ein, der auf einen httptest.Server zeigt, und fährt damit den echten Weg
// (Anfrage, Antwort, Parsen, Cache) ohne endoflife.date zu behelligen.
type EOLChecker struct {
	db         *pgxpool.Pool
	httpClient *http.Client

	// baseURL ist die Test-Naht. Produktion lässt sie leer und bekommt
	// endoflife.date; ein Test zeigt sie auf einen httptest.Server und fährt damit
	// den echten Weg — Anfrage, Statuscodes, Parsen, Cache — ohne einen fremden
	// Dienst zu befragen (und ohne von ihm abzuhängen, wenn er mal weg ist).
	baseURL string
}

// eolAPIBaseURL ist die öffentliche EOL-Datenbank. Konstante, nicht konfigurierbar:
// Es gibt keinen Grund, warum eine Kundeninstanz woanders hinfragen sollte, und
// jede Konfigurierbarkeit wäre eine weitere Fläche für Datenabfluss.
const eolAPIBaseURL = "https://endoflife.date/api"

// api liefert die Basis-URL für Anfragen — die Naht, wenn gesetzt, sonst die echte.
func (c *EOLChecker) api() string {
	if c.baseURL != "" {
		return c.baseURL
	}
	return eolAPIBaseURL
}

// NewEOLChecker creates a new EOLChecker backed by the given database pool.
//
// GuardedClient: endoflife.date ist ein fester, öffentlicher Host. Löst er auf eine
// private Adresse auf, ist das kein Sonderfall, sondern ein DNS-Rebind — deshalb
// allowPrivate=false. Der Guard löst den Namen auf und wählt die aufgelöste IP, so
// dass die geprüfte Adresse auch die verbundene ist.
func NewEOLChecker(db *pgxpool.Pool) *EOLChecker {
	return &EOLChecker{
		db:         db,
		httpClient: httputil.GuardedClient(10*time.Second, false),
	}
}

// eolResult bundles an EOL resolution for a single component.
type eolResult struct {
	componentID string
	eolStatus   string
	eolDate     *string
}

// CheckComponents looks up EOL status for every component in the given SBOM,
// using a 24-hour cache stored in vb_eol_cache.
//
// Performance improvements over the naive sequential approach:
//  1. All cache rows for this SBOM are loaded in a single batch query.
//  2. HTTP requests for cache-miss entries are performed in parallel (max 5).
//  3. All EOL updates are written in a single batch INSERT … ON CONFLICT.
func (c *EOLChecker) CheckComponents(ctx context.Context, orgID, sbomID string) error {
	repo := NewRepository(c.db)

	components, err := repo.listComponentsBySBOM(ctx, sbomID)
	if err != nil {
		return fmt.Errorf("list components: %w", err)
	}

	// ── 1. Build the list of (product, cycle) pairs we care about ─────────────
	type compEntry struct {
		comp    componentRow
		product string
		cycle   string
	}

	var entries []compEntry
	for _, comp := range components {
		slug, ok := productMap[strings.ToLower(comp.Name)]
		if !ok {
			continue
		}
		entries = append(entries, compEntry{comp: comp, product: slug, cycle: majorCycle(comp.Version)})
	}

	if len(entries) == 0 {
		return nil
	}

	// ── 2. Batch-load cache rows for all (product, cycle) pairs ───────────────
	cacheMap, err := repo.batchGetEOLCache(ctx, func() [][2]string {
		pairs := make([][2]string, len(entries))
		for i, e := range entries {
			pairs[i] = [2]string{e.product, e.cycle}
		}
		return pairs
	}())
	if err != nil {
		log.Warn().Err(err).Msg("EOL batch cache load failed — will fetch individually")
		cacheMap = make(map[[2]string]eolCacheRow)
	}

	// ── 3. Identify cache misses; fetch them in parallel (max 5) ──────────────
	var (
		missEntries []compEntry
		results     []eolResult
		mu          sync.Mutex
	)

	for _, e := range entries {
		key := [2]string{e.product, e.cycle}
		if row, ok := cacheMap[key]; ok && time.Since(row.fetchedAt) < 24*time.Hour && row.payload != nil {
			status, eolDate, err := parseEOLPayload(row.payload)
			if err == nil {
				results = append(results, eolResult{componentID: e.comp.ID, eolStatus: status, eolDate: eolDate})
				continue
			}
		}
		missEntries = append(missEntries, e)
	}

	// Parallel HTTP fetches with a semaphore of 5.
	const maxParallel = 5
	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup

	// ADR-0018: EOL-Fetcher-Fanout über safego.Run mit Parent-Context.
	for _, e := range missEntries {
		e := e
		wg.Add(1)
		sem <- struct{}{}
		safego.Run(ctx, "vaktscan.eol.fetch", func(c2 context.Context) error {
			defer wg.Done()
			defer func() { <-sem }()

			status, eolDate, fetchedPayload, err := c.fetchEOL(c2, e.product, e.cycle)
			if err != nil {
				log.Warn().Err(err).Str("component", e.comp.Name).Msg("EOL fetch failed")
				return nil
			}
			// Persist to cache (best-effort).
			if upsertErr := repo.upsertEOLCache(c2, e.product, e.cycle, fetchedPayload); upsertErr != nil {
				log.Warn().Err(upsertErr).Str("product", e.product).Msg("EOL cache upsert failed")
			}
			mu.Lock()
			results = append(results, eolResult{componentID: e.comp.ID, eolStatus: status, eolDate: eolDate})
			mu.Unlock()
			return nil
		})
	}
	wg.Wait()

	// ── 4. Batch-write all EOL results ────────────────────────────────────────
	if len(results) > 0 {
		if err := repo.batchUpdateComponentEOL(ctx, results); err != nil {
			log.Error().Err(err).Msg("batch update component EOL failed")
		}
	}

	return nil
}

// fetchEOL fetches EOL data from endoflife.date for the given product/cycle and
// returns (status, eolDate, rawPayload, error). rawPayload is nil on 404.
func (c *EOLChecker) fetchEOL(ctx context.Context, product, cycle string) (string, *string, []byte, error) {
	url := fmt.Sprintf("%s/%s.json", c.api(), product)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "unknown", nil, nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "unknown", nil, nil, fmt.Errorf("http get %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "unknown", nil, nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return "unknown", nil, nil, fmt.Errorf("endoflife.date returned %d for %s", resp.StatusCode, product)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "unknown", nil, nil, fmt.Errorf("read body: %w", err)
	}

	// Die Einträge bleiben als ROHE Bytes erhalten, statt geparst und wieder
	// zurückserialisiert zu werden.
	//
	// ── Warum das kein Stilfrage ist (2026-07-14) ─────────────────────────────
	//
	// Vorher stand hier `payload, _ := json.Marshal(cycles[i])`, also die
	// Rück-Serialisierung des geparsten Zyklus. `eolValue` hat aber nur ein
	// UnmarshalJSON und KEIN MarshalJSON: Aus dem API-Wert `"2022-05-24"` wurde
	// beim Zurückschreiben `{"IsEOL":true,"Date":"2022-05-24"}` — unsere interne
	// Struktur statt der API-Form. Die Zeile direkt darunter las das wieder ein
	// und scheiterte („expected bool or string").
	//
	// Folge: fetchEOL gab für JEDEN Zyklus, den endoflife.date tatsächlich kennt,
	// einen Fehler zurück. Der Aufrufer behandelt einen Fetch-Fehler mit
	// `log.Warn` + `return nil` — die Komponente fiel also **still heraus**, nicht
	// einmal als „unbekannt". Das EOL-Tracking hat damit nie eine einzige
	// veraltete Komponente gemeldet; sichtbar war es nur als WARN im Log.
	//
	// Die Bytes der API sind ohnehin das, was `rawPayload` verspricht und was der
	// Cache speichern soll. Alte, kaputt gecachte Einträge heilen sich selbst: Sie
	// scheitern beim Lesen, gelten als Cache-Miss und werden neu geholt.
	var entries []json.RawMessage
	if err := json.Unmarshal(body, &entries); err != nil {
		return "unknown", nil, nil, fmt.Errorf("parse endoflife.date response: %w", err)
	}

	for _, entry := range entries {
		var parsed eolCycle
		if err := json.Unmarshal(entry, &parsed); err != nil {
			// Ein unlesbarer Eintrag kostet diesen Eintrag, nicht die Abfrage.
			continue
		}
		if normaliseCycle(parsed.Cycle) == normaliseCycle(cycle) {
			status, eolDate, err := parseEOLPayload(entry)
			return status, eolDate, entry, err
		}
	}

	// Cycle not listed.
	return "unknown", nil, nil, nil
}

// majorCycle extracts the major.minor cycle string from a semver version, e.g. "3.9.7" → "3.9".
func majorCycle(version string) string {
	parts := strings.SplitN(strings.TrimPrefix(version, "v"), ".", 3)
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return parts[0]
}

// normaliseCycle lowercases and trims a leading "v" for endoflife.date comparison.
// Lowercase must happen before TrimPrefix so that "V3.9" → "3.9", not "v3.9".
func normaliseCycle(cycle string) string {
	return strings.TrimPrefix(strings.ToLower(cycle), "v")
}

// parseEOLPayload interprets a cached JSON payload and returns (eol_status, eol_date, error).
func parseEOLPayload(payload []byte) (string, *string, error) {
	if len(payload) == 0 {
		return "unknown", nil, nil
	}

	var entry eolCycle
	if err := json.Unmarshal(payload, &entry); err != nil {
		return "unknown", nil, fmt.Errorf("parse cached payload: %w", err)
	}

	if !entry.EOL.IsEOL {
		return "supported", nil, nil
	}
	if entry.EOL.Date != "" {
		date := entry.EOL.Date
		return "eol", &date, nil
	}
	return "eol", nil, nil
}

// componentRow is an internal struct for listing components by SBOM.
type componentRow struct {
	ID      string
	Name    string
	Version string
}
