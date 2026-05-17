package secvitals

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/sechealth-app/sechealth/internal/shared/ai"
	"github.com/sechealth-app/sechealth/internal/shared/dashboard"
	"github.com/sechealth-app/sechealth/internal/shared/notify"
)

// ErrDORANotEnabled is returned when DORA framework is not enabled for the organisation.
var ErrDORANotEnabled = errors.New("DORA framework not enabled")

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// Service handles ComplyKit business logic.
type Service struct {
	db       *pgxpool.Pool
	rdb      *redis.Client
	repo     *Repository
	notifSvc notifyService
	aiClient *ai.AIClient
}

// notifyService abstracts the notify.Service dependency for testability.
type notifyService interface {
	Notify(ctx context.Context, msg notify.Message) error
}

// NewService creates a new ComplyKit service.
func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		db:   db,
		repo: NewRepository(db),
	}
}

// WithRedis sets the Redis client used for dashboard cache invalidation.
func (s *Service) WithRedis(rdb *redis.Client) {
	s.rdb = rdb
}

// invalidateDashboardCache deletes the cached dashboard aggregate for the given
// org from Redis. It is a no-op when Redis is not configured.
func (s *Service) invalidateDashboardCache(ctx context.Context, orgID string) {
	if err := dashboard.InvalidateDashboardCache(ctx, s.rdb, orgID); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("secvitals: dashboard cache invalidation failed")
	}
}

// WithNotifyService sets the notification service used for external email delivery.
func (s *Service) WithNotifyService(n notifyService) {
	s.notifSvc = n
}

// WithAIClient sets the AI client used for policy draft generation.
func (s *Service) WithAIClient(c *ai.AIClient) {
	s.aiClient = c
}

// Repo exposes the underlying repository for use by ancillary services (e.g. EvidenceFileService).
func (s *Service) Repo() *Repository {
	return s.repo
}

// --- Frameworks ---

// ListFrameworks returns all frameworks enabled for the given organisation.
func (s *Service) ListFrameworks(ctx context.Context, orgID string) ([]Framework, error) {
	frameworks, err := s.repo.ListFrameworks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list frameworks: %w", err)
	}
	return frameworks, nil
}

// DeleteFramework removes a framework and all associated data.
func (s *Service) DeleteFramework(ctx context.Context, orgID, frameworkID string) error {
	return s.repo.DeleteFramework(ctx, orgID, frameworkID)
}

// EnableFramework creates a new framework (and seeds its controls) for the organisation.
// If the framework is already enabled, it returns the existing record.
func (s *Service) EnableFramework(ctx context.Context, orgID, name string) (*Framework, error) {
	exists, err := s.repo.FrameworkExists(ctx, orgID, name)
	if err != nil {
		return nil, err
	}
	if exists {
		// Return the already-enabled framework.
		frameworks, err := s.repo.ListFrameworks(ctx, orgID)
		if err != nil {
			return nil, err
		}
		for i := range frameworks {
			if frameworks[i].Name == name {
				return &frameworks[i], nil
			}
		}
	}

	// Determine version from built-in templates.
	version := builtinVersion(name)
	isBuiltin := version != ""
	if version == "" {
		version = "1.0"
	}

	fw, err := s.repo.CreateFramework(ctx, orgID, name, version, isBuiltin)
	if err != nil {
		return nil, fmt.Errorf("enable framework %s: %w", name, err)
	}

	// Seed controls from built-in template.
	controls := builtinControls(fw.ID, orgID, name)
	if len(controls) > 0 {
		if err := s.repo.BulkInsertControls(ctx, controls); err != nil {
			log.Warn().Err(err).Str("framework", name).Msg("failed to seed controls")
		}
	}

	// Auto-seed TISAX ↔ ISO 27001 mappings when either framework is enabled.
	if name == "TISAX" || name == "ISO27001" {
		if seedErr := s.SeedTISAXMappings(ctx, orgID); seedErr != nil {
			log.Warn().Err(seedErr).Str("framework", name).Msg("failed to seed TISAX↔ISO27001 mappings (non-critical)")
		}
	}

	// Auto-seed DSGVO-TOM ↔ ISO 27001 mappings when either framework is enabled.
	if name == "DSGVO-TOM" || name == "ISO27001" {
		if seedErr := s.SeedDSGVOMappings(ctx, orgID); seedErr != nil {
			log.Warn().Err(seedErr).Str("framework", name).Msg("failed to seed DSGVO↔ISO27001 mappings (non-critical)")
		}
	}

	return fw, nil
}

// ListAvailableFrameworks returns all frameworks that can be enabled, merged with current org state.
func (s *Service) ListAvailableFrameworks(ctx context.Context, orgID string) ([]AvailableFramework, error) {
	enabled, err := s.repo.ListFrameworks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list frameworks: %w", err)
	}
	enabledByName := make(map[string]bool, len(enabled))
	for _, fw := range enabled {
		enabledByName[fw.Name] = true
	}

	result := make([]AvailableFramework, 0, len(builtinAvailable))
	for _, b := range builtinAvailable {
		result = append(result, AvailableFramework{
			Name:        b.name,
			Version:     builtinVersion(b.name),
			Description: b.description,
			IsBuiltin:   true,
			IsEnabled:   enabledByName[b.name],
		})
	}
	return result, nil
}

// InstallFrameworkPlugin parses a FrameworkPlugin and creates the framework with its controls.
func (s *Service) InstallFrameworkPlugin(ctx context.Context, orgID string, plugin *FrameworkPlugin) (*Framework, error) {
	if plugin.Name == "" {
		return nil, fmt.Errorf("plugin name is required")
	}
	version := plugin.Version
	if version == "" {
		version = "1.0"
	}

	fw, err := s.repo.CreateFramework(ctx, orgID, plugin.Name, version, false)
	if err != nil {
		return nil, fmt.Errorf("create plugin framework %s: %w", plugin.Name, err)
	}

	controls := make([]Control, 0, len(plugin.Controls))
	for _, pc := range plugin.Controls {
		evType := pc.EvidenceType
		if evType == "" {
			evType = "manual"
		}
		controls = append(controls, Control{
			FrameworkID:  fw.ID,
			OrgID:        orgID,
			ControlID:    pc.ID,
			Title:        pc.Title,
			Description:  pc.Description,
			Domain:       pc.Domain,
			EvidenceType: evType,
			Weight:       pc.Weight,
		})
	}
	if len(controls) > 0 {
		if err := s.repo.BulkInsertControls(ctx, controls); err != nil {
			return nil, fmt.Errorf("seed plugin controls: %w", err)
		}
	}
	return fw, nil
}

// ReseedBuiltinControls reseeds controls for all builtin frameworks across all orgs.
// Called on startup after migrations to ensure controls are up-to-date.
func (s *Service) ReseedBuiltinControls(ctx context.Context) {
	frameworks, err := s.repo.ListAllBuiltinFrameworks(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("reseed: failed to list builtin frameworks")
		return
	}
	for _, fw := range frameworks {
		controls := builtinControls(fw.ID, fw.OrgID, fw.Name)
		if len(controls) == 0 {
			continue
		}
		if err := s.repo.BulkInsertControls(ctx, controls); err != nil {
			log.Warn().Err(err).Str("framework", fw.Name).Msg("reseed: failed to insert controls")
		} else {
			log.Info().Str("framework", fw.Name).Int("controls", len(controls)).Msg("reseeded builtin controls")
		}
	}
}

// GetControlMappings returns all global cross-framework mappings for a given control,
// resolved to org-specific control UUIDs.
func (s *Service) GetControlMappings(ctx context.Context, orgID, controlID string) ([]ControlMapping, error) {
	mappings, err := s.repo.GetMappingsForControl(ctx, orgID, controlID)
	if err != nil {
		return nil, fmt.Errorf("get control mappings: %w", err)
	}
	return mappings, nil
}

// SeedFrameworkMappings idempotently seeds the global ISO 27001 ↔ NIS2 and
// ISO 27001 ↔ BSI cross-framework control mappings into ck_framework_control_mappings.
// These are global text-code entries — no org_id required.
// Called once on startup alongside ReseedBuiltinControls.
func (s *Service) SeedFrameworkMappings(ctx context.Context) error {
	type entry struct {
		srcFW, srcCode, tgtFW, tgtCode, mappingType string
	}

	// Framework slugs must match the lower(name) LIKE '%<slug>%' pattern used at query time.
	// Use the exact framework names stored in ck_frameworks.
	const (
		iso  = "ISO27001"
		nis2 = "NIS2"
		bsi  = "BSI"
	)

	// ISO 27001 (2013 numbering as stored in DB) ↔ NIS2 bidirectional mappings.
	// NIS2 Art. 21 §2 sub-clauses mapped to the nearest ISO 27001 controls present in the DB.
	isoNIS2 := []entry{
		// §2(a) — risk analysis / policies: A.5.1 ↔ NIS2-A.1 (IS-Richtlinie)
		{iso, "A.5.1", nis2, "NIS2-A.1", "equivalent"},
		// §2(b) — incident handling: A.16.1 ↔ NIS2-B.1 (Incident-Response-Richtlinie)
		{iso, "A.16.1", nis2, "NIS2-B.1", "equivalent"},
		// §2(b) — incident handling: A.16.1 ↔ NIS2-B.5 (24h-Meldung)
		{iso, "A.16.1", nis2, "NIS2-B.5", "partial"},
		// §2(c) — monitoring / business continuity: A.12.1 ↔ NIS2-C.1 (BCM-Richtlinie)
		{iso, "A.12.1", nis2, "NIS2-C.1", "partial"},
		// §2(c) — business continuity: A.17.1 ↔ NIS2-C.1
		{iso, "A.17.1", nis2, "NIS2-C.1", "equivalent"},
		// §2(e) — vulnerability management: A.12.6 ↔ NIS2-E.3
		{iso, "A.12.6", nis2, "NIS2-E.3", "equivalent"},
		// §2(e) — vulnerability management: A.12.6 ↔ NIS2-E.4 (Patch-Management)
		{iso, "A.12.6", nis2, "NIS2-E.4", "partial"},
		// §2(g) — awareness training: A.8.1 is asset mgmt; closest for training in DB is A.6.1.1 (roles)
		// Map to NIS2-G.2 (Sicherheitsschulung)
		{iso, "A.6.1", nis2, "NIS2-G.2", "partial"},
		// §2(h) — cryptography: A.10.1 ↔ NIS2-H.1 (Kryptographierichtlinie)
		{iso, "A.10.1", nis2, "NIS2-H.1", "equivalent"},
		// §2(i) — supply chain: A.18.1 ↔ NIS2-D.1 (Lieferanten-Sicherheitsrichtlinie)
		{iso, "A.18.1", nis2, "NIS2-D.1", "partial"},
	}

	// ISO 27001 (2013 numbering) ↔ BSI Grundschutz bidirectional mappings.
	// BSI codes as stored in DB: BSI-ORP.1, BSI-ORP.2, BSI-DER.2.1, BSI-OPS.1.1.2, BSI-CON.3, BSI-NET.1.1, BSI-SYS.1.1.
	isoBSI := []entry{
		// A.5.1 (Policies) ↔ BSI-ORP.1 (Organisation)
		{iso, "A.5.1", bsi, "BSI-ORP.1", "equivalent"},
		// A.6.1 (Internal Organisation / Personnel) ↔ BSI-ORP.2 (Personnel)
		{iso, "A.6.1", bsi, "BSI-ORP.2", "partial"},
		// A.8.1 (Asset Management) ↔ BSI-OPS.1.1.2 (IT-Administration/Operations)
		{iso, "A.8.1", bsi, "BSI-OPS.1.1.2", "partial"},
		// A.12.6 (Vulnerability Management / Patch) ↔ BSI-OPS.1.1.2 (proper IT administration incl. patching)
		{iso, "A.12.6", bsi, "BSI-OPS.1.1.2", "equivalent"},
		// A.12.3 (Data Backup) ↔ BSI-CON.3 (Data Backup Policy)
		{iso, "A.12.3", bsi, "BSI-CON.3", "equivalent"},
		// A.12.1 (Operations / Monitoring) ↔ BSI-OPS.1.1.2
		{iso, "A.12.1", bsi, "BSI-OPS.1.1.2", "partial"},
		// A.10.1 (Cryptography) — no direct BSI-CON.1 in DB; map to BSI-SYS.1.1 (General Server, incl. crypto hardening)
		{iso, "A.10.1", bsi, "BSI-SYS.1.1", "informative"},
		// A.16.1 (Incident Management) ↔ BSI-DER.2.1 (Incident Management)
		{iso, "A.16.1", bsi, "BSI-DER.2.1", "equivalent"},
		// A.9.1 (Zugangskontrolle) ↔ BSI-NET.1.1 (Network architecture — includes access control)
		{iso, "A.9.1", bsi, "BSI-NET.1.1", "informative"},
	}

	seed := func(entries []entry) error {
		for _, e := range entries {
			if err := s.repo.SeedGlobalControlMapping(ctx, e.srcFW, e.srcCode, e.tgtFW, e.tgtCode, e.mappingType); err != nil {
				log.Warn().Err(err).
					Str("src", e.srcFW+"/"+e.srcCode).
					Str("tgt", e.tgtFW+"/"+e.tgtCode).
					Msg("seed framework mapping failed (non-critical)")
			}
			// Reverse direction
			if err := s.repo.SeedGlobalControlMapping(ctx, e.tgtFW, e.tgtCode, e.srcFW, e.srcCode, e.mappingType); err != nil {
				log.Warn().Err(err).
					Str("src", e.tgtFW+"/"+e.tgtCode).
					Str("tgt", e.srcFW+"/"+e.srcCode).
					Msg("seed reverse framework mapping failed (non-critical)")
			}
		}
		return nil
	}

	if err := seed(isoNIS2); err != nil {
		return err
	}
	return seed(isoBSI)
}

// GetFramework returns a single framework by ID.
func (s *Service) GetFramework(ctx context.Context, orgID, frameworkID string) (*Framework, error) {
	return s.repo.GetFramework(ctx, orgID, frameworkID)
}

// GetReadinessReport computes a full readiness report for a framework.
func (s *Service) GetReadinessReport(ctx context.Context, orgID, frameworkID string) (*ReadinessReport, error) {
	fw, err := s.repo.GetFramework(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("get framework: %w", err)
	}

	controls, err := s.repo.ListControls(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("list controls: %w", err)
	}

	evidenceCounts, err := s.repo.CountEvidenceByControl(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("count evidence: %w", err)
	}

	report := computeReadinessReport(fw, controls, evidenceCounts)
	if fw.Name == "TISAX" {
		report.TISAXMaturity = computeTISAXMaturity(controls)
	}
	return report, nil
}

// GetGapAnalysis returns controls that are missing or at-risk evidence.
func (s *Service) GetGapAnalysis(ctx context.Context, orgID, frameworkID string) (*GapAnalysis, error) {
	controls, err := s.repo.ListControls(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("list controls: %w", err)
	}

	evidenceCounts, err := s.repo.CountEvidenceByControl(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("count evidence: %w", err)
	}

	// Expiring evidence: anything expiring in the next 30 days.
	threshold := time.Now().UTC().Add(30 * 24 * time.Hour)
	expiring, err := s.repo.GetExpiringEvidence(ctx, orgID, frameworkID, threshold)
	if err != nil {
		return nil, fmt.Errorf("get expiring evidence: %w", err)
	}

	// Build expiry map keyed by control UUID.
	expiryMap := make(map[string]*time.Time)
	for i := range expiring {
		expiryMap[expiring[i].ControlID] = expiring[i].ExpiresAt
	}

	analysis := &GapAnalysis{FrameworkID: frameworkID}
	for i := range controls {
		c := controls[i]
		count := evidenceCounts[c.ID]
		if count == 0 {
			analysis.Gaps = append(analysis.Gaps, ControlGap{
				Control: c,
				Reason:  "no_evidence",
			})
		} else if ea, ok := expiryMap[c.ID]; ok {
			analysis.Gaps = append(analysis.Gaps, ControlGap{
				Control:   c,
				Reason:    "evidence_expiring",
				ExpiresAt: ea,
			})
		}
	}

	return analysis, nil
}

// --- Controls ---

// ListControls returns all controls for a framework within an organisation.
func (s *Service) ListControls(ctx context.Context, orgID, frameworkID string) ([]Control, error) {
	controls, err := s.repo.ListControls(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("list controls: %w", err)
	}

	// Enrich with evidence counts.
	counts, err := s.repo.CountEvidenceByControl(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("count evidence for controls: %w", err)
	}

	for i := range controls {
		controls[i].EvidenceCount = counts[controls[i].ID]
		controls[i].Status = resolveStatus(controls[i])
		if strings.HasPrefix(controls[i].ControlID, "DORA-") {
			if m, ok := doraISO27001Mapping[controls[i].ControlID]; ok {
				controls[i].ISO27001Mapping = m
			}
		}
	}

	return controls, nil
}

// UpdateControl updates not_applicable, reason, manual_status, and optionally maturity_score on a control.
func (s *Service) UpdateControl(ctx context.Context, orgID, controlID string, input UpdateControlInput) (*Control, error) {
	if input.MaturityScore != nil && (*input.MaturityScore < 0 || *input.MaturityScore > 3) {
		return nil, fmt.Errorf("maturity_score must be between 0 and 3")
	}
	if err := s.repo.UpdateControl(ctx, orgID, controlID, input.NotApplicable, input.Reason, input.ManualStatus, input.MaturityScore); err != nil {
		return nil, fmt.Errorf("update control: %w", err)
	}
	s.invalidateDashboardCache(ctx, orgID)
	return s.GetControl(ctx, orgID, controlID)
}

// filterTISAXByProtectionLevel filters controls based on protection level.
// When protectionLevel != "very_high", controls with ControlID prefix "TISAX-15" are excluded.
func filterTISAXByProtectionLevel(controls []Control, protectionLevel string) []Control {
	if protectionLevel == "very_high" {
		return controls
	}
	filtered := make([]Control, 0, len(controls))
	for _, c := range controls {
		if !strings.HasPrefix(c.ControlID, "TISAX-15") {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// buildTISAXGapAnalysis constructs a TISAXGapAnalysis from a control slice.
func buildTISAXGapAnalysis(frameworkID string, controls []Control) *TISAXGapAnalysis {
	analysis := &TISAXGapAnalysis{
		FrameworkID: frameworkID,
		TargetScore: 3,
	}
	for _, c := range controls {
		if c.MaturityScore < 3 {
			analysis.Gaps = append(analysis.Gaps, TISAXControlGap{
				Control:      c,
				MaturityGap:  3 - c.MaturityScore,
				CurrentScore: c.MaturityScore,
			})
		}
	}
	return analysis
}

// computeTISAXMaturity computes the TISAX maturity summary from a set of controls.
// Controls are grouped by Domain; per-domain stats (avg, total, fully_mature, color) are computed.
// Chapters are sorted by domain name for stable output.
func computeTISAXMaturity(controls []Control) *TISAXMaturitySummary {
	if len(controls) == 0 {
		return &TISAXMaturitySummary{
			AvgScore:         0.0,
			ByChapter:        []ChapterMaturity{},
			ReadinessPercent: 0.0,
		}
	}

	// Group by domain.
	type domainAcc struct {
		sum         int
		total       int
		fullyMature int
	}
	domainMap := make(map[string]*domainAcc)
	var totalSum, totalCount int
	for _, c := range controls {
		acc := domainMap[c.Domain]
		if acc == nil {
			acc = &domainAcc{}
			domainMap[c.Domain] = acc
		}
		acc.sum += c.MaturityScore
		acc.total++
		if c.MaturityScore == 3 {
			acc.fullyMature++
		}
		totalSum += c.MaturityScore
		totalCount++
	}

	// Sort domain names.
	domains := make([]string, 0, len(domainMap))
	for d := range domainMap {
		domains = append(domains, d)
	}
	sort.Strings(domains)

	chapters := make([]ChapterMaturity, 0, len(domains))
	for _, d := range domains {
		acc := domainMap[d]
		avg := float64(acc.sum) / float64(acc.total)
		color := "red"
		if avg >= 2.5 {
			color = "green"
		} else if avg >= 1.5 {
			color = "yellow"
		}
		chapters = append(chapters, ChapterMaturity{
			Domain:        d,
			AvgScore:      avg,
			TotalControls: acc.total,
			FullyMature:   acc.fullyMature,
			Color:         color,
		})
	}

	var avgScore float64
	if totalCount > 0 {
		avgScore = float64(totalSum) / float64(totalCount)
	}

	return &TISAXMaturitySummary{
		AvgScore:         avgScore,
		ByChapter:        chapters,
		ReadinessPercent: (avgScore / 3.0) * 100.0,
	}
}

// ListTISAXControls returns controls for a TISAX framework, filtered by protection level.
// When protectionLevel != "very_high", controls with ControlID prefix "TISAX-15" are excluded.
func (s *Service) ListTISAXControls(ctx context.Context, orgID, frameworkID, protectionLevel string) ([]Control, error) {
	controls, err := s.ListControls(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("list tisax controls: %w", err)
	}
	return filterTISAXByProtectionLevel(controls, protectionLevel), nil
}

// GetTISAXGapAnalysis returns TISAX controls that have not yet reached full maturity (score < 3).
func (s *Service) GetTISAXGapAnalysis(ctx context.Context, orgID, frameworkID string) (*TISAXGapAnalysis, error) {
	controls, err := s.ListControls(ctx, orgID, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("get tisax gap analysis: %w", err)
	}
	return buildTISAXGapAnalysis(frameworkID, controls), nil
}

// tisaxToISO27001Mappings is the static TISAX → ISO 27001 control mapping table.
var tisaxToISO27001Mappings = map[string]string{
	"TISAX-1.1.1": "A.5.1.1", "TISAX-1.1.2": "A.5.1.2", "TISAX-1.1.3": "A.6.1.1",
	"TISAX-2.1.1": "A.6.1.1", "TISAX-2.1.3": "A.6.1.5", "TISAX-2.1.4": "A.6.2.1",
	"TISAX-3.1.1": "A.7.1.1", "TISAX-3.1.2": "A.7.2.2", "TISAX-3.1.3": "A.7.2.3", "TISAX-3.1.4": "A.7.3.1",
	"TISAX-4.1.1": "A.8.1.1", "TISAX-4.1.2": "A.8.1.2", "TISAX-4.1.3": "A.8.2.1", "TISAX-4.1.4": "A.8.2.2", "TISAX-4.1.5": "A.8.3.1",
	"TISAX-5.1.1": "A.9.1.1", "TISAX-5.1.2": "A.9.2.1", "TISAX-5.1.3": "A.9.2.3", "TISAX-5.1.4": "A.9.4.2", "TISAX-5.1.5": "A.9.1.2",
	"TISAX-6.1.1": "A.10.1.1", "TISAX-6.1.2": "A.10.1.2", "TISAX-6.1.3": "A.10.1.1",
	"TISAX-7.1.1": "A.11.1.1", "TISAX-7.1.2": "A.11.1.2", "TISAX-7.1.3": "A.11.2.1", "TISAX-7.1.4": "A.11.2.9",
	"TISAX-8.1.2": "A.12.1.2", "TISAX-8.1.3": "A.12.2.1", "TISAX-8.1.4": "A.12.3.1", "TISAX-8.1.5": "A.12.4.1", "TISAX-8.1.6": "A.12.6.1",
	"TISAX-9.1.1": "A.13.1.1", "TISAX-9.1.2": "A.13.2.1", "TISAX-9.1.3": "A.13.2.4",
	"TISAX-11.1.1": "A.15.1.1", "TISAX-11.1.2": "A.15.1.2", "TISAX-11.1.3": "A.15.2.1",
	"TISAX-12.1.1": "A.16.1.1", "TISAX-12.1.2": "A.16.1.2", "TISAX-12.1.4": "A.16.1.6",
	"TISAX-13.1.1": "A.17.1.1", "TISAX-13.1.2": "A.17.1.3",
	"TISAX-14.1.1": "A.18.1.1", "TISAX-14.1.2": "A.18.2.2",
}

// SeedTISAXMappings idempotently seeds the static TISAX → ISO 27001 mappings into ck_framework_mappings.
// Returns nil if either framework is not yet enabled.
func (s *Service) SeedTISAXMappings(ctx context.Context, orgID string) error {
	tisaxFW, err := s.repo.FindFrameworkByName(ctx, orgID, "TISAX")
	if err != nil {
		return fmt.Errorf("find TISAX framework: %w", err)
	}
	if tisaxFW == nil {
		return nil // TISAX not enabled yet — skip silently
	}

	isoFW, err := s.repo.FindFrameworkByName(ctx, orgID, "ISO27001")
	if err != nil {
		return fmt.Errorf("find ISO27001 framework: %w", err)
	}
	if isoFW == nil {
		return nil // ISO27001 not enabled yet — skip silently
	}

	// Build lookup maps: controlID string → UUID string
	tisaxControls, err := s.repo.ListControls(ctx, orgID, tisaxFW.ID)
	if err != nil {
		return fmt.Errorf("list TISAX controls for seed: %w", err)
	}
	isoControls, err := s.repo.ListControls(ctx, orgID, isoFW.ID)
	if err != nil {
		return fmt.Errorf("list ISO27001 controls for seed: %w", err)
	}

	tisaxByControlID := make(map[string]string, len(tisaxControls))
	for _, c := range tisaxControls {
		tisaxByControlID[c.ControlID] = c.ID
	}
	isoByControlID := make(map[string]string, len(isoControls))
	for _, c := range isoControls {
		isoByControlID[c.ControlID] = c.ID
	}

	for tisaxID, isoID := range tisaxToISO27001Mappings {
		tisaxUUID, ok1 := tisaxByControlID[tisaxID]
		isoUUID, ok2 := isoByControlID[isoID]
		if !ok1 || !ok2 {
			continue // control not found in DB — skip silently
		}
		if _, err := s.repo.CreateMapping(ctx, orgID, tisaxUUID, isoUUID); err != nil {
			log.Warn().Err(err).Str("tisax", tisaxID).Str("iso", isoID).Msg("seed mapping failed")
		}
	}
	return nil
}

// GetTISAXCoverageByISO computes, for each TISAX control, whether the mapped ISO 27001 control is covered.
// A control is covered when its manual_status == "implemented" OR evidence_count >= 1.
func (s *Service) GetTISAXCoverageByISO(ctx context.Context, orgID, tisaxFrameworkID string) ([]MappingResult, error) {
	tisaxControls, err := s.ListControls(ctx, orgID, tisaxFrameworkID)
	if err != nil {
		return nil, fmt.Errorf("list TISAX controls: %w", err)
	}

	// Find ISO27001 framework — if not enabled, return all covered=false.
	isoFW, err := s.repo.FindFrameworkByName(ctx, orgID, "ISO27001")
	if err != nil {
		return nil, fmt.Errorf("find ISO27001 framework: %w", err)
	}

	var isoControls []Control
	var evidenceCounts map[string]int
	if isoFW != nil {
		isoControls, err = s.ListControls(ctx, orgID, isoFW.ID)
		if err != nil {
			return nil, fmt.Errorf("list ISO27001 controls: %w", err)
		}
		evidenceCounts, err = s.repo.CountEvidenceByControl(ctx, orgID, isoFW.ID)
		if err != nil {
			return nil, fmt.Errorf("count ISO27001 evidence: %w", err)
		}
	}

	// Build lookup: ISO control UUID → Control
	isoByUUID := make(map[string]Control, len(isoControls))
	for _, c := range isoControls {
		isoByUUID[c.ID] = c
	}

	// Load mappings for all TISAX control UUIDs.
	tisaxUUIDs := make([]string, 0, len(tisaxControls))
	for _, c := range tisaxControls {
		tisaxUUIDs = append(tisaxUUIDs, c.ID)
	}
	mappings, err := s.repo.GetMappingsBySourceControlIDs(ctx, orgID, tisaxUUIDs)
	if err != nil {
		return nil, fmt.Errorf("get framework mappings: %w", err)
	}

	results := make([]MappingResult, 0, len(tisaxControls))
	for _, tc := range tisaxControls {
		mr := MappingResult{
			TISAXControlID:    tc.ControlID,
			TISAXControlTitle: tc.Title,
		}

		if mapping, hasMapped := mappings[tc.ID]; hasMapped {
			if iso, hasISO := isoByUUID[mapping.TargetControlID]; hasISO {
				mr.ISOControlID = iso.ControlID
				mr.ISOControlTitle = iso.Title
				// Covered if implemented or has evidence.
				mr.Covered = iso.ManualStatus == "implemented" || evidenceCounts[iso.ID] >= 1
			}
		}

		results = append(results, mr)
	}
	return results, nil
}

// GetTISAXGapsAfterISO returns TISAX controls that are NOT covered by the mapped ISO 27001 control.
func (s *Service) GetTISAXGapsAfterISO(ctx context.Context, orgID, tisaxFrameworkID string) ([]Control, error) {
	results, err := s.GetTISAXCoverageByISO(ctx, orgID, tisaxFrameworkID)
	if err != nil {
		return nil, err
	}

	// Load all TISAX controls to return full Control objects.
	allControls, err := s.ListControls(ctx, orgID, tisaxFrameworkID)
	if err != nil {
		return nil, fmt.Errorf("list tisax controls for gap filter: %w", err)
	}
	controlByID := make(map[string]Control, len(allControls))
	for _, c := range allControls {
		controlByID[c.ControlID] = c
	}

	var gaps []Control
	for _, r := range results {
		if !r.Covered {
			if c, ok := controlByID[r.TISAXControlID]; ok {
				gaps = append(gaps, c)
			}
		}
	}
	return gaps, nil
}

// FindFrameworkByName returns a framework by name for an organisation, or nil if not found.
func (s *Service) FindFrameworkByName(ctx context.Context, orgID, name string) (*Framework, error) {
	return s.repo.FindFrameworkByName(ctx, orgID, name)
}

// ListFrameworkMappings returns all framework mappings for an organisation.
func (s *Service) ListFrameworkMappings(ctx context.Context, orgID string) ([]FrameworkMapping, error) {
	return s.repo.ListMappingsByOrg(ctx, orgID)
}

// DeleteFrameworkMapping removes a framework mapping by ID within an organisation.
func (s *Service) DeleteFrameworkMapping(ctx context.Context, orgID, mappingID string) error {
	return s.repo.DeleteMapping(ctx, orgID, mappingID)
}

// GetControl returns a single control by its UUID.
func (s *Service) GetControl(ctx context.Context, orgID, controlID string) (*Control, error) {
	c, err := s.repo.GetControl(ctx, orgID, controlID)
	if err != nil {
		return nil, fmt.Errorf("get control: %w", err)
	}
	if strings.HasPrefix(c.ControlID, "DORA-") {
		if m, ok := doraISO27001Mapping[c.ControlID]; ok {
			c.ISO27001Mapping = m
		}
	}
	return c, nil
}

// --- Evidence ---

// AddEvidence stores a new evidence item for a control.
func (s *Service) AddEvidence(ctx context.Context, orgID, controlID, userID string, input AddEvidenceInput) (*Evidence, error) {
	// Verify the control belongs to this org.
	if _, err := s.repo.GetControl(ctx, orgID, controlID); err != nil {
		return nil, fmt.Errorf("control not found: %w", err)
	}

	ev, err := s.repo.AddEvidence(ctx, orgID, controlID, userID, input)
	if err != nil {
		return nil, fmt.Errorf("add evidence: %w", err)
	}
	return ev, nil
}

// ListEvidence returns all evidence items for a control.
func (s *Service) ListEvidence(ctx context.Context, orgID, controlID string) ([]Evidence, error) {
	return s.repo.ListEvidence(ctx, orgID, controlID)
}

// ReviewEvidence updates the review status of an evidence item.
// status must be one of: "approved", "rejected".
func (s *Service) ReviewEvidence(ctx context.Context, orgID, evidenceID, reviewerID, status, _ string) error {
	if status != "approved" && status != "rejected" {
		return fmt.Errorf("invalid review status: %s (must be approved or rejected)", status)
	}
	return s.repo.ReviewEvidence(ctx, orgID, evidenceID, reviewerID, status)
}

// GetExpiringEvidenceAll returns evidence expiring within the given number of days, across all frameworks.
func (s *Service) GetExpiringEvidenceAll(ctx context.Context, orgID string, days int) ([]Evidence, error) {
	threshold := time.Now().UTC().AddDate(0, 0, days)
	items, err := s.repo.GetExpiringEvidenceAllFrameworks(ctx, orgID, threshold)
	if err != nil {
		return nil, fmt.Errorf("get expiring evidence all: %w", err)
	}
	if items == nil {
		items = []Evidence{}
	}
	return items, nil
}

// CollectEvidence runs the named collector and stores the result as an evidence item.
func (s *Service) CollectEvidence(ctx context.Context, orgID, controlID, userID string, cfg CollectorConfig) (*Evidence, error) {
	collector, err := GetCollector(cfg.Type)
	if err != nil {
		return nil, err
	}

	data, err := collector.Collect(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("collector %s: %w", cfg.Type, err)
	}

	title := fmt.Sprintf("Auto-collected: %s (%s)", cfg.Type, time.Now().UTC().Format(time.DateOnly))
	return s.repo.AddCollectorEvidence(ctx, orgID, controlID, userID, cfg.Type, title, data)
}

// --- Auditor links ---

// CreateAuditorLink generates a time-limited read-only access token for an external auditor.
// Returns the raw (unhashed) token that should be delivered to the auditor.
func (s *Service) CreateAuditorLink(ctx context.Context, orgID, frameworkID, userID string, expiresIn time.Duration) (string, error) {
	rawToken, tokenHash, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate auditor token: %w", err)
	}

	expiresAt := time.Now().UTC().Add(expiresIn)
	_, err = s.repo.CreateAuditorLink(ctx, orgID, frameworkID, userID, tokenHash, expiresAt)
	if err != nil {
		return "", fmt.Errorf("create auditor link: %w", err)
	}

	return rawToken, nil
}

// ValidateAuditorLink looks up an auditor link by its raw token, increments usage,
// and returns the associated framework.
func (s *Service) ValidateAuditorLink(ctx context.Context, rawToken string) (*Framework, error) {
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])

	al, err := s.repo.GetAuditorLinkByHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("invalid auditor link")
	}

	if time.Now().UTC().After(al.ExpiresAt) {
		return nil, fmt.Errorf("auditor link expired")
	}

	if err := s.repo.IncrementAuditorLinkUsage(ctx, al.ID); err != nil {
		log.Warn().Err(err).Str("link_id", al.ID).Msg("failed to increment auditor link usage")
	}

	return s.repo.GetFramework(ctx, al.OrgID, al.FrameworkID)
}

