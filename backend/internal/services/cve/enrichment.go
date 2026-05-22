package cve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	nvdBaseURL      = "https://services.nvd.nist.gov/rest/json/cves/2.0"
	cacheTTL        = 24 * time.Hour
	retryBackoff    = 10 * time.Second
	maxRetries      = 2
	cacheKeyPrefix  = "cve:"
)

// CVEDetails holds the enriched data for a single CVE.
type CVEDetails struct {
	ID          string
	Description string
	CVSSScore   float64
	Published   time.Time
}

// CVEEnrichmentService fetches CVE details from the NVD API and caches them in Redis.
// This is a pull integration: data flows FROM NVD TO Vakt — no Vakt data is sent out.
type CVEEnrichmentService struct {
	redis      *redis.Client
	httpClient *http.Client
}

// NewCVEEnrichmentService creates a new CVEEnrichmentService.
func NewCVEEnrichmentService(redisClient *redis.Client, httpClient *http.Client) *CVEEnrichmentService {
	return &CVEEnrichmentService{
		redis:      redisClient,
		httpClient: httpClient,
	}
}

// Enrich returns CVE details for the given CVE ID.
// Results are cached in Redis with a 24h TTL.
// Returns an error if cveID is empty or does not start with "CVE-".
func (s *CVEEnrichmentService) Enrich(ctx context.Context, cveID string) (*CVEDetails, error) {
	if cveID == "" {
		return nil, errors.New("cve: ID must not be empty")
	}
	if !strings.HasPrefix(cveID, "CVE-") {
		return nil, fmt.Errorf("cve: invalid ID %q — must start with \"CVE-\"", cveID)
	}

	// Check cache first.
	key := cacheKeyPrefix + cveID
	if cached, err := s.redis.Get(ctx, key).Bytes(); err == nil {
		var details CVEDetails
		if err := json.Unmarshal(cached, &details); err == nil {
			return &details, nil
		}
	}

	// Fetch from NVD with retry on 429.
	details, err := s.fetchFromNVD(ctx, cveID)
	if err != nil {
		return nil, err
	}

	// Persist to cache. Cache errors are non-fatal.
	if data, err := json.Marshal(details); err == nil {
		_ = s.redis.Set(ctx, key, data, cacheTTL).Err()
	}

	return details, nil
}

// fetchFromNVD calls the NVD REST API v2.0 with up to maxRetries retries on 429.
func (s *CVEEnrichmentService) fetchFromNVD(ctx context.Context, cveID string) (*CVEDetails, error) {
	url := fmt.Sprintf("%s?cveId=%s", nvdBaseURL, cveID)

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Back off before retrying.
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryBackoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("cve: build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := s.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("cve: http request: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("cve: NVD API rate limited (429)")
			continue
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("cve: NVD API returned status %d", resp.StatusCode)
		}

		var nvdResp nvdResponse
		if err := json.NewDecoder(resp.Body).Decode(&nvdResp); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("cve: decode NVD response: %w", err)
		}
		_ = resp.Body.Close()

		return parseNVDResponse(cveID, &nvdResp)
	}

	return nil, fmt.Errorf("cve: exhausted retries for %s: %w", cveID, lastErr)
}

// parseNVDResponse extracts CVEDetails from the raw NVD API response.
func parseNVDResponse(cveID string, r *nvdResponse) (*CVEDetails, error) {
	if len(r.Vulnerabilities) == 0 {
		return nil, fmt.Errorf("cve: no vulnerability data found for %s", cveID)
	}

	item := r.Vulnerabilities[0].CVE
	details := &CVEDetails{ID: item.ID}

	// Prefer English description; fall back to first available.
	for _, d := range item.Descriptions {
		if d.Lang == "en" {
			details.Description = d.Value
			break
		}
	}
	if details.Description == "" && len(item.Descriptions) > 0 {
		details.Description = item.Descriptions[0].Value
	}

	// Parse published date (RFC3339 / ISO 8601).
	if item.Published != "" {
		t, err := time.Parse(time.RFC3339, item.Published)
		if err != nil {
			// NVD sometimes omits the Z; try without timezone.
			t, err = time.Parse("2006-01-02T15:04:05.000", item.Published)
			if err == nil {
				details.Published = t.UTC()
			}
		} else {
			details.Published = t.UTC()
		}
	}

	// Extract CVSS v3 base score (prefer v3.1, fall back to v3.0).
	if m := item.Metrics; m != nil {
		for _, cv3 := range m.CVSSMetricV31 {
			details.CVSSScore = cv3.CVSSData.BaseScore
			break
		}
		if details.CVSSScore == 0 {
			for _, cv3 := range m.CVSSMetricV30 {
				details.CVSSScore = cv3.CVSSData.BaseScore
				break
			}
		}
	}

	return details, nil
}

// ---- NVD API response structures ----

type nvdResponse struct {
	Vulnerabilities []nvdVulnerabilityWrapper `json:"vulnerabilities"`
}

type nvdVulnerabilityWrapper struct {
	CVE nvdCVE `json:"cve"`
}

type nvdCVE struct {
	ID           string           `json:"id"`
	Published    string           `json:"published"`
	Descriptions []nvdDescription `json:"descriptions"`
	Metrics      *nvdMetrics      `json:"metrics"`
}

type nvdDescription struct {
	Lang  string `json:"lang"`
	Value string `json:"value"`
}

type nvdMetrics struct {
	CVSSMetricV31 []nvdCVSSEntry `json:"cvssMetricV31"`
	CVSSMetricV30 []nvdCVSSEntry `json:"cvssMetricV30"`
}

type nvdCVSSEntry struct {
	CVSSData nvdCVSSData `json:"cvssData"`
}

type nvdCVSSData struct {
	BaseScore float64 `json:"baseScore"`
}
