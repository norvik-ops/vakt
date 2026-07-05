package vaktscan

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// SBOMDocument is a minimal CycloneDX SBOM structure for parsing Syft output.
type SBOMDocument struct {
	BOMFormat   string          `json:"bomFormat"`
	SpecVersion string          `json:"specVersion"`
	Components  []SBOMComponent `json:"components"`
}

// SBOMComponent represents a single software component in a CycloneDX SBOM.
type SBOMComponent struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Version string `json:"version"`
	PURL    string `json:"purl,omitempty"`
}

// RunSyftScan executes syft against the given target, parses the CycloneDX JSON
// output, and persists the SBOM and its components to the database.
func RunSyftScan(ctx context.Context, db *pgxpool.Pool, orgID, assetID, target string) error {
	// Verify syft is available before attempting the scan.
	if _, err := exec.LookPath("syft"); err != nil {
		return fmt.Errorf("syft not found in PATH: install syft (https://github.com/anchore/syft)")
	}

	// Reject argument-injection patterns in the target (same guard as RunTrivyScan/RunNucleiScan).
	if strings.HasPrefix(target, "-") || strings.ContainsAny(target, `\`) {
		return fmt.Errorf("syft: invalid scan target %q", target)
	}

	log.Info().
		Str("org_id", orgID).
		Str("asset_id", assetID).
		Str("target", target).
		Msg("starting syft SBOM scan")

	// Same subprocess-memory concern as trivy/nuclei — share the scan slots.
	if err := acquireScanSlot(ctx); err != nil {
		return err
	}
	out, err := exec.CommandContext(ctx, "syft", "packages", target, "-o", "cyclonedx-json", "--quiet").Output()
	releaseScanSlot()
	if err != nil {
		return fmt.Errorf("syft exec: %w", err)
	}

	var doc SBOMDocument
	if err := json.Unmarshal(out, &doc); err != nil {
		return fmt.Errorf("parse syft output: %w", err)
	}

	repo := NewRepository(db)
	sbomID, err := repo.CreateSBOM(ctx, orgID, assetID, doc)
	if err != nil {
		return fmt.Errorf("persist SBOM: %w", err)
	}

	log.Info().
		Str("sbom_id", sbomID).
		Str("asset_id", assetID).
		Int("components", len(doc.Components)).
		Msg("syft SBOM scan complete")
	return nil
}