// validateAuditorToken resolves a raw token to an AuditorLink, enforcing expiry and revocation.
// Returns the internal AuditorLink (not exposed to callers directly).
func (s *Service) validateAuditorToken(ctx context.Context, rawToken string) (*AuditorLink, error) {
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])

	al, err := s.repo.GetAuditorLinkByHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("invalid auditor link")
	}
	if time.Now().UTC().After(al.ExpiresAt) {
		return nil, fmt.Errorf("auditor link expired")
	}
	// Update access tracking (best-effort).
	if err := s.repo.UpdateAuditorLinkAccess(ctx, al.ID); err != nil {
		log.Warn().Err(err).Str("link_id", al.ID).Msg("failed to update auditor link access")
	}
	return al, nil
}

// PreflightAuditorExport validates a token and returns the framework name without
// incrementing the access counter. Used by the handler to set Content-Disposition
// before streaming the ZIP body (ExportAuditorBundle increments on its own call).
func (s *Service) PreflightAuditorExport(ctx context.Context, rawToken string) (string, error) {
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])

	al, err := s.repo.GetAuditorLinkByHash(ctx, tokenHash)
	if err != nil {
		return "", fmt.Errorf("invalid auditor link")
	}
	if time.Now().UTC().After(al.ExpiresAt) {
		return "", fmt.Errorf("auditor link expired")
	}

	fw, err := s.repo.GetFramework(ctx, al.OrgID, al.FrameworkID)
	if err != nil {
		return "", fmt.Errorf("get framework: %w", err)
	}
	return fw.Name, nil
}

// ListAuditorLinks returns all auditor links for the given organisation.
func (s *Service) ListAuditorLinks(ctx context.Context, orgID string) ([]AuditorLinkListItem, error) {
	links, err := s.repo.ListAuditorLinks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list auditor links: %w", err)
	}
	return links, nil
}

// RevokeAuditorLink marks an auditor link as revoked so it can no longer be used.
func (s *Service) RevokeAuditorLink(ctx context.Context, orgID, linkID string) error {
	if err := s.repo.RevokeAuditorLink(ctx, orgID, linkID); err != nil {
		return fmt.Errorf("revoke auditor link: %w", err)
	}
	return nil
}

// AuditorViewDetailed validates the token and returns the framework, readiness report,
// and each control with its evidence items — for the enhanced auditor portal (E09.2).
func (s *Service) AuditorViewDetailed(ctx context.Context, rawToken string) (*AuditorDetailView, error) {
	al, err := s.validateAuditorToken(ctx, rawToken)
	if err != nil {
		return nil, err
	}

	fw, err := s.repo.GetFramework(ctx, al.OrgID, al.FrameworkID)
	if err != nil {
		return nil, fmt.Errorf("get framework: %w", err)
	}

	controls, err := s.repo.ListControls(ctx, al.OrgID, al.FrameworkID)
	if err != nil {
		return nil, fmt.Errorf("list controls: %w", err)
	}

	evidenceCounts, err := s.repo.CountEvidenceByControl(ctx, al.OrgID, al.FrameworkID)
	if err != nil {
		return nil, fmt.Errorf("count evidence: %w", err)
	}

	report := computeReadinessReport(fw, controls, evidenceCounts)

	// Collect all control IDs for a single batch query instead of N per-control queries.
	controlIDs := make([]string, len(controls))
	for i, c := range controls {
		controlIDs[i] = c.ID
	}
	evidenceByControl, err := s.repo.ListEvidenceByControls(ctx, al.OrgID, controlIDs)
	if err != nil {
		return nil, fmt.Errorf("list evidence batch: %w", err)
	}

	withEvidence := make([]ControlWithEvidence, 0, len(controls))
	for i := range controls {
		c := controls[i]
		c.EvidenceCount = evidenceCounts[c.ID]
		c.Status = resolveStatus(c)

		items := evidenceByControl[c.ID]
		if items == nil {
			items = []Evidence{}
		}
		withEvidence = append(withEvidence, ControlWithEvidence{
			Control:  c,
			Evidence: items,
		})
	}

	return &AuditorDetailView{
		Framework: *fw,
		Report:    report,
		Controls:  withEvidence,
	}, nil
}

// ExportAuditorBundle validates the token and writes a ZIP to w with structure:
//
//	<framework_name>/
//	  <domain>/
//	    <control_code>/
//	      evidence_metadata.json
func (s *Service) ExportAuditorBundle(ctx context.Context, rawToken string, w io.Writer) (string, error) {
	al, err := s.validateAuditorToken(ctx, rawToken)
	if err != nil {
		return "", err
	}

	fw, err := s.repo.GetFramework(ctx, al.OrgID, al.FrameworkID)
	if err != nil {
		return "", fmt.Errorf("get framework: %w", err)
	}

	controls, err := s.repo.ListControls(ctx, al.OrgID, al.FrameworkID)
	if err != nil {
		return "", fmt.Errorf("list controls: %w", err)
	}

	// Batch-load all evidence in one query before writing the ZIP.
	controlIDs := make([]string, len(controls))
	for i, c := range controls {
		controlIDs[i] = c.ID
	}
	evidenceByControl, err := s.repo.ListEvidenceByControls(ctx, al.OrgID, controlIDs)
	if err != nil {
		return "", fmt.Errorf("list evidence batch: %w", err)
	}

	zw := zip.NewWriter(w)
	defer func() { _ = zw.Close() }()

	for i := range controls {
		c := controls[i]
		items := evidenceByControl[c.ID]
		if items == nil {
			items = []Evidence{}
		}

		path := fmt.Sprintf("%s/%s/%s/evidence_metadata.json", fw.Name, c.Domain, c.ControlID)
		f, err := zw.Create(path)
		if err != nil {
			return "", fmt.Errorf("create zip entry %s: %w", path, err)
		}
		meta := EvidenceMetadata{Control: c, Evidence: items}
		if err := json.NewEncoder(f).Encode(meta); err != nil {
			return "", fmt.Errorf("encode metadata for %s: %w", c.ControlID, err)
		}
	}

	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("close zip: %w", err)
	}

	return fw.Name, nil
}

// --- Internal helpers ---

// computeReadinessReport calculates readiness metrics given controls and evidence counts.
func computeReadinessReport(fw *Framework, controls []Control, evidenceCounts map[string]int) *ReadinessReport {
	report := &ReadinessReport{
		FrameworkID:   fw.ID,
		FrameworkName: fw.Name,
		TotalControls: len(controls),
	}

	// Per-domain tracking.
	domainTotal := make(map[string]int)
	domainCovered := make(map[string]int)

	for _, c := range controls {
		count := evidenceCounts[c.ID]
		domainTotal[c.Domain]++

		switch {
		case count >= 2:
			report.Covered++
			domainCovered[c.Domain]++
		case count == 1:
			report.Partial++
			domainCovered[c.Domain]++ // partial counts as half for domain score
		default:
			report.Missing++
		}
	}

	// Overall readiness score.
	if report.TotalControls > 0 {
		report.ReadinessScore = readinessScore(report.Covered, report.Partial, report.TotalControls)
	}

	// Per-domain scores.
	for domain, total := range domainTotal {
		if total == 0 {
			continue
		}
		covered := domainCovered[domain]
		score := readinessScore(covered, 0, total)
		report.ByDomain = append(report.ByDomain, DomainScore{
			Domain:  domain,
			Score:   score,
			Total:   total,
			Covered: covered,
		})
	}

	return report
}

// readinessScore calculates a 0–100 readiness score.
// Covered controls count fully; partial controls count as half-weight.
func readinessScore(covered, partial, total int) float64 {
	if total == 0 {
		return 0
	}
	weighted := float64(covered) + float64(partial)*0.5
	return (weighted / float64(total)) * 100
}

// resolveStatus determines the effective status of a control.
// Priority: not_applicable > manual_status > computed from evidence.
func resolveStatus(c Control) string {
	if c.NotApplicable {
		return "not_applicable"
	}
	if c.ManualStatus != "" {
		return c.ManualStatus
	}
	return controlStatus(c.EvidenceCount)
}

// controlStatus returns a computed coverage label for a control.
func controlStatus(evidenceCount int) string {
	switch {
	case evidenceCount >= 2:
		return "covered"
	case evidenceCount == 1:
		return "partial"
	default:
		return "missing"
	}
}

// generateToken creates a cryptographically random 32-byte hex token.
func generateToken() (rawToken, tokenHash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate token bytes: %w", err)
	}
	rawToken = hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash = hex.EncodeToString(sum[:])
	return rawToken, tokenHash, nil
}

// --- Risk Assessment (FR-CK12) ---

func (s *Service) ListRisks(ctx context.Context, orgID string) ([]Risk, error) {
	risks, err := s.repo.ListRisks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list risks: %w", err)
	}
	if risks == nil {
		risks = []Risk{}
	}
	return risks, nil
}

func (s *Service) GetRisk(ctx context.Context, orgID, id string) (*Risk, error) {
	return s.repo.GetRisk(ctx, orgID, id)
}

func (s *Service) CreateRisk(ctx context.Context, orgID string, in CreateRiskInput) (*Risk, error) {
	risk, err := s.repo.CreateRisk(ctx, orgID, in)
	if err != nil {
		return nil, err
	}
	s.invalidateDashboardCache(ctx, orgID)
	return risk, nil
}

func (s *Service) UpdateRisk(ctx context.Context, orgID, id string, in UpdateRiskInput) (*Risk, error) {
	risk, err := s.repo.UpdateRisk(ctx, orgID, id, in)
	if err != nil {
		return nil, err
	}
	s.invalidateDashboardCache(ctx, orgID)
	return risk, nil
}

// UpdateRiskTreatment patches the treatment workflow fields of a risk (ISO 27001 Clause 6).
func (s *Service) UpdateRiskTreatment(ctx context.Context, orgID, id string, in UpdateRiskTreatmentInput) (*Risk, error) {
	return s.repo.UpdateRiskTreatment(ctx, orgID, id, in)
}

// --- Risk ↔ Control Links ---

func (s *Service) LinkRiskControl(ctx context.Context, orgID, riskID, controlID string) error {
	return s.repo.LinkRiskControl(ctx, orgID, riskID, controlID)
}

func (s *Service) UnlinkRiskControl(ctx context.Context, orgID, riskID, controlID string) error {
	return s.repo.UnlinkRiskControl(ctx, orgID, riskID, controlID)
}

func (s *Service) ListRiskControls(ctx context.Context, orgID, riskID string) ([]Control, error) {
	controls, err := s.repo.ListRiskControls(ctx, orgID, riskID)
	if err != nil {
		return nil, fmt.Errorf("list risk controls: %w", err)
	}
	if controls == nil {
		controls = []Control{}
	}
	return controls, nil
}

// --- Incident Register (FR-CK13) ---

func (s *Service) ListIncidents(ctx context.Context, orgID string) ([]Incident, error) {
	incidents, err := s.repo.ListIncidents(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	if incidents == nil {
		incidents = []Incident{}
	}
	for i := range incidents {
		incidents[i].DeadlineStatus = computeDeadlineStatus(&incidents[i])
	}
	return incidents, nil
}

func (s *Service) GetIncident(ctx context.Context, orgID, id string) (*Incident, error) {
	inc, err := s.repo.GetIncident(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	inc.DeadlineStatus = computeDeadlineStatus(inc)
	return inc, nil
}

func (s *Service) CreateIncident(ctx context.Context, orgID string, in CreateIncidentInput) (*Incident, error) {
	if in.AffectedSystems == nil {
		in.AffectedSystems = []string{}
	}
	deadlines := computeDeadlines(in.IncidentType, in.DiscoveredAt)
	inc, err := s.repo.CreateIncident(ctx, orgID, in, deadlines)
	if err != nil {
		return nil, err
	}
	inc.DeadlineStatus = computeDeadlineStatus(inc)
	return inc, nil
}

func (s *Service) UpdateIncident(ctx context.Context, orgID, id string, in UpdateIncidentInput) (*Incident, error) {
	if in.AffectedSystems == nil {
		in.AffectedSystems = []string{}
	}
	inc, err := s.repo.UpdateIncident(ctx, orgID, id, in)
	if err != nil {
		return nil, err
	}
	inc.DeadlineStatus = computeDeadlineStatus(inc)
	return inc, nil
}

func (s *Service) MarkDeadlineReported(ctx context.Context, orgID, id, deadline string) (*Incident, error) {
	inc, err := s.repo.MarkDeadlineReported(ctx, orgID, id, deadline)
	if err != nil {
		return nil, err
	}
	inc.DeadlineStatus = computeDeadlineStatus(inc)
	return inc, nil
}

// AssessReportability evaluates NIS2 meldepflicht based on a short questionnaire,
// persists the answers, and updates reporting_obligation + notification_authority.
func (s *Service) AssessReportability(ctx context.Context, orgID, incidentID string, in AssessReportabilityInput) (*ReportabilityResult, error) {
	var obligation, explanation string
	switch {
	case in.AffectsEssentialService:
		obligation = "required"
		explanation = "Essenzieller Dienst betroffen — NIS2-Meldepflicht wahrscheinlich (§ 32 BSIG-neu)."
	case in.AffectsExternalData:
		obligation = "unknown"
		explanation = "Externe Kundendaten betroffen, aber kein essenzieller Dienst identifiziert — bitte rechtlich prüfen."
	default:
		obligation = "not_required"
		explanation = "Keine Hinweise auf NIS2-Meldepflicht nach aktuellem Bewertungsstand."
	}

	authority := s.primaryAuthorityForOrg(ctx, orgID)

	answersJSON, err := json.Marshal(in.ReportabilityAnswers)
	if err != nil {
		return nil, fmt.Errorf("marshal reportability answers: %w", err)
	}
	if err := s.repo.UpdateIncidentReportability(ctx, orgID, incidentID, obligation, authority, in.PersonalDataCompromised, answersJSON); err != nil {
		return nil, err
	}
	return &ReportabilityResult{
		Obligation:            obligation,
		GDPRRequired:         in.PersonalDataCompromised,
		NotificationAuthority: authority,
		Explanation:          explanation,
		Answers:              in.ReportabilityAnswers,
	}, nil
}

// CheckOverdueDeadlines iterates all DORA/NIS2 incidents for the given org and
// sends in-app and e-mail notifications for overdue or soon-due deadlines.
// The 12h-before warning is guarded by notified_warn_* flags to prevent repeats.
// It is called by the secvitals:incident_deadline_check cron job.
func (s *Service) CheckOverdueDeadlines(ctx context.Context, orgID string) error {
	now := time.Now().UTC()

	// Fetch admin e-mails once per org run (non-fatal if lookup fails).
	adminEmails, _ := s.repo.GetAdminEmails(ctx, orgID)

	// sendEmail delivers an e-mail to all admins (non-fatal).
	sendEmail := func(subject, body string) {
		if s.notifSvc == nil {
			return
		}
		for _, email := range adminEmails {
			if err := s.notifSvc.Notify(ctx, notify.Message{
				Title:   subject,
				Body:    body,
				OrgID:   orgID,
				Channel: notify.ChannelEmail,
				Target:  email,
			}); err != nil {
				log.Warn().Err(err).Str("to", email).Msg("deadline_check: email send failed")
			}
		}
	}

	// Check both DORA and NIS2 incident types.
	for _, incType := range []string{"dora", "nis2"} {
		incidents, err := s.repo.ListIncidentsByType(ctx, orgID, incType)
		if err != nil {
			return fmt.Errorf("list %s incidents: %w", incType, err)
		}

		type deadlinePair struct {
			deadline    *time.Time
			reportedAt  *time.Time
			label       string
			warnAlready bool // true if 12h warning already sent
		}

		for i := range incidents {
			inc := &incidents[i]
			pairs := []deadlinePair{
				{inc.Deadline24h, inc.Reported24hAt, "24h", inc.NotifiedWarn24h},
				{inc.Deadline72h, inc.Reported72hAt, "72h", inc.NotifiedWarn72h},
				{inc.Deadline30d, inc.Reported30dAt, "30d", inc.NotifiedWarn30d},
			}
			for _, p := range pairs {
				if p.deadline == nil || p.reportedAt != nil {
					continue
				}
				hoursLeft := p.deadline.Sub(now).Hours()
				if now.After(*p.deadline) {
					// Overdue — in-app notification (sent every cron run until reported).
					var notifTitle, notifType string
					switch incType {
					case "nis2":
						notifTitle = fmt.Sprintf("NIS2-Meldefrist überschritten: %s", inc.Title)
						notifType = "nis2_deadline_overdue"
					default:
						notifTitle = fmt.Sprintf("DORA-Meldefrist überschritten: %s", inc.Title)
						notifType = "dora_deadline_overdue"
					}
					body := fmt.Sprintf(
						"Die %s-Meldefrist für den Vorfall \"%s\" wurde überschritten und ist noch nicht als gemeldet markiert.",
						p.label, inc.Title,
					)
					notify.Send(ctx, s.db, orgID, notifTitle, body, notifType, "secvitals")
					emailSubj := fmt.Sprintf("[Vakt Comply] %s", notifTitle)
					sendEmail(emailSubj, body)
					log.Warn().Str("org_id", orgID).Str("incident_id", inc.ID).Str("deadline", p.label).
						Msg("incident_deadline_check: overdue notification sent")
				} else if hoursLeft <= 12 && !p.warnAlready {
					// 12h-before warning — sent exactly once (guarded by notified_warn_* flag).
					var notifTitle, notifType string
					switch incType {
					case "nis2":
						notifTitle = fmt.Sprintf("NIS2-Meldefrist in %.0fh: %s", hoursLeft, inc.Title)
						notifType = "nis2_deadline_warning"
					default:
						notifTitle = fmt.Sprintf("DORA-Meldefrist in %.0fh: %s", hoursLeft, inc.Title)
						notifType = "dora_deadline_warning"
					}
					body := fmt.Sprintf(
						"Die %s-Meldefrist für den Vorfall \"%s\" läuft in %.0f Stunden ab.",
						p.label, inc.Title, hoursLeft,
					)
					notify.Send(ctx, s.db, orgID, notifTitle, body, notifType, "secvitals")
					emailSubj := fmt.Sprintf("[Vakt Comply] %s", notifTitle)
					sendEmail(emailSubj, body)
					// Mark as notified so this warning isn't repeated.
					if err := s.repo.MarkIncidentWarnNotified(ctx, orgID, inc.ID, p.label); err != nil {
						log.Warn().Err(err).Str("incident_id", inc.ID).Str("deadline", p.label).
							Msg("incident_deadline_check: failed to mark warn notified")
					}
					log.Info().Str("org_id", orgID).Str("incident_id", inc.ID).Str("deadline", p.label).
						Msg("incident_deadline_check: 12h warning sent")
				}
			}
		}
	}
	return nil
}

// GenerateIncidentReportForm generates a NIS2 Meldungsformular PDF and saves it
// in the ck_incident_reports archive. Returns the archived report and raw PDF bytes.
func (s *Service) GenerateIncidentReportForm(ctx context.Context, orgID, incidentID, reportType, orgName string) (*IncidentReport, []byte, error) {
	inc, err := s.repo.GetIncident(ctx, orgID, incidentID)
	if err != nil {
		return nil, nil, err
	}
	if reportType != "24h" && reportType != "72h" && reportType != "30d" {
		return nil, nil, fmt.Errorf("invalid report_type: %s", reportType)
	}

	pdfBytes, err := GenerateNIS2ReportFormPDF(inc, reportType, orgName)
	if err != nil {
		return nil, nil, fmt.Errorf("generate nis2 report form pdf: %w", err)
	}

	authority := inc.NotificationAuthority
	if authority == "" {
		authority = "BSI"
	}

	meta, _ := json.Marshal(map[string]string{
		"incident_title": inc.Title,
		"report_type":    reportType,
		"authority":      authority,
	})

	report, err := s.repo.SaveIncidentReport(ctx, orgID, incidentID, reportType, authority, pdfBytes, meta)
	if err != nil {
		return nil, nil, err
	}
	return report, pdfBytes, nil
}

// ListIncidentReports returns all archived Meldungsformulare for an incident.
func (s *Service) ListIncidentReports(ctx context.Context, orgID, incidentID string) ([]IncidentReport, error) {
	return s.repo.ListIncidentReports(ctx, orgID, incidentID)
}

// GetIncidentReportPDF returns the stored PDF bytes for a specific report.
func (s *Service) GetIncidentReportPDF(ctx context.Context, orgID, reportID string) ([]byte, error) {
	return s.repo.GetIncidentReportPDF(ctx, orgID, reportID)
}

// GetAuthorityInfo returns submission channel info for a given authority key.
func GetAuthorityInfo(authority string) (AuthorityInfo, bool) {
	info, ok := incidentAuthorityDirectory[authority]
	return info, ok
}

// GetOrgSector returns the sector and federal state configured for the org.
func (s *Service) GetOrgSector(ctx context.Context, orgID string) (*OrgSectorSettings, error) {
	return s.repo.GetOrgSector(ctx, orgID)
}

// UpdateOrgSector sets the org's sector and federal state.
func (s *Service) UpdateOrgSector(ctx context.Context, orgID string, in UpdateOrgSectorInput) (*OrgSectorSettings, error) {
	if err := s.repo.UpdateOrgSector(ctx, orgID, in.Sector, in.FederalState); err != nil {
		return nil, err
	}
	return s.repo.GetOrgSector(ctx, orgID)
}

// GetAuthoritiesForOrg returns the relevant NIS2 authorities for the org's configured sector.
func (s *Service) GetAuthoritiesForOrg(ctx context.Context, orgID string) ([]AuthorityInfo, error) {
	settings, err := s.repo.GetOrgSector(ctx, orgID)
	if err != nil {
		// Fallback to BSI if org lookup fails.
		return []AuthorityInfo{incidentAuthorityDirectory["BSI"]}, nil
	}
	keys, ok := sectorAuthorityMap[settings.Sector]
	if !ok {
		keys = []string{"BSI"}
	}
	var infos []AuthorityInfo
	for _, k := range keys {
		if info, exists := incidentAuthorityDirectory[k]; exists {
			infos = append(infos, info)
		}
	}
	return infos, nil
}

// ListAllAuthorities returns all known reporting authorities.
func ListAllAuthorities() []AuthorityInfo {
	order := []string{"BSI", "BaFin", "BNetzA", "LBA"}
	var all []AuthorityInfo
	for _, k := range order {
		if info, ok := incidentAuthorityDirectory[k]; ok {
			all = append(all, info)
		}
	}
	return all
}

// primaryAuthorityForOrg returns the first authority for the org's sector (used in reportability assessment).
func (s *Service) primaryAuthorityForOrg(ctx context.Context, orgID string) string {
	settings, err := s.repo.GetOrgSector(ctx, orgID)
	if err != nil {
		return "BSI"
	}
	keys, ok := sectorAuthorityMap[settings.Sector]
	if !ok || len(keys) == 0 {
		return "BSI"
	}
	return keys[0]
}

// computeDeadlines calculates absolute deadline timestamps for NIS2 and DORA incident types.
func computeDeadlines(incidentType string, discoveredAt time.Time) map[string]*time.Time {
	result := map[string]*time.Time{"4h": nil, "24h": nil, "72h": nil, "30d": nil}
	switch incidentType {
	case "dora":
		t4h := discoveredAt.Add(4 * time.Hour)
		t24h := discoveredAt.Add(24 * time.Hour)
		t72h := discoveredAt.Add(72 * time.Hour)
		t30d := discoveredAt.AddDate(0, 0, 30)
		result["4h"] = &t4h
		result["24h"] = &t24h
		result["72h"] = &t72h
		result["30d"] = &t30d
	case "nis2":
		t24h := discoveredAt.Add(24 * time.Hour)
		t72h := discoveredAt.Add(72 * time.Hour)
		t30d := discoveredAt.AddDate(0, 0, 30)
		result["24h"] = &t24h
		result["72h"] = &t72h
		result["30d"] = &t30d
	}
	return result
}

// computeDeadlineStatus builds the computed deadline status for a given incident.
func computeDeadlineStatus(inc *Incident) *IncidentDeadlineStatus {
	if inc.Deadline4h == nil && inc.Deadline24h == nil && inc.Deadline72h == nil && inc.Deadline30d == nil {
		return nil
	}
	now := time.Now().UTC()
	status := &IncidentDeadlineStatus{
		Has4h:  inc.Deadline4h != nil,
		Has24h: inc.Deadline24h != nil,
		Has72h: inc.Deadline72h != nil,
		Has30d: inc.Deadline30d != nil,
	}
	if inc.Deadline4h != nil {
		status.D4h = deadlineInfo(inc.Deadline4h, inc.Reported4hAt, now)
	}
	if inc.Deadline24h != nil {
		status.D24h = deadlineInfo(inc.Deadline24h, inc.Reported24hAt, now)
	}
	if inc.Deadline72h != nil {
		status.D72h = deadlineInfo(inc.Deadline72h, inc.Reported72hAt, now)
	}
	if inc.Deadline30d != nil {
		status.D30d = deadlineInfo(inc.Deadline30d, inc.Reported30dAt, now)
	}
	return status
}

func deadlineInfo(deadline, reportedAt *time.Time, now time.Time) *DeadlineInfo {
	info := &DeadlineInfo{
		Deadline:   deadline,
		ReportedAt: reportedAt,
		HoursLeft:  deadline.Sub(now).Hours(),
	}
	if reportedAt != nil {
		info.Status = "done"
	} else if now.After(*deadline) {
		info.Status = "red"
	} else if info.HoursLeft <= 6 {
		info.Status = "yellow"
	} else {
		info.Status = "green"
	}
	return info
}

// --- Supplier Register ---

// computeContractStatus returns "expired", "expiring_soon", or "active" based on contractEnd.
func computeContractStatus(contractEnd *time.Time, now time.Time) string {
	if contractEnd == nil {
		return "active"
	}
	if contractEnd.Before(now) {
		return "expired"
	}
	if contractEnd.Before(now.Add(30 * 24 * time.Hour)) {
		return "expiring_soon"
	}
	return "active"
}

func (s *Service) ListSuppliers(ctx context.Context, orgID string, filter *SupplierFilter) ([]Supplier, error) {
	suppliers, err := s.repo.ListSuppliers(ctx, orgID, filter)
	if err != nil {
		return nil, err
	}
	if suppliers == nil {
		suppliers = []Supplier{}
	}
	now := time.Now().UTC()
	for i := range suppliers {
		suppliers[i].ContractStatus = computeContractStatus(suppliers[i].ContractEnd, now)
	}
	return suppliers, nil
}

func (s *Service) GetSupplier(ctx context.Context, orgID, id string) (*Supplier, error) {
	supplier, err := s.repo.GetSupplier(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	supplier.ContractStatus = computeContractStatus(supplier.ContractEnd, time.Now().UTC())
	return supplier, nil
}

func (s *Service) CreateSupplier(ctx context.Context, orgID string, in CreateSupplierInput) (*Supplier, error) {
	supplier, err := s.repo.CreateSupplier(ctx, orgID, in)
	if err != nil {
		return nil, err
	}
	supplier.ContractStatus = computeContractStatus(supplier.ContractEnd, time.Now().UTC())
	return supplier, nil
}

func (s *Service) UpdateSupplier(ctx context.Context, orgID, id string, in UpdateSupplierInput) (*Supplier, error) {
	supplier, err := s.repo.UpdateSupplier(ctx, orgID, id, in)
	if err != nil {
		return nil, err
	}
	supplier.ContractStatus = computeContractStatus(supplier.ContractEnd, time.Now().UTC())
	return supplier, nil
}

func (s *Service) DeleteSupplier(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteSupplier(ctx, orgID, id)
}

// ListIncidentsBySupplier returns all incidents linked to a given supplier.
func (s *Service) ListIncidentsBySupplier(ctx context.Context, orgID, supplierID string) ([]Incident, error) {
	incidents, err := s.repo.ListIncidentsBySupplier(ctx, orgID, supplierID)
	if err != nil {
		return nil, err
	}
	if incidents == nil {
		incidents = []Incident{}
	}
	return incidents, nil
}

// LinkSupplierRisk links a risk to a supplier.
func (s *Service) LinkSupplierRisk(ctx context.Context, orgID, supplierID, riskID string) error {
	return s.repo.LinkSupplierRisk(ctx, orgID, supplierID, riskID)
}

// UnlinkSupplierRisk removes a risk link from a supplier.
func (s *Service) UnlinkSupplierRisk(ctx context.Context, orgID, supplierID, riskID string) error {
	return s.repo.UnlinkSupplierRisk(ctx, orgID, supplierID, riskID)
}

// ListSupplierRisks returns all risks linked to the given supplier.
func (s *Service) ListSupplierRisks(ctx context.Context, orgID, supplierID string) ([]Risk, error) {
	risks, err := s.repo.ListSupplierRisks(ctx, orgID, supplierID)
	if err != nil {
		return nil, err
	}
	if risks == nil {
		risks = []Risk{}
	}
	return risks, nil
}

// supplierCSVRow holds a parsed (but not yet saved) supplier row from a CSV import.
type supplierCSVRow struct {
	Name         string
	ContactName  string
	ContactEmail string
	ServiceType  string
	Criticality  string
	NIS2Relevant bool
	DORARelevant bool
}

// parseSupplierCSVRows parses a CSV string and returns valid rows.
// Rows with missing name or invalid criticality are silently skipped (for test use).
func parseSupplierCSVRows(content string) ([]supplierCSVRow, error) {
	reader := csv.NewReader(strings.NewReader(content))
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv header: %w", err)
	}
	colIdx := make(map[string]int, len(header))
	for i, col := range header {
		colIdx[strings.TrimSpace(col)] = i
	}

	validCriticalities := map[string]bool{"low": true, "medium": true, "high": true, "critical": true, "standard": true, "important": true}

	parseBool := func(v string) bool {
		return strings.EqualFold(v, "true") || v == "1"
	}

	getCol := func(record []string, name string) string {
		idx, ok := colIdx[name]
		if !ok || idx >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[idx])
	}

	rows := []supplierCSVRow{}
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			continue
		}
		name := getCol(record, "name")
		if name == "" {
			continue
		}
		crit := getCol(record, "criticality")
		if crit != "" && !validCriticalities[crit] {
			continue
		}
		rows = append(rows, supplierCSVRow{
			Name:         name,
			ContactName:  getCol(record, "contact_name"),
			ContactEmail: getCol(record, "contact_email"),
			ServiceType:  getCol(record, "service_type"),
			Criticality:  crit,
			NIS2Relevant: parseBool(getCol(record, "nis2_relevant")),
			DORARelevant: parseBool(getCol(record, "dora_relevant")),
		})
	}
	return rows, nil
}

