// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

const ionosSource = "ionos-collector"
const ionosDefaultAPIBase = "https://api.ionos.com/cloudapi/v6"

// IONOSCollector collects compliance evidence from an IONOS Cloud account.
type IONOSCollector struct {
	db         *pgxpool.Pool
	evidence   EvidenceWriter
	httpClient *http.Client
	apiBase    string // empty → uses ionosDefaultAPIBase; non-empty only in tests
}

// NewIONOSCollector creates a new IONOSCollector.
func NewIONOSCollector(db *pgxpool.Pool, evidence EvidenceWriter) *IONOSCollector {
	return &IONOSCollector{
		db:         db,
		evidence:   evidence,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *IONOSCollector) base() string {
	if c.apiBase != "" {
		return c.apiBase
	}
	return ionosDefaultAPIBase
}

// ionosDatacenter is an internal API response type.
type ionosDatacenter struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Collect runs all IONOS evidence collectors for the given org and config.
func (c *IONOSCollector) Collect(ctx context.Context, orgID string, cfg IONOSConfig) (int, error) {
	inventoryControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"asset", "inventory", "server"})
	networkControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"network", "firewall", "port"})
	accessControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"ssh", "access", "privileged", "key"})
	backupControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"backup", "snapshot", "recovery"})

	dcs, err := c.listDatacenters(ctx, cfg)
	if err != nil {
		return 0, fmt.Errorf("ionos: list datacenters: %w", err)
	}

	total := 0
	// F3/R-H20: accumulate sub-collector failures so a total failure surfaces as
	// last_sync_status='error' instead of a false 'success' with zero evidence.
	var errs []error

	for _, dc := range dcs {
		if n, err := c.collectServers(ctx, cfg, orgID, dc.ID, dc.Name, inventoryControls); err != nil {
			log.Warn().Err(err).Str("dc", dc.Name).Msg("ionos_collector: server collection failed")
			errs = append(errs, err)
		} else {
			total += n
		}
	}

	if n, err := c.collectSSHKeys(ctx, cfg, orgID, accessControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("ionos_collector: ssh key collection failed")
		errs = append(errs, err)
	} else {
		total += n
	}

	if n, err := c.collectSnapshots(ctx, cfg, orgID, backupControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("ionos_collector: snapshot collection failed")
		errs = append(errs, err)
	} else {
		total += n
	}

	_ = networkControls // Firewall collection requires per-server/NIC traversal; evidence covered by server summary
	// Only a TOTAL failure (zero evidence despite sub-collector errors) is reported as
	// an error → last_sync_status='error' (the D14-08 case: all sub-collectors failed).
	// A partial collection still produced evidence and counts as a (partial) success.
	if total == 0 && len(errs) > 0 {
		return 0, errors.Join(errs...)
	}
	return total, nil
}

// CountDatacenters returns the number of datacenters (used by GetIONOSStatus).
func (c *IONOSCollector) CountDatacenters(ctx context.Context, cfg IONOSConfig) (int, error) {
	dcs, err := c.listDatacenters(ctx, cfg)
	if err != nil {
		return 0, err
	}
	return len(dcs), nil
}

func (c *IONOSCollector) doRequest(ctx context.Context, cfg IONOSConfig, path string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	} else if cfg.Username != "" {
		req.SetBasicAuth(cfg.Username, cfg.Password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get %s: %w", path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("authentication failed (401)")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ionos api %s returned %d", path, resp.StatusCode)
	}

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return result, nil
}

func (c *IONOSCollector) listDatacenters(ctx context.Context, cfg IONOSConfig) ([]ionosDatacenter, error) {
	result, err := c.doRequest(ctx, cfg, "/datacenters")
	if err != nil {
		return nil, err
	}

	var dcs []ionosDatacenter
	if items, ok := result["items"].([]any); ok {
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				dc := ionosDatacenter{}
				if id, ok := m["id"].(string); ok {
					dc.ID = id
				}
				if props, ok := m["properties"].(map[string]any); ok {
					if name, ok := props["name"].(string); ok {
						dc.Name = name
					}
				}
				if dc.ID != "" {
					dcs = append(dcs, dc)
				}
			}
		}
	}
	return dcs, nil
}

func (c *IONOSCollector) collectServers(ctx context.Context, cfg IONOSConfig, orgID, dcID, dcName string, controls []ControlMatch) (int, error) {
	result, err := c.doRequest(ctx, cfg, "/datacenters/"+dcID+"/servers")
	if err != nil {
		return 0, err
	}

	summaries := []map[string]any{}
	if items, ok := result["items"].([]any); ok {
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				entry := map[string]any{"datacenter": dcName}
				if id, ok := m["id"].(string); ok {
					entry["id"] = id
				}
				if props, ok := m["properties"].(map[string]any); ok {
					for _, k := range []string{"name", "vmState", "cpuFamily"} {
						if v, ok := props[k]; ok {
							entry[k] = v
						}
					}
				}
				summaries = append(summaries, entry)
			}
		}
	}

	details := map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"datacenter":   dcName,
		"server_count": len(summaries),
		"servers":      summaries,
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID, fmt.Sprintf("IONOS Server-Inventar (%s)", dcName), details); err != nil {
		return 0, err
	}
	return 1, nil
}

func (c *IONOSCollector) collectSSHKeys(ctx context.Context, cfg IONOSConfig, orgID string, controls []ControlMatch) (int, error) {
	result, err := c.doRequest(ctx, cfg, "/sshkeys")
	if err != nil {
		return 0, err
	}

	keyCount := 0
	if items, ok := result["items"].([]any); ok {
		keyCount = len(items)
	}

	details := map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"key_count":    keyCount,
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID, "IONOS SSH-Keys im Account", details); err != nil {
		return 0, err
	}
	return 1, nil
}

func (c *IONOSCollector) collectSnapshots(ctx context.Context, cfg IONOSConfig, orgID string, controls []ControlMatch) (int, error) {
	result, err := c.doRequest(ctx, cfg, "/snapshots")
	if err != nil {
		return 0, err
	}

	snapshotCount := 0
	recentCount := 0
	sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour)

	if items, ok := result["items"].([]any); ok {
		snapshotCount = len(items)
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				if meta, ok := m["metadata"].(map[string]any); ok {
					if createdStr, ok := meta["createdDate"].(string); ok {
						if created, err := time.Parse(time.RFC3339, createdStr); err == nil {
							if created.After(sevenDaysAgo) {
								recentCount++
							}
						}
					}
				}
			}
		}
	}

	details := map[string]any{
		"collected_at":          time.Now().UTC().Format(time.RFC3339),
		"snapshot_count":        snapshotCount,
		"snapshots_last_7_days": recentCount,
	}

	title := "IONOS Snapshots (Backup-Nachweis)"
	if recentCount == 0 && snapshotCount > 0 {
		details["warning"] = "Kein Snapshot in den letzten 7 Tagen erstellt."
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID, title, details); err != nil {
		return 0, err
	}
	return 1, nil
}

func (c *IONOSCollector) addEvidence(ctx context.Context, orgID, controlID, title string, details map[string]any) error {
	data, _ := json.Marshal(details)
	if controlID == "" {
		log.Debug().Str("org_id", orgID).Str("title", title).Msg("ionos_collector: no matching control, skipping")
		return nil
	}
	return c.evidence.AddCollectorEvidence(ctx, orgID, controlID, "", ionosSource, title, data)
}
