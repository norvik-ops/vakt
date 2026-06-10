// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

const hetznerSource = "hetzner-collector"

// HetznerCollector collects compliance evidence from a Hetzner Cloud account.
type HetznerCollector struct {
	db         *pgxpool.Pool
	evidence   EvidenceWriter
	clientOpts []hcloud.ClientOption // non-empty only in tests; allows endpoint override
}

// NewHetznerCollector creates a new HetznerCollector.
func NewHetznerCollector(db *pgxpool.Pool, evidence EvidenceWriter) *HetznerCollector {
	return &HetznerCollector{db: db, evidence: evidence}
}

// Collect runs all Hetzner evidence collectors for the given org and config.
func (c *HetznerCollector) Collect(ctx context.Context, orgID string, cfg HetznerConfig) (int, error) {
	opts := append([]hcloud.ClientOption{hcloud.WithToken(cfg.APIToken)}, c.clientOpts...)
	client := hcloud.NewClient(opts...)

	inventoryControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"asset", "inventory", "server"})
	networkControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"network", "firewall", "port"})
	accessControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"ssh", "access", "privileged", "key"})
	backupControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"backup", "snapshot", "recovery"})

	total := 0

	if n, err := c.collectServers(ctx, client, orgID, cfg.Location, inventoryControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("hetzner_collector: server collection failed")
	} else {
		total += n
	}

	if n, err := c.collectFirewalls(ctx, client, orgID, networkControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("hetzner_collector: firewall collection failed")
	} else {
		total += n
	}

	if n, err := c.collectSSHKeys(ctx, client, orgID, accessControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("hetzner_collector: ssh key collection failed")
	} else {
		total += n
	}

	if n, err := c.collectSnapshots(ctx, client, orgID, backupControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("hetzner_collector: snapshot collection failed")
	} else {
		total += n
	}

	return total, nil
}

// CountServers returns the current server count for a given org + token (used by GetHetznerStatus).
func (c *HetznerCollector) CountServers(ctx context.Context, cfg HetznerConfig) (int, error) {
	clientOpts := append([]hcloud.ClientOption{hcloud.WithToken(cfg.APIToken)}, c.clientOpts...)
	client := hcloud.NewClient(clientOpts...)
	listOpts := hcloud.ServerListOpts{}
	if cfg.Location != "" {
		listOpts.ListOpts = hcloud.ListOpts{LabelSelector: ""}
	}
	servers, _, err := client.Server.List(ctx, listOpts)
	if err != nil {
		return 0, err
	}
	return len(servers), nil
}

func (c *HetznerCollector) addEvidence(ctx context.Context, orgID, controlID, title string, details map[string]any) error {
	data, _ := json.Marshal(details)
	if controlID == "" {
		log.Debug().Str("org_id", orgID).Str("title", title).Msg("hetzner_collector: no matching control, skipping")
		return nil
	}
	return c.evidence.AddCollectorEvidence(ctx, orgID, controlID, "", hetznerSource, title, data)
}

func (c *HetznerCollector) collectServers(ctx context.Context, client *hcloud.Client, orgID, location string, controls []ControlMatch) (int, error) {
	opts := hcloud.ServerListOpts{}
	servers, _, err := client.Server.List(ctx, opts)
	if err != nil {
		return 0, fmt.Errorf("list servers: %w", err)
	}

	summaries := make([]map[string]any, 0, len(servers))
	for _, s := range servers {
		if location != "" && s.Location.Name != location {
			continue
		}
		entry := map[string]any{
			"name":     s.Name,
			"type":     s.ServerType.Name,
			"status":   string(s.Status),
			"location": s.Location.Name,
		}
		if s.Image != nil {
			entry["os"] = s.Image.Name
		}
		summaries = append(summaries, entry)
	}

	details := map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"server_count": len(summaries),
		"servers":      summaries,
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID, "Hetzner Server-Inventar", details); err != nil {
		return 0, err
	}
	return 1, nil
}