// ParseAndImportSupplierCSV reads a CSV stream and imports valid rows as suppliers.
// Expected header: name,contact_name,contact_email,service_type,criticality,nis2_relevant,dora_relevant
func (s *Service) ParseAndImportSupplierCSV(ctx context.Context, orgID string, r io.Reader) (*CSVImportResult, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv header: %w", err)
	}

	// Build column index map.
	colIdx := make(map[string]int, len(header))
	for i, col := range header {
		colIdx[strings.TrimSpace(col)] = i
	}

	validCriticalities := map[string]bool{"low": true, "medium": true, "high": true, "critical": true, "standard": true, "important": true}

	result := &CSVImportResult{
		Errors: []CSVImportError{},
	}

	rowNum := 1 // header is row 0
	for {
		rowNum++
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			result.Skipped++
			result.Errors = append(result.Errors, CSVImportError{Row: rowNum, Message: fmt.Sprintf("read error: %s", err.Error())})
			continue
		}

		getCol := func(name string) string {
			idx, ok := colIdx[name]
			if !ok || idx >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[idx])
		}

		name := getCol("name")
		if name == "" {
			result.Skipped++
			result.Errors = append(result.Errors, CSVImportError{Row: rowNum, Message: "required field 'name' is empty"})
			continue
		}

		criticality := getCol("criticality")
		if criticality != "" && !validCriticalities[criticality] {
			result.Skipped++
			result.Errors = append(result.Errors, CSVImportError{Row: rowNum, Message: fmt.Sprintf("invalid criticality %q: must be one of standard, important, critical", criticality)})
			continue
		}

		parseBool := func(v string) bool {
			return strings.EqualFold(v, "true") || v == "1"
		}

		in := CreateSupplierInput{
			Name:         name,
			ContactName:  getCol("contact_name"),
			ContactEmail: getCol("contact_email"),
			ServiceType:  getCol("service_type"),
			Criticality:  criticality,
			NIS2Relevant: parseBool(getCol("nis2_relevant")),
			DORARelevant: parseBool(getCol("dora_relevant")),
		}

		if _, err := s.CreateSupplier(ctx, orgID, in); err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, CSVImportError{Row: rowNum, Message: fmt.Sprintf("create failed: %s", err.Error())})
			continue
		}
		result.Imported++
	}

	return result, nil
}

// GenerateSupplierCSV generates a CSV export of suppliers.
// sub_suppliers are encoded as semicolon-separated values in one cell.
func GenerateSupplierCSV(suppliers []Supplier) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	header := []string{
		"id", "name", "contact_name", "contact_email", "service_type",
		"criticality", "dora_relevant", "nis2_relevant",
		"contract_end", "contract_status", "data_location",
		"exit_strategy_exists", "sub_suppliers", "notes",
		"assessment_status", "last_assessment_at",
	}
	if err := w.Write(header); err != nil {
		return nil, fmt.Errorf("csv write header: %w", err)
	}

	for _, s := range suppliers {
		contractEnd := ""
		if s.ContractEnd != nil {
			contractEnd = s.ContractEnd.Format(time.RFC3339)
		}
		lastAssessmentAt := ""
		if s.LastAssessmentAt != nil {
			lastAssessmentAt = s.LastAssessmentAt.Format(time.RFC3339)
		}
		subSuppliers := strings.Join(s.SubSuppliers, ";")
		row := []string{
			s.ID,
			s.Name,
			s.ContactName,
			s.ContactEmail,
			s.ServiceType,
			s.Criticality,
			strconv.FormatBool(s.DORARelevant),
			strconv.FormatBool(s.NIS2Relevant),
			contractEnd,
			s.ContractStatus,
			s.DataLocation,
			strconv.FormatBool(s.ExitStrategyExists),
			subSuppliers,
			s.Notes,
			s.AssessmentStatus,
			lastAssessmentAt,
		}
		if err := w.Write(row); err != nil {
			return nil, fmt.Errorf("csv write row: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("csv flush: %w", err)
	}
	return buf.Bytes(), nil
}

// --- AI System Inventory ---

func (s *Service) ListAISystems(ctx context.Context, orgID string, filters AISystemFilters) ([]AISystem, error) {
	systems, err := s.repo.ListAISystems(ctx, orgID, filters)
	if err != nil {
		return nil, err
	}
	if systems == nil {
		systems = []AISystem{}
	}
	return systems, nil
}

func (s *Service) GetAISystem(ctx context.Context, orgID, id string) (*AISystem, error) {
	return s.repo.GetAISystem(ctx, orgID, id)
}

func (s *Service) CreateAISystem(ctx context.Context, orgID string, in CreateAISystemInput) (*AISystem, error) {
	return s.repo.CreateAISystem(ctx, orgID, in)
}

func (s *Service) UpdateAISystem(ctx context.Context, orgID, id string, in UpdateAISystemInput) (*AISystem, error) {
	return s.repo.UpdateAISystem(ctx, orgID, id, in)
}

func (s *Service) DeleteAISystem(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteAISystem(ctx, orgID, id)
}

// --- Policy Management (FR-CK14) ---

func (s *Service) ListPolicies(ctx context.Context, orgID string) ([]Policy, error) {
	policies, err := s.repo.ListPolicies(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list policies: %w", err)
	}
	if policies == nil {
		policies = []Policy{}
	}
	return policies, nil
}

func (s *Service) GetPolicy(ctx context.Context, orgID, id string) (*Policy, error) {
	return s.repo.GetPolicy(ctx, orgID, id)
}

func (s *Service) CreatePolicy(ctx context.Context, orgID string, in CreatePolicyInput) (*Policy, error) {
	return s.repo.CreatePolicy(ctx, orgID, in)
}

func (s *Service) UpdatePolicy(ctx context.Context, orgID, id string, in UpdatePolicyInput) (*Policy, error) {
	return s.repo.UpdatePolicy(ctx, orgID, id, in)
}

// ListPolicyVersions returns all historical version snapshots for a policy (Migration 076).
func (s *Service) ListPolicyVersions(ctx context.Context, orgID, policyID string) ([]PolicyVersion, error) {
	return s.repo.ListPolicyVersions(ctx, orgID, policyID)
}

// GetPolicyVersion returns a single historical version snapshot (Migration 076).
func (s *Service) GetPolicyVersion(ctx context.Context, orgID, policyID string, version int) (PolicyVersion, error) {
	return s.repo.GetPolicyVersion(ctx, orgID, policyID, version)
}

// --- Internal Audit Records (FR-CK15) ---

func (s *Service) ListAuditRecords(ctx context.Context, orgID string) ([]AuditRecord, error) {
	records, err := s.repo.ListAuditRecords(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list audit records: %w", err)
	}
	if records == nil {
		records = []AuditRecord{}
	}
	return records, nil
}

func (s *Service) GetAuditRecord(ctx context.Context, orgID, id string) (*AuditRecord, error) {
	return s.repo.GetAuditRecord(ctx, orgID, id)
}

func (s *Service) CreateAuditRecord(ctx context.Context, orgID string, in CreateAuditRecordInput) (*AuditRecord, error) {
	return s.repo.CreateAuditRecord(ctx, orgID, in)
}

func (s *Service) UpdateAuditRecord(ctx context.Context, orgID, id string, in UpdateAuditRecordInput) (*AuditRecord, error) {
	return s.repo.UpdateAuditRecord(ctx, orgID, id, in)
}

// --- Control Tasks ---

func (s *Service) ListControlTasks(ctx context.Context, orgID, controlID string) ([]ControlTask, error) {
	tasks, err := s.repo.ListControlTasks(ctx, orgID, controlID)
	if err != nil {
		return nil, err
	}
	if tasks == nil {
		tasks = []ControlTask{}
	}
	return tasks, nil
}

func (s *Service) CreateControlTask(ctx context.Context, orgID, controlID string, in CreateControlTaskInput) (*ControlTask, error) {
	return s.repo.CreateControlTask(ctx, orgID, controlID, in)
}

func (s *Service) UpdateControlTask(ctx context.Context, orgID, controlID, taskID string, in UpdateControlTaskInput) (*ControlTask, error) {
	return s.repo.UpdateControlTask(ctx, orgID, controlID, taskID, in)
}

func (s *Service) DeleteControlTask(ctx context.Context, orgID, controlID, taskID string) error {
	return s.repo.DeleteControlTask(ctx, orgID, controlID, taskID)
}

// --- Resilience Tests (DORA Art. 24-27) ---

// isTLPTOverdue returns true when no TLPT test exists in the last 3 years.
func isTLPTOverdue(tests []ResilienceTest, now time.Time) bool {
	threshold := now.AddDate(-3, 0, 0)
	for _, t := range tests {
		if t.Type == "tlpt" && t.TestDate.After(threshold) {
			return false
		}
	}
	return true
}

// ListResilienceTests returns all resilience tests for the organisation, with computed OverdueWarning per entry.
// It also returns whether there is a global TLPT overdue warning.
func (s *Service) ListResilienceTests(ctx context.Context, orgID string) ([]ResilienceTest, bool, error) {
	tests, err := s.repo.ListResilienceTests(ctx, orgID)
	if err != nil {
		return nil, false, fmt.Errorf("list resilience tests: %w", err)
	}
	if tests == nil {
		tests = []ResilienceTest{}
	}
	now := time.Now().UTC()
	threshold := now.AddDate(-3, 0, 0)
	for i := range tests {
		if tests[i].Type == "tlpt" && tests[i].TestDate.Before(threshold) {
			tests[i].OverdueWarning = true
		}
	}
	tlptOverdue := isTLPTOverdue(tests, now)
	return tests, tlptOverdue, nil
}

// GetResilienceTest returns a single resilience test with computed OverdueWarning.
func (s *Service) GetResilienceTest(ctx context.Context, orgID, id string) (*ResilienceTest, error) {
	t, err := s.repo.GetResilienceTest(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if t.Type == "tlpt" && t.TestDate.Before(time.Now().UTC().AddDate(-3, 0, 0)) {
		t.OverdueWarning = true
	}
	return t, nil
}

// CreateResilienceTest creates a new resilience test entry.
func (s *Service) CreateResilienceTest(ctx context.Context, orgID string, in CreateResilienceTestInput) (*ResilienceTest, error) {
	return s.repo.CreateResilienceTest(ctx, orgID, in)
}

// UpdateResilienceTest updates an existing resilience test entry.
func (s *Service) UpdateResilienceTest(ctx context.Context, orgID, id string, in UpdateResilienceTestInput) (*ResilienceTest, error) {
	t, err := s.repo.UpdateResilienceTest(ctx, orgID, id, in)
	if err != nil {
		return nil, err
	}
	if t.Type == "tlpt" && t.TestDate.Before(time.Now().UTC().AddDate(-3, 0, 0)) {
		t.OverdueWarning = true
	}
	return t, nil
}

// DeleteResilienceTest removes a resilience test entry.
func (s *Service) DeleteResilienceTest(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteResilienceTest(ctx, orgID, id)
}

// AttachResilienceTestFile saves an uploaded file to disk and updates the attachment_url.
// Files are stored at uploadDir/resilience-tests/{id}/{filename}.
func (s *Service) AttachResilienceTestFile(ctx context.Context, orgID, id, uploadDir string, fileBytes []byte, filename string) (*ResilienceTest, error) {
	dir := fmt.Sprintf("%s/resilience-tests/%s", uploadDir, id)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create upload dir: %w", err)
	}
	destPath := fmt.Sprintf("%s/%s", dir, filename)
	if err := os.WriteFile(destPath, fileBytes, 0o640); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}
	if err := s.repo.UpdateResilienceTestAttachment(ctx, orgID, id, destPath); err != nil {
		return nil, fmt.Errorf("update attachment: %w", err)
	}
	return s.GetResilienceTest(ctx, orgID, id)
}

// --- Built-in framework templates ---

// builtinVersion returns the canonical version string for a well-known framework name.
func builtinVersion(name string) string {
	versions := map[string]string{
		"NIS2":      "2022",
		"ISO27001":  "2022",
		"BSI":       "2023",
		"CRA":       "2024",
		"DORA":      "2022",
		"EUAIACT":   "2024",
		"ISO42001":  "2023",
		"TISAX":     "6.0",
		"DSGVO-TOM": "2018",
	}
	return versions[name]
}

// builtinControls seeds a small set of representative controls for well-known frameworks.
// In production expand or load from embedded JSON/CSV files.
func builtinControls(frameworkID, orgID, name string) []Control {
	switch name {
	case "NIS2":
		return nis2Controls(frameworkID, orgID)
	case "ISO27001":
		return iso27001Controls(frameworkID, orgID)
	case "BSI":
		return bsiControls(frameworkID, orgID)
	case "CRA":
		return craControls(frameworkID, orgID)
	case "DORA":
		return doraControls(frameworkID, orgID)
	case "EUAIACT":
		return euAiActControls(frameworkID, orgID)
	case "ISO42001":
		return iso42001Controls(frameworkID, orgID)
	case "TISAX":
		return tisaxControls(frameworkID, orgID)
	case "DSGVO-TOM":
		return dsgvoTOMControls(frameworkID, orgID)
	}
	return nil
}

func nis2Controls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// Art. 21(2)(a) — Risikomanagement
		c("NIS2-A.1", "Informationssicherheitsrichtlinie",
			"Erstelle und genehmige eine schriftliche Informationssicherheitsrichtlinie. Sie muss Schutzziele, Geltungsbereich, Verantwortlichkeiten und Überprüfungsintervall enthalten. Nachweis: unterschriebenes Richtliniendokument mit Versionsnummer und Genehmigungsdatum.",
			"Risikomanagement", "manual", 3),
		c("NIS2-A.2", "Risikomanagement-Framework",
			"Implementiere einen formalen Prozess zur Identifikation, Bewertung und Behandlung von Informationssicherheitsrisiken. Nachweis: Risikomanagement-Prozessbeschreibung, Risikoregister.",
			"Risikomanagement", "manual", 3),
		c("NIS2-A.3", "Risikoanalyse und -bewertung",
			"Führe mindestens jährlich eine strukturierte Risikoanalyse durch. Bewerte Eintrittswahrscheinlichkeit und Auswirkung für alle relevanten Bedrohungen. Nachweis: ausgefülltes Risikoregister mit Bewertungsmatrix.",
			"Risikomanagement", "manual", 3),
		c("NIS2-A.4", "Risikobehandlungsplan",
			"Definiere für alle identifizierten Risiken konkrete Maßnahmen (Vermeiden, Reduzieren, Übertragen, Akzeptieren) mit Verantwortlichen und Fristen. Nachweis: Risikobehandlungsplan mit Umsetzungsstatus.",
			"Risikomanagement", "manual", 3),
		c("NIS2-A.5", "Sicherheitsziele und Governance",
			"Lege messbare Sicherheitsziele auf Organisations- und Abteilungsebene fest. Stelle sicher, dass die Geschäftsführung die IS-Governance trägt. Nachweis: dokumentierte Sicherheitsziele, Protokolle von Management-Reviews.",
			"Risikomanagement", "manual", 2),
		c("NIS2-A.6", "Rollen und Verantwortlichkeiten IS",
			"Benenne einen Informationssicherheitsbeauftragten (ISB) und dokumentiere alle sicherheitsrelevanten Rollen und deren Verantwortlichkeiten. Nachweis: Organigramm, Stellenbeschreibungen, Beauftragungsschreiben.",
			"Risikomanagement", "manual", 2),
		c("NIS2-A.7", "Richtlinienüberprüfung und Genehmigung",
			"Überprüfe alle Sicherheitsrichtlinien mindestens jährlich oder nach wesentlichen Änderungen und hole erneute Genehmigung ein. Nachweis: Änderungshistorie der Richtlinien mit Genehmigungsnachweisen.",
			"Risikomanagement", "manual", 1),
		c("NIS2-A.8", "Asset-Inventar und Klassifizierung",
			"Führe ein aktuelles Inventar aller informationsverarbeitenden Assets (Hardware, Software, Daten). Klassifiziere Assets nach Schutzbedarf. Nachweis: Asset-Register mit Klassifizierungsschema.",
			"Risikomanagement", "manual", 2),
		c("NIS2-A.9", "Bedrohungsanalyse (Threat Intelligence)",
			"Abonniere relevante Bedrohungsinformationen (CERT-Bund, BSI-Warnmeldungen, CVE-Feeds) und integriere sie in den Risikoprozess. Nachweis: Abonnementbestätigung, Prozessdokumentation zur Verarbeitung.",
			"Risikomanagement", "manual", 2),
		c("NIS2-A.10", "Compliance-Management",
			"Identifiziere alle anwendbaren gesetzlichen, regulatorischen und vertraglichen Anforderungen (NIS2, DSGVO, branchenspezifisch) und verfolge deren Einhaltung. Nachweis: Compliance-Register, Auditberichte.",
			"Risikomanagement", "manual", 2),

		// Art. 21(2)(b) — Incident Handling
		c("NIS2-B.1", "Incident-Response-Richtlinie",
			"Erstelle eine schriftliche Incident-Response-Richtlinie mit Klassifizierungsschema, Eskalationspfaden und Reaktionszeiten. Nachweis: genehmigtes Richtliniendokument.",
			"Incident Management", "manual", 3),
		c("NIS2-B.2", "Erkennung und Überwachung von Vorfällen",
			"Implementiere technische Erkennungsmechanismen (SIEM, IDS, Log-Monitoring). Stelle sicher, dass Alarme rund um die Uhr überwacht werden. Nachweis: SIEM-Konfiguration, Monitoring-Dashboard.",
			"Incident Management", "automated", 3),
		c("NIS2-B.3", "Incident-Response-Team (CSIRT)",
			"Bilde ein benanntes Incident-Response-Team mit klaren Rollen. Stelle Erreichbarkeit und Eskalationspfade sicher. Nachweis: Teambesetzungsplan, Kontaktliste, Beauftragungsschreiben.",
			"Incident Management", "manual", 2),
		c("NIS2-B.4", "Klassifizierung und Priorisierung von Vorfällen",
			"Definiere ein Klassifizierungsschema (Schweregrade 1–4 o.ä.) mit konkreten Kriterien und daraus abgeleiteten Reaktionszeiten. Nachweis: Klassifizierungsmatrix im Incident-Response-Plan.",
			"Incident Management", "manual", 2),
		c("NIS2-B.5", "Meldung an Behörde innerhalb 24 Stunden",
			"Stelle sicher, dass erhebliche Sicherheitsvorfälle gem. Art. 23 NIS2 innerhalb von 24 Stunden an das BSI/zuständige CSIRT gemeldet werden. Nachweis: Meldeprozess-Dokumentation, ggf. Muster-Meldung.",
			"Incident Management", "manual", 3),
		c("NIS2-B.6", "Detaillierter Vorfallsbericht innerhalb 72 Stunden",
			"Erstelle innerhalb von 72 Stunden nach Ersterkennung einen detaillierten Vorfallsbericht an die Aufsichtsbehörde. Nachweis: Berichtsvorlage, Eskalationsplan mit Fristen.",
			"Incident Management", "manual", 3),
		c("NIS2-B.7", "Post-Incident-Review",
			"Führe nach jedem erheblichen Vorfall eine strukturierte Nachbesprechung (Post-Mortem) durch und dokumentiere Erkenntnisse und Verbesserungsmaßnahmen. Nachweis: Post-Incident-Review-Berichte.",
			"Incident Management", "manual", 2),
		c("NIS2-B.8", "Kommunikations- und Eskalationsplan",
			"Dokumentiere interne und externe Kommunikationswege für den Krisenfall inkl. Pressestelle, Juristen, Behörden. Nachweis: Kommunikationsplan mit Kontaktlisten.",
			"Incident Management", "manual", 2),
		c("NIS2-B.9", "Forensische Beweissicherung",
			"Definiere Verfahren zur gerichtsfesten Sicherung digitaler Beweise bei Vorfällen. Stelle notwendige Tools und Schulung bereit. Nachweis: Forensik-Checkliste, Tool-Dokumentation.",
			"Incident Management", "manual", 1),

		// Art. 21(2)(c) — Business Continuity
		c("NIS2-C.1", "Business-Continuity-Richtlinie",
			"Erstelle eine BCM-Richtlinie, die Geltungsbereich, Verantwortlichkeiten und Ziele des Business-Continuity-Managements festlegt. Nachweis: genehmigtes BCM-Richtliniendokument.",
			"Business Continuity", "manual", 2),
		c("NIS2-C.2", "Business Impact Analysis (BIA)",
			"Analysiere alle kritischen Geschäftsprozesse hinsichtlich Auswirkung und maximaler Ausfallzeit. Nachweis: BIA-Dokument mit MTPD und MBCO-Angaben.",
			"Business Continuity", "manual", 3),
		c("NIS2-C.3", "RTO/RPO-Ziele definiert",
			"Lege für alle kritischen Systeme konkrete Recovery Time Objectives (RTO) und Recovery Point Objectives (RPO) fest. Nachweis: RTO/RPO-Tabelle, abgestimmt mit BIA.",
			"Business Continuity", "manual", 3),
		c("NIS2-C.4", "Backup-Richtlinie und -Verfahren",
			"Definiere Backup-Häufigkeit, Aufbewahrungsdauer, Speicherort (3-2-1-Regel) und Verschlüsselung. Nachweis: Backup-Richtlinie, Backup-Job-Konfiguration.",
			"Business Continuity", "automated", 3),
		c("NIS2-C.5", "Backup-Tests und -Überprüfung",
			"Teste Backups mindestens vierteljährlich durch tatsächliche Wiederherstellung. Dokumentiere Ergebnisse. Nachweis: Backup-Testberichte mit Datum und Ergebnis.",
			"Business Continuity", "automated", 3),
		c("NIS2-C.6", "Notfallwiederherstellungsplan (DR)",
			"Erstelle einen detaillierten Disaster-Recovery-Plan mit konkreten Wiederherstellungsschritten je Kritisch-System. Nachweis: DR-Plan-Dokument.",
			"Business Continuity", "manual", 3),
		c("NIS2-C.7", "DR-Tests und -Übungen",
			"Führe mindestens jährlich einen DR-Test (Tabletop-Übung oder Live-Test) durch. Nachweis: Übungsprotokoll mit Ergebnissen und Verbesserungsmaßnahmen.",
			"Business Continuity", "manual", 2),
		c("NIS2-C.8", "Krisenkommunkationsplan",
			"Dokumentiere Kommunikationswege für den Krisenfall: interne Benachrichtigung, externe Kommunikation (Kunden, Medien, Behörden). Nachweis: Kommunikationsplan.",
			"Business Continuity", "manual", 1),
		c("NIS2-C.9", "Redundanz und Hochverfügbarkeit",
			"Implementiere technische Redundanz für kritische Systeme (Failover, Load Balancing, georedundante Standorte). Nachweis: Architektur-Diagramm, SLA-Dokumentation.",
			"Business Continuity", "automated", 2),

		// Art. 21(2)(d) — Supply Chain Security
		c("NIS2-D.1", "Lieferanten-Sicherheitsrichtlinie",
			"Definiere Mindest-Sicherheitsanforderungen für alle IKT-Lieferanten und Dienstleister. Nachweis: Lieferanten-Sicherheitsrichtlinie.",
			"Supply Chain", "manual", 2),
		c("NIS2-D.2", "Lieferanten-Risikobewertung",
			"Bewerte das Sicherheitsrisiko aller wesentlichen Lieferanten vor Vertragsabschluss und danach jährlich. Nachweis: Lieferanten-Risikobewertungsberichte.",
			"Supply Chain", "manual", 3),
		c("NIS2-D.3", "Sicherheitsanforderungen in Verträgen",
			"Verankere verbindliche Sicherheitsanforderungen (DSGVO-AVV, ISO 27001, Auditrechte) in allen IKT-Verträgen. Nachweis: Vertragsklauseln, AVV-Mustervorlage.",
			"Supply Chain", "manual", 3),
		c("NIS2-D.4", "Zugriffsverwaltung für Drittparteien",
			"Steuere und überwache Remote-Zugriffe von Lieferanten und externen Dienstleistern. Nachweis: Zugriffskonzept, Protokolle externer Zugriffe.",
			"Supply Chain", "manual", 2),
		c("NIS2-D.5", "Software-Lieferkettensicherheit",
			"Prüfe eingesetzte Open-Source- und Third-Party-Software auf bekannte Schwachstellen (SBOM, Dependency-Scanning). Nachweis: SBOM, Scanner-Berichte.",
			"Supply Chain", "manual", 3),
		c("NIS2-D.6", "Sicherheitsprüfung von IKT-Produkten",
			"Führe vor dem Einsatz neuer IKT-Produkte eine Sicherheitsprüfung durch (Zertifizierungen, Herstellernachweise). Nachweis: Produktprüfungs-Checkliste.",
			"Supply Chain", "manual", 2),
		c("NIS2-D.7", "Lieferanten-Monitoring",
			"Überwache laufend Sicherheitsmeldungen und Statusänderungen kritischer Lieferanten. Nachweis: Monitoring-Prozess, Eskalationsverfahren.",
			"Supply Chain", "manual", 1),
		c("NIS2-D.8", "Subunternehmer- und Outsourcing-Management",
			"Stelle sicher, dass Sicherheitsanforderungen bei Weitervergabe an Subunternehmer gewahrt bleiben. Nachweis: Outsourcing-Richtlinie, Vertragsklauseln.",
			"Supply Chain", "manual", 1),

		// Art. 21(2)(e) — Netz- und IS-Sicherheit
		c("NIS2-E.1", "Sicherer Entwicklungszyklus (SDLC)",
			"Integriere Sicherheitsanforderungen in alle Phasen des Softwareentwicklungsprozesses (Threat Modeling, Code Review, Security Testing). Nachweis: SDLC-Dokumentation, Review-Nachweise.",
			"Netz- & IS-Sicherheit", "manual", 2),
		c("NIS2-E.2", "Sicherheitsanforderungen bei Systembeschaffung",
			"Definiere und prüfe Sicherheitsanforderungen vor Beschaffung neuer IT-Systeme. Nachweis: Beschaffungs-Checkliste mit Sicherheitskriterien.",
			"Netz- & IS-Sicherheit", "manual", 2),
		c("NIS2-E.3", "Schwachstellenmanagement-Programm",
			"Betreibe ein strukturiertes Programm zur Identifikation, Bewertung und Behebung technischer Schwachstellen. Nachweis: Scanner-Berichte, Ticket-System-Auszüge.",
			"Netz- & IS-Sicherheit", "automated", 3),
		c("NIS2-E.4", "Patch-Management",
			"Stelle sicher, dass Sicherheits-Patches für kritische Systeme innerhalb definierter Fristen eingespielt werden (kritisch: ≤72 h). Nachweis: Patch-Berichte, SLA-Dokumentation.",
			"Netz- & IS-Sicherheit", "automated", 3),
		c("NIS2-E.5", "Penetrationstests",
			"Führe mindestens jährlich Penetrationstests durch kritische Systeme durch. Nachweis: Pentest-Berichte mit Datum, Scope und Ergebnissen.",
			"Netz- & IS-Sicherheit", "manual", 2),
		c("NIS2-E.6", "Responsible Vulnerability Disclosure",
			"Etabliere einen Prozess zur Entgegennahme und Bearbeitung extern gemeldeter Schwachstellen. Nachweis: Responsible-Disclosure-Policy (z.B. security.txt).",
			"Netz- & IS-Sicherheit", "manual", 2),
		c("NIS2-E.7", "Änderungsmanagement (Change Management)",
			"Stelle sicher, dass alle Änderungen an IT-Systemen genehmigt, getestet und dokumentiert werden. Nachweis: Change-Management-Prozess, Genehmigungsnachweise.",
			"Netz- & IS-Sicherheit", "manual", 2),
		c("NIS2-E.8", "Netzarchitektur und Segmentierung",
			"Segmentiere das Netzwerk nach Schutzbedarf (DMZ, Produktions- vs. Entwicklungsnetz, OT-Trennung). Nachweis: Netzplan, Firewall-Regeln.",
			"Netz- & IS-Sicherheit", "automated", 3),
		c("NIS2-E.9", "Firewall und Perimetersicherheit",
			"Betreibe Firewalls an allen Netzübergängen nach dem Least-Privilege-Prinzip. Überprüfe Regeln mindestens jährlich. Nachweis: Firewall-Konfiguration, Regelreviews.",
			"Netz- & IS-Sicherheit", "automated", 3),
		c("NIS2-E.10", "Einbruchserkennung und -prävention (IDS/IPS)",
			"Setze IDS/IPS-Systeme an kritischen Netzpunkten ein und stelle sicher, dass Alarme zeitnah bearbeitet werden. Nachweis: IDS/IPS-Konfiguration, Alarmierungsprotokoll.",
			"Netz- & IS-Sicherheit", "automated", 2),
		c("NIS2-E.11", "Sichere Konfigurationsverwaltung",
			"Nutze Hardening-Leitlinien (CIS Benchmarks, BSI SiM) für alle eingesetzten Systeme. Nachweis: Konfigurationsbaselines, Compliance-Scan-Berichte.",
			"Netz- & IS-Sicherheit", "automated", 2),

		// Art. 21(2)(f) — Wirksamkeitsbewertung
		c("NIS2-F.1", "Cybersicherheits-KPIs und Metriken",
			"Definiere messbare KPIs für die Sicherheitsleistung (z.B. MTTR, offene Schwachstellen, Patch-Compliance-Rate). Nachweis: KPI-Definition, monatliche Berichte.",
			"Wirksamkeitsbewertung", "manual", 2),
		c("NIS2-F.2", "Internes Sicherheitsauditprogramm",
			"Führe mindestens jährlich interne IS-Audits durch und dokumentiere Befunde und Maßnahmen. Nachweis: Auditplan, Auditberichte.",
			"Wirksamkeitsbewertung", "manual", 2),
		c("NIS2-F.3", "Management-Review der Sicherheitsleistung",
			"Halte mindestens jährlich ein Management-Review der IS-Leistung ab. Nachweis: Meeting-Protokolle, Entscheidungsdokumentation.",
			"Wirksamkeitsbewertung", "manual", 2),
		c("NIS2-F.4", "Kontinuierlicher Verbesserungsprozess",
			"Etabliere einen formalen KVP, der Erkenntnisse aus Audits, Vorfällen und Reviews in konkrete Verbesserungen überführt. Nachweis: Maßnahmenverfolgung (z.B. Ticketsystem).",
			"Wirksamkeitsbewertung", "manual", 1),
		c("NIS2-F.5", "Externe Zertifizierung und Auditierung",
			"Plane externe Audits oder Zertifizierungen (z.B. ISO 27001) als Nachweis gegenüber Kunden und Behörden. Nachweis: Zertifikat, Auditbericht.",
			"Wirksamkeitsbewertung", "manual", 1),

		// Art. 21(2)(g) — Cyber-Hygiene und Schulungen
		c("NIS2-G.1", "Cybersicherheits-Awareness-Programm",
			"Betreibe ein dauerhaftes Awareness-Programm (Newsletter, Intranet, Poster) zur Sensibilisierung aller Mitarbeitenden. Nachweis: Programmbeschreibung, Materialien.",
			"Cyber-Hygiene & Training", "manual", 2),
		c("NIS2-G.2", "Sicherheitsschulung für alle Mitarbeitenden",
			"Schule alle Mitarbeitenden mindestens jährlich zu grundlegenden Sicherheitsthemen (Phishing, Passwortsicherheit, Datenschutz). Nachweis: Schulungsnachweise, Teilnehmerlisten.",
			"Cyber-Hygiene & Training", "manual", 3),
		c("NIS2-G.3", "Rollenbasierte Sicherheitsschulung",
			"Biete zusätzliche Schulungen für sicherheitskritische Rollen an (Admins, Entwickler, Management). Nachweis: rollenspezifische Schulungspläne und Teilnahmenachweise.",
			"Cyber-Hygiene & Training", "manual", 2),
		c("NIS2-G.4", "Phishing-Simulationen",
			"Führe regelmäßige (mind. 2x/Jahr) Phishing-Simulationen durch und nutze Ergebnisse für gezielte Nachschulung. Nachweis: Simulationsberichte mit Klickraten und Folgemaßnahmen.",
			"Cyber-Hygiene & Training", "automated", 2),
		c("NIS2-G.5", "Passwort- und Authentifizierungsrichtlinie",
			"Lege Mindestanforderungen für Passwörter und Authentifizierung fest (Länge, Komplexität, Wiederverwendung, Passwortmanager). Nachweis: Richtliniendokument, technische Durchsetzung.",
			"Cyber-Hygiene & Training", "manual", 3),
		c("NIS2-G.6", "E-Mail-Sicherheitskontrollen",
			"Implementiere E-Mail-Sicherheitsmaßnahmen (SPF, DKIM, DMARC, Anti-Spam, Anti-Phishing). Nachweis: DNS-Einträge, E-Mail-Gateway-Konfiguration.",
			"Cyber-Hygiene & Training", "automated", 2),
		c("NIS2-G.7", "Malware-Schutz und Antivirus",
			"Setze Endpoint-Protection-Software auf allen Endgeräten ein und stelle automatische Signatur-Updates sicher. Nachweis: AV-Konfiguration, Scan-Berichte.",
			"Cyber-Hygiene & Training", "automated", 3),
		c("NIS2-G.8", "Endpoint Detection and Response (EDR)",
			"Implementiere EDR-Software zur verhaltensbasierten Erkennung von Angriffen auf Endgeräten. Nachweis: EDR-Konfiguration, Alarmierungsprotokoll.",
			"Cyber-Hygiene & Training", "automated", 2),
		c("NIS2-G.9", "Web-Filterung und DNS-Sicherheit",
			"Setze Web-Proxy oder DNS-Filtering ein, um den Aufruf schädlicher Websites zu verhindern. Nachweis: Filterlisten-Konfiguration, DNS-Sicherheitsberichte.",
			"Cyber-Hygiene & Training", "automated", 2),

		// Art. 21(2)(h) — Kryptographie
		c("NIS2-H.1", "Kryptographierichtlinie",
			"Erstelle eine Richtlinie zu zulässigen kryptographischen Verfahren und deren Einsatzgebieten. Nachweis: genehmigtes Richtliniendokument.",
			"Kryptographie", "manual", 2),
		c("NIS2-H.2", "Schlüsselverwaltungsverfahren",
			"Dokumentiere den gesamten Lebenszyklus kryptographischer Schlüssel (Generierung, Verteilung, Speicherung, Widerruf, Vernichtung). Nachweis: Schlüsselverwaltungsverfahren, KMS-Konfiguration.",
			"Kryptographie", "manual", 2),
		c("NIS2-H.3", "Verschlüsselung ruhender Daten",
			"Verschlüssele alle sensiblen Daten in Ruhe (Datenbanken, Backups, Dateisysteme) mit aktuellen Verfahren (AES-256). Nachweis: Verschlüsselungskonfiguration, Scanner-Berichte.",
			"Kryptographie", "automated", 3),
		c("NIS2-H.4", "Verschlüsselung übertragener Daten (TLS)",
			"Stelle sicher, dass alle Datenübertragungen verschlüsselt erfolgen (TLS 1.2+, keine veralteten Protokolle). Nachweis: TLS-Scan-Bericht (z.B. SSL Labs), Konfigurationsdokumentation.",
			"Kryptographie", "automated", 3),
		c("NIS2-H.5", "Zertifikats-Lifecycle-Management",
			"Verwalte alle TLS/SSL-Zertifikate zentral, überwache Ablaufdaten und erneuere rechtzeitig. Nachweis: Zertifikatsregister, Erneuerungsprozess.",
			"Kryptographie", "automated", 2),
		c("NIS2-H.6", "Zulässige kryptographische Algorithmen",
			"Führe eine Liste genehmigter Algorithmen und Schlüssellängen (BSI TR-02102) und schließe veraltete Verfahren (MD5, SHA-1, DES) aus. Nachweis: Algorithmenliste, Konfigurationsprüfung.",
			"Kryptographie", "manual", 1),

		// Art. 21(2)(i) — HR-Sicherheit, Zugriffskontrolle, Asset-Management
		c("NIS2-I.1", "HR-Sicherheitsrichtlinie",
			"Definiere Sicherheitsanforderungen für alle Phasen des Beschäftigungsverhältnisses (Einstellung, laufend, Austritt). Nachweis: HR-Sicherheitsrichtlinie.",
			"Zugang & Identität", "manual", 2),
		c("NIS2-I.2", "Hintergrundüberprüfungen (Screening)",
			"Führe bei Einstellung und für sicherheitskritische Rollen Hintergrundüberprüfungen durch (soweit gesetzlich zulässig). Nachweis: Screening-Richtlinie, Nachweisarchivierung.",
			"Zugang & Identität", "manual", 2),
		c("NIS2-I.3", "Richtlinie zur akzeptablen Nutzung",
			"Kommuniziere eine verbindliche Richtlinie zur akzeptablen Nutzung von IT-Ressourcen an alle Mitarbeitenden. Nachweis: Richtlinie, Unterschriften/Bestätigungen der Mitarbeitenden.",
			"Zugang & Identität", "manual", 2),
		c("NIS2-I.4", "Offboarding- und Kündigungsverfahren",
			"Stelle sicher, dass beim Austritt alle Zugänge zeitnah gesperrt, Assets zurückgegeben und Wissenstransfer sichergestellt wird. Nachweis: Offboarding-Checkliste.",
			"Zugang & Identität", "manual", 2),
		c("NIS2-I.5", "Zugriffskontrollrichtlinie",
			"Definiere das Prinzip der minimalen Rechtevergabe und dokumentiere den Genehmigungsprozess für Zugriffsrechte. Nachweis: Zugriffskontrollrichtlinie.",
			"Zugang & Identität", "manual", 3),
		c("NIS2-I.6", "Identity- und Access-Management (IAM)",
			"Betreibe ein zentrales IAM-System für die Verwaltung aller Benutzerkonten und -rechte. Nachweis: IAM-Systemdokumentation, Provisionierungsprozess.",
			"Zugang & Identität", "automated", 3),
		c("NIS2-I.7", "Privileged Access Management (PAM)",
			"Verwalte privilegierte Konten (Admins, Root) gesondert mit PAM-Lösung, Vier-Augen-Prinzip und vollständigem Logging. Nachweis: PAM-Konfiguration, Zugriffsprotokoll.",
			"Zugang & Identität", "automated", 3),
		c("NIS2-I.8", "Rollenbasierte Zugriffssteuerung (RBAC)",
			"Implementiere rollenbasierte Berechtigungskonzepte für alle kritischen Systeme. Nachweis: Rollenmatrix, Berechtigungskonzept.",
			"Zugang & Identität", "manual", 2),
		c("NIS2-I.9", "Regelmäßige Zugriffsüberprüfungen",
			"Überprüfe mindestens halbjährlich alle vergebenen Zugriffsrechte auf Aktualität und Notwendigkeit. Nachweis: Prüfprotokolle, Bereinigungsnachweise.",
			"Zugang & Identität", "manual", 2),
		c("NIS2-I.10", "Physische Sicherheitsmaßnahmen",
			"Sichere Serverräume, Büros und Arbeitsplätze physisch gegen unbefugten Zugang (Zutrittskontrolle, CCTV, Clean-Desk). Nachweis: Zutrittskontrollkonzept, Begehungsprotokoll.",
			"Zugang & Identität", "manual", 2),
		c("NIS2-I.11", "Asset-Erfassung, -Kennzeichnung und -Entsorgung",
			"Kennzeichne alle Hardware-Assets, erfasse sie im Inventar und stelle datensichere Entsorgung sicher (z.B. DSGVO-konformes Löschen). Nachweis: Asset-Register, Entsorgungsnachweise.",
			"Zugang & Identität", "manual", 1),
		c("NIS2-I.12", "Mobile-Device- und BYOD-Management",
			"Verwalte Mobilgeräte über MDM-Lösung, setze Geräteverschlüsselung und Remote-Wipe durch. Nachweis: MDM-Konfiguration, BYOD-Richtlinie.",
			"Zugang & Identität", "manual", 2),
		c("NIS2-I.13", "Logging, Monitoring und SIEM",
			"Protokolliere sicherheitsrelevante Ereignisse auf allen kritischen Systemen und überwache zentral via SIEM. Nachweis: Log-Konfiguration, SIEM-Architektur, Aufbewahrungsrichtlinie.",
			"Zugang & Identität", "automated", 3),

		// Art. 21(2)(j) — MFA und sichere Kommunikation
		c("NIS2-J.1", "Multi-Faktor-Authentifizierung (MFA)",
			"Erzwinge MFA für alle Benutzer bei Zugriff auf Unternehmensanwendungen und -systeme. Nachweis: MFA-Konfiguration, Ausnahmeliste mit Begründungen.",
			"Authentifizierung & Kommunikation", "automated", 3),
		c("NIS2-J.2", "MFA für privilegierte und Remote-Konten",
			"Stelle sicher, dass Administratoren und Remote-Nutzer ausnahmslos MFA verwenden. Nachweis: PAM-Konfiguration, VPN-Zugangsprotokolle.",
			"Authentifizierung & Kommunikation", "automated", 3),
		c("NIS2-J.3", "Richtlinie für Remote-Zugang",
			"Definiere zulässige Methoden und Anforderungen für Remote-Zugang (VPN, Zero Trust, MFA, Gerätezertifikate). Nachweis: Remote-Access-Richtlinie.",
			"Authentifizierung & Kommunikation", "manual", 3),
		c("NIS2-J.4", "VPN und sicherer Remote-Zugang",
			"Setze ein verschlüsseltes VPN oder Zero-Trust-Netzwerkzugangslösung für alle Remote-Verbindungen ein. Nachweis: VPN-Konfiguration, Zertifikatsdokumentation.",
			"Authentifizierung & Kommunikation", "automated", 2),
		c("NIS2-J.5", "Verschlüsselte Kommunikation (Sprache, Video, Text)",
			"Nutze ausschließlich verschlüsselte Kommunikationstools für dienstliche Kommunikation (Signal, Teams mit E2E, etc.). Nachweis: Tool-Richtlinie, Konfiguration.",
			"Authentifizierung & Kommunikation", "automated", 2),
		c("NIS2-J.6", "Endpunktsicherheit für Remote-Zugang",
			"Stelle sicher, dass Remote-Endgeräte Sicherheitsanforderungen erfüllen (Verschlüsselung, aktuelle AV, MDM). Nachweis: Endpoint-Compliance-Berichte.",
			"Authentifizierung & Kommunikation", "automated", 2),
		c("NIS2-J.7", "Mobile-Device-Sicherheit",
			"Konfiguriere mobile Geräte mit Bildschirmsperre, Verschlüsselung und Remote-Wipe-Fähigkeit. Nachweis: MDM-Konfiguration, Compliance-Bericht.",
			"Authentifizierung & Kommunikation", "automated", 2),
		c("NIS2-J.8", "Notfallkommunikationssysteme",
			"Halte Notfallkommunikationsmittel bereit, die unabhängig von der normalen IT-Infrastruktur funktionieren (Satelliten-Telefon, Out-of-Band-Kommunikation). Nachweis: Inventarliste, Testprotokoll.",
			"Authentifizierung & Kommunikation", "manual", 1),
	}
}

