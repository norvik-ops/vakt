// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S76-2: CIA-Schutzbedarfsvererbung nach BSI 200-2 Kap. 8.2 (ADR-0054)

package vaktcomply

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ── Protection level ordering ─────────────────────────────────────────────────

var ciaOrder = map[string]int{"normal": 0, "hoch": 1, "sehr_hoch": 2}

func maxCIA(a, b *string) *string {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if ciaOrder[*b] > ciaOrder[*a] {
		return b
	}
	return a
}

// ── Graph node used during traversal ─────────────────────────────────────────

type targetNode struct {
	obj      BSITargetObject
	incoming []string // source IDs pointing at this node
}

// loadOrgGraph fetches all objects + dependencies for an org in two queries.
func (s *Service) loadOrgGraph(ctx context.Context, orgID string) (map[string]*targetNode, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, org_id, name, type, description,
		       protection_c, protection_i, protection_a,
		       absicherungsniveau,
		       override_c, override_i, override_a, override_reason, override_effect,
		       created_at, updated_at
		FROM ck_bsi_target_objects
		WHERE org_id = $1`, orgID)
	if err != nil {
		return nil, fmt.Errorf("load bsi graph objects: %w", err)
	}
	defer rows.Close()

	nodes := make(map[string]*targetNode)
	for rows.Next() {
		var o BSITargetObject
		if err := rows.Scan(
			&o.ID, &o.OrgID, &o.Name, &o.Type, &o.Description,
			&o.ProtectionC, &o.ProtectionI, &o.ProtectionA,
			&o.Absicherungsniveau,
			&o.OverrideC, &o.OverrideI, &o.OverrideA, &o.OverrideReason, &o.OverrideEffect,
			&o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan bsi target object: %w", err)
		}
		nodes[o.ID] = &targetNode{obj: o}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	depRows, err := s.db.Query(ctx, `
		SELECT source_id, target_id
		FROM ck_bsi_target_object_dependencies
		WHERE org_id = $1`, orgID)
	if err != nil {
		return nil, fmt.Errorf("load bsi graph deps: %w", err)
	}
	defer depRows.Close()

	for depRows.Next() {
		var srcID, tgtID string
		if err := depRows.Scan(&srcID, &tgtID); err != nil {
			return nil, fmt.Errorf("scan bsi dep: %w", err)
		}
		if n, ok := nodes[tgtID]; ok {
			n.incoming = append(n.incoming, srcID)
		}
	}
	return nodes, depRows.Err()
}

// ── Effective protection computation ─────────────────────────────────────────

type effectiveValues struct {
	c, i, a    *string
	inheritedC *string
	inheritedI *string
	inheritedA *string
}

// computeEffective runs DFS for one node, memoised in `memo`.
// visiting is the current DFS path to detect cycles (should not happen at
// runtime since cycles are rejected on insert, but guard defensively).
func computeEffective(id string, nodes map[string]*targetNode, memo map[string]*effectiveValues, visiting map[string]bool) *effectiveValues {
	if v, ok := memo[id]; ok {
		return v
	}
	if visiting[id] {
		// Cycle guard — return own value without inheritance
		n := nodes[id]
		return &effectiveValues{c: n.obj.ProtectionC, i: n.obj.ProtectionI, a: n.obj.ProtectionA}
	}
	visiting[id] = true
	defer func() { delete(visiting, id) }()

	n, ok := nodes[id]
	if !ok {
		memo[id] = &effectiveValues{}
		return memo[id]
	}

	ev := &effectiveValues{
		c: n.obj.ProtectionC,
		i: n.obj.ProtectionI,
		a: n.obj.ProtectionA,
	}

	for _, srcID := range n.incoming {
		srcEV := computeEffective(srcID, nodes, memo, visiting)

		if better := maxCIA(ev.c, srcEV.c); better != nil && better != ev.c {
			ev.c = better
			ev.inheritedC = &srcID
		}
		if better := maxCIA(ev.i, srcEV.i); better != nil && better != ev.i {
			ev.i = better
			ev.inheritedI = &srcID
		}
		if better := maxCIA(ev.a, srcEV.a); better != nil && better != ev.a {
			ev.a = better
			ev.inheritedA = &srcID
		}
	}

	// Apply override if set AND reason is non-empty
	obj := n.obj
	if obj.OverrideC != nil && obj.OverrideReason != nil && *obj.OverrideReason != "" {
		ev.c = obj.OverrideC
		ev.inheritedC = nil
	}
	if obj.OverrideI != nil && obj.OverrideReason != nil && *obj.OverrideReason != "" {
		ev.i = obj.OverrideI
		ev.inheritedI = nil
	}
	if obj.OverrideA != nil && obj.OverrideReason != nil && *obj.OverrideReason != "" {
		ev.a = obj.OverrideA
		ev.inheritedA = nil
	}

	memo[id] = ev
	return ev
}

// ComputeEffectiveProtectionNeeds computes effective CIA values for all objects
// in the org, following the Maximumprinzip (BSI 200-2 §8.2).
// Returns a map keyed by object UUID.
func (s *Service) ComputeEffectiveProtectionNeeds(ctx context.Context, orgID string) (map[string]*effectiveValues, error) {
	nodes, err := s.loadOrgGraph(ctx, orgID)
	if err != nil {
		return nil, err
	}
	memo := make(map[string]*effectiveValues, len(nodes))
	visiting := make(map[string]bool)
	for id := range nodes {
		computeEffective(id, nodes, memo, visiting)
	}
	return memo, nil
}

// ── enrichTargetObjects applies effective values to a slice of objects ────────

func (s *Service) enrichWithEffective(ctx context.Context, orgID string, objects []BSITargetObject) ([]BSITargetObject, error) {
	effective, err := s.ComputeEffectiveProtectionNeeds(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for i := range objects {
		ev := effective[objects[i].ID]
		if ev == nil {
			continue
		}
		objects[i].EffectiveC = ev.c
		objects[i].EffectiveI = ev.i
		objects[i].EffectiveA = ev.a
		objects[i].InheritedFromC = ev.inheritedC
		objects[i].InheritedFromI = ev.inheritedI
		objects[i].InheritedFromA = ev.inheritedA
	}
	return objects, nil
}

// ── Dependency CRUD ───────────────────────────────────────────────────────────

// ListBSIObjectDependencies returns all dependency edges where the given object
// is the target (i.e., its incoming edges).
func (s *Service) ListBSIObjectDependencies(ctx context.Context, orgID, objectID string) ([]BSIObjectDependency, error) {
	rows, err := s.db.Query(ctx, `
		SELECT d.id, d.org_id, d.source_id, src.name, d.target_id, tgt.name,
		       d.dependency_type, d.created_at
		FROM ck_bsi_target_object_dependencies d
		JOIN ck_bsi_target_objects src ON src.id = d.source_id
		JOIN ck_bsi_target_objects tgt ON tgt.id = d.target_id
		WHERE d.org_id = $1
		  AND (d.source_id = $2 OR d.target_id = $2)
		ORDER BY d.created_at`, orgID, objectID)
	if err != nil {
		return nil, fmt.Errorf("list bsi object deps: %w", err)
	}
	defer rows.Close()

	var out []BSIObjectDependency
	for rows.Next() {
		var d BSIObjectDependency
		if err := rows.Scan(&d.ID, &d.OrgID, &d.SourceID, &d.SourceName,
			&d.TargetID, &d.TargetName, &d.DependencyType, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan bsi dep: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// CreateBSIObjectDependency adds a dependency edge from sourceID to input.TargetID.
// Returns ErrConflict if the edge already exists, ErrCycle if adding it would
// create a cycle, or ErrNotFound if either endpoint is not in the org.
func (s *Service) CreateBSIObjectDependency(ctx context.Context, orgID, sourceID string, in CreateBSIObjectDependencyInput) (*BSIObjectDependency, error) {
	if sourceID == in.TargetID {
		return nil, ErrCycle
	}
	// Verify both objects belong to this org
	if _, err := s.GetBSITargetObject(ctx, orgID, sourceID); err != nil {
		return nil, err
	}
	if _, err := s.GetBSITargetObject(ctx, orgID, in.TargetID); err != nil {
		return nil, err
	}

	// Cycle check: would adding source→target create a cycle?
	// A cycle exists if target is reachable from source via existing edges.
	if err := s.checkNoCycle(ctx, orgID, sourceID, in.TargetID); err != nil {
		return nil, err
	}

	var d BSIObjectDependency
	err := s.db.QueryRow(ctx, `
		INSERT INTO ck_bsi_target_object_dependencies
		  (org_id, source_id, target_id, dependency_type)
		VALUES ($1, $2, $3, $4)
		RETURNING id, org_id, source_id, target_id, dependency_type, created_at`,
		orgID, sourceID, in.TargetID, in.DependencyType).
		Scan(&d.ID, &d.OrgID, &d.SourceID, &d.TargetID, &d.DependencyType, &d.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("create bsi dep: %w", err)
	}

	// Populate names for the response
	if src, err2 := s.GetBSITargetObject(ctx, orgID, sourceID); err2 == nil {
		d.SourceName = src.Name
	}
	if tgt, err2 := s.GetBSITargetObject(ctx, orgID, in.TargetID); err2 == nil {
		d.TargetName = tgt.Name
	}
	return &d, nil
}

// DeleteBSIObjectDependency removes a dependency edge by its UUID.
func (s *Service) DeleteBSIObjectDependency(ctx context.Context, orgID, depID string) error {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM ck_bsi_target_object_dependencies WHERE org_id=$1 AND id=$2`, orgID, depID)
	if err != nil {
		return fmt.Errorf("delete bsi dep: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateBSIObjectProtectionOverride sets or clears the manual CIA override on a
// Zielobjekt. Overrides with any non-nil dimension require a non-empty reason.
func (s *Service) UpdateBSIObjectProtectionOverride(ctx context.Context, orgID, id string, in UpdateBSIObjectProtectionOverrideInput) (*BSITargetObject, error) {
	if (in.OverrideC != nil || in.OverrideI != nil || in.OverrideA != nil) && in.OverrideReason == "" {
		return nil, ErrOverrideReasonMissing
	}

	var reason *string
	if in.OverrideReason != "" {
		reason = &in.OverrideReason
	}

	var o BSITargetObject
	err := s.db.QueryRow(ctx, `
		UPDATE ck_bsi_target_objects
		SET override_c=$3, override_i=$4, override_a=$5,
		    override_reason=$6, override_effect=$7,
		    updated_at=NOW()
		WHERE org_id=$1 AND id=$2
		RETURNING id, org_id, name, type, description,
		          protection_c, protection_i, protection_a,
		          absicherungsniveau,
		          override_c, override_i, override_a, override_reason, override_effect,
		          created_at, updated_at`,
		orgID, id,
		in.OverrideC, in.OverrideI, in.OverrideA, reason, in.OverrideEffect).
		Scan(&o.ID, &o.OrgID, &o.Name, &o.Type, &o.Description,
			&o.ProtectionC, &o.ProtectionI, &o.ProtectionA,
			&o.Absicherungsniveau,
			&o.OverrideC, &o.OverrideI, &o.OverrideA, &o.OverrideReason, &o.OverrideEffect,
			&o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update bsi protection override: %w", err)
	}

	// Compute and attach effective values
	objs, err := s.enrichWithEffective(ctx, orgID, []BSITargetObject{o})
	if err != nil {
		return nil, err
	}
	return &objs[0], nil
}

// ── Cycle detection ───────────────────────────────────────────────────────────

// checkNoCycle returns ErrCycle if adding source→newTarget would create a cycle.
// Strategy: can newTarget reach source via existing edges? If yes → cycle.
func (s *Service) checkNoCycle(ctx context.Context, orgID, sourceID, newTargetID string) error {
	// Build adjacency list (source → targets) for existing edges
	rows, err := s.db.Query(ctx, `
		SELECT source_id, target_id
		FROM ck_bsi_target_object_dependencies
		WHERE org_id = $1`, orgID)
	if err != nil {
		return fmt.Errorf("cycle check query: %w", err)
	}
	defer rows.Close()

	adj := make(map[string][]string)
	for rows.Next() {
		var src, tgt string
		if err := rows.Scan(&src, &tgt); err != nil {
			return err
		}
		adj[src] = append(adj[src], tgt)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// DFS from newTargetID following existing edges; if we reach sourceID → cycle
	visited := make(map[string]bool)
	var dfs func(id string) bool
	dfs = func(id string) bool {
		if id == sourceID {
			return true
		}
		if visited[id] {
			return false
		}
		visited[id] = true
		for _, next := range adj[id] {
			if dfs(next) {
				return true
			}
		}
		return false
	}

	if dfs(newTargetID) {
		return ErrCycle
	}
	return nil
}