func (c *HetznerCollector) collectFirewalls(ctx context.Context, client *hcloud.Client, orgID string, controls []ControlMatch) (int, error) {
	firewalls, _, err := client.Firewall.List(ctx, hcloud.FirewallListOpts{})
	if err != nil {
		return 0, fmt.Errorf("list firewalls: %w", err)
	}

	summaries := make([]map[string]any, 0, len(firewalls))
	for _, fw := range firewalls {
		rules := make([]map[string]any, 0, len(fw.Rules))
		for _, r := range fw.Rules {
			rule := map[string]any{
				"direction": string(r.Direction),
				"protocol":  string(r.Protocol),
			}
			if r.Port != nil {
				rule["port"] = *r.Port
			}
			rules = append(rules, rule)
		}
		summaries = append(summaries, map[string]any{
			"name":       fw.Name,
			"rule_count": len(fw.Rules),
			"rules":      rules,
		})
	}

	details := map[string]any{
		"collected_at":   time.Now().UTC().Format(time.RFC3339),
		"firewall_count": len(firewalls),
		"firewalls":      summaries,
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID, "Hetzner Firewall-Regeln", details); err != nil {
		return 0, err
	}
	return 1, nil
}

func (c *HetznerCollector) collectSSHKeys(ctx context.Context, client *hcloud.Client, orgID string, controls []ControlMatch) (int, error) {
	keys, _, err := client.SSHKey.List(ctx, hcloud.SSHKeyListOpts{})
	if err != nil {
		return 0, fmt.Errorf("list ssh keys: %w", err)
	}

	summaries := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		summaries = append(summaries, map[string]any{
			"name":        k.Name,
			"fingerprint": k.Fingerprint,
			"created_at":  k.Created.Format(time.RFC3339),
		})
	}

	details := map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"key_count":    len(keys),
		"ssh_keys":     summaries,
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID, "Hetzner SSH-Keys", details); err != nil {
		return 0, err
	}
	return 1, nil
}

func (c *HetznerCollector) collectSnapshots(ctx context.Context, client *hcloud.Client, orgID string, controls []ControlMatch) (int, error) {
	servers, _, err := client.Server.List(ctx, hcloud.ServerListOpts{})
	if err != nil {
		return 0, fmt.Errorf("list servers for snapshot check: %w", err)
	}

	snapshots, _, err := client.Image.List(ctx, hcloud.ImageListOpts{Type: []hcloud.ImageType{hcloud.ImageTypeSnapshot}})
	if err != nil {
		return 0, fmt.Errorf("list snapshots: %w", err)
	}

	// Build set of server IDs that have at least one snapshot
	snapshotServerIDs := map[int64]bool{}
	for _, snap := range snapshots {
		if snap.CreatedFrom != nil {
			snapshotServerIDs[snap.CreatedFrom.ID] = true
		}
	}

	withoutBackup := []string{}
	for _, s := range servers {
		if !snapshotServerIDs[s.ID] {
			withoutBackup = append(withoutBackup, s.Name)
		}
	}

	details := map[string]any{
		"collected_at":             time.Now().UTC().Format(time.RFC3339),
		"server_count":             len(servers),
		"snapshot_count":           len(snapshots),
		"servers_without_snapshot": withoutBackup,
	}

	title := "Hetzner Snapshot-Nachweis (Backup)"
	controlID := firstControlID(controls)

	// One evidence entry for the overall backup status
	if err := c.addEvidence(ctx, orgID, controlID, title, details); err != nil {
		return 0, err
	}
	total := 1

	// Additional warning evidence per server without snapshot
	for _, name := range withoutBackup {
		warnDetails := map[string]any{
			"collected_at": time.Now().UTC().Format(time.RFC3339),
			"server_name":  name,
			"warning":      fmt.Sprintf("Server %s hat keinen aktuellen Snapshot (Backup fehlt).", name),
		}
		warnData, _ := json.Marshal(warnDetails)
		warnTitle := fmt.Sprintf("Hetzner Backup-Warnung: %s", name)
		if controlID != "" {
			_ = c.evidence.AddCollectorEvidence(ctx, orgID, controlID, "", hetznerSource, warnTitle, warnData)
		}
		total++
	}

	return total, nil
}