func iso27001Controls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// A.5 — Informationssicherheitsrichtlinien
		c("A.5.1", "Richtlinien zur Informationssicherheit", "Definiere den Rahmen für alle IS-Richtlinien der Organisation.", "Richtlinien", "manual", 2),
		c("A.5.1.1", "Richtlinien für Informationssicherheit", "Erstelle ein vollständiges Set genehmigter IS-Richtlinien. Nachweis: aktuelle, unterschriebene Richtliniendokumente.", "Richtlinien", "manual", 2),
		c("A.5.1.2", "Überprüfung der Richtlinien für Informationssicherheit", "Überprüfe alle Richtlinien mindestens jährlich. Nachweis: Revisionshistorie mit Datum und Genehmigung.", "Richtlinien", "manual", 1),

		// A.6 — Organisation der Informationssicherheit
		c("A.6.1", "Interne Organisation", "Stelle sicher, dass IS-Verantwortlichkeiten klar geregelt sind.", "Organisation", "manual", 2),
		c("A.6.1.1", "Rollen und Verantwortlichkeiten für Informationssicherheit", "Weise IS-Rollen (ISB, Datenschutzbeauftragter, etc.) explizit zu. Nachweis: Stellenbeschreibungen, Beauftragungsschreiben.", "Organisation", "manual", 2),
		c("A.6.1.2", "Aufgabentrennung", "Trenne unvereinbare Aufgaben (z.B. Entwicklung/Freigabe). Nachweis: Rollenmatrix mit Trennungsnachweis.", "Organisation", "manual", 1),
		c("A.6.1.3", "Kontakt mit Behörden", "Pflege aktuelle Kontaktinformationen zu relevanten Behörden (BSI, Datenschutzbehörden). Nachweis: Kontaktliste.", "Organisation", "manual", 1),
		c("A.6.1.5", "Informationssicherheit im Projektmanagement", "Integriere IS-Anforderungen in alle Projektprozesse. Nachweis: Projektcheckliste mit IS-Punkten.", "Organisation", "manual", 1),
		c("A.6.2", "Mobilgeräte und Telearbeit", "Manage Risiken durch mobile Geräte und Heimarbeit.", "Organisation", "manual", 2),
		c("A.6.2.1", "Richtlinie für mobile Geräte", "Definiere zulässige Nutzung und Sicherheitsanforderungen für mobile Geräte. Nachweis: MDM-Konfiguration, Richtliniendokument.", "Organisation", "manual", 2),
		c("A.6.2.2", "Telearbeit", "Stelle sichere Arbeitsmöglichkeiten für Heimarbeitsplätze sicher. Nachweis: Telearbeitsrichtlinie, VPN-Konfiguration.", "Organisation", "manual", 1),

		// A.8 — Asset Management
		c("A.8.1", "Verantwortung für Assets", "Inventarisiere und klassifiziere alle Informationsassets.", "Asset Management", "automated", 2),
		c("A.8.1.1", "Inventarisierung von Assets", "Führe ein vollständiges, aktuelles Asset-Register. Nachweis: Asset-Inventar mit letztem Aktualisierungsdatum.", "Asset Management", "automated", 2),
		c("A.8.1.2", "Eigentümerschaft von Assets", "Weise jedem Asset einen verantwortlichen Eigentümer zu. Nachweis: Asset-Register mit Eigentümerfeld.", "Asset Management", "manual", 1),
		c("A.8.1.3", "Zulässige Nutzung von Assets", "Dokumentiere akzeptable Nutzungsregeln für alle Asset-Klassen. Nachweis: Acceptable-Use-Policy.", "Asset Management", "manual", 1),
		c("A.8.1.4", "Rückgabe von Assets", "Stelle Rückgabe aller Assets bei Beschäftigungsende sicher. Nachweis: Offboarding-Checkliste.", "Asset Management", "manual", 1),

		// A.9 — Zugangskontrolle
		c("A.9.1", "Geschäftsanforderungen an die Zugangskontrolle", "Definiere Zugangskontrollrichtlinie basierend auf Geschäftsbedarf.", "Zugangskontrolle", "automated", 3),
		c("A.9.1.1", "Zugangssteuerungsrichtlinie", "Erstelle eine schriftliche Zugangskontrollrichtlinie (Need-to-know, Least Privilege). Nachweis: genehmigtes Dokument.", "Zugangskontrolle", "manual", 3),
		c("A.9.1.2", "Zugang zu Netzwerken und Netzwerkdiensten", "Beschränke Netzwerkzugänge auf autorisierte Nutzer und Geräte. Nachweis: NAC-Konfiguration, Firewall-Regeln.", "Zugangskontrolle", "automated", 2),
		c("A.9.2", "Benutzerzugangsverwaltung", "Manage Benutzerkonten über den gesamten Lebenszyklus.", "Zugangskontrolle", "automated", 3),
		c("A.9.2.1", "Registrierung und Deregistrierung von Benutzern", "Formalisiere Onboarding/Offboarding-Prozesse für Konten. Nachweis: Provisionierungs-Workflow.", "Zugangskontrolle", "automated", 2),
		c("A.9.2.2", "Benutzerzugangsprovisionierung", "Stelle sicher, dass Zugriffsrechte nur nach Genehmigung erteilt werden. Nachweis: Genehmigungsprotokoll.", "Zugangskontrolle", "automated", 2),
		c("A.9.2.3", "Verwaltung privilegierter Zugriffsrechte", "Verwalte Admin-Rechte restriktiv mit PAM-Lösung. Nachweis: PAM-Konfiguration, Zugriffsprotokoll.", "Zugangskontrolle", "automated", 3),
		c("A.9.2.5", "Überprüfung von Benutzerzugriffsrechten", "Überprüfe Zugriffsrechte halbjährlich auf Aktualität. Nachweis: Review-Protokolle.", "Zugangskontrolle", "manual", 2),
		c("A.9.4", "Zugangs- und Passwortverwaltung", "Sichere Systemzugänge durch technische Maßnahmen.", "Zugangskontrolle", "automated", 3),
		c("A.9.4.1", "Zugang zu Informationen einschränken", "Setze Least-Privilege auf Applikationsebene durch. Nachweis: Berechtigungskonzept.", "Zugangskontrolle", "automated", 2),
		c("A.9.4.2", "Sichere Anmeldeverfahren", "Erzwinge MFA und sichere Login-Mechanismen. Nachweis: MFA-Konfiguration.", "Zugangskontrolle", "automated", 3),
		c("A.9.4.3", "Passwortverwaltungssystem", "Setze einen Passwort-Manager oder Single-Sign-On ein. Nachweis: Tool-Konfiguration, Richtlinie.", "Zugangskontrolle", "automated", 3),

		// A.10 — Kryptographie
		c("A.10.1", "Kryptographische Maßnahmen", "Stelle den richtigen Einsatz von Kryptographie sicher.", "Kryptographie", "manual", 2),
		c("A.10.1.1", "Richtlinie für den Einsatz kryptographischer Maßnahmen", "Definiere zulässige Algorithmen, Schlüssellängen und Einsatzgebiete. Nachweis: Kryptographierichtlinie.", "Kryptographie", "manual", 2),
		c("A.10.1.2", "Schlüsselverwaltung", "Dokumentiere Schlüssellebenszyklus (Generierung, Verteilung, Widerruf). Nachweis: Key-Management-Prozess, KMS-Konfiguration.", "Kryptographie", "manual", 2),

		// A.12 — Betrieb
		c("A.12.1", "Betriebsverfahren und Verantwortlichkeiten", "Dokumentiere und manage IT-Betriebsprozesse.", "Betrieb", "manual", 2),
		c("A.12.1.1", "Dokumentierte Betriebsverfahren", "Erstelle schriftliche Betriebshandbücher für alle kritischen Systeme. Nachweis: Betriebsdokumentation.", "Betrieb", "manual", 2),
		c("A.12.1.2", "Änderungsmanagement", "Stelle sicher, dass alle IT-Änderungen geplant, genehmigt und dokumentiert werden. Nachweis: Change-Tickets.", "Betrieb", "manual", 2),
		c("A.12.3", "Datensicherung", "Stelle Datenverfügbarkeit durch regelmäßige Backups sicher.", "Betrieb", "automated", 3),
		c("A.12.3.1", "Sicherung von Informationen", "Implementiere automatisierte Backups nach 3-2-1-Prinzip. Nachweis: Backup-Job-Konfiguration, Testberichte.", "Betrieb", "automated", 3),
		c("A.12.6", "Management technischer Schwachstellen", "Reduziere Angriffsfläche durch zeitnahes Schwachstellenmanagement.", "Betrieb", "automated", 3),
		c("A.12.6.1", "Management technischer Schwachstellen", "Scanne regelmäßig auf Schwachstellen und behebe kritische innerhalb definierter Fristen. Nachweis: Scanner-Berichte, Patch-Protokoll.", "Betrieb", "automated", 3),

		// A.14 — Systembeschaffung, -entwicklung und -wartung
		c("A.14.1", "Sicherheitsanforderungen an Informationssysteme", "Definiere IS-Anforderungen bei Beschaffung und Entwicklung.", "Systementwicklung", "manual", 2),
		c("A.14.1.1", "Analyse und Spezifikation von Anforderungen an die Informationssicherheit", "Dokumentiere IS-Anforderungen in Pflichtenheften. Nachweis: Anforderungsdokument.", "Systementwicklung", "manual", 2),
		c("A.14.1.2", "Absicherung von Anwendungsdiensten in öffentlichen Netzen", "Sichere Web-Dienste gegen OWASP Top 10. Nachweis: Pentest-Bericht, WAF-Konfiguration.", "Systementwicklung", "manual", 2),
		c("A.14.2", "Sicherheit in Entwicklungs- und Unterstützungsprozessen", "Integriere Sicherheit in den gesamten SDLC.", "Systementwicklung", "manual", 2),
		c("A.14.2.1", "Richtlinie zur sicheren Entwicklung", "Erstelle und kommuniziere Secure-Coding-Richtlinien. Nachweis: Richtliniendokument, Schulungsnachweise.", "Systementwicklung", "manual", 2),
		c("A.14.2.8", "Testen der Systemsicherheit", "Führe Sicherheitstests (SAST, DAST, Pentest) vor Releases durch. Nachweis: Testberichte.", "Systementwicklung", "manual", 2),

		// A.16 — Handhabung von Informationssicherheitsvorfällen
		c("A.16.1", "Management von Informationssicherheitsvorfällen", "Etabliere einen strukturierten Prozess zur Vorfallbehandlung.", "Vorfallmanagement", "manual", 3),
		c("A.16.1.1", "Verantwortlichkeiten und Verfahren", "Definiere klare Rollen und Abläufe für Vorfallreaktionen. Nachweis: IR-Plan, Teambesetzungsplan.", "Vorfallmanagement", "manual", 2),
		c("A.16.1.2", "Meldung von Informationssicherheitsereignissen", "Etabliere einfache Meldekanäle für alle Mitarbeitenden. Nachweis: Meldeprozess, Kontaktinfos.", "Vorfallmanagement", "manual", 3),
		c("A.16.1.4", "Bewertung von und Entscheidung über Informationssicherheitsereignisse", "Stelle sicher, dass Ereignisse zeitnah klassifiziert werden. Nachweis: Klassifizierungsmatrix.", "Vorfallmanagement", "manual", 2),
		c("A.16.1.5", "Reaktion auf Informationssicherheitsvorfälle", "Definiere konkrete Reaktionsschritte je Vorfallklasse. Nachweis: IR-Playbooks.", "Vorfallmanagement", "manual", 3),
		c("A.16.1.6", "Erkenntnisse aus Informationssicherheitsvorfällen", "Führe Post-Incident-Reviews durch und leite Verbesserungen ab. Nachweis: Review-Berichte.", "Vorfallmanagement", "manual", 2),

		// A.17 — Business Continuity
		c("A.17.1", "Kontinuität der Informationssicherheit", "Stelle IS-Kontinuität im Krisenfall sicher.", "Business Continuity", "manual", 2),
		c("A.17.1.1", "Planung der Kontinuität der Informationssicherheit", "Erstelle BCM-Plan mit IS-Komponente. Nachweis: BCM-Plan-Dokument.", "Business Continuity", "manual", 2),
		c("A.17.1.2", "Implementierung der Kontinuität der Informationssicherheit", "Setze BCM-Maßnahmen technisch und organisatorisch um. Nachweis: Implementierungsnachweis.", "Business Continuity", "manual", 2),
		c("A.17.1.3", "Überprüfung, Überarbeitung und Bewertung der Kontinuität der Informationssicherheit", "Teste und überprüfe BCM-Pläne mindestens jährlich. Nachweis: Testberichte.", "Business Continuity", "manual", 1),

		// A.18 — Compliance
		c("A.18.1", "Einhaltung gesetzlicher und vertraglicher Anforderungen", "Identifiziere und erfülle alle anwendbaren Rechtspflichten.", "Compliance", "third_party", 2),
		c("A.18.1.1", "Identifizierung anwendbarer Gesetze und vertraglicher Anforderungen", "Pflege ein Compliance-Register aller relevanten Gesetze und Verträge. Nachweis: Compliance-Register.", "Compliance", "manual", 2),
		c("A.18.1.3", "Schutz von Aufzeichnungen", "Stelle Aufbewahrung und Schutz von Aufzeichnungen gem. gesetzlicher Fristen sicher. Nachweis: Aufbewahrungsrichtlinie.", "Compliance", "manual", 1),
		c("A.18.1.4", "Datenschutz und Schutz von personenbezogenen Daten", "Stelle DSGVO-Konformität sicher. Nachweis: Verzeichnis der Verarbeitungstätigkeiten, DSFA.", "Compliance", "manual", 3),
		c("A.18.2", "Überprüfung der Informationssicherheit", "Prüfe regelmäßig die Einhaltung der IS-Vorgaben.", "Compliance", "manual", 2),
		c("A.18.2.2", "Einhaltung von Sicherheitsrichtlinien und -standards", "Überprüfe technische Systeme auf Konformität mit IS-Richtlinien. Nachweis: Compliance-Scan-Berichte.", "Compliance", "manual", 2),
	}
}

func bsiControls(frameworkID, orgID string) []Control {
	return []Control{
		{FrameworkID: frameworkID, OrgID: orgID, ControlID: "BSI-ORP.1", Title: "Organisation", Domain: "Organisation", EvidenceType: "manual", Weight: 2},
		{FrameworkID: frameworkID, OrgID: orgID, ControlID: "BSI-ORP.2", Title: "Personnel", Domain: "Human Resources", EvidenceType: "manual", Weight: 1},
		{FrameworkID: frameworkID, OrgID: orgID, ControlID: "BSI-CON.3", Title: "Data Backup Policy", Domain: "Data Management", EvidenceType: "automated", Weight: 3},
		{FrameworkID: frameworkID, OrgID: orgID, ControlID: "BSI-NET.1.1", Title: "Network Architecture and Design", Domain: "Network Security", EvidenceType: "manual", Weight: 3},
		{FrameworkID: frameworkID, OrgID: orgID, ControlID: "BSI-SYS.1.1", Title: "General Server", Domain: "System Hardening", EvidenceType: "automated", Weight: 2},
		{FrameworkID: frameworkID, OrgID: orgID, ControlID: "BSI-OPS.1.1.2", Title: "Proper IT Administration", Domain: "Operations", EvidenceType: "manual", Weight: 2},
		{FrameworkID: frameworkID, OrgID: orgID, ControlID: "BSI-DER.2.1", Title: "Incident Management", Domain: "Incident Management", EvidenceType: "manual", Weight: 3},
	}
}

