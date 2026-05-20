// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secpulse

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

// eolCycle is one entry returned by the endoflife.date API.
type eolCycle struct {
	Cycle             string      `json:"cycle"`
	EOL               interface{} `json:"eol"` // can be bool or "YYYY-MM-DD" string
	LatestReleaseDate string      `json:"latestReleaseDate,omitempty"`
}

// EOLChecker checks components against the endoflife.date API and stores results.
type EOLChecker struct {
	db         *pgxpool.Pool
	httpClient *http.Client
}

// NewEOLChecker creates a new EOLChecker backed by the given database pool.
func NewEOLChecker(db *pgxpool.Pool) *EOLChecker {
	return &EOLChecker{
		db:         db,
		httpClient: &http.Client{Timeout: 10 * time.Second},
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

	for _, e := range missEntries {
		e := e
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					log.Error().Interface("panic", r).Msg("eol goroutine panic recovered")
				}
			}()

			status, eolDate, fetchedPayload, err := c.fetchEOL(ctx, e.product, e.cycle)
			if err != nil {
				log.Warn().Err(err).Str("component", e.comp.Name).Msg("EOL fetch failed")
				return
			}
			// Persist to cache (best-effort).
			if upsertErr := repo.upsertEOLCache(ctx, e.product, e.cycle, fetchedPayload); upsertErr != nil {
				log.Warn().Err(upsertErr).Str("product", e.product).Msg("EOL cache upsert failed")
			}
			mu.Lock()
			results = append(results, eolResult{componentID: e.comp.ID, eolStatus: status, eolDate: eolDate})
			mu.Unlock()
		}()
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
	url := fmt.Sprintf("https://endoflife.date/api/%s.json", product)
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

	var cycles []eolCycle
	if err := json.Unmarshal(body, &cycles); err != nil {
		return "unknown", nil, nil, fmt.Errorf("parse endoflife.date response: %w", err)
	}

	for i := range cycles {
		if normaliseCycle(cycles[i].Cycle) == normaliseCycle(cycle) {
			payload, _ := json.Marshal(cycles[i])
			status, eolDate, err := parseEOLPayload(payload)
			return status, eolDate, payload, err
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

// normaliseCycle trims leading "v" and lowercases for comparison.
func normaliseCycle(cycle string) string {
	return strings.ToLower(strings.TrimPrefix(cycle, "v"))
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

	switch v := entry.EOL.(type) {
	case bool:
		if v {
			return "eol", nil, nil
		}
		return "supported", nil, nil
	case string:
		// Non-empty string is the EOL date.
		if v == "" || v == "false" {
			return "supported", nil, nil
		}
		if v == "true" {
			return "eol", nil, nil
		}
		// It's a date string like "2024-10-31".
		return "eol", &v, nil
	}

	return "unknown", nil, nil
}

// componentRow is an internal struct for listing components by SBOM.
type componentRow struct {
	ID      string
	Name    string
	Version string
}
