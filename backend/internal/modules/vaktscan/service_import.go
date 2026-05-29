package vaktscan

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// SARIF 2.1.0
// ---------------------------------------------------------------------------

type sarif struct {
	Runs []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name string `json:"name"` // e.g. "Snyk", "CodeQL", "Semgrep"
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Message   sarifMessage    `json:"message"`
	Level     string          `json:"level"` // error|warning|note|none
	Locations []sarifLocation `json:"locations"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

// ImportSARIF parses a SARIF 2.1.0 JSON payload and inserts findings.
// Returns the number of imported findings.
func (s *Service) ImportSARIF(ctx context.Context, orgID, assetID string, data []byte) (int, error) {
	if err := s.validateAssetOwnership(ctx, orgID, assetID); err != nil {
		return 0, err
	}

	var doc sarif
	if err := json.Unmarshal(data, &doc); err != nil {
		return 0, fmt.Errorf("parse SARIF JSON: %w", err)
	}

	cfg, _ := s.repo.GetSLAConfig(ctx, orgID)

	imported := 0
	for _, run := range doc.Runs {
		toolName := run.Tool.Driver.Name
		if toolName == "" {
			toolName = "sarif"
		}

		for _, result := range run.Results {
			severity := sarifLevelToSeverity(result.Level)

			// Build title: "{ruleId}: {message.text}" truncated to 200 chars.
			titleBase := result.Message.Text
			if result.RuleID != "" {
				titleBase = result.RuleID + ": " + result.Message.Text
			}
			title := sarifTruncate(titleBase, 200)
			if title == "" {
				title = "SARIF finding"
			}

			// Extract location info.
			var locationInfo string
			if len(result.Locations) > 0 {
				loc := result.Locations[0].PhysicalLocation
				uri := loc.ArtifactLocation.URI
				line := loc.Region.StartLine
				if uri != "" && line > 0 {
					locationInfo = fmt.Sprintf("\n\nLocation: %s (line %d)", uri, line)
				} else if uri != "" {
					locationInfo = fmt.Sprintf("\n\nLocation: %s", uri)
				}
			}

			description := result.Message.Text + locationInfo

			rawID := result.RuleID
			if rawID == "" {
				rawID = result.Message.Text
			}

			slaDueAt := calcSLADueAt(cfg, severity)

			f := Finding{
				OrgID:       orgID,
				AssetID:     assetID,
				Title:       title,
				Description: description,
				Severity:    severity,
				Status:      "open",
				Scanner:     strings.ToLower(toolName),
				RawID:       rawID,
				SLADueAt:    slaDueAt,
			}

			if _, err := s.repo.UpsertFindingByRawID(ctx, orgID, f); err != nil {
				return imported, fmt.Errorf("upsert SARIF finding %q: %w", rawID, err)
			}
			imported++
		}
	}
	return imported, nil
}

// sarifLevelToSeverity maps a SARIF result level to a SecPulse severity string.
// Mapping per SARIF 2.1.0 spec: error→high, warning→medium, note→low, none→info.
func sarifLevelToSeverity(level string) string {
	switch level {
	case "error":
		return "high"
	case "warning":
		return "medium"
	case "note":
		return "low"
	default: // "none" or absent
		return "info"
	}
}

// sarifTruncate truncates s to at most max runes.
func sarifTruncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// ---------------------------------------------------------------------------
// CycloneDX 1.4 (JSON)
// ---------------------------------------------------------------------------

type cdxBOM struct {
	Vulnerabilities []cdxVuln `json:"vulnerabilities"`
}

type cdxVuln struct {
	ID      string      `json:"id"` // CVE-ID
	Source  cdxSource   `json:"source"`
	Ratings []cdxRating `json:"ratings"`
	Detail  string      `json:"detail"`
}

type cdxRating struct {
	Score    float64 `json:"score"`
	Severity string  `json:"severity"` // critical/high/medium/low
	Method   string  `json:"method"`   // CVSS_31 etc.
}

type cdxSource struct {
	Name string `json:"name"`
}

// ImportCycloneDX parses a CycloneDX 1.4 JSON BOM and inserts vulnerability findings.
// Returns the number of imported findings.
func (s *Service) ImportCycloneDX(ctx context.Context, orgID, assetID string, data []byte) (int, error) {
	if err := s.validateAssetOwnership(ctx, orgID, assetID); err != nil {
		return 0, err
	}

	var bom cdxBOM
	if err := json.Unmarshal(data, &bom); err != nil {
		return 0, fmt.Errorf("parse CycloneDX JSON: %w", err)
	}

	cfg, _ := s.repo.GetSLAConfig(ctx, orgID)

	imported := 0
	for _, vuln := range bom.Vulnerabilities {
		severity, cvssScore := extractCDXRating(vuln.Ratings)
		cveID := vuln.ID
		rawID := vuln.ID
		if rawID == "" {
			rawID = "cdx-" + strconv.Itoa(imported)
		}
		title := vuln.ID
		if title == "" {
			title = "CycloneDX vulnerability"
		}

		slaDueAt := calcSLADueAt(cfg, severity)

		var cvePtr *string
		if cveID != "" {
			cvePtr = &cveID
		}

		var cvssPtr *float64
		if cvssScore > 0 {
			cvssPtr = &cvssScore
		}

		f := Finding{
			OrgID:       orgID,
			AssetID:     assetID,
			CVEID:       cvePtr,
			Title:       title,
			Description: vuln.Detail,
			Severity:    severity,
			CVSSScore:   cvssPtr,
			Status:      "open",
			Scanner:     "cyclonedx",
			RawID:       rawID,
			SLADueAt:    slaDueAt,
		}

		if _, err := s.repo.UpsertFindingByRawID(ctx, orgID, f); err != nil {
			return imported, fmt.Errorf("upsert CycloneDX finding %q: %w", rawID, err)
		}
		imported++
	}
	return imported, nil
}

// extractCDXRating picks the most relevant rating from a CycloneDX vulnerability.
// Prefers CVSS_31, then CVSS_30, then the first entry. Returns severity and score.
func extractCDXRating(ratings []cdxRating) (string, float64) {
	if len(ratings) == 0 {
		return "medium", 0
	}

	best := ratings[0]
	for _, r := range ratings {
		if strings.HasPrefix(strings.ToUpper(r.Method), "CVSS_3") {
			best = r
			break
		}
	}

	severity := strings.ToLower(best.Severity)
	if severity == "" {
		severity = scoreToSeverity(best.Score)
	}
	return severity, best.Score
}

func scoreToSeverity(score float64) string {
	switch {
	case score >= 9.0:
		return "critical"
	case score >= 7.0:
		return "high"
	case score >= 4.0:
		return "medium"
	default:
		return "low"
	}
}

// ---------------------------------------------------------------------------
// Generic CSV
// ---------------------------------------------------------------------------

// ImportCSV parses a CSV with header: title,severity,cve_id,description,cvss_score
// (only title is required; all other fields are optional).
// Returns the number of imported findings.
func (s *Service) ImportCSV(ctx context.Context, orgID, assetID string, data []byte) (int, error) {
	if err := s.validateAssetOwnership(ctx, orgID, assetID); err != nil {
		return 0, err
	}

	cfg, _ := s.repo.GetSLAConfig(ctx, orgID)

	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return 0, fmt.Errorf("read CSV header: %w", err)
	}

	colIdx := make(map[string]int, len(header))
	for i, h := range header {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}

	if _, ok := colIdx["title"]; !ok {
		return 0, fmt.Errorf("CSV missing required column \"title\"")
	}

	imported := 0
	for {
		record, readErr := reader.Read()
		if readErr != nil {
			break
		}

		col := func(name string) string {
			if i, ok := colIdx[name]; ok && i < len(record) {
				return strings.TrimSpace(record[i])
			}
			return ""
		}

		title := col("title")
		if title == "" {
			continue
		}

		severity := strings.ToLower(col("severity"))
		switch severity {
		case "critical", "high", "medium", "low", "info":
		default:
			severity = "medium"
		}

		cveIDStr := col("cve_id")
		description := col("description")
		cvssStr := col("cvss_score")

		var cvePtr *string
		if cveIDStr != "" {
			cvePtr = &cveIDStr
		}

		var cvssPtr *float64
		if cvssStr != "" {
			if v, parseErr := strconv.ParseFloat(cvssStr, 64); parseErr == nil {
				cvssPtr = &v
			}
		}

		slaDueAt := calcSLADueAt(cfg, severity)
		rawID := title
		if cveIDStr != "" {
			rawID = cveIDStr
		}

		f := Finding{
			OrgID:       orgID,
			AssetID:     assetID,
			CVEID:       cvePtr,
			Title:       title,
			Description: description,
			Severity:    severity,
			CVSSScore:   cvssPtr,
			Status:      "open",
			Scanner:     "csv",
			RawID:       rawID,
			SLADueAt:    slaDueAt,
		}

		if _, err := s.repo.UpsertFindingByRawID(ctx, orgID, f); err != nil {
			return imported, fmt.Errorf("upsert CSV finding %q: %w", title, err)
		}
		imported++
	}
	return imported, nil
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// validateAssetOwnership checks that the asset exists and belongs to the org.
func (s *Service) validateAssetOwnership(ctx context.Context, orgID, assetID string) error {
	if orgID == "" {
		return fmt.Errorf("orgID is required")
	}
	if assetID == "" {
		return fmt.Errorf("assetID is required")
	}
	if _, err := s.repo.GetAsset(ctx, orgID, assetID); err != nil {
		return fmt.Errorf("asset not found or not accessible: %w", err)
	}
	return nil
}

// calcSLADueAt computes the sla_due_at timestamp from the org's SLA config.
func calcSLADueAt(cfg *SLAConfig, severity string) *time.Time {
	days := slaDaysForSeverity(cfg, severity)
	t := time.Now().UTC().AddDate(0, 0, days)
	return &t
}