// craControls returns controls for the EU Cyber Resilience Act (CRA, 2024).
// Applies to manufacturers of products with digital elements sold in the EU.
func craControls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// Art. 13 — Pflichten der Hersteller
		c("CRA-1.1", "Sicherheit durch Design (Security by Design)",
			"Integriere Sicherheitsanforderungen bereits in der Entwurfsphase des Produkts. Nachweis: Threat-Modeling-Dokument, Sicherheitsarchitektur, Design-Review-Protokoll.",
			"Produktsicherheit", "manual", 3),
		c("CRA-1.2", "Risikobewertung für Produkte mit digitalen Elementen",
			"Führe eine Cybersecurity-Risikobewertung für dein Produkt durch und dokumentiere identifizierte Risiken und Gegenmaßnahmen. Nachweis: Risikoanalyse-Bericht.",
			"Produktsicherheit", "manual", 3),
		c("CRA-1.3", "Schwachstellenbehandlungsrichtlinie (PSIRT)",
			"Richte einen Product Security Incident Response Team (PSIRT)-Prozess ein. Definiere Reaktionszeiten und Kommunikationswege für gemeldete Schwachstellen. Nachweis: PSIRT-Richtlinie, Responsible-Disclosure-Policy.",
			"Produktsicherheit", "manual", 3),
		c("CRA-1.4", "Software-Stückliste (SBOM)",
			"Erstelle und pflege eine vollständige Software Bill of Materials (SBOM) für jede Produktversion im SPDX- oder CycloneDX-Format. Nachweis: SBOM-Datei, Automatisierungsnachweis im CI/CD.",
			"Produktsicherheit", "automated", 3),
		c("CRA-1.5", "Sichere Standardkonfiguration (Secure by Default)",
			"Stelle sicher, dass das Produkt in der Standardkonfiguration sicher ist (keine Standard-Passwörter, minimale offene Ports, Least Privilege). Nachweis: Konfigurationsdokumentation, Hardening-Guide.",
			"Produktsicherheit", "manual", 2),
		c("CRA-1.6", "Sicherheitsupdates und Patch-Management",
			"Stelle sicher, dass Sicherheitsupdates für mindestens 5 Jahre nach Markteinführung bereitgestellt werden. Nachweis: Update-Richtlinie, Patch-Veröffentlichungsprozess.",
			"Produktsicherheit", "manual", 3),
		c("CRA-1.7", "Schutz vor bekannten Schwachstellen",
			"Scanne alle Abhängigkeiten regelmäßig auf bekannte CVEs und behebe kritische Schwachstellen innerhalb definierter Fristen. Nachweis: Dependency-Scan-Berichte, CVE-Tracking.",
			"Produktsicherheit", "automated", 3),
		c("CRA-1.8", "Sichere Authentifizierung und Zugangskontrolle",
			"Implementiere sichere Authentifizierungsmechanismen im Produkt (keine Hardcoded-Credentials, MFA-Unterstützung, Least Privilege). Nachweis: Authentifizierungskonzept, Code-Review.",
			"Produktsicherheit", "automated", 3),
		c("CRA-1.9", "Datenschutz und Datenverschlüsselung",
			"Schütze Nutzerdaten durch Verschlüsselung (at rest und in transit). Minimiere Datenerhebung (Privacy by Design). Nachweis: Datenschutzarchitektur, Verschlüsselungsdokumentation.",
			"Produktsicherheit", "automated", 2),
		c("CRA-1.10", "Protokollierung und Überwachbarkeit",
			"Implementiere sicherheitsrelevante Protokollierung im Produkt, die Angriffe und Fehlverhalten erkennbar macht. Nachweis: Logging-Konzept, Protokollbeispiele.",
			"Produktsicherheit", "automated", 2),
		// Art. 14 — Meldepflichten
		c("CRA-2.1", "Meldung aktiv ausgenutzter Schwachstellen (ENISA)",
			"Melde aktiv ausgenutzter Schwachstellen innerhalb von 24 Stunden an ENISA bzw. die nationale CSIRT. Nachweis: Meldeprozessdokumentation, Meldungsarchiv.",
			"Meldepflichten", "manual", 3),
		c("CRA-2.2", "Schwachstellen-Offenlegungspolitik (VDP)",
			"Veröffentliche eine Vulnerability Disclosure Policy (VDP) und stelle Sicherheitsforschern einen sicheren Meldeweg bereit. Nachweis: Öffentliche VDP-Seite, security.txt.",
			"Meldepflichten", "manual", 2),
		c("CRA-2.3", "Koordinierte Schwachstellenoffenlegung (CVD)",
			"Koordiniere die Offenlegung von Schwachstellen mit Meldenden nach anerkanntem CVD-Prozess (z.B. ISO 29147). Nachweis: CVD-Prozessdokumentation.",
			"Meldepflichten", "manual", 2),
		// Anhang I — Sicherheitsanforderungen
		c("CRA-3.1", "Sichere Entwicklungsprozesse (SDLC)",
			"Integriere Security-Testing (SAST, DAST, Dependency Scanning, Fuzz Testing) in den Entwicklungslebenszyklus. Nachweis: CI/CD-Pipeline-Konfiguration, Test-Berichte.",
			"Entwicklungsprozess", "automated", 3),
		c("CRA-3.2", "Penetrationstests",
			"Führe regelmäßige Penetrationstests für das Produkt durch (mind. jährlich oder nach wesentlichen Änderungen). Nachweis: Pentest-Berichte, Maßnahmentracking.",
			"Entwicklungsprozess", "manual", 2),
		c("CRA-3.3", "Konfigurationsmanagement und Härtung",
			"Dokumentiere sichere Konfigurationsempfehlungen für Betreiber. Vermeide unsichere Protokolle und Dienste im Auslieferungszustand. Nachweis: Hardening-Guide, Konfigurationsbaseline.",
			"Entwicklungsprozess", "manual", 2),
	}
}

// doraControls returns controls for DORA — Digital Operational Resilience Act (EU 2022/2554).
// Applies to financial entities (banks, insurers, investment firms, fintechs) and their ICT providers.
func doraControls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// Art. 5-16 — ICT-Risikomanagement
		c("DORA-1.1", "ICT-Risikomanagement-Framework",
			"Implementiere ein umfassendes ICT-Risikomanagement-Framework gem. Art. 5 DORA. Identifiziere, klassifiziere und manage alle ICT-Risiken. Nachweis: ICT-Risikoregister, Framework-Dokumentation.",
			"ICT-Risikomanagement", "manual", 3),
		c("DORA-1.2", "ICT-Strategie und Governance",
			"Stelle sicher, dass die Geschäftsleitung die digitale Resilienzstrategie trägt und überwacht. Nachweis: Management-Beschlüsse, Strategie-Dokument, Governance-Struktur.",
			"ICT-Risikomanagement", "manual", 3),
		c("DORA-1.3", "Asset-Inventar (ICT-Assets)",
			"Führe ein vollständiges, aktuelles Inventar aller ICT-Assets und deren Abhängigkeiten. Nachweis: Asset-Register mit Klassifizierung und letztem Aktualisierungsdatum.",
			"ICT-Risikomanagement", "automated", 3),
		c("DORA-1.4", "Schutzmaßnahmen und Prävention",
			"Implementiere technische und organisatorische Maßnahmen zum Schutz kritischer ICT-Systeme. Nachweis: Maßnahmenkatalog, Technische Konfigurationen.",
			"ICT-Risikomanagement", "manual", 3),
		c("DORA-1.5", "Erkennung von ICT-Anomalien und -Vorfällen",
			"Implementiere Systeme zur frühzeitigen Erkennung von Anomalien, Cyberangriffen und ICT-Vorfällen (SIEM, IDS/IPS). Nachweis: SIEM-Konfiguration, Alarmierungsprotokoll.",
			"ICT-Risikomanagement", "automated", 3),
		c("DORA-1.6", "ICT-Business-Continuity-Management",
			"Erstelle und teste BCM-Pläne für alle kritischen ICT-Systeme. Definiere RTO und RPO. Nachweis: BCM-Plan, Testergebnisse, RTO/RPO-Dokumentation.",
			"ICT-Risikomanagement", "manual", 3),
		c("DORA-1.7", "Backup und Wiederherstellung",
			"Implementiere regelmäßige Backups mit verifizierten Wiederherstellungstests. Nachweis: Backup-Konfiguration, Restore-Test-Protokolle.",
			"ICT-Risikomanagement", "automated", 3),
		c("DORA-1.8", "Patch- und Schwachstellenmanagement",
			"Scanne regelmäßig auf Schwachstellen und stelle zeitnahes Patching sicher. Nachweis: Scan-Berichte, Patch-Protokoll mit Fristen.",
			"ICT-Risikomanagement", "automated", 2),
		// Art. 17-23 — ICT-bezogenes Vorfallmanagement
		c("DORA-2.1", "Klassifizierung von ICT-Vorfällen",
			"Klassifiziere ICT-Vorfälle nach den DORA-Kriterien (Art. 18) hinsichtlich Schwere und Auswirkung. Nachweis: Klassifizierungsschema, Anwendungsbeispiele.",
			"Vorfallmanagement", "manual", 3),
		c("DORA-2.2", "Meldung schwerwiegender ICT-Vorfälle (BaFin/EBA)",
			"Melde schwerwiegende ICT-Vorfälle fristgerecht an die zuständige Aufsichtsbehörde (BaFin, EBA, ECB). Nachweis: Meldetemplate, Meldungsarchiv.",
			"Vorfallmanagement", "manual", 3),
		c("DORA-2.3", "Incident-Response-Prozess",
			"Definiere klare Prozesse für Erkennung, Eindämmung, Beseitigung und Nachbereitung von ICT-Vorfällen. Nachweis: IR-Richtlinie, Playbooks, Eskalationsmatrix.",
			"Vorfallmanagement", "manual", 3),
		c("DORA-2.4", "Post-Incident-Review",
			"Führe nach jedem schwerwiegenden Vorfall eine strukturierte Nachbereitung durch (Root Cause Analysis, Lessons Learned). Nachweis: Review-Berichte.",
			"Vorfallmanagement", "manual", 2),
		// Art. 24-27 — Digital Operational Resilience Testing
		c("DORA-3.1", "Jährliche ICT-Resilienz-Tests",
			"Führe jährliche Resilienz-Tests aller kritischen ICT-Systeme durch (Vulnerability Assessments, Penetrationstests). Nachweis: Testpläne, Berichte.",
			"Resilienztests", "manual", 3),
		c("DORA-3.2", "Threat-Led Penetration Testing (TLPT)",
			"Führe für systemrelevante Institute alle 3 Jahre DORA-konforme TLPT durch. Nachweis: TLPT-Bericht (von akkreditiertem Anbieter).",
			"Resilienztests", "manual", 2),
		c("DORA-3.3", "Szenarienbasierte Resilienztests",
			"Simuliere realistische Angriffsszenarien (Red-Team-Übungen, Tabletop-Exercises) und dokumentiere Ergebnisse. Nachweis: Übungsberichte.",
			"Resilienztests", "manual", 2),
		// Art. 28-44 — IKT-Drittparteienrisiken
		c("DORA-4.1", "IKT-Drittparteienrisiko-Management",
			"Implementiere ein formales Management-Framework für IKT-Drittparteienrisiken. Nachweis: Drittparteienregister, Risikobewertungsmatrix.",
			"Drittparteienrisiken", "manual", 3),
		c("DORA-4.2", "Vertragsanforderungen für IKT-Drittanbieter",
			"Stelle sicher, dass alle IKT-Dienstleisterverträge die DORA-Mindestanforderungen (Art. 30) erfüllen. Nachweis: Vertragsvorlagen, Prüfnachweis.",
			"Drittparteienrisiken", "manual", 3),
		c("DORA-4.3", "Ausstiegsstrategie für kritische IKT-Drittanbieter",
			"Entwickle Ausstiegsstrategien für kritische IKT-Abhängigkeiten. Nachweis: Exit-Plan-Dokument.",
			"Drittparteienrisiken", "manual", 2),
	}
}

// doraISO27001Mapping maps each DORA control code to the corresponding ISO 27001:2022 Annex A clauses.
var doraISO27001Mapping = map[string]string{
	"DORA-1.1": "A.5.30, A.8.6, A.6.4",
	"DORA-1.2": "A.5.1, A.5.2, A.6.1",
	"DORA-1.3": "A.8.1, A.8.2",
	"DORA-1.4": "A.8.7, A.8.8, A.8.20",
	"DORA-1.5": "A.8.15, A.8.16",
	"DORA-1.6": "A.8.13, A.8.14",
	"DORA-1.7": "A.8.13",
	"DORA-1.8": "A.8.8, A.8.19",
	"DORA-2.1": "A.5.24, A.5.25",
	"DORA-2.2": "A.5.24, A.5.26",
	"DORA-2.3": "A.5.26, A.5.27",
	"DORA-2.4": "A.5.27",
	"DORA-3.1": "A.5.36, A.8.8",
	"DORA-3.2": "A.8.8",
	"DORA-3.3": "A.5.36",
	"DORA-4.1": "A.5.19, A.5.20",
	"DORA-4.2": "A.5.20, A.5.21",
	"DORA-4.3": "A.5.19",
}

// euAiActControls returns controls for the EU AI Act (Verordnung (EU) 2024/1689).
// Focuses on high-risk AI systems (Annex III) and general-purpose AI models.
func euAiActControls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// Art. 9 — Risikomanagementsystem
		c("AIACT-1.1", "KI-Risikomanagementsystem",
			"Implementiere ein dokumentiertes Risikomanagementsystem für Hochrisiko-KI-Systeme gem. Art. 9 EU AI Act. Identifiziere bekannte und vorhersehbare Risiken. Nachweis: Risikoregister, Framework-Dokumentation.",
			"Risikomanagement", "manual", 3),
		c("AIACT-1.2", "KI-Risikobewertung und Risikominderung",
			"Bewerte Risiken für Gesundheit, Sicherheit und Grundrechte. Implementiere Maßnahmen zur Risikominderung. Nachweis: Risikobewertungsbericht, Maßnahmenkatalog.",
			"Risikomanagement", "manual", 3),
		c("AIACT-1.3", "Klassifizierung des KI-Systems",
			"Klassifiziere alle eingesetzten KI-Systeme nach EU AI Act (verboten / Hochrisiko / begrenztes Risiko / minimales Risiko). Nachweis: Klassifizierungsmatrix mit Begründungen.",
			"Risikomanagement", "manual", 3),
		// Art. 10 — Daten und Datenverwaltung
		c("AIACT-2.1", "Qualität der Trainingsdaten",
			"Stelle sicher, dass Trainingsdaten relevant, repräsentativ und frei von systematischen Fehlern sind. Nachweis: Daten-Governance-Dokumentation, Datenqualitätsbericht.",
			"Datenverwaltung", "manual", 3),
		c("AIACT-2.2", "Datenverwaltung und Datensätze",
			"Dokumentiere Herkunft, Umfang und Verarbeitungsmethoden aller für KI verwendeten Datensätze. Nachweis: Daten-Lineage-Dokumentation, Datensatz-Inventar.",
			"Datenverwaltung", "manual", 2),
		// Art. 11 — Technische Dokumentation
		c("AIACT-3.1", "Technische Dokumentation (Annex IV)",
			"Erstelle die technische Dokumentation gem. Anhang IV EU AI Act für alle Hochrisiko-KI-Systeme. Nachweis: Technisches Dossier.",
			"Dokumentation", "manual", 3),
		c("AIACT-3.2", "Konformitätserklärung und CE-Kennzeichnung",
			"Stelle eine EU-Konformitätserklärung aus und bringe für einschlägige Hochrisiko-KI-Systeme die CE-Kennzeichnung an. Nachweis: Konformitätserklärung, Kennzeichnungsnachweis.",
			"Dokumentation", "manual", 2),
		// Art. 12 — Aufzeichnungspflichten (Logging)
		c("AIACT-4.1", "Automatisches Logging des KI-Systems",
			"Implementiere automatisches Logging für alle Hochrisiko-KI-Systeme, das Ereignisse aufzeichnet, die für Überwachung und nachträgliche Untersuchung relevant sind. Nachweis: Logging-Konzept, Protokollbeispiele.",
			"Transparenz & Logging", "automated", 3),
		// Art. 13 — Transparenz und Nutzerinformation
		c("AIACT-5.1", "Transparenz gegenüber Nutzern",
			"Informiere Nutzer klar darüber, dass sie mit einem KI-System interagieren, und stelle verständliche Informationen über Fähigkeiten und Grenzen bereit. Nachweis: Nutzerdokumentation, Informationsmaterial.",
			"Transparenz & Logging", "manual", 2),
		c("AIACT-5.2", "Kennzeichnung KI-generierter Inhalte",
			"Kennzeichne KI-generierte Inhalte (insb. Deepfakes, synthetische Medien) als solche. Nachweis: Technische Implementierung, Richtlinie.",
			"Transparenz & Logging", "manual", 2),
		// Art. 14 — Menschliche Aufsicht
		c("AIACT-6.1", "Menschliche Aufsicht (Human Oversight)",
			"Stelle sicher, dass Hochrisiko-KI-Systeme wirksam von Menschen überwacht werden können und Stopp-Mechanismen vorhanden sind. Nachweis: Aufsichtskonzept, Nachweis der Implementierung.",
			"Menschliche Aufsicht", "manual", 3),
		c("AIACT-6.2", "Schulung der Aufsichtspersonen",
			"Schule alle Personen, die KI-Systeme überwachen, zu deren Fähigkeiten, Grenzen und möglichen Risiken. Nachweis: Schulungsnachweise, Schulungsmaterial.",
			"Menschliche Aufsicht", "manual", 2),
		// Art. 15 — Genauigkeit, Robustheit und Cybersicherheit
		c("AIACT-7.1", "Genauigkeit und Leistungsmetriken",
			"Definiere und überwache Genauigkeitsmetriken für Hochrisiko-KI-Systeme. Nachweis: Leistungsberichte, Benchmark-Ergebnisse.",
			"Sicherheit & Robustheit", "automated", 2),
		c("AIACT-7.2", "Robustheit gegen adversarielle Angriffe",
			"Teste das KI-System auf Robustheit gegen adversarielle Eingaben und Data-Poisoning. Nachweis: Robustheitstests, Red-Team-Berichte.",
			"Sicherheit & Robustheit", "manual", 2),
		c("AIACT-7.3", "Cybersicherheit des KI-Systems",
			"Stelle sicher, dass das KI-System gegen Cyberangriffe geschützt ist (sichere API, Authentifizierung, Eingabevalidierung). Nachweis: Security-Review, Pentest-Bericht.",
			"Sicherheit & Robustheit", "manual", 3),
		// Art. 26 — Pflichten der Nutzer von Hochrisiko-KI-Systemen
		c("AIACT-8.1", "Konformitätsbewertung vor Inbetriebnahme",
			"Führe vor dem Einsatz von Hochrisiko-KI-Systemen eine Konformitätsbewertung durch. Nachweis: Konformitätsbewertungsbericht.",
			"Compliance", "manual", 3),
		c("AIACT-8.2", "Einschränkung auf vorgesehene Verwendung",
			"Stelle sicher, dass KI-Systeme ausschließlich für ihren vorgesehenen Verwendungszweck eingesetzt werden. Nachweis: Nutzungsrichtlinie, Schulungsnachweise.",
			"Compliance", "manual", 2),
	}
}

// iso42001Controls returns controls for ISO/IEC 42001:2023 — AI Management System Standard.
func iso42001Controls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// Kap. 4 — Kontext der Organisation
		c("42001-4.1", "Verständnis der Organisation und ihres Kontexts",
			"Bestimme interne und externe Faktoren, die für den KI-Managementsystem-Zweck relevant sind. Nachweis: Kontextanalyse-Dokument.",
			"Organisationskontext", "manual", 2),
		c("42001-4.2", "Interessierte Parteien und deren Anforderungen",
			"Identifiziere alle relevanten Stakeholder (Nutzer, Regulatoren, Betroffene) und deren Anforderungen an das KI-MS. Nachweis: Stakeholder-Register.",
			"Organisationskontext", "manual", 2),
		c("42001-4.3", "KI-Politik und Anwendungsbereich",
			"Definiere den Anwendungsbereich des KI-Managementsystems und erstelle eine KI-Politik. Nachweis: KI-Politik-Dokument, Anwendungsbereichsdefinition.",
			"Organisationskontext", "manual", 2),
		// Kap. 5 — Führung
		c("42001-5.1", "Führung und Commitment für KI-Governance",
			"Stelle sicher, dass die Unternehmensführung Verantwortung für das KI-Managementsystem übernimmt. Nachweis: Management-Beschlüsse, Governance-Dokument.",
			"Führung", "manual", 3),
		c("42001-5.2", "KI-Rollen und Verantwortlichkeiten",
			"Weise klare Rollen und Verantwortlichkeiten für KI-Entwicklung, -Betrieb und -Governance zu. Nachweis: Organigramm, Stellenbeschreibungen, Beauftragungsschreiben.",
			"Führung", "manual", 2),
		// Kap. 6 — Planung
		c("42001-6.1", "KI-Risikobeurteilung",
			"Identifiziere und bewerte Risiken aus dem Einsatz von KI-Systemen, einschließlich ethischer und gesellschaftlicher Risiken. Nachweis: KI-Risikoregister.",
			"Planung", "manual", 3),
		c("42001-6.2", "KI-Ziele und Maßnahmen",
			"Definiere messbare KI-Ziele und leite konkrete Maßnahmen zur Zielerreichung ab. Nachweis: Zieldokument, Maßnahmenplan.",
			"Planung", "manual", 2),
		// Kap. 7 — Unterstützung
		c("42001-7.1", "Kompetenz und Schulung für KI",
			"Stelle sicher, dass alle Personen, die KI-Systeme entwickeln, betreiben oder überwachen, ausreichend kompetent sind. Nachweis: Schulungspläne, Kompetenzmatrix.",
			"Unterstützung", "manual", 2),
		c("42001-7.2", "Bewusstsein für KI-Risiken",
			"Sensibilisiere alle Mitarbeitenden für KI-spezifische Risiken und ethische Aspekte. Nachweis: Awareness-Materialien, Schulungsnachweise.",
			"Unterstützung", "manual", 2),
		c("42001-7.3", "Dokumentenlenkung für KI-Artefakte",
			"Führe und kontrolliere alle KI-relevanten Dokumente (Modelle, Daten, Entscheidungen) gemäß Dokumentenlenkungsverfahren. Nachweis: Dokumentenregister, Versionskontrolle.",
			"Unterstützung", "manual", 1),
		// Kap. 8 — Betrieb
		c("42001-8.1", "KI-Lebenszyklusmanagement",
			"Manage alle KI-Systeme über ihren vollständigen Lebenszyklus (Konzeption, Entwicklung, Deployment, Betrieb, Abkündigung). Nachweis: Lebenszyklusplan, Abkündigungsrichtlinie.",
			"Betrieb", "manual", 3),
		c("42001-8.2", "KI-Impact-Assessment",
			"Führe vor der Inbetriebnahme neuer KI-Systeme ein Impact Assessment durch (ethisch, gesellschaftlich, sicherheitsbezogen). Nachweis: Assessment-Bericht.",
			"Betrieb", "manual", 3),
		c("42001-8.3", "Responsible AI — Fairness und Nicht-Diskriminierung",
			"Teste KI-Systeme auf systematische Diskriminierung (Bias) und dokumentiere Maßnahmen zur Fairness-Sicherstellung. Nachweis: Bias-Testing-Berichte, Fairness-Metriken.",
			"Betrieb", "manual", 3),
		c("42001-8.4", "Erklärbarkeit von KI-Entscheidungen",
			"Stelle sicher, dass KI-Entscheidungen in für Nutzer verständlicher Form erklärt werden können (Explainability/XAI). Nachweis: Erklärbarkeits-Konzept, Beispiele.",
			"Betrieb", "manual", 2),
		c("42001-8.5", "Überwachung und Monitoring von KI-Systemen",
			"Implementiere laufendes Monitoring der KI-System-Performance und -Drift. Nachweis: Monitoring-Dashboard, Alerting-Konfiguration.",
			"Betrieb", "automated", 2),
		// Kap. 9 — Leistungsbewertung
		c("42001-9.1", "Interne Audits des KI-Managementsystems",
			"Führe regelmäßige interne Audits des KI-MS durch. Nachweis: Auditplan, Auditberichte, Maßnahmentracking.",
			"Leistungsbewertung", "manual", 2),
		c("42001-9.2", "Management-Review für KI-Governance",
			"Halte mindestens jährlich ein Management-Review des KI-MS ab. Nachweis: Review-Protokoll, Entscheidungsdokumentation.",
			"Leistungsbewertung", "manual", 2),
		// Kap. 10 — Verbesserung
		c("42001-10.1", "Kontinuierliche Verbesserung des KI-MS",
			"Etabliere einen systematischen KVP für das KI-Managementsystem. Nachweis: Verbesserungsmaßnahmen-Tracking.",
			"Verbesserung", "manual", 1),
	}
}

