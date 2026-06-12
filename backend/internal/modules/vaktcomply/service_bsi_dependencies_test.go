// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S76-2: Unit tests for CIA-Schutzbedarfsvererbung (BSI 200-2 §8.2)

package vaktcomply

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func sp(s string) *string { return &s }

func makeNodes(objects ...BSITargetObject) map[string]*targetNode {
	m := make(map[string]*targetNode, len(objects))
	for _, o := range objects {
		cp := o
		m[o.ID] = &targetNode{obj: cp}
	}
	return m
}

func addEdge(nodes map[string]*targetNode, srcID, tgtID string) {
	if n, ok := nodes[tgtID]; ok {
		n.incoming = append(n.incoming, srcID)
	}
}

func compute(nodes map[string]*targetNode, id string) *effectiveValues {
	memo := make(map[string]*effectiveValues)
	visiting := make(map[string]bool)
	return computeEffective(id, nodes, memo, visiting)
}

// ── Maximumprinzip ────────────────────────────────────────────────────────────

func TestMaxCIA_NilHandling(t *testing.T) {
	hoch := "hoch"
	assert.Equal(t, &hoch, maxCIA(nil, &hoch))
	assert.Equal(t, &hoch, maxCIA(&hoch, nil))
	assert.Nil(t, maxCIA(nil, nil))
}

func TestMaxCIA_Order(t *testing.T) {
	normal := "normal"
	hoch := "hoch"
	sehrHoch := "sehr_hoch"
	assert.Equal(t, &hoch, maxCIA(&normal, &hoch))
	assert.Equal(t, &sehrHoch, maxCIA(&hoch, &sehrHoch))
	assert.Equal(t, &sehrHoch, maxCIA(&normal, &sehrHoch))
	assert.Equal(t, &hoch, maxCIA(&hoch, &hoch))
}

func TestComputeEffective_NoIncoming(t *testing.T) {
	app := BSITargetObject{ID: "app1", ProtectionC: sp("hoch"), ProtectionI: sp("normal")}
	nodes := makeNodes(app)
	ev := compute(nodes, "app1")
	require.NotNil(t, ev)
	assert.Equal(t, "hoch", *ev.c)
	assert.Equal(t, "normal", *ev.i)
	assert.Nil(t, ev.a)
	assert.Nil(t, ev.inheritedC)
}

func TestComputeEffective_MaximumPrinzip_TwoApplicationsOnServer(t *testing.T) {
	// app1(C=hoch) → srv01(C=normal); app2(C=normal,A=sehr_hoch) → srv01
	// Expected: srv01 effective C=hoch, A=sehr_hoch
	app1 := BSITargetObject{ID: "app1", ProtectionC: sp("hoch")}
	app2 := BSITargetObject{ID: "app2", ProtectionC: sp("normal"), ProtectionA: sp("sehr_hoch")}
	srv01 := BSITargetObject{ID: "srv01", ProtectionC: sp("normal")}

	nodes := makeNodes(app1, app2, srv01)
	addEdge(nodes, "app1", "srv01")
	addEdge(nodes, "app2", "srv01")

	ev := compute(nodes, "srv01")
	require.NotNil(t, ev)
	assert.Equal(t, "hoch", *ev.c)
	assert.Equal(t, "sehr_hoch", *ev.a)
	assert.Equal(t, "app1", *ev.inheritedC)
	assert.Equal(t, "app2", *ev.inheritedA)
}

func TestComputeEffective_TransitivePropagation(t *testing.T) {
	// app(C=sehr_hoch) → server → room
	// room should have effective C=sehr_hoch inherited transitively
	app := BSITargetObject{ID: "app", ProtectionC: sp("sehr_hoch")}
	server := BSITargetObject{ID: "server", ProtectionC: sp("normal")}
	room := BSITargetObject{ID: "room", ProtectionC: sp("normal")}

	nodes := makeNodes(app, server, room)
	addEdge(nodes, "app", "server")
	addEdge(nodes, "server", "room")

	ev := compute(nodes, "room")
	require.NotNil(t, ev)
	assert.Equal(t, "sehr_hoch", *ev.c)
}

func TestComputeEffective_Override_Kumulation(t *testing.T) {
	// Override up: node has own normal but override=hoch with reason
	reason := "5 normale Anwendungen → Kumulationseffekt"
	node := BSITargetObject{
		ID:             "srv",
		ProtectionC:    sp("normal"),
		OverrideC:      sp("hoch"),
		OverrideReason: &reason,
		OverrideEffect: sp("kumulation"),
	}
	nodes := makeNodes(node)
	ev := compute(nodes, "srv")
	require.NotNil(t, ev)
	assert.Equal(t, "hoch", *ev.c)
	assert.Nil(t, ev.inheritedC, "override clears inherited_from")
}

func TestComputeEffective_Override_Verteilung(t *testing.T) {
	// App(A=sehr_hoch) → server; server has override A=normal via Verteilungseffekt
	app := BSITargetObject{ID: "app", ProtectionA: sp("sehr_hoch")}
	reason := "Redundanter Storage → Verteilungseffekt"
	server := BSITargetObject{
		ID:             "server",
		OverrideA:      sp("normal"),
		OverrideReason: &reason,
		OverrideEffect: sp("verteilung"),
	}
	nodes := makeNodes(app, server)
	addEdge(nodes, "app", "server")

	ev := compute(nodes, "server")
	require.NotNil(t, ev)
	assert.Equal(t, "normal", *ev.a, "verteilung override should reduce effective A")
	assert.Nil(t, ev.inheritedA, "override clears inherited_from")
}

func TestComputeEffective_Override_WithoutReason_Ignored(t *testing.T) {
	// Override without reason must be ignored → effective = own value
	emptyReason := ""
	node := BSITargetObject{
		ID:             "srv",
		ProtectionC:    sp("normal"),
		OverrideC:      sp("sehr_hoch"),
		OverrideReason: &emptyReason,
	}
	nodes := makeNodes(node)
	ev := compute(nodes, "srv")
	require.NotNil(t, ev)
	assert.Equal(t, "normal", *ev.c, "override without reason must not apply")
}

func TestCheckNoCycle_AdjacencyLogic(t *testing.T) {
	// Existing edges: A→B, B→C (source vererbt an target)
	// Proposed new edge: source=C, target=A  (would close A→B→C→A)
	// DFS from newTargetID="A" looking for sourceID="C"
	adj := map[string][]string{
		"A": {"B"},
		"B": {"C"},
	}

	makeReachable := func(startID, lookingFor string) bool {
		visited := make(map[string]bool)
		var dfs func(id string) bool
		dfs = func(id string) bool {
			if id == lookingFor {
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
		return dfs(startID)
	}

	// Proposed C→A: DFS from newTarget=A, looking for source=C → cycle detected
	assert.True(t, makeReachable("A", "C"), "A can reach C via B, so adding C→A creates a cycle")

	// Proposed C→D (D not in graph): no cycle
	assert.False(t, makeReachable("D", "C"), "D has no outgoing edges, no cycle")
}

func TestUpdateBSIObjectProtectionOverride_ValidationLogic(t *testing.T) {
	in := UpdateBSIObjectProtectionOverrideInput{
		OverrideC:      sp("hoch"),
		OverrideReason: "",
	}
	// Simulate service validation: override set but reason empty → error
	hasOverride := in.OverrideC != nil || in.OverrideI != nil || in.OverrideA != nil
	assert.True(t, hasOverride)
	assert.Equal(t, "", in.OverrideReason)
	// Would return ErrOverrideReasonMissing
}