// tisaxControls returns controls for TISAX® / VDA ISA 6.0.
// TISAX (Trusted Information Security Assessment Exchange) is mandatory for
// automotive suppliers handling sensitive OEM data (BMW, Mercedes, VW, Bosch, etc.).
func tisaxControls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// Kap. 1 — Informationssicherheitsrichtlinien
		c("TISAX-1.1.1", "IS-Politik und -Ziele definiert",
			"Definiere eine von der Unternehmensleitung unterzeichnete Informationssicherheitspolitik mit konkreten Schutzzielen und Geltungsbereich. Kommuniziere sie an alle Mitarbeitenden. Nachweis: genehmigtes IS-Politik-Dokument mit Datum und Unterschrift, Kommunikationsnachweis.",
			"Informationssicherheitsrichtlinien", "manual", 3),
		c("TISAX-1.1.2", "IS-Politik regelmäßig überprüft",
			"Überprüfe und aktualisiere die IS-Politik mindestens jährlich oder bei wesentlichen Änderungen der Organisation. Nachweis: Revisionshistorie mit Datum, Genehmigungsprotokoll der Unternehmensleitung.",
			"Informationssicherheitsrichtlinien", "manual", 2),
		c("TISAX-1.1.3", "Führung und Commitment der Unternehmensleitung",
			"Stelle sicher, dass die Unternehmensleitung aktiv die IS-Ziele unterstützt, ausreichende Ressourcen bereitstellt und die Wichtigkeit des ISMS kommuniziert. Nachweis: Management-Beschlüsse, Organigramm mit IS-Rolle.",
			"Informationssicherheitsrichtlinien", "manual", 3),

		// Kap. 2 — Organisation der Informationssicherheit
		c("TISAX-2.1.1", "Rollen und Verantwortlichkeiten IS",
			"Benenne einen Informationssicherheitsbeauftragten (ISB) und dokumentiere alle IS-Rollen mit Aufgaben und Befugnissen. Stelle Unabhängigkeit und ausreichende Ressourcen sicher. Nachweis: Beauftragungsschreiben, Stellenbeschreibungen, Organigramm.",
			"Organisation", "manual", 3),
		c("TISAX-2.1.2", "Kontakt zu Behörden und Fachgruppen",
			"Pflege aktuelle Kontaktinformationen zu relevanten Behörden (BSI, CERT-Bund) und Branchengruppen (VDA, ENX). Dokumentiere die Eskalationswege. Nachweis: Kontaktliste, Mitgliedschaftsnachweise.",
			"Organisation", "manual", 1),
		c("TISAX-2.1.3", "IS im Projektmanagement",
			"Integriere IS-Anforderungen in alle Projektphasen (Anforderungsanalyse, Design, Test, Abnahme). Stelle sicher, dass IS-Risiken in Projekten bewertet und behandelt werden. Nachweis: Projektcheckliste mit IS-Anforderungen, Review-Nachweise.",
			"Organisation", "manual", 2),
		c("TISAX-2.1.4", "Sicherheit beim mobilen Arbeiten",
			"Definiere Regeln und technische Maßnahmen für mobiles Arbeiten und Telearbeit (VPN, Geräteverschlüsselung, Clear-Screen). Nachweis: Mobile-Work-Richtlinie, MDM-Konfiguration, VPN-Setup.",
			"Organisation", "manual", 2),

		// Kap. 3 — Personalsicherheit
		c("TISAX-3.1.1", "Überprüfung vor der Anstellung",
			"Führe angemessene Hintergrundüberprüfungen (Lebenslauf, Zeugnisse, ggf. Führungszeugnis) vor der Einstellung durch, insbesondere für sicherheitskritische Positionen. Nachweis: Screening-Richtlinie, Dokumentation der Prüfung.",
			"Personalsicherheit", "manual", 2),
		c("TISAX-3.1.2", "IS-Bewusstsein und Schulung",
			"Schule alle Mitarbeitenden mit Zugang zu vertraulichen OEM-Informationen mindestens jährlich zu IS-Grundlagen, Umgang mit sensitiven Daten und Meldepflichten. Nachweis: Schulungsnachweise, Teilnehmerlisten, Schulungsinhalt.",
			"Personalsicherheit", "manual", 3),
		c("TISAX-3.1.3", "Disziplinarmaßnahmen bei IS-Verstößen",
			"Definiere und kommuniziere Konsequenzen bei Verstößen gegen die IS-Politik. Stelle sicher, dass Verstöße gemeldet und verfolgt werden. Nachweis: HR-Richtlinie mit Sanktionsregelung, Kommunikationsnachweis.",
			"Personalsicherheit", "manual", 2),
		c("TISAX-3.1.4", "Beendigung und Wechsel des Arbeitsverhältnisses",
			"Stelle beim Ausscheiden oder Rollenwechsel sicher, dass alle Zugänge gesperrt, Assets zurückgegeben und Vertraulichkeitspflichten kommuniziert werden. Nachweis: Offboarding-Checkliste mit Nachweisen.",
			"Personalsicherheit", "manual", 2),

		// Kap. 4 — Asset-Management
		c("TISAX-4.1.1", "Inventar der Informationsassets",
			"Führe ein vollständiges, aktuelles Inventar aller Informationsassets (Hardware, Software, Daten, Dienste) mit Eigentümer und Schutzbedarf. Nachweis: Asset-Register mit letztem Aktualisierungsdatum.",
			"Asset-Management", "automated", 3),
		c("TISAX-4.1.2", "Eigentümerschaft der Assets",
			"Weise jedem Asset einen verantwortlichen Eigentümer zu, der die Klassifizierung und Schutzmaßnahmen verantwortet. Nachweis: Asset-Register mit Eigentümer-Feld, Verantwortungsmatrix.",
			"Asset-Management", "manual", 2),
		c("TISAX-4.1.3", "Klassifizierung von Informationen",
			"Klassifiziere alle Informationen nach Schutzbedarf (mind. vertraulich/intern/öffentlich) basierend auf der Vereinbarung mit dem OEM. Beachte die VDA-Schutzklassen. Nachweis: Klassifizierungsrichtlinie, Beispiele klassifizierter Dokumente.",
			"Asset-Management", "manual", 3),
		c("TISAX-4.1.4", "Kennzeichnung von Informationen",
			"Kennzeichne alle sensitiven Dokumente und Datenträger gemäß ihrer Klassifizierung (Stempel, Metadaten, Dateinamen-Konvention). Nachweis: Kennzeichnungsrichtlinie, Beispieldokumente.",
			"Asset-Management", "manual", 2),
		c("TISAX-4.1.5", "Handhabung und Entsorgung von Assets",
			"Definiere Regeln für den sicheren Transport, die Handhabung und die datenschutzkonforme Entsorgung sensitiver Informationen und Datenträger. Nachweis: Handhabungsrichtlinie, Vernichtungsnachweise.",
			"Asset-Management", "manual", 2),

		// Kap. 5 — Zugangskontrolle
		c("TISAX-5.1.1", "Zugangskontrollrichtlinie",
			"Erstelle eine schriftliche Zugangskontrollrichtlinie nach dem Need-to-know- und Least-Privilege-Prinzip. Definiere Genehmigungsprozesse für Zugriffsrechte. Nachweis: genehmigtes Richtliniendokument.",
			"Zugangskontrolle", "manual", 3),
		c("TISAX-5.1.2", "Benutzerzugangsverwaltung",
			"Verwalte alle Benutzerkonten über einen definierten Prozess (Anlage, Änderung, Sperrung, Löschung). Überprüfe Zugriffsrechte mindestens halbjährlich. Nachweis: Provisionierungsprozess, Review-Protokolle.",
			"Zugangskontrolle", "automated", 3),
		c("TISAX-5.1.3", "Privilegierte Zugriffsrechte",
			"Verwalte Administrator- und Root-Rechte restriktiv. Nutze PAM-Lösung, Vier-Augen-Prinzip und vollständiges Logging für privilegierte Aktionen. Nachweis: PAM-Konfiguration, Admin-Protokolle.",
			"Zugangskontrolle", "automated", 3),
		c("TISAX-5.1.4", "Multi-Faktor-Authentifizierung",
			"Erzwinge MFA für den Zugriff auf Systeme mit vertraulichen OEM-Informationen und für alle Remote-Zugänge. Nachweis: MFA-Konfiguration, Ausnahmeliste mit Begründungen.",
			"Zugangskontrolle", "automated", 3),
		c("TISAX-5.1.5", "Zugang zu Netzwerken und Diensten",
			"Beschränke Netzwerkzugänge auf autorisierte Nutzer und Geräte (NAC, VPN, Zero Trust). Segmentiere Netzwerke nach Schutzbedarf. Nachweis: Netzwerkarchitektur, Zugangskontrollkonfiguration.",
			"Zugangskontrolle", "automated", 3),

		// Kap. 6 — Kryptographie
		c("TISAX-6.1.1", "Kryptographierichtlinie",
			"Definiere zulässige kryptographische Verfahren und Schlüssellängen (gemäß BSI TR-02102) für alle Anwendungsfälle. Schließe veraltete Algorithmen aus. Nachweis: Kryptographierichtlinie.",
			"Kryptographie", "manual", 2),
		c("TISAX-6.1.2", "Schlüsselverwaltung",
			"Dokumentiere den vollständigen Schlüssellebenszyklus (Generierung, Verteilung, Speicherung, Widerruf, Vernichtung). Nutze ein dediziertes Key-Management-System. Nachweis: Schlüsselverwaltungsverfahren, KMS-Konfiguration.",
			"Kryptographie", "manual", 2),
		c("TISAX-6.1.3", "Verschlüsselung sensitiver Daten",
			"Verschlüssele alle OEM-sensitiven Daten in Ruhe (AES-256) und bei der Übertragung (TLS 1.2+). Nachweis: Verschlüsselungskonfiguration, TLS-Scan-Bericht.",
			"Kryptographie", "automated", 3),

		// Kap. 7 — Physische Sicherheit
		c("TISAX-7.1.1", "Physischer Sicherheitsperimeter",
			"Definiere und sichere physische Sicherheitsbereiche (Serverräume, Büros, Entwicklungsbereiche) mit angemessenen Zugangskontrollen. Nachweis: Raumkonzept, Zutrittskontrollsystem-Dokumentation.",
			"Physische Sicherheit", "manual", 3),
		c("TISAX-7.1.2", "Zugangskontrollen für Sicherheitsbereiche",
			"Implementiere elektronische Zutrittskontrolle für Sicherheitsbereiche mit individueller Authentifizierung und Protokollierung. Beschränke den Zugang auf Befugte. Nachweis: Zutrittskontrollsystem, Zugangsprotokolle.",
			"Physische Sicherheit", "manual", 3),
		c("TISAX-7.1.3", "Sicherung von Geräten",
			"Schütze IT-Geräte physisch vor Diebstahl und unbefugtem Zugriff (Kabelsicherung, abschließbare Schränke, Bildschirmsperren). Nachweis: Sicherheitskonzept, Begehungsprotokoll.",
			"Physische Sicherheit", "manual", 2),
		c("TISAX-7.1.4", "Clear-Desk und Clear-Screen",
			"Setze Clear-Desk- und Clear-Screen-Richtlinien durch: automatische Bildschirmsperre, keine offengelegten sensitiven Dokumente. Nachweis: Richtlinie, Stichprobenprotokoll.",
			"Physische Sicherheit", "manual", 2),

		// Kap. 8 — Betriebssicherheit
		c("TISAX-8.1.1", "Dokumentierte Betriebsverfahren",
			"Erstelle und pflege aktuelle Betriebsdokumentation für alle kritischen IT-Systeme (Betriebshandbücher, Verfahrensanweisungen). Nachweis: Betriebsdokumentation mit Versionierung.",
			"Betriebssicherheit", "manual", 2),
		c("TISAX-8.1.2", "Änderungsmanagement",
			"Stelle sicher, dass alle Änderungen an IT-Systemen geplant, bewertet, genehmigt, getestet und dokumentiert werden. Nachweis: Change-Management-Prozess, Genehmigungsnachweise.",
			"Betriebssicherheit", "manual", 2),
		c("TISAX-8.1.3", "Schutz vor Schadsoftware",
			"Implementiere Endpoint-Protection-Software mit automatischen Updates auf allen Systemen mit OEM-Datenzugang. Ergänze durch EDR, E-Mail-Sicherheit und Web-Filtering. Nachweis: AV/EDR-Konfiguration, Update-Protokoll.",
			"Betriebssicherheit", "automated", 3),
		c("TISAX-8.1.4", "Datensicherung (Backup)",
			"Implementiere regelmäßige Backups nach 3-2-1-Prinzip mit Verschlüsselung. Teste die Wiederherstellung mindestens vierteljährlich. Nachweis: Backup-Konfiguration, Restore-Test-Protokolle.",
			"Betriebssicherheit", "automated", 3),
		c("TISAX-8.1.5", "Protokollierung und Überwachung",
			"Protokolliere sicherheitsrelevante Ereignisse auf allen kritischen Systemen und überwache sie zentral (SIEM). Bewahre Logs mindestens 90 Tage auf. Nachweis: Logging-Konfiguration, SIEM-Dashboard.",
			"Betriebssicherheit", "automated", 3),
		c("TISAX-8.1.6", "Schwachstellenmanagement",
			"Scanne Systeme regelmäßig auf bekannte Schwachstellen (mind. monatlich) und behebe kritische Schwachstellen innerhalb definierter Fristen. Nachweis: Scan-Berichte, Patch-Protokoll.",
			"Betriebssicherheit", "automated", 3),
		c("TISAX-8.1.7", "Trennung von Entwicklung, Test und Betrieb",
			"Trenne Entwicklungs-, Test- und Produktivumgebungen strikt. Verwende keine Produktionsdaten in Testumgebungen ohne Anonymisierung. Nachweis: Umgebungskonzept, Datenschutz-Maßnahmen.",
			"Betriebssicherheit", "manual", 2),

		// Kap. 9 — Kommunikationssicherheit
		c("TISAX-9.1.1", "Netzwerksicherheit und -segmentierung",
			"Segmentiere Netzwerke nach Schutzbedarf (DMZ, Produktions-/Entwicklungsnetz, OT-Trennung). Überwache Netzwerkverkehr auf Anomalien. Nachweis: Netzwerkplan, Firewall-Regeln, IDS-Konfiguration.",
			"Kommunikationssicherheit", "automated", 3),
		c("TISAX-9.1.2", "Sichere Datenübertragung",
			"Verschlüssele alle Übertragungen sensitiver OEM-Daten (TLS 1.2+, sichere Dateiübertragung). Schließe unsichere Protokolle (FTP, HTTP, Telnet) aus. Nachweis: Protokoll-Konfiguration, TLS-Scan.",
			"Kommunikationssicherheit", "automated", 3),
		c("TISAX-9.1.3", "Vertraulichkeitsvereinbarungen (NDAs)",
			"Stelle sicher, dass alle Personen mit Zugang zu OEM-sensitiven Informationen aktuelle NDAs unterzeichnet haben. Nachweis: NDA-Vorlagen, unterzeichnete Vereinbarungen.",
			"Kommunikationssicherheit", "manual", 3),

		// Kap. 10 — Systembeschaffung und -entwicklung
		c("TISAX-10.1.1", "Sicherheitsanforderungen für Systeme",
			"Definiere IS-Sicherheitsanforderungen vor der Beschaffung oder Entwicklung neuer Systeme, die sensitiven OEM-Daten verarbeiten. Nachweis: Anforderungsdokumentation, Beschaffungs-Checkliste.",
			"Systementwicklung", "manual", 2),
		c("TISAX-10.1.2", "Sichere Entwicklungsprozesse",
			"Integriere Sicherheit in den gesamten Entwicklungslebenszyklus (Secure SDLC): Threat Modeling, Security Code Reviews, SAST/DAST, Dependency Scanning. Nachweis: SDLC-Dokumentation, Tool-Konfiguration.",
			"Systementwicklung", "automated", 2),
		c("TISAX-10.1.3", "Sicherheitstests",
			"Führe vor jeder Produktivsetzung von Systemen mit OEM-Datenzugang Sicherheitstests durch (Penetrationstests, Schwachstellenscans). Nachweis: Testberichte, Testpläne.",
			"Systementwicklung", "manual", 2),

		// Kap. 11 — Lieferantenbeziehungen
		c("TISAX-11.1.1", "Lieferanten-Sicherheitsanforderungen",
			"Definiere IS-Mindestanforderungen für alle Lieferanten und Dienstleister mit Zugang zu sensitiven OEM-Informationen oder IS-relevanten Systemen. Nachweis: Lieferanten-Sicherheitsrichtlinie.",
			"Lieferantensicherheit", "manual", 3),
		c("TISAX-11.1.2", "Sicherheitsanforderungen in Lieferantenverträgen",
			"Verankere verbindliche IS-Anforderungen in allen relevanten Lieferantenverträgen (NDAs, AVV, Auditrechte, Vorfallmeldepflicht). Nachweis: Vertragsklauseln, Musterverträge.",
			"Lieferantensicherheit", "manual", 3),
		c("TISAX-11.1.3", "Überwachung der Lieferanten-IS-Leistung",
			"Überprüfe regelmäßig die IS-Leistung kritischer Lieferanten (Fragebögen, Audits, Zertifikate). Nachweis: Bewertungsberichte, Auditprotokolle, TISAX-Nachweise von Lieferanten.",
			"Lieferantensicherheit", "manual", 2),

		// Kap. 12 — Sicherheitsvorfälle
		c("TISAX-12.1.1", "Incident-Response-Prozess",
			"Definiere und dokumentiere einen Prozess zur Erkennung, Meldung, Bewertung, Reaktion und Nachbereitung von IS-Vorfällen. Stelle Erreichbarkeit des IR-Teams sicher. Nachweis: IR-Richtlinie, IR-Playbooks, Teambesetzungsplan.",
			"Vorfallmanagement", "manual", 3),
		c("TISAX-12.1.2", "Meldung von Vorfällen und Schwächen",
			"Etabliere einfache Meldekanäle für alle Mitarbeitenden zur Meldung von IS-Vorfällen und Schwachstellen. Garantiere Schutz vor Repressalien. Nachweis: Meldeprozess, Kontaktinformationen, Kommunikationsnachweis.",
			"Vorfallmanagement", "manual", 3),
		c("TISAX-12.1.3", "Meldepflicht gegenüber OEMs",
			"Stelle sicher, dass Vorfälle, die OEM-sensitive Daten betreffen, unverzüglich dem betroffenen OEM gemäß vertraglicher Vereinbarung gemeldet werden. Nachweis: Meldeprozess, OEM-Kontaktliste, Meldungsarchiv.",
			"Vorfallmanagement", "manual", 3),
		c("TISAX-12.1.4", "Post-Incident-Review und Lessons Learned",
			"Führe nach jedem wesentlichen Vorfall eine strukturierte Nachbereitung durch und implementiere Verbesserungsmaßnahmen. Nachweis: Post-Incident-Review-Berichte, Maßnahmentracking.",
			"Vorfallmanagement", "manual", 2),

		// Kap. 13 — Business Continuity
		c("TISAX-13.1.1", "Business-Continuity-Planung",
			"Erstelle BCM-Pläne für alle Geschäftsprozesse mit OEM-Datenzugang. Definiere RTO und RPO. Nachweis: BCM-Plan, BIA-Dokument, RTO/RPO-Tabelle.",
			"Business Continuity", "manual", 3),
		c("TISAX-13.1.2", "BCM-Tests und -Übungen",
			"Teste BCM-Pläne mindestens jährlich durch Übungen (Tabletop oder Live-Test) und dokumentiere Ergebnisse und Verbesserungen. Nachweis: Übungsprotokolle, Verbesserungsmaßnahmen.",
			"Business Continuity", "manual", 2),

		// Kap. 14 — Compliance
		c("TISAX-14.1.1", "Einhaltung gesetzlicher und vertraglicher Anforderungen",
			"Identifiziere alle anwendbaren gesetzlichen Anforderungen (DSGVO, Exportkontrolle) und vertraglichen Verpflichtungen gegenüber OEMs. Nachweis: Compliance-Register, rechtliche Prüfungsnachweise.",
			"Compliance", "manual", 3),
		c("TISAX-14.1.2", "Interne IS-Audits",
			"Führe mindestens jährlich interne IS-Audits durch und dokumentiere Befunde, Maßnahmen und Umsetzungsstatus. Nachweis: Auditplan, Auditberichte, Maßnahmentracking.",
			"Compliance", "manual", 3),
		c("TISAX-14.1.3", "TISAX-Assessment Vorbereitung",
			"Stelle sicher, dass alle TISAX-Anforderungen des gewählten Assessment-Levels (AL1/AL2/AL3) und der Schutzbedarfskategorie (Normal/Hoch/Sehr hoch) erfüllt sind. Nachweis: Gap-Analyse, Maßnahmenplan, Assessment-Bereitschaftsbericht.",
			"Compliance", "manual", 3),

		// Kap. 15 — Prototypenschutz (nur bei Prototypen-Schutzbedarf)
		c("TISAX-15.1.1", "Physische Absicherung von Prototypen",
			"Sichere Fahrzeugprototypen und Prototypenteile mit geeigneten physischen Maßnahmen (abgeschlossene Garagen, Zugangskontrolle, CCTV). Nachweis: Sicherheitskonzept Prototypenschutz, Begehungsprotokoll.",
			"Prototypenschutz", "manual", 3),
		c("TISAX-15.1.2", "Kennzeichnung von Prototypen",
			"Kennzeichne Prototypen und Prototypenteile gemäß OEM-Vorgaben (Tarnung, Abdeckungen, Kennzeichnungspflicht). Nachweis: Kennzeichnungsrichtlinie, Fotodokumentation.",
			"Prototypenschutz", "manual", 3),
		c("TISAX-15.1.3", "Transport von Prototypen",
			"Sichere den Transport von Prototypen durch geeignete Maßnahmen (abgedunkelter Transport, GPS-Tracking, Protokollierung). Nachweis: Transportrichtlinie, Transportprotokolle.",
			"Prototypenschutz", "manual", 2),
		c("TISAX-15.1.4", "Fotografierverbot und digitale Sicherheit",
			"Verbiete das unbefugte Fotografieren von Prototypen und treffe technische Maßnahmen gegen unbefugte Bildaufnahmen (Abschirmung, Kamerasperren in Sicherheitsbereichen). Nachweis: Richtlinie, technische Maßnahmen.",
			"Prototypenschutz", "manual", 3),
	}
}

// ExportFrameworkPDF generates a human-readable compliance overview PDF.
// Returns (pdfBytes, filename, error).
func (s *Service) ExportFrameworkPDF(ctx context.Context, orgID, frameworkID string) ([]byte, string, error) {
	report, err := s.GetReadinessReport(ctx, orgID, frameworkID)
	if err != nil {
		return nil, "", fmt.Errorf("get readiness report: %w", err)
	}
	gaps, err := s.GetGapAnalysis(ctx, orgID, frameworkID)
	if err != nil {
		return nil, "", fmt.Errorf("get gap analysis: %w", err)
	}
	var orgName string
	_ = s.db.QueryRow(ctx, `SELECT name FROM organizations WHERE id=$1::uuid`, orgID).Scan(&orgName)

	pdfBytes, err := GenerateFrameworkPDF(report, gaps, orgName)
	if err != nil {
		return nil, "", fmt.Errorf("generate pdf: %w", err)
	}
	filename := report.FrameworkName + " Compliance-Übersicht.pdf"
	return pdfBytes, filename, nil
}

// ExportSoAPDF generates an ISO 27001 Statement of Applicability PDF for the given framework.
// Returns (pdfBytes, filename, error).
func (s *Service) ExportSoAPDF(ctx context.Context, orgID, frameworkID string) ([]byte, string, error) {
	fw, err := s.repo.GetFramework(ctx, orgID, frameworkID)
	if err != nil {
		return nil, "", fmt.Errorf("get framework: %w", err)
	}
	rows, err := s.repo.ListControlsForSoA(ctx, orgID, frameworkID)
	if err != nil {
		return nil, "", fmt.Errorf("list controls for soa: %w", err)
	}
	var orgName string
	_ = s.db.QueryRow(ctx, `SELECT name FROM organizations WHERE id=$1::uuid`, orgID).Scan(&orgName)

	pdfBytes, err := GenerateSoAPDF(rows, fw.Name, orgName, time.Now())
	if err != nil {
		return nil, "", fmt.Errorf("generate soa pdf: %w", err)
	}
	filename := fw.Name + " — Statement of Applicability.pdf"
	return pdfBytes, filename, nil
}

// UpdateSoAMetadata persists the SoA-specific fields for a single control.
func (s *Service) UpdateSoAMetadata(ctx context.Context, orgID, controlID string, in UpdateSoAMetadataInput) error {
	return s.repo.UpdateSoAMetadata(ctx, orgID, controlID, in.Justification, in.Implementation, in.Responsible)
}

// ExportTISAXReportPDF generates a TISAX® Bereitschaftsbericht PDF.
// Returns (pdfBytes, filename, error).
func (s *Service) ExportTISAXReportPDF(ctx context.Context, orgID, frameworkID, protectionLevel, assessmentLevel string) ([]byte, string, error) {
	// Validate and default protectionLevel.
	if protectionLevel == "" {
		protectionLevel = "normal"
	}
	validProtectionLevels := map[string]bool{"normal": true, "high": true, "very_high": true}
	if !validProtectionLevels[protectionLevel] {
		return nil, "", fmt.Errorf("invalid protection_level %q: must be one of normal, high, very_high", protectionLevel)
	}

	// Validate and default assessmentLevel.
	if assessmentLevel == "" {
		assessmentLevel = "AL2"
	}
	validAssessmentLevels := map[string]bool{"AL1": true, "AL2": true, "AL3": true}
	if !validAssessmentLevels[assessmentLevel] {
		return nil, "", fmt.Errorf("invalid assessment_level %q: must be one of AL1, AL2, AL3", assessmentLevel)
	}

	report, err := s.GetReadinessReport(ctx, orgID, frameworkID)
	if err != nil {
		return nil, "", fmt.Errorf("get readiness report: %w", err)
	}

	controls, err := s.ListTISAXControls(ctx, orgID, frameworkID, protectionLevel)
	if err != nil {
		return nil, "", fmt.Errorf("list tisax controls: %w", err)
	}

	gaps, err := s.GetTISAXGapAnalysis(ctx, orgID, frameworkID)
	if err != nil {
		return nil, "", fmt.Errorf("get tisax gap analysis: %w", err)
	}

	var orgName string
	_ = s.db.QueryRow(ctx, `SELECT name FROM organizations WHERE id=$1::uuid`, orgID).Scan(&orgName)
	if orgName == "" {
		orgName = orgID
	}

	assessmentDate := time.Now().UTC()
	pdfBytes, err := GenerateTISAXReportPDF(report, controls, gaps, orgName, protectionLevel, assessmentLevel, assessmentDate)
	if err != nil {
		return nil, "", fmt.Errorf("generate tisax pdf: %w", err)
	}

	filename := "tisax-bereitschaftsbericht-" + assessmentDate.Format("2006-01-02") + ".pdf"
	return pdfBytes, filename, nil
}

// --- DORA Dashboard (Story 27.5) ---

// computeNextDeadline returns the nearest unreported DORA deadline across a list of incidents.
// A deadline qualifies if: it is non-nil, non-zero, in the future (after now), and has not been reported.
func computeNextDeadline(incidents []Incident, now time.Time) *NextDeadline {
	type candidate struct {
		incidentID   string
		title        string
		deadlineType string
		deadlineAt   time.Time
	}

	var best *candidate
	for _, inc := range incidents {
		type dlPair struct {
			deadline   *time.Time
			reportedAt *time.Time
			label      string
		}
		pairs := []dlPair{
			{inc.Deadline4h, inc.Reported4hAt, "4h"},
			{inc.Deadline24h, inc.Reported24hAt, "24h"},
			{inc.Deadline72h, inc.Reported72hAt, "72h"},
			{inc.Deadline30d, inc.Reported30dAt, "30d"},
		}
		for _, p := range pairs {
			if p.deadline == nil || p.deadline.IsZero() {
				continue
			}
			if !p.deadline.After(now) {
				continue
			}
			if p.reportedAt != nil {
				continue
			}
			// This is a valid future, unreported deadline.
			if best == nil || p.deadline.Before(best.deadlineAt) {
				best = &candidate{
					incidentID:   inc.ID,
					title:        inc.Title,
					deadlineType: p.label,
					deadlineAt:   *p.deadline,
				}
			}
		}
	}

	if best == nil {
		return nil
	}
	return &NextDeadline{
		IncidentID:   best.incidentID,
		Title:        best.title,
		DeadlineType: best.deadlineType,
		DeadlineAt:   best.deadlineAt,
	}
}

// GetDORADashboard assembles the DORA readiness dashboard for the given organisation.
// Returns ErrDORANotEnabled if DORA framework is not enabled for the org.
func (s *Service) GetDORADashboard(ctx context.Context, orgID string) (*DORADashboard, error) {
	// 1. Look up DORA framework for this org.
	framework, err := s.repo.FindFrameworkByName(ctx, orgID, "DORA")
	if err != nil {
		return nil, fmt.Errorf("find DORA framework: %w", err)
	}
	if framework == nil {
		return nil, ErrDORANotEnabled
	}

	// 2. Readiness score.
	report, err := s.GetReadinessReport(ctx, orgID, framework.ID)
	if err != nil {
		return nil, fmt.Errorf("get readiness report: %w", err)
	}

	// 3. Open critical controls (Weight >= 3, status not "covered").
	controls, err := s.ListControls(ctx, orgID, framework.ID)
	if err != nil {
		return nil, fmt.Errorf("list controls: %w", err)
	}
	openCritical := 0
	for _, c := range controls {
		if c.Weight >= 3 && c.Status != "covered" && c.Status != "not_applicable" {
			openCritical++
		}
	}

	// 4. Next deadline from DORA incidents.
	incidents, err := s.repo.ListIncidentsByType(ctx, orgID, "dora")
	if err != nil {
		return nil, fmt.Errorf("list dora incidents: %w", err)
	}
	nextDeadline := computeNextDeadline(incidents, time.Now().UTC())

	// 5. Expired suppliers.
	suppliers, err := s.ListSuppliers(ctx, orgID, nil)
	if err != nil {
		return nil, fmt.Errorf("list suppliers: %w", err)
	}
	expiredSuppliers := 0
	for _, sup := range suppliers {
		if sup.ContractStatus == "expired" {
			expiredSuppliers++
		}
	}

	// 6. TLPT overdue warning.
	_, tlptOverdue, err := s.ListResilienceTests(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list resilience tests: %w", err)
	}

	return &DORADashboard{
		ReadinessPct:         report.ReadinessScore,
		OpenCriticalControls: openCritical,
		NextDeadline:         nextDeadline,
		ExpiredSuppliers:     expiredSuppliers,
		TLPTOverdueWarning:   tlptOverdue,
	}, nil
}

// ExportDORAPDF generates the DORA readiness PDF for the given organisation.
func (s *Service) ExportDORAPDF(ctx context.Context, orgID string) ([]byte, error) {
	dashboard, err := s.GetDORADashboard(ctx, orgID)
	if err != nil {
		return nil, err
	}
	var orgName string
	_ = s.db.QueryRow(ctx, `SELECT name FROM organizations WHERE id=$1::uuid`, orgID).Scan(&orgName)
	if orgName == "" {
		orgName = orgID
	}
	return GenerateDORAPDF(dashboard, orgName)
}

// --- Questionnaire Builder (Story 29.2) ---

// needsSeed returns true when no templates are present and seeding is required.
func needsSeed(templates []Questionnaire) bool {
	return len(templates) == 0
}

// SeedBuiltinQuestionnaires creates the 3 built-in questionnaire templates if they don't exist.
// Idempotent: does nothing if templates are already present.
func (s *Service) SeedBuiltinQuestionnaires(ctx context.Context, orgID string) error {
	isTemplate := true
	existing, err := s.repo.ListQuestionnaires(ctx, orgID, &isTemplate)
	if err != nil {
		return fmt.Errorf("seed questionnaires: list existing: %w", err)
	}
	if !needsSeed(existing) {
		return nil
	}

	type templateDef struct {
		name      string
		questions []string
	}
	templates := []templateDef{
		{
			name: "NIS2 Lieferanten-Assessment",
			questions: []string{
				"Netzwerksicherheit",
				"Zugriffskontrollen",
				"Incident-Response",
				"Backup",
				"Patch-Management",
				"Supply-Chain-Checks",
				"Kryptographie",
				"Physische Sicherheit",
				"Personalschulungen",
				"Auditlogs",
			},
		},
		{
			name: "DORA IKT-Drittanbieter",
			questions: []string{
				"IKT-Risikomanagement",
				"Incident-Klassifizierung",
				"Resilienztests",
				"Drittanbieter-Verträge",
				"Informationsaustausch",
				"Wiederherstellungstests",
				"Aufsichtsmeldung",
				"Kontrollrahmen",
			},
		},
		{
			name: "ISO 27001 Basischeck",
			questions: []string{
				"Asset-Inventar",
				"Risikobehandlung",
				"Zugriffsrechte",
				"Kryptographie",
				"Lieferantensicherheit",
				"Compliance",
				"Awareness",
				"Audit",
				"Business-Continuity",
				"HR-Sicherheit",
				"Physische Kontrollen",
				"Kommunikationssicherheit",
			},
		},
	}

	for _, t := range templates {
		q, err := s.repo.CreateQuestionnaire(ctx, orgID, t.name, "", true)
		if err != nil {
			return fmt.Errorf("seed questionnaire %q: %w", t.name, err)
		}
		for _, text := range t.questions {
			if _, err := s.repo.CreateQuestion(ctx, q.ID, text, "yes_no", nil, true, nil); err != nil {
				return fmt.Errorf("seed question %q: %w", text, err)
			}
		}
	}
	return nil
}

// ListTemplates seeds built-in templates (if needed) then returns all templates.
func (s *Service) ListTemplates(ctx context.Context, orgID string) ([]Questionnaire, error) {
	if err := s.SeedBuiltinQuestionnaires(ctx, orgID); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("seed built-in questionnaires")
	}
	isTemplate := true
	templates, err := s.repo.ListQuestionnaires(ctx, orgID, &isTemplate)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	// Load questions for each template.
	for i := range templates {
		questions, err := s.repo.ListQuestions(ctx, templates[i].ID)
		if err != nil {
			return nil, fmt.Errorf("list template questions: %w", err)
		}
		templates[i].Questions = questions
	}
	return templates, nil
}

// ListQuestionnaires returns questionnaires optionally filtered by is_template.
func (s *Service) ListQuestionnaires(ctx context.Context, orgID string, isTemplate *bool) ([]Questionnaire, error) {
	return s.repo.ListQuestionnaires(ctx, orgID, isTemplate)
}

// GetQuestionnaire returns a single questionnaire with its questions.
func (s *Service) GetQuestionnaire(ctx context.Context, orgID, id string) (*Questionnaire, error) {
	return s.repo.GetQuestionnaire(ctx, orgID, id)
}

// CreateQuestionnaire creates a new questionnaire, cloning from a source if CloneFromID is set.
func (s *Service) CreateQuestionnaire(ctx context.Context, orgID string, in CreateQuestionnaireInput) (*Questionnaire, error) {
	if in.CloneFromID != "" {
		return s.CloneQuestionnaire(ctx, orgID, in.CloneFromID, in.Name)
	}
	return s.repo.CreateQuestionnaire(ctx, orgID, in.Name, in.Description, in.IsTemplate)
}

// CloneQuestionnaire copies a questionnaire and all its questions.
func (s *Service) CloneQuestionnaire(ctx context.Context, orgID, sourceID, name string) (*Questionnaire, error) {
	return s.repo.CloneQuestionnaire(ctx, orgID, sourceID, name)
}

// UpdateQuestionnaire updates questionnaire metadata.
func (s *Service) UpdateQuestionnaire(ctx context.Context, orgID, id string, in UpdateQuestionnaireInput) (*Questionnaire, error) {
	return s.repo.UpdateQuestionnaire(ctx, orgID, id, in.Name, in.Description, in.IsTemplate)
}

// DeleteQuestionnaire removes a questionnaire.
func (s *Service) DeleteQuestionnaire(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteQuestionnaire(ctx, orgID, id)
}

// AddQuestion adds a question to a questionnaire.
// For multiple_choice type, options must be non-empty.
func (s *Service) AddQuestion(ctx context.Context, orgID, questionnaireID string, in CreateQuestionInput) (*Question, error) {
	if in.QuestionType == "multiple_choice" && len(in.Options) == 0 {
		return nil, fmt.Errorf("multiple_choice question requires non-empty options")
	}
	// Verify org owns the questionnaire.
	if _, err := s.repo.GetQuestionnaire(ctx, orgID, questionnaireID); err != nil {
		return nil, fmt.Errorf("questionnaire not found or access denied: %w", err)
	}
	var controlID *string
	if in.ControlID != "" {
		controlID = &in.ControlID
	}
	return s.repo.CreateQuestion(ctx, questionnaireID, in.QuestionText, in.QuestionType, in.Options, in.Required, controlID)
}

// UpdateQuestion updates an existing question.
func (s *Service) UpdateQuestion(ctx context.Context, orgID, questionnaireID, questionID string, in CreateQuestionInput) (*Question, error) {
	if in.QuestionType == "multiple_choice" && len(in.Options) == 0 {
		return nil, fmt.Errorf("multiple_choice question requires non-empty options")
	}
	if _, err := s.repo.GetQuestionnaire(ctx, orgID, questionnaireID); err != nil {
		return nil, fmt.Errorf("questionnaire not found or access denied: %w", err)
	}
	var controlID *string
	if in.ControlID != "" {
		controlID = &in.ControlID
	}
	return s.repo.UpdateQuestion(ctx, questionnaireID, questionID, in.QuestionText, in.QuestionType, in.Options, in.Required, controlID)
}

// DeleteQuestion removes a question from a questionnaire.
func (s *Service) DeleteQuestion(ctx context.Context, orgID, questionnaireID, questionID string) error {
	if _, err := s.repo.GetQuestionnaire(ctx, orgID, questionnaireID); err != nil {
		return fmt.Errorf("questionnaire not found or access denied: %w", err)
	}
	return s.repo.DeleteQuestion(ctx, questionnaireID, questionID)
}

// ReorderQuestions updates the order of questions in a questionnaire.
func (s *Service) ReorderQuestions(ctx context.Context, orgID, questionnaireID string, order []string) error {
	if _, err := s.repo.GetQuestionnaire(ctx, orgID, questionnaireID); err != nil {
		return fmt.Errorf("questionnaire not found or access denied: %w", err)
	}
	return s.repo.ReorderQuestions(ctx, questionnaireID, order)
}

// --- Supplier Portal Assessments (Story 29.3) ---

// ErrAssessmentExpiredOrSubmitted is returned when a token references an expired or already-submitted assessment.
var ErrAssessmentExpiredOrSubmitted = errors.New("assessment_expired_or_submitted")

// hashToken computes the SHA-256 hex hash of a raw token. Exported for testing.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// CreateAssessment generates a token, inserts a supplier assessment, sends an invite email,
// and returns the assessment with share URL and the raw token.
func (s *Service) CreateAssessment(ctx context.Context, orgID, supplierID string, in CreateAssessmentInput, baseURL string) (*AssessmentWithQuestionnaire, string, error) {
	// Validate org owns supplier.
	supplier, err := s.repo.GetSupplier(ctx, orgID, supplierID)
	if err != nil {
		return nil, "", fmt.Errorf("supplier not found: %w", err)
	}

	// Validate questionnaire belongs to org.
	qnr, err := s.repo.GetQuestionnaire(ctx, orgID, in.QuestionnaireID)
	if err != nil {
		return nil, "", fmt.Errorf("questionnaire not found: %w", err)
	}

	rawToken, tokenHash, err := generateToken()
	if err != nil {
		return nil, "", fmt.Errorf("generate assessment token: %w", err)
	}

	expiresAt := time.Now().UTC().Add(time.Duration(in.ExpiresInDays) * 24 * time.Hour)

	a := Assessment{
		OrgID:           orgID,
		SupplierID:      supplierID,
		QuestionnaireID: in.QuestionnaireID,
		TokenHash:       tokenHash,
		ExpiresAt:       expiresAt,
		Status:          "pending",
	}
	if err := s.repo.CreateAssessment(ctx, a); err != nil {
		return nil, "", fmt.Errorf("create assessment: %w", err)
	}

	// Fetch the newly created assessment by token hash.
	created, err := s.repo.GetAssessmentByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, "", fmt.Errorf("fetch created assessment: %w", err)
	}

	shareURL := baseURL + "/supplier/" + rawToken

	// Send invite email to supplier contact via SMTP (non-fatal).
	if supplier.ContactEmail != "" && s.notifSvc != nil {
		body := strings.ReplaceAll(EmailSupplierInviteBodyDE, "{{.ShareURL}}", shareURL)
		body = strings.ReplaceAll(body, "{{.ExpiresAt}}", expiresAt.Format("02.01.2006"))
		if err := s.notifSvc.Notify(ctx, notify.Message{
			Title:   EmailSupplierInviteSubjectDE,
			Body:    body,
			OrgID:   orgID,
			Channel: notify.ChannelEmail,
			Target:  supplier.ContactEmail,
		}); err != nil {
			log.Warn().Err(err).Str("supplier_id", supplierID).Msg("send assessment invite email")
		}
	}

	result := &AssessmentWithQuestionnaire{
		Assessment:    *created,
		Questionnaire: qnr,
		ShareURL:      shareURL,
	}
	return result, rawToken, nil
}

// GetAssessmentForPortal looks up an assessment by raw token, transitions pending→in_progress,
// and returns the assessment with questionnaire+questions.
func (s *Service) GetAssessmentForPortal(ctx context.Context, rawToken string) (*AssessmentWithQuestionnaire, error) {
	hash := hashToken(rawToken)
	a, err := s.repo.GetAssessmentByTokenHash(ctx, hash)
	if err != nil {
		return nil, ErrAssessmentExpiredOrSubmitted
	}
	if time.Now().UTC().After(a.ExpiresAt) || a.Status == "submitted" || a.Status == "reviewed" {
		return nil, ErrAssessmentExpiredOrSubmitted
	}

	// Transition pending → in_progress.
	if a.Status == "pending" {
		if err := s.repo.UpdateAssessmentStatus(ctx, a.ID, "in_progress", nil, "", ""); err != nil {
			log.Warn().Err(err).Str("assessment_id", a.ID).Msg("failed to transition assessment to in_progress")
		} else {
			a.Status = "in_progress"
		}
	}

	result, err := s.repo.GetAssessmentWithQuestionnaire(ctx, a.ID)
	if err != nil {
		return nil, fmt.Errorf("get assessment with questionnaire: %w", err)
	}
	return result, nil
}

// SaveAnswers upserts answers for an in-progress assessment (intermediate save).
func (s *Service) SaveAnswers(ctx context.Context, rawToken string, in SaveAnswersInput) error {
	hash := hashToken(rawToken)
	a, err := s.repo.GetAssessmentByTokenHash(ctx, hash)
	if err != nil {
		return ErrAssessmentExpiredOrSubmitted
	}
	if time.Now().UTC().After(a.ExpiresAt) || a.Status == "submitted" || a.Status == "reviewed" {
		return ErrAssessmentExpiredOrSubmitted
	}
	return s.repo.UpsertAnswers(ctx, a.ID, in.Answers)
}

// SubmitAssessment upserts final answers, marks the assessment as submitted,
// and sends confirmation emails.
func (s *Service) SubmitAssessment(ctx context.Context, rawToken, clientIP, userAgent string, in SaveAnswersInput) error {
	hash := hashToken(rawToken)
	a, err := s.repo.GetAssessmentByTokenHash(ctx, hash)
	if err != nil {
		return ErrAssessmentExpiredOrSubmitted
	}
	if time.Now().UTC().After(a.ExpiresAt) || a.Status == "submitted" || a.Status == "reviewed" {
		return ErrAssessmentExpiredOrSubmitted
	}

	if err := s.repo.UpsertAnswers(ctx, a.ID, in.Answers); err != nil {
		return fmt.Errorf("upsert answers on submit: %w", err)
	}

	now := time.Now().UTC()
	if err := s.repo.UpdateAssessmentStatus(ctx, a.ID, "submitted", &now, clientIP, userAgent); err != nil {
		if strings.Contains(err.Error(), "already submitted") {
			return ErrAssessmentExpiredOrSubmitted
		}
		return fmt.Errorf("update assessment status: %w", err)
	}

	// Update supplier assessment_status to completed.
	_ = s.repo.UpdateSupplierAssessmentStatus(ctx, a.OrgID, a.SupplierID, "completed", &now)

	// Send confirmation email to supplier contact (non-fatal) + in-app internal notification.
	if s.notifSvc != nil {
		if supplier, sErr := s.repo.GetSupplier(ctx, a.OrgID, a.SupplierID); sErr == nil && supplier.ContactEmail != "" {
			if err := s.notifSvc.Notify(ctx, notify.Message{
				Title:   EmailSupplierConfirmSubjectDE,
				Body:    EmailSupplierConfirmBodyDE,
				OrgID:   a.OrgID,
				Channel: notify.ChannelEmail,
				Target:  supplier.ContactEmail,
			}); err != nil {
				log.Warn().Err(err).Str("assessment_id", a.ID).Msg("send assessment confirmation email")
			}
		}
	}

	internalBody := strings.ReplaceAll(EmailComplianceNotifyBodyDE, "{{.AssessmentID}}", a.ID)
	internalBody = strings.ReplaceAll(internalBody, "{{.SupplierID}}", a.SupplierID)
	notify.Send(ctx, s.db, a.OrgID, EmailComplianceNotifySubjectDE, internalBody, "supplier_assessment_submitted", "secvitals")

	return nil
}

// ListAssessmentsForSupplier returns all assessments for a given supplier.
func (s *Service) ListAssessmentsForSupplier(ctx context.Context, orgID, supplierID string) ([]Assessment, error) {
	return s.repo.ListAssessmentsForSupplier(ctx, orgID, supplierID)
}

// GetAssessment returns a single assessment by ID (with questionnaire).
func (s *Service) GetAssessment(ctx context.Context, orgID, id string) (*AssessmentWithQuestionnaire, error) {
	a, err := s.repo.GetAssessmentWithQuestionnaire(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("assessment not found: %w", err)
	}
	if a.OrgID != orgID {
		return nil, fmt.Errorf("assessment not found")
	}
	return a, nil
}

// --- Assessment Review (Story 29.4) ---

// computeStatus is a pure function (no DB) — fully unit-testable.
// Returns green/yellow/red based on assessment state and supplier metadata.
func computeStatus(supplier Supplier, assessments []Assessment, answers []AnswerWithReview, now time.Time) SupplierStatus {
	st := SupplierStatus{
		SupplierID: supplier.ID,
		Details:    map[string]any{},
	}

	// No assessment ever: red for critical, yellow otherwise
	if len(assessments) == 0 {
		if supplier.Criticality == "critical" {
			st.Status = "red"
			st.Score = 0
			st.Details["reason"] = "no_assessment_critical"
		} else {
			st.Status = "yellow"
			st.Score = 25
			st.Details["reason"] = "no_assessment"
		}
		return st
	}

	latest := assessments[0]

	// Pending or in-progress assessment → yellow
	if latest.Status == "pending" || latest.Status == "in_progress" {
		st.Status = "yellow"
		st.Score = 50
		st.Details["reason"] = "assessment_pending"
		st.Details["assessment_status"] = latest.Status
		return st
	}

	// Contract ending within 90 days → at most yellow
	contractWarning := false
	if supplier.ContractEnd != nil {
		daysLeft := supplier.ContractEnd.Sub(now).Hours() / 24
		if daysLeft >= 0 && daysLeft < 90 {
			contractWarning = true
			st.Details["contract_days_left"] = int(daysLeft)
		}
	}

	// Check review results when assessment is reviewed
	if latest.Status == "reviewed" && len(answers) > 0 {
		total := len(answers)
		accepted := 0
		rework := 0
		for _, a := range answers {
			if a.ReviewStatus != nil {
				switch *a.ReviewStatus {
				case "accepted":
					accepted++
				case "needs_rework":
					rework++
				}
			}
		}
		score := 0
		if total > 0 {
			score = (accepted * 100) / total
		}
		st.Score = score
		st.Details["total_answers"] = total
		st.Details["accepted"] = accepted
		st.Details["needs_rework"] = rework

		if rework > 0 {
			st.Status = "red"
			st.Details["reason"] = "needs_rework"
			return st
		}
		if contractWarning {
			st.Status = "yellow"
			st.Details["reason"] = "contract_expiring"
			return st
		}
		st.Status = "green"
		return st
	}

	// Submitted but not yet reviewed
	if latest.Status == "submitted" {
		st.Status = "yellow"
		st.Score = 60
		st.Details["reason"] = "awaiting_review"
		return st
	}

	// Fallback
	if contractWarning {
		st.Status = "yellow"
		st.Score = 40
		st.Details["reason"] = "contract_expiring"
		return st
	}
	st.Status = "yellow"
	st.Score = 30
	st.Details["reason"] = "incomplete"
	return st
}

// ComputeSupplierStatus fetches supplier + assessment data and delegates to computeStatus.
func (s *Service) ComputeSupplierStatus(ctx context.Context, orgID, supplierID string) (*SupplierStatus, error) {
	supplier, err := s.repo.GetSupplier(ctx, orgID, supplierID)
	if err != nil {
		return nil, fmt.Errorf("compute supplier status: get supplier: %w", err)
	}
	assessments, err := s.repo.GetAssessmentsForSupplier(ctx, orgID, supplierID)
	if err != nil {
		return nil, fmt.Errorf("compute supplier status: get assessments: %w", err)
	}
	var answers []AnswerWithReview
	if len(assessments) > 0 && assessments[0].Status == "reviewed" {
		answers, err = s.repo.GetAnswersForAssessment(ctx, orgID, assessments[0].ID)
		if err != nil {
			return nil, fmt.Errorf("compute supplier status: get answers: %w", err)
		}
	}
	result := computeStatus(*supplier, assessments, answers, time.Now().UTC())
	return &result, nil
}

// ReviewAnswer validates input, saves review status, and optionally creates evidence.
// Returns the created evidence ID (or nil if none was created).
func (s *Service) ReviewAnswer(ctx context.Context, orgID, assessmentID, answerID string, in ReviewAnswerInput) (*string, error) {
	if in.ReviewStatus != "accepted" && in.ReviewStatus != "needs_rework" {
		return nil, fmt.Errorf("review_status must be accepted or needs_rework")
	}
	if err := s.repo.UpdateAnswerReview(ctx, orgID, assessmentID, answerID, in.ReviewStatus, in.ReworkNote); err != nil {
		return nil, err
	}
	if in.ReviewStatus != "accepted" {
		return nil, nil
	}
	// Load answer+question to check for control_id
	aq, err := s.repo.GetAnswerWithQuestion(ctx, orgID, answerID)
	if err != nil || aq.ControlID == nil {
		return nil, nil
	}
	// Load supplier name for the evidence title
	a, err := s.repo.GetAssessmentWithQuestionnaire(ctx, assessmentID)
	if err != nil {
		return nil, nil
	}
	supplier, err := s.repo.GetSupplier(ctx, orgID, a.SupplierID)
	if err != nil {
		return nil, nil
	}
	title := "Lieferant " + supplier.Name + ": " + aq.QuestionText
	ev, err := s.repo.AddEvidence(ctx, orgID, *aq.ControlID, orgID, AddEvidenceInput{
		Title:  title,
		Source: "supplier_assessment",
	})
	if err != nil {
		log.Warn().Err(err).Str("answer_id", answerID).Msg("create evidence from assessment")
		return nil, nil
	}
	return &ev.ID, nil
}

// MarkAssessmentReviewed sets assessment=reviewed and supplier=completed.
func (s *Service) MarkAssessmentReviewed(ctx context.Context, orgID, assessmentID string) error {
	return s.repo.MarkAssessmentReviewed(ctx, orgID, assessmentID)
}

// GetAnswersForAssessment returns all answers for the review UI.
func (s *Service) GetAnswersForAssessment(ctx context.Context, orgID, assessmentID string) ([]AnswerWithReview, error) {
	return s.repo.GetAnswersForAssessment(ctx, orgID, assessmentID)
}

// FindExpiringCertificates returns certificate answers expiring within withinDays.
func (s *Service) FindExpiringCertificates(ctx context.Context, orgID string, withinDays int) ([]CertExpiryWarning, error) {
	before := time.Now().UTC().AddDate(0, 0, withinDays)
	return s.repo.FindExpiringCerts(ctx, orgID, before)
}

// GenerateAssessmentReportPDF builds a PDF report for an assessment.
func (s *Service) GenerateAssessmentReportPDF(ctx context.Context, orgID, assessmentID string) ([]byte, error) {
	asm, err := s.repo.GetAssessmentWithQuestionnaire(ctx, assessmentID)
	if err != nil || asm.OrgID != orgID {
		return nil, ErrNotFound
	}
	supplier, err := s.repo.GetSupplier(ctx, orgID, asm.SupplierID)
	if err != nil {
		return nil, fmt.Errorf("generate assessment pdf: get supplier: %w", err)
	}
	answers, err := s.repo.GetAnswersForAssessment(ctx, orgID, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("generate assessment pdf: get answers: %w", err)
	}
	assessments, _ := s.repo.GetAssessmentsForSupplier(ctx, orgID, asm.SupplierID)
	status := computeStatus(*supplier, assessments, answers, time.Now().UTC())
	return GenerateAssessmentReportPDFBytes(asm, supplier, answers, status)
}

// ClassifyAISystem saves a classification from the wizard and updates the AI system record.
func (s *Service) ClassifyAISystem(ctx context.Context, orgID, systemID string, in ClassifyAISystemInput) error {
	validClasses := map[string]bool{"minimal": true, "limited": true, "high": true, "unacceptable": true}
	if !validClasses[in.RiskClass] {
		return fmt.Errorf("risk_class: must be one of minimal, limited, high, unacceptable")
	}
	if _, err := s.repo.InsertAIClassification(ctx, orgID, systemID, in); err != nil {
		return fmt.Errorf("insert ai classification: %w", err)
	}
	return s.repo.UpdateAISystemClassification(ctx, orgID, systemID, in)
}

// ListAIClassifications returns the classification history for an AI system.
func (s *Service) ListAIClassifications(ctx context.Context, orgID, systemID string) ([]AIClassification, error) {
	return s.repo.ListAIClassifications(ctx, orgID, systemID)
}

// SaveAIDocumentation creates a new documentation version for an AI system.
func (s *Service) SaveAIDocumentation(ctx context.Context, orgID, systemID string, in UpsertAIDocumentationInput) (*AIDocumentation, error) {
	return s.repo.UpsertAIDocumentation(ctx, orgID, systemID, in)
}

// GetLatestAIDocumentation returns the most recent documentation for an AI system.
func (s *Service) GetLatestAIDocumentation(ctx context.Context, orgID, systemID string) (*AIDocumentation, error) {
	doc, err := s.repo.GetLatestAIDocumentation(ctx, orgID, systemID)
	if err != nil {
		return nil, ErrNotFound
	}
	return doc, nil
}

// ListAIDocumentationVersions returns all saved versions.
func (s *Service) ListAIDocumentationVersions(ctx context.Context, orgID, systemID string) ([]AIDocumentation, error) {
	return s.repo.ListAIDocumentationVersions(ctx, orgID, systemID)
}

// ExportAIDocumentationPDF generates the PDF technical dossier for an AI system.
func (s *Service) ExportAIDocumentationPDF(ctx context.Context, orgID, systemID string) ([]byte, string, error) {
	system, err := s.repo.GetAISystem(ctx, orgID, systemID)
	if err != nil {
		return nil, "", ErrNotFound
	}
	doc, err := s.repo.GetLatestAIDocumentation(ctx, orgID, systemID)
	if err != nil {
		// Return PDF with empty documentation fields if none saved yet
		doc = &AIDocumentation{AISystemID: systemID, Version: 0}
	}
	pdfBytes, err := GenerateAIDocumentationPDF(system, doc)
	if err != nil {
		return nil, "", fmt.Errorf("generate ai documentation pdf: %w", err)
	}
	filename := fmt.Sprintf("ai-dossier-%s-v%d.pdf", system.Name, doc.Version)
	return pdfBytes, filename, nil
}

const euAIActHighRiskDeadline = "2026-08-02"

// GetEUAIActDashboard builds the EU AI Act compliance dashboard for an organisation.
func (s *Service) GetEUAIActDashboard(ctx context.Context, orgID string) (*EUAIActDashboard, error) {
	total, byRisk, byStatus, withoutDocs, err := s.repo.GetEUAIActStats(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("get eu ai act dashboard: %w", err)
	}
	deadline, _ := time.Parse("2006-01-02", euAIActHighRiskDeadline)
	daysLeft := int(time.Until(deadline).Hours() / 24)
	if daysLeft < 0 {
		daysLeft = 0
	}
	return &EUAIActDashboard{
		TotalSystems:             total,
		SystemsByRiskClass:       byRisk,
		SystemsByStatus:          byStatus,
		SystemsWithoutDocs:       withoutDocs,
		HighRiskDeadline:         euAIActHighRiskDeadline,
		HighRiskDeadlineDaysLeft: daysLeft,
		ISO27001Mappings:         euAIActISOMappings,
	}, nil
}

// ExportEUAIActReportPDF generates the full EU AI Act compliance report PDF.
func (s *Service) ExportEUAIActReportPDF(ctx context.Context, orgID string) ([]byte, error) {
	dashboard, err := s.GetEUAIActDashboard(ctx, orgID)
	if err != nil {
		return nil, err
	}
	systems, err := s.repo.ListAISystems(ctx, orgID, AISystemFilters{})
	if err != nil {
		return nil, fmt.Errorf("list ai systems for pdf: %w", err)
	}
	return GenerateEUAIActReportPDF(dashboard, systems)
}

// dsgvoToISOMappings maps each DSGVO Art. 32 TOM control ID to its primary ISO 27001 control ID.
var dsgvoToISOMappings = map[string]string{
	"TOM-1":  "A.9.1.2",  // Zutrittskontrolle → Netzwerkzugänge
	"TOM-2":  "A.9.4.2",  // Zugangskontrolle → MFA/Anmeldeverfahren
	"TOM-3":  "A.9.2.2",  // Zugriffskontrolle → Zugangsprovisionierung
	"TOM-4":  "A.14.1.2", // Weitergabekontrolle → Absicherung öffentlicher Dienste
	"TOM-5":  "A.12.1.1", // Eingabekontrolle → Betriebsverfahren/Protokollierung
	"TOM-6":  "A.18.1.1", // Auftragskontrolle → Compliance-Anforderungen
	"TOM-7":  "A.12.3.1", // Verfügbarkeitskontrolle → Datensicherung
	"TOM-8":  "A.6.1.2",  // Trennungsgebot → Aufgabentrennung
	"TOM-9":  "A.10.1.1", // Pseudonymisierung → Kryptographierichtlinie
	"TOM-10": "A.10.1.2", // Verschlüsselung → Schlüsselverwaltung
	"TOM-11": "A.12.1.2", // Integrität → Änderungsmanagement
	"TOM-12": "A.17.1.2", // Wiederherstellung → BCM-Implementierung
	"TOM-13": "A.18.1.1", // Überprüfungsverfahren → Compliance-Register
}

// SeedDSGVOMappings idempotently seeds DSGVO-TOM → ISO 27001 mappings.
// Returns nil if either framework is not yet enabled.
func (s *Service) SeedDSGVOMappings(ctx context.Context, orgID string) error {
	dsgvoFW, err := s.repo.FindFrameworkByName(ctx, orgID, "DSGVO-TOM")
	if err != nil {
		return fmt.Errorf("find DSGVO-TOM framework: %w", err)
	}
	if dsgvoFW == nil {
		return nil
	}

	isoFW, err := s.repo.FindFrameworkByName(ctx, orgID, "ISO27001")
	if err != nil {
		return fmt.Errorf("find ISO27001 framework: %w", err)
	}
	if isoFW == nil {
		return nil
	}

	dsgvoControls, err := s.repo.ListControls(ctx, orgID, dsgvoFW.ID)
	if err != nil {
		return fmt.Errorf("list DSGVO-TOM controls: %w", err)
	}
	isoControls, err := s.repo.ListControls(ctx, orgID, isoFW.ID)
	if err != nil {
		return fmt.Errorf("list ISO27001 controls: %w", err)
	}

	dsgvoByID := make(map[string]string, len(dsgvoControls))
	for _, c := range dsgvoControls {
		dsgvoByID[c.ControlID] = c.ID
	}
	isoByID := make(map[string]string, len(isoControls))
	for _, c := range isoControls {
		isoByID[c.ControlID] = c.ID
	}

	for tomID, isoID := range dsgvoToISOMappings {
		tomUUID, ok1 := dsgvoByID[tomID]
		isoUUID, ok2 := isoByID[isoID]
		if !ok1 || !ok2 {
			continue
		}
		if _, err := s.repo.CreateMapping(ctx, orgID, tomUUID, isoUUID); err != nil {
			log.Warn().Err(err).Str("tom", tomID).Str("iso", isoID).Msg("seed DSGVO mapping failed")
		}
	}
	return nil
}

// GetDSGVOTOMCoverage returns coverage status for each TOM based on mapped ISO 27001 controls.
func (s *Service) GetDSGVOTOMCoverage(ctx context.Context, orgID, dsgvoFrameworkID string) ([]MappingResult, error) {
	tomControls, err := s.ListControls(ctx, orgID, dsgvoFrameworkID)
	if err != nil {
		return nil, fmt.Errorf("list DSGVO-TOM controls: %w", err)
	}

	isoFW, err := s.repo.FindFrameworkByName(ctx, orgID, "ISO27001")
	if err != nil {
		return nil, fmt.Errorf("find ISO27001 framework: %w", err)
	}

	var isoControls []Control
	var evidenceCounts map[string]int
	if isoFW != nil {
		isoControls, err = s.ListControls(ctx, orgID, isoFW.ID)
		if err != nil {
			return nil, fmt.Errorf("list ISO27001 controls: %w", err)
		}
		evidenceCounts, err = s.repo.CountEvidenceByControl(ctx, orgID, isoFW.ID)
		if err != nil {
			return nil, fmt.Errorf("count ISO27001 evidence: %w", err)
		}
	}

	isoByControlID := make(map[string]Control, len(isoControls))
	for _, c := range isoControls {
		isoByControlID[c.ControlID] = c
	}
	if evidenceCounts == nil {
		evidenceCounts = map[string]int{}
	}

	results := make([]MappingResult, 0, len(tomControls))
	for _, tom := range tomControls {
		isoControlID, hasMapped := dsgvoToISOMappings[tom.ControlID]
		isoControl, hasISO := isoByControlID[isoControlID]

		covered := false
		if hasMapped && hasISO {
			evCount := evidenceCounts[isoControl.ID]
			covered = isoControl.Status == "covered" || isoControl.Status == "implemented" || evCount > 0
		}

		isoTitle := isoControlID
		if c, ok := isoByControlID[isoControlID]; ok {
			isoTitle = c.Title
		}

		results = append(results, MappingResult{
			TISAXControlID:    tom.ID,
			TISAXControlTitle: tom.Title,
			ISOControlID:      isoControlID,
			ISOControlTitle:   isoTitle,
			Covered:           covered,
		})
	}
	return results, nil
}

// auditControlEntry groups evidence items under a single control for the audit export.
type auditControlEntry struct {
	ControlID    string // control_id column value, e.g. "A.5.1"
	ControlTitle string
	Evidence     []EvidenceForExport
}

// ExportAuditPackage erstellt ein ZIP-Archiv mit allen Compliance-Nachweisen für ein Framework.
// Die ZIP enthält:
//   - INDEX.pdf    — Übersicht aller Controls mit Status und Evidence-Liste
//   - summary.json — maschinenlesbare Zusammenfassung
//   - evidence/    — Ordner pro Control mit je einer Textdatei pro Evidence
func (s *Service) ExportAuditPackage(ctx context.Context, orgID, frameworkID string) (zipData []byte, filename string, err error) {
	// 1. Load framework metadata.
	fw, err := s.repo.GetFramework(ctx, orgID, frameworkID)
	if err != nil {
		return nil, "", fmt.Errorf("get framework: %w", err)
	}

	// 2. Load org name.
	var orgName string
	_ = s.db.QueryRow(ctx, `SELECT name FROM organizations WHERE id=$1::uuid`, orgID).Scan(&orgName)
	if orgName == "" {
		orgName = orgID
	}

	// 3. Load all evidence + control metadata in a single query.
	items, err := s.repo.ListEvidenceForFramework(ctx, orgID, frameworkID)
	if err != nil {
		return nil, "", fmt.Errorf("list evidence for framework: %w", err)
	}

	// 4. Build per-control groupings.
	var controlOrder []string
	controlMap := make(map[string]*auditControlEntry)
	evidenceTotal := 0

	for i := range items {
		item := &items[i]
		if _, seen := controlMap[item.ControlID]; !seen {
			controlOrder = append(controlOrder, item.ControlID)
			controlMap[item.ControlID] = &auditControlEntry{
				ControlID:    item.ControlDomain,
				ControlTitle: item.ControlTitle,
			}
		}
		if item.EvidenceID != "" {
			controlMap[item.ControlID].Evidence = append(controlMap[item.ControlID].Evidence, *item)
			evidenceTotal++
		}
	}

	controlsWithEvidence := 0
	for _, ce := range controlMap {
		if len(ce.Evidence) > 0 {
			controlsWithEvidence++
		}
	}
	controlsTotal := len(controlOrder)

	// 5. Generate INDEX.pdf.
	indexPDF, err := GenerateAuditIndexPDF(fw.Name, orgName, controlOrder, controlMap, time.Now())
	if err != nil {
		return nil, "", fmt.Errorf("generate index pdf: %w", err)
	}

	// 6. Build summary.json.
	type summaryJSON struct {
		Framework               string    `json:"framework"`
		Org                     string    `json:"org"`
		ExportedAt              time.Time `json:"exported_at"`
		ControlsTotal           int       `json:"controls_total"`
		ControlsWithEvidence    int       `json:"controls_with_evidence"`
		ControlsWithoutEvidence int       `json:"controls_without_evidence"`
		EvidenceTotal           int       `json:"evidence_total"`
	}
	summaryData, err := json.Marshal(summaryJSON{
		Framework:               fw.Name,
		Org:                     orgName,
		ExportedAt:              time.Now().UTC(),
		ControlsTotal:           controlsTotal,
		ControlsWithEvidence:    controlsWithEvidence,
		ControlsWithoutEvidence: controlsTotal - controlsWithEvidence,
		EvidenceTotal:           evidenceTotal,
	})
	if err != nil {
		return nil, "", fmt.Errorf("marshal summary: %w", err)
	}

	// 7. Assemble ZIP.
	exportDate := time.Now().UTC().Format("2006-01-02")
	safeName := strings.Map(func(r rune) rune {
		if r == ' ' {
			return '-'
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, fw.Name)
	filename = fmt.Sprintf("audit-package-%s-%s.zip", safeName, exportDate)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// INDEX.pdf
	if f, zipErr := zw.Create("INDEX.pdf"); zipErr == nil {
		_, _ = f.Write(indexPDF)
	}

	// summary.json
	if f, zipErr := zw.Create("summary.json"); zipErr == nil {
		_, _ = f.Write(summaryData)
	}

	// evidence/ folder — one .txt file per evidence item.
	for _, ctrlID := range controlOrder {
		ce := controlMap[ctrlID]
		if len(ce.Evidence) == 0 {
			continue
		}
		folderName := auditSanitizePath(ce.ControlID + "-" + ce.ControlTitle)
		for i, ev := range ce.Evidence {
			entryName := fmt.Sprintf("evidence/%s/evidence_%03d.txt", folderName, i+1)
			f, zipErr := zw.Create(entryName)
			if zipErr != nil {
				continue
			}
			_, _ = fmt.Fprintf(f, "Evidence: %s\n", ev.EvidenceTitle)
			_, _ = fmt.Fprintf(f, "Control: %s — %s\n", ce.ControlID, ce.ControlTitle)
			_, _ = fmt.Fprintf(f, "Source: %s\n", ev.EvidenceSource)
			_, _ = fmt.Fprintf(f, "Collected: %s\n", ev.CollectedAt.UTC().Format("2006-01-02 15:04 UTC"))
			if ev.EvidenceDesc != "" {
				_, _ = fmt.Fprintf(f, "\nDescription:\n%s\n", ev.EvidenceDesc)
			}
			if ev.EvidenceFilePath != "" {
				_, _ = fmt.Fprintf(f, "\nFile reference: %s\n", ev.EvidenceFilePath)
			}
		}
	}

	if err := zw.Close(); err != nil {
		return nil, "", fmt.Errorf("close zip: %w", err)
	}
	return buf.Bytes(), filename, nil
}

// auditSanitizePath removes characters unsafe for ZIP entry paths.
func auditSanitizePath(s string) string {
	if len(s) > 60 {
		s = s[:60]
	}
	return strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		return r
	}, s)
}

// --- AI Policy Generator ---

// GeneratePolicyDraft generates a policy draft in German using the configured AI provider.
// It returns the generated text; the caller decides whether to persist it.
func (s *Service) GeneratePolicyDraft(ctx context.Context, orgID string, in GeneratePolicyDraftInput) (string, error) {
	if s.aiClient == nil {
		return "", fmt.Errorf("AI-Features nicht konfiguriert. Bitte VAKT_AI_BASE_URL und VAKT_AI_PROVIDER setzen")
	}

	// Resolve org name if not provided.
	orgName := in.OrgName
	if orgName == "" {
		_ = s.db.QueryRow(ctx, `SELECT name FROM organizations WHERE id = $1::uuid`, orgID).Scan(&orgName)
		if orgName == "" {
			orgName = "Ihr Unternehmen"
		}
	}

	// Optionally load top-10 framework controls for context.
	frameworkContext := ""
	if in.FrameworkID != "" {
		rows, err := s.db.Query(ctx, `
			SELECT control_id, title FROM ck_controls
			WHERE framework_id = $1::uuid AND org_id = $2::uuid
			ORDER BY weight DESC LIMIT 10`,
			in.FrameworkID, orgID,
		)
		if err == nil {
			defer rows.Close()
			var lines []string
			for rows.Next() {
				var controlID, title string
				if rows.Scan(&controlID, &title) == nil {
					lines = append(lines, controlID+": "+title)
				}
			}
			if len(lines) > 0 {
				frameworkContext = "Relevante ISO 27001 Anforderungen als Kontext:\n" + strings.Join(lines, "\n")
			}
		}
	}

	customContext := ""
	if in.CustomContext != "" {
		customContext = "Zusätzlicher Kontext vom Nutzer:\n" + in.CustomContext
	}

	prompt := fmt.Sprintf(`Du bist ein erfahrener Datenschutz- und IT-Sicherheitsexperte in Deutschland.
Erstelle eine professionelle %s für das Unternehmen "%s".

Die Richtlinie muss:
- Den Anforderungen von ISO 27001:2022 entsprechen
- Auf Deutsch verfasst sein
- Eine klare Struktur haben: Zweck, Geltungsbereich, Grundsätze, Verantwortlichkeiten, Maßnahmen, Gültigkeitsdauer
- Praxistauglich und verständlich für Mitarbeiter ohne technischen Hintergrund sein
- Zwischen 400 und 800 Wörtern lang sein

%s
%s

Erstelle jetzt die vollständige Richtlinie:`,
		in.PolicyType, orgName, frameworkContext, customContext,
	)

	return s.aiClient.Generate(ctx, prompt)
}

func dsgvoTOMControls(frameworkID, orgID string) []Control {
	c := func(id, title, desc string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: "Technische und organisatorische Maßnahmen", EvidenceType: "manual", Weight: w}
	}
	return []Control{
		c("TOM-1", "Zutrittskontrolle", "Maßnahmen zur Verhinderung unbefugten Zutritts zu Datenverarbeitungsanlagen (Schlösser, Alarmanlagen, Zutrittskontrollen). Nachweis: Zutrittskonzept, Protokoll.", 3),
		c("TOM-2", "Zugangskontrolle", "Technische Maßnahmen zur Authentifizierung (Passwörter, MFA, Token). Nachweis: MFA-Konfiguration, Passwortrichtlinie.", 3),
		c("TOM-3", "Zugriffskontrolle", "Berechtigungskonzept nach Need-to-Know. Nur autorisierte Personen können auf personenbezogene Daten zugreifen. Nachweis: Berechtigungsmatrix.", 3),
		c("TOM-4", "Weitergabekontrolle", "Schutz bei Übertragung personenbezogener Daten (TLS, VPN, Verschlüsselung). Nachweis: Transportverschlüsselungs-Konfiguration.", 2),
		c("TOM-5", "Eingabekontrolle", "Protokollierung aller Eingaben, Änderungen und Löschungen personenbezogener Daten (Audit-Trail). Nachweis: Logging-Konzept, Log-Beispiele.", 2),
		c("TOM-6", "Auftragskontrolle", "Kontrolle von Auftragsverarbeitern: AVV abgeschlossen, Weisungsgebundenheit sichergestellt. Nachweis: AVV-Dokumente, Prüfnachweise.", 2),
		c("TOM-7", "Verfügbarkeitskontrolle", "Schutz vor Datenverlust durch Backup, Redundanz und Notfallkonzept. Nachweis: Backup-Protokolle, Recovery-Tests.", 3),
		c("TOM-8", "Trennungsgebot", "Personenbezogene Daten verschiedener Verantwortlicher/Zwecke werden getrennt verarbeitet. Nachweis: Architektur- oder Datenflussdokumentation.", 2),
		c("TOM-9", "Pseudonymisierung", "Personenbezogene Daten werden pseudonymisiert, soweit möglich. Nachweis: Pseudonymisierungskonzept, technische Umsetzung.", 2),
		c("TOM-10", "Verschlüsselung", "Verschlüsselung ruhender und übertragener personenbezogener Daten (AES-256 oder gleichwertig). Nachweis: Verschlüsselungskonzept, Konfiguration.", 3),
		c("TOM-11", "Integrität", "Sicherstellung, dass personenbezogene Daten nicht unbefugt verändert werden (Hashes, digitale Signaturen). Nachweis: Integritätskonzept.", 2),
		c("TOM-12", "Wiederherstellung", "Fähigkeit zur schnellen Wiederherstellung von Verfügbarkeit und Zugang nach Zwischenfällen. Nachweis: BCM-Plan, Wiederherstellungstests.", 3),
		c("TOM-13", "Überprüfungsverfahren", "Regelmäßige Überprüfung und Bewertung der Wirksamkeit der TOMs (mindestens jährlich). Nachweis: Prüfberichte, Revisionsprotokoll.", 2),
	}
}

// --- Maßnahmen-Katalog (control measures) ---

// ListMeasures returns all measures for a control.
func (s *Service) ListMeasures(ctx context.Context, orgID, controlID string) ([]ControlMeasure, error) {
	return s.repo.ListMeasures(ctx, orgID, controlID)
}

// CreateMeasure creates a new custom measure for a control.
func (s *Service) CreateMeasure(ctx context.Context, orgID, controlID string, in CreateMeasureInput) (ControlMeasure, error) {
	return s.repo.CreateMeasure(ctx, orgID, controlID, in)
}

// UpdateMeasure updates an existing measure.
func (s *Service) UpdateMeasure(ctx context.Context, orgID, measureID string, in UpdateMeasureInput) (ControlMeasure, error) {
	return s.repo.UpdateMeasure(ctx, orgID, measureID, in)
}

// DeleteMeasure deletes a non-builtin measure.
func (s *Service) DeleteMeasure(ctx context.Context, orgID, measureID string) error {
	return s.repo.DeleteMeasure(ctx, orgID, measureID)
}

// SeedBuiltinMeasures seeds the default recommended measures for important ISO 27001 controls
// across all organisations. Called on startup after ReseedBuiltinControls.
func (s *Service) SeedBuiltinMeasures(ctx context.Context) {
	orgs, err := s.repo.ListAllOrgs(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("seed measures: failed to list orgs")
		return
	}

	catalogue := builtinMeasures()

	for _, orgID := range orgs {
		for controlCode, measures := range catalogue {
			controlUUID, err := s.repo.FindControlByCode(ctx, orgID, controlCode)
			if err != nil {
				log.Warn().Err(err).Str("control", controlCode).Str("org_id", orgID).Msg("seed measures: find control")
				continue
			}
			if controlUUID == "" {
				// Control not yet seeded for this org — skip silently.
				continue
			}
			if err := s.repo.SeedMeasuresForControl(ctx, orgID, controlUUID, measures); err != nil {
				log.Warn().Err(err).Str("control", controlCode).Str("org_id", orgID).Msg("seed measures: insert")
			}
		}
		log.Info().Str("org_id", orgID).Msg("seeded builtin measures")
	}
}

// builtinMeasures returns the catalogue of recommended measures keyed by ISO 27001 control_id code.
func builtinMeasures() map[string][]CreateMeasureInput {
	m := func(title, desc, diff string) CreateMeasureInput {
		return CreateMeasureInput{Title: title, Description: desc, Difficulty: diff}
	}
	return map[string][]CreateMeasureInput{
		// A.5.1 — Informationssicherheitsrichtlinien
		"A.5.1": {
			m("Richtliniendokument erstellen", "Erstellen Sie ein zentrales IS-Richtliniendokument mit Geltungsbereich, Verantwortlichkeiten und Grundsätzen. Vorlage: Mindestens 3 Seiten, jährlich überprüft.", "easy"),
			m("Freigabe durch Geschäftsführung einholen", "Lassen Sie die Richtlinie formal durch die Geschäftsführung genehmigen und unterschreiben. Dokumentieren Sie das Datum der Genehmigung.", "easy"),
			m("Richtlinie kommunizieren", "Verteilen Sie die Richtlinie an alle Mitarbeiter (z.B. per E-Mail, Intranet). Dokumentieren Sie den Versand als Nachweis.", "easy"),
		},
		// A.5.1.1 — same measures apply to the sub-control
		"A.5.1.1": {
			m("Richtliniendokument erstellen", "Erstellen Sie ein zentrales IS-Richtliniendokument mit Geltungsbereich, Verantwortlichkeiten und Grundsätzen. Vorlage: Mindestens 3 Seiten, jährlich überprüft.", "easy"),
			m("Freigabe durch Geschäftsführung einholen", "Lassen Sie die Richtlinie formal durch die Geschäftsführung genehmigen und unterschreiben. Dokumentieren Sie das Datum der Genehmigung.", "easy"),
			m("Richtlinie kommunizieren", "Verteilen Sie die Richtlinie an alle Mitarbeiter (z.B. per E-Mail, Intranet). Dokumentieren Sie den Versand als Nachweis.", "easy"),
		},
		// A.5.24 — Planung und Vorbereitung des IS-Vorfallmanagements
		"A.5.24": {
			m("Incident-Response-Plan erstellen", "Definieren Sie klare Eskalationswege, Kontaktlisten und Erstmaßnahmen für Sicherheitsvorfälle.", "medium"),
			m("Meldepflichten dokumentieren", "Dokumentieren Sie gesetzliche Meldepflichten (NIS2: 24h Erstmeldung, BSI: 72h DSGVO). Erstellen Sie eine Meldecheckliste.", "medium"),
			m("Übung durchführen", "Führen Sie mindestens jährlich eine Tabletop-Übung für einen fiktiven Vorfall durch. Protokollieren Sie die Ergebnisse.", "hard"),
		},
		// A.6.3 — Informationssicherheitsbewusstsein
		"A.6.3": {
			m("Awareness-Training planen", "Planen Sie ein jährliches Pflichttraining für alle Mitarbeiter. Nutzen Sie SecReflex für Phishing-Simulationen.", "easy"),
			m("Schulungsnachweis führen", "Dokumentieren Sie Teilnahme und Datum jeder Schulung pro Mitarbeiter als Compliance-Nachweis.", "easy"),
		},
		// A.8.8 — Management technischer Schwachstellen
		"A.8.8": {
			m("Schwachstellen-Scanner einrichten", "Richten Sie regelmäßige automatische Scans ein (z.B. Trivy für Container, Nuclei für Web-Apps). Nutzen Sie SecPulse.", "medium"),
			m("Patch-Prozess definieren", "Legen Sie SLAs für Patches fest: Kritisch ≤24h, Hoch ≤7d, Mittel ≤30d. Dokumentieren Sie Ausnahmen.", "medium"),
			m("Schwachstellen-Register pflegen", "Führen Sie ein aktuelles Register aller bekannten Schwachstellen mit Status und Verantwortlichem.", "easy"),
		},
		// A.12.6 / A.12.6.1 — Management technischer Schwachstellen (ältere ISO-Nummerierung)
		"A.12.6": {
			m("Schwachstellen-Scanner einrichten", "Richten Sie regelmäßige automatische Scans ein (z.B. Trivy für Container, Nuclei für Web-Apps). Nutzen Sie SecPulse.", "medium"),
			m("Patch-Prozess definieren", "Legen Sie SLAs für Patches fest: Kritisch ≤24h, Hoch ≤7d, Mittel ≤30d. Dokumentieren Sie Ausnahmen.", "medium"),
			m("Schwachstellen-Register pflegen", "Führen Sie ein aktuelles Register aller bekannten Schwachstellen mit Status und Verantwortlichem.", "easy"),
		},
		"A.12.6.1": {
			m("Schwachstellen-Scanner einrichten", "Richten Sie regelmäßige automatische Scans ein (z.B. Trivy für Container, Nuclei für Web-Apps). Nutzen Sie SecPulse.", "medium"),
			m("Patch-Prozess definieren", "Legen Sie SLAs für Patches fest: Kritisch ≤24h, Hoch ≤7d, Mittel ≤30d. Dokumentieren Sie Ausnahmen.", "medium"),
			m("Schwachstellen-Register pflegen", "Führen Sie ein aktuelles Register aller bekannten Schwachstellen mit Status und Verantwortlichem.", "easy"),
		},
		// A.8.13 — Informationssicherung (Backup)
		"A.8.13": {
			m("Backup-Konzept erstellen", "Dokumentieren Sie Backup-Frequenz (täglich), Aufbewahrungszeit und Speicherorte (3-2-1-Regel).", "easy"),
			m("Wiederherstellung testen", "Testen Sie mindestens jährlich die Wiederherstellung aus Backups. Protokollieren Sie RPO und RTO.", "medium"),
		},
		// A.12.3 / A.12.3.1 — Datensicherung (ältere ISO-Nummerierung)
		"A.12.3": {
			m("Backup-Konzept erstellen", "Dokumentieren Sie Backup-Frequenz (täglich), Aufbewahrungszeit und Speicherorte (3-2-1-Regel).", "easy"),
			m("Wiederherstellung testen", "Testen Sie mindestens jährlich die Wiederherstellung aus Backups. Protokollieren Sie RPO und RTO.", "medium"),
		},
		"A.12.3.1": {
			m("Backup-Konzept erstellen", "Dokumentieren Sie Backup-Frequenz (täglich), Aufbewahrungszeit und Speicherorte (3-2-1-Regel).", "easy"),
			m("Wiederherstellung testen", "Testen Sie mindestens jährlich die Wiederherstellung aus Backups. Protokollieren Sie RPO und RTO.", "medium"),
		},
		// A.8.16 — Überwachungsaktivitäten
		"A.8.16": {
			m("Log-Management einrichten", "Zentralisieren Sie System- und Sicherheitslogs. Definieren Sie Aufbewahrungsdauer (mind. 12 Monate für NIS2).", "medium"),
			m("Alerting konfigurieren", "Richten Sie automatische Alarme für kritische Ereignisse ein (failed logins, privilege escalation, etc.).", "medium"),
		},
		// A.5.21 — Lieferkettensicherheit
		"A.5.21": {
			m("Lieferanten-Register erstellen", "Führen Sie ein Register aller IT-Dienstleister mit Risikoeinstufung und Vertragsreferenz.", "easy"),
			m("AVV abschließen", "Stellen Sie sicher, dass alle Auftragsverarbeiter einen gültigen AVV nach Art. 28 DSGVO unterzeichnet haben.", "medium"),
			m("Lieferanten-Audit planen", "Führen Sie für kritische Lieferanten mindestens jährlich ein Sicherheits-Assessment durch.", "hard"),
		},
		// A.5.22 — Lieferkettenüberwachung
		"A.5.22": {
			m("Lieferanten-Register erstellen", "Führen Sie ein Register aller IT-Dienstleister mit Risikoeinstufung und Vertragsreferenz.", "easy"),
			m("AVV abschließen", "Stellen Sie sicher, dass alle Auftragsverarbeiter einen gültigen AVV nach Art. 28 DSGVO unterzeichnet haben.", "medium"),
			m("Lieferanten-Audit planen", "Führen Sie für kritische Lieferanten mindestens jährlich ein Sicherheits-Assessment durch.", "hard"),
		},
		// A.8.24 — Kryptographie
		"A.8.24": {
			m("Kryptokonzept erstellen", "Dokumentieren Sie erlaubte Verschlüsselungsalgorithmen, Schlüssellängen und Zertifikats-Management-Prozesse.", "medium"),
			m("Zertifikate inventarisieren", "Führen Sie eine Liste aller TLS-Zertifikate mit Ablaufdatum. Richten Sie Erneuerungs-Alerts ein.", "easy"),
		},
		// A.10.1 / A.10.1.1 / A.10.1.2 — Kryptographie (ältere ISO-Nummerierung)
		"A.10.1": {
			m("Kryptokonzept erstellen", "Dokumentieren Sie erlaubte Verschlüsselungsalgorithmen, Schlüssellängen und Zertifikats-Management-Prozesse.", "medium"),
			m("Zertifikate inventarisieren", "Führen Sie eine Liste aller TLS-Zertifikate mit Ablaufdatum. Richten Sie Erneuerungs-Alerts ein.", "easy"),
		},
		"A.10.1.1": {
			m("Kryptokonzept erstellen", "Dokumentieren Sie erlaubte Verschlüsselungsalgorithmen, Schlüssellängen und Zertifikats-Management-Prozesse.", "medium"),
			m("Zertifikate inventarisieren", "Führen Sie eine Liste aller TLS-Zertifikate mit Ablaufdatum. Richten Sie Erneuerungs-Alerts ein.", "easy"),
		},
		"A.10.1.2": {
			m("Kryptokonzept erstellen", "Dokumentieren Sie erlaubte Verschlüsselungsalgorithmen, Schlüssellängen und Zertifikats-Management-Prozesse.", "medium"),
			m("Zertifikate inventarisieren", "Führen Sie eine Liste aller TLS-Zertifikate mit Ablaufdatum. Richten Sie Erneuerungs-Alerts ein.", "easy"),
		},
	}
}

// --- Collaborative Tasks ---

// ListTasks returns all tasks for the given compliance entity.
func (s *Service) ListTasks(ctx context.Context, orgID, entityType, entityID string) ([]Task, error) {
	tasks, err := s.repo.ListTasks(ctx, orgID, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	if tasks == nil {
		tasks = []Task{}
	}
	return tasks, nil
}

// CreateTask creates a new collaborative task for a compliance entity.
func (s *Service) CreateTask(ctx context.Context, orgID, entityType, entityID string, in CreateTaskInput) (Task, error) {
	return s.repo.CreateTask(ctx, orgID, entityType, entityID, in)
}

// UpdateTask applies a partial update to a task.
func (s *Service) UpdateTask(ctx context.Context, orgID, taskID string, in UpdateTaskInput) (Task, error) {
	return s.repo.UpdateTask(ctx, orgID, taskID, in)
}

// DeleteTask removes a task.
func (s *Service) DeleteTask(ctx context.Context, orgID, taskID string) error {
	return s.repo.DeleteTask(ctx, orgID, taskID)
}

// ListOverdueTasks returns open tasks past their due date for the org.
func (s *Service) ListOverdueTasks(ctx context.Context, orgID string) ([]Task, error) {
	tasks, err := s.repo.ListOverdueTasks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list overdue tasks: %w", err)
	}
	if tasks == nil {
		tasks = []Task{}
	}
	return tasks, nil
}

// --- Comments ---

// ListComments returns all comments for a compliance entity.
func (s *Service) ListComments(ctx context.Context, orgID, entityType, entityID string) ([]Comment, error) {
	comments, err := s.repo.ListComments(ctx, orgID, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	if comments == nil {
		comments = []Comment{}
	}
	return comments, nil
}

// CreateComment posts a comment on a compliance entity.
func (s *Service) CreateComment(ctx context.Context, orgID, entityType, entityID string, in CreateCommentInput) (Comment, error) {
	return s.repo.CreateComment(ctx, orgID, entityType, entityID, in)
}

// DeleteComment removes a comment.
func (s *Service) DeleteComment(ctx context.Context, orgID, commentID string) error {
	return s.repo.DeleteComment(ctx, orgID, commentID)
}

// --- CAPA (Corrective and Preventive Actions) ---

// ListCAPAs returns CAPAs for an organisation, optionally filtered by status.
func (s *Service) ListCAPAs(ctx context.Context, orgID string, statusFilter string) ([]CAPA, error) {
	return s.repo.ListCAPAs(ctx, orgID, statusFilter)
}

// ListCAPAsForSource returns CAPAs linked to a specific source entity.
func (s *Service) ListCAPAsForSource(ctx context.Context, orgID, sourceType, sourceID string) ([]CAPA, error) {
	return s.repo.ListCAPAsForSource(ctx, orgID, sourceType, sourceID)
}

// GetCAPA returns a single CAPA by ID.
func (s *Service) GetCAPA(ctx context.Context, orgID, capaID string) (CAPA, error) {
	return s.repo.GetCAPA(ctx, orgID, capaID)
}

// CreateCAPA creates a new CAPA record.
func (s *Service) CreateCAPA(ctx context.Context, orgID string, in CreateCAPAInput) (CAPA, error) {
	return s.repo.CreateCAPA(ctx, orgID, in)
}

// UpdateCAPA applies partial updates to a CAPA.
func (s *Service) UpdateCAPA(ctx context.Context, orgID, capaID string, in UpdateCAPAInput) (CAPA, error) {
	return s.repo.UpdateCAPA(ctx, orgID, capaID, in)
}

// DeleteCAPA removes a CAPA record.
func (s *Service) DeleteCAPA(ctx context.Context, orgID, capaID string) error {
	return s.repo.DeleteCAPA(ctx, orgID, capaID)
}

// --- Control Review Cycles (Migration 075) ---

// RecordControlReview records a periodic review event for a compliance control.
// It updates the control's review timestamps and appends a row to the review history log.
func (s *Service) RecordControlReview(ctx context.Context, orgID, controlID string, in RecordReviewInput) (Control, error) {
	// Fetch current control to capture status_at_review.
	ctrl, err := s.repo.GetControl(ctx, orgID, controlID)
	if err != nil {
		return Control{}, fmt.Errorf("get control for review: %w", err)
	}
	statusAtReview := ctrl.Status
	if statusAtReview == "" {
		statusAtReview = ctrl.ManualStatus
	}
	return s.repo.RecordControlReview(ctx, orgID, controlID, in, statusAtReview)
}

// ListControlReviews returns the review history for a control.
func (s *Service) ListControlReviews(ctx context.Context, orgID, controlID string) ([]ControlReview, error) {
	return s.repo.ListControlReviews(ctx, orgID, controlID)
}

// ListOverdueControls returns controls whose review is past due.
func (s *Service) ListOverdueControls(ctx context.Context, orgID string) ([]Control, error) {
	return s.repo.ListOverdueControls(ctx, orgID)
}

// --- Paginated list methods (used by pagination-aware handlers) ---

// ListControlsPaged returns a page of controls with evidence counts, plus the total count.
func (s *Service) ListControlsPaged(ctx context.Context, orgID, frameworkID string, offset, limit int) ([]Control, int, error) {
	controls, total, err := s.repo.ListControlsPaged(ctx, orgID, frameworkID, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list controls paged: %w", err)
	}

	// Enrich with evidence counts (using counts for the full framework so we don't need extra per-page queries).
	counts, err := s.repo.CountEvidenceByControl(ctx, orgID, frameworkID)
	if err != nil {
		return nil, 0, fmt.Errorf("count evidence for controls paged: %w", err)
	}
	for i := range controls {
		controls[i].EvidenceCount = counts[controls[i].ID]
		controls[i].Status = resolveStatus(controls[i])
		if strings.HasPrefix(controls[i].ControlID, "DORA-") {
			if m, ok := doraISO27001Mapping[controls[i].ControlID]; ok {
				controls[i].ISO27001Mapping = m
			}
		}
	}
	return controls, total, nil
}

// ListRisksPaged returns a page of risks plus the total count.
func (s *Service) ListRisksPaged(ctx context.Context, orgID string, offset, limit int) ([]Risk, int, error) {
	return s.repo.ListRisksPaged(ctx, orgID, offset, limit)
}

// ListIncidentsPaged returns a page of incidents plus the total count.
func (s *Service) ListIncidentsPaged(ctx context.Context, orgID string, offset, limit int) ([]Incident, int, error) {
	incidents, total, err := s.repo.ListIncidentsPaged(ctx, orgID, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list incidents paged: %w", err)
	}
	for i := range incidents {
		incidents[i].DeadlineStatus = computeDeadlineStatus(&incidents[i])
	}
	return incidents, total, nil
}

// ListPoliciesPaged returns a page of policies plus the total count.
func (s *Service) ListPoliciesPaged(ctx context.Context, orgID string, offset, limit int) ([]Policy, int, error) {
	return s.repo.ListPoliciesPaged(ctx, orgID, offset, limit)
}

// ListCAPAsPaged returns a page of CAPAs plus the total count.
func (s *Service) ListCAPAsPaged(ctx context.Context, orgID, statusFilter string, offset, limit int) ([]CAPA, int, error) {
	return s.repo.ListCAPAsPaged(ctx, orgID, statusFilter, offset, limit)
}

// --- Score History ---

// RecordScoreSnapshotForAllOrgs iterates all non-deleted organisations and captures
// the current compliance score (org-wide + per-framework) into ck_score_history.
// Called daily by the Asynq scheduler.
func (s *Service) RecordScoreSnapshotForAllOrgs(ctx context.Context) error {
	rows, err := s.repo.db.Query(ctx, `SELECT id::text FROM organizations WHERE is_deleted = false`)
	if err != nil {
		return fmt.Errorf("score_snapshot: list orgs: %w", err)
	}
	defer rows.Close()

	var orgIDs []string
	for rows.Next() {
		var id string
		if scanErr := rows.Scan(&id); scanErr != nil {
			continue
		}
		orgIDs = append(orgIDs, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, orgID := range orgIDs {
		if err := s.recordOrgScoreSnapshot(ctx, orgID); err != nil {
			log.Error().Err(err).Str("org_id", orgID).Msg("score_snapshot: failed for org")
			// Continue with next org — don't abort the whole run.
		}
	}
	return nil
}

// recordOrgScoreSnapshot captures one org-wide + per-framework score row.
func (s *Service) recordOrgScoreSnapshot(ctx context.Context, orgID string) error {
	frameworks, err := s.repo.ListFrameworks(ctx, orgID)
	if err != nil {
		return fmt.Errorf("list frameworks: %w", err)
	}

	var totalAll, implementedAll int

	for _, fw := range frameworks {
		controls, err := s.repo.ListControls(ctx, orgID, fw.ID)
		if err != nil {
			log.Warn().Err(err).Str("framework_id", fw.ID).Msg("score_snapshot: list controls failed")
			continue
		}
		evidenceCounts, err := s.repo.CountEvidenceByControl(ctx, orgID, fw.ID)
		if err != nil {
			log.Warn().Err(err).Str("framework_id", fw.ID).Msg("score_snapshot: count evidence failed")
			continue
		}

		report := computeReadinessReport(&fw, controls, evidenceCounts)
		totalAll += report.TotalControls
		implementedAll += report.Covered

		// Per-framework snapshot.
		fwID := fw.ID
		if insertErr := s.repo.InsertScoreSnapshot(ctx, orgID, &fwID, report.ReadinessScore, report.TotalControls, report.Covered); insertErr != nil {
			log.Warn().Err(insertErr).Str("framework_id", fw.ID).Msg("score_snapshot: insert per-framework failed")
		}
	}

	// Org-wide snapshot (framework_id = NULL).
	var orgScore float64
	if totalAll > 0 {
		orgScore = float64(implementedAll) / float64(totalAll) * 100
	}
	if insertErr := s.repo.InsertScoreSnapshot(ctx, orgID, nil, orgScore, totalAll, implementedAll); insertErr != nil {
		return fmt.Errorf("insert org-wide snapshot: %w", insertErr)
	}
	return nil
}

// GetScoreHistory returns daily score history for an organisation (org-wide snapshots).
func (s *Service) GetScoreHistory(ctx context.Context, orgID string, days int) ([]ScoreHistoryEntry, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	return s.repo.GetScoreHistory(ctx, orgID, days)
}

// ExecutiveSummaryData holds all data gathered for the Executive Summary PDF.
type ExecutiveSummaryData struct {
	OrgName      string
	GeneratedAt  time.Time
	// Section 1 — Overall compliance score
	OverallScore float64 // 0–100, weighted average across all frameworks
	// Section 2 — Framework overview
	Frameworks []ExecutiveFrameworkRow
	// Section 3 — Top 5 open risks (by score)
	TopRisks []ExecutiveRiskRow
	// Section 4 — Last 30 days activity
	Last30DaysActivity ExecutiveActivity
}

// ExecutiveFrameworkRow is one row in the framework table.
type ExecutiveFrameworkRow struct {
	Name        string
	Score       float64
	Implemented int
	Total       int
}

// ExecutiveRiskRow is one of the top-5 open risks.
type ExecutiveRiskRow struct {
	Title    string
	Score    int
	Severity string // "critical" | "high" | "medium" | "low"
}

// ExecutiveActivity holds counts of key activities in the last 30 days.
type ExecutiveActivity struct {
	ClosedControls   int
	NewIncidents     int
	ResolvedFindings int
}

// GetExecutiveSummaryData collects data required for the Executive Summary PDF.
func (s *Service) GetExecutiveSummaryData(ctx context.Context, orgID string) (*ExecutiveSummaryData, error) {
	d := &ExecutiveSummaryData{GeneratedAt: time.Now().UTC()}

	// Org name (soft-fail)
	_ = s.db.QueryRow(ctx, `SELECT name FROM organizations WHERE id=$1::uuid`, orgID).Scan(&d.OrgName)
	if d.OrgName == "" {
		d.OrgName = orgID
	}

	// Framework scores
	rows, err := s.db.Query(ctx, `
		SELECT f.name,
		       COUNT(c.id)::int                                                    AS total,
		       COUNT(c.id) FILTER (WHERE c.manual_status = 'implemented')::int     AS implemented
		FROM ck_frameworks f
		LEFT JOIN ck_controls c ON c.framework_id = f.id AND c.org_id = f.org_id
		WHERE f.org_id = $1::uuid
		GROUP BY f.name
		ORDER BY f.name
	`, orgID)
	if err != nil {
		log.Warn().Err(err).Msg("executive summary: frameworks query")
	} else {
		defer rows.Close()
		var totalWeight, weightedSum float64
		for rows.Next() {
			var r ExecutiveFrameworkRow
			if err := rows.Scan(&r.Name, &r.Total, &r.Implemented); err != nil {
				continue
			}
			if r.Total > 0 {
				r.Score = float64(r.Implemented) / float64(r.Total) * 100
			}
			d.Frameworks = append(d.Frameworks, r)
			weightedSum += r.Score * float64(r.Total)
			totalWeight += float64(r.Total)
		}
		_ = rows.Err()
		if totalWeight > 0 {
			d.OverallScore = weightedSum / totalWeight
		}
	}

	// Top 5 risks by score (likelihood * impact)
	riskRows, err := s.db.Query(ctx, `
		SELECT title,
		       (likelihood * impact)::int AS score,
		       CASE
		           WHEN (likelihood * impact) >= 15 THEN 'critical'
		           WHEN (likelihood * impact) >= 9  THEN 'high'
		           WHEN (likelihood * impact) >= 4  THEN 'medium'
		           ELSE 'low'
		       END AS severity
		FROM ck_risks
		WHERE org_id = $1::uuid AND status = 'open'
		ORDER BY score DESC, updated_at DESC
		LIMIT 5
	`, orgID)
	if err != nil {
		log.Warn().Err(err).Msg("executive summary: risks query")
	} else {
		defer riskRows.Close()
		for riskRows.Next() {
			var r ExecutiveRiskRow
			if err := riskRows.Scan(&r.Title, &r.Score, &r.Severity); err != nil {
				continue
			}
			d.TopRisks = append(d.TopRisks, r)
		}
		_ = riskRows.Err()
	}

	// Last 30 days activity
	since := time.Now().UTC().Add(-30 * 24 * time.Hour)
	_ = s.db.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM ck_controls
		WHERE org_id=$1::uuid AND manual_status='implemented' AND updated_at >= $2
	`, orgID, since).Scan(&d.Last30DaysActivity.ClosedControls)

	_ = s.db.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM ck_incidents
		WHERE org_id=$1::uuid AND created_at >= $2
	`, orgID, since).Scan(&d.Last30DaysActivity.NewIncidents)

	_ = s.db.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM vb_findings
		WHERE org_id=$1::uuid AND status='resolved' AND updated_at >= $2
	`, orgID, since).Scan(&d.Last30DaysActivity.ResolvedFindings)

	return d, nil
}

// ExportExecutiveSummaryPDF generates the Executive Summary PDF bytes.
func (s *Service) ExportExecutiveSummaryPDF(ctx context.Context, orgID string) ([]byte, string, error) {
	data, err := s.GetExecutiveSummaryData(ctx, orgID)
	if err != nil {
		return nil, "", fmt.Errorf("gather executive summary data: %w", err)
	}
	pdfBytes, err := GenerateExecutiveSummaryPDF(data)
	if err != nil {
		return nil, "", fmt.Errorf("generate executive summary pdf: %w", err)
	}
	filename := fmt.Sprintf("executive-summary-%s.pdf", data.GeneratedAt.Format("2006-01-02"))
	return pdfBytes, filename, nil
}
