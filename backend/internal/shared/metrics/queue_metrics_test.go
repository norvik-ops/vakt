// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package metrics

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// R1-19-02: Der Rückstand einer Warteschlange war unsichtbar.
//
// `vakt_queue_depth` rechnete `Pending + Active`. Ein Auftrag, der dauerhaft
// fehlschlägt, wandert aber nach `Retry` und von dort nach `Archived` — beides
// zählte niemand. Live lagen 8 Wiederholungs- und 20 Archiv-Einträge über
// sieben Warteschlangen, während /metrics für jede einzelne 0 meldete.

// TestQueueMetricsShowRetryAndArchived ist die Regressionsprobe: Genau die
// Zahlen, die live unsichtbar waren, müssen jetzt in der Ausgabe stehen.
func TestQueueMetricsShowRetryAndArchived(t *testing.T) {
	var sb strings.Builder
	writeQueueMetrics(&sb, []queueSnapshot{
		{Name: "default", Pending: 2, Active: 1, Retry: 8, Archived: 20},
	})
	out := sb.String()

	assert.Contains(t, out, `vakt_queue_retry{queue="default"} 8`,
		"Wiederholungen fehlen in der Ausgabe — ein Auftrag, der immer wieder "+
			"fehlschlägt, bliebe unsichtbar")
	assert.Contains(t, out, `vakt_queue_archived{queue="default"} 20`,
		"endgültig aufgegebene Aufträge fehlen — genau der Zustand, der nicht "+
			"von selbst leerläuft")
	assert.Contains(t, out, `vakt_queue_depth{queue="default"} 3`,
		"depth muss weiterhin pending+active sein (2+1)")
}

// TestQueueDepthExcludesRetryAndArchived hält die Einordnung fest.
//
// Archivierte Aufträge dürfen NICHT in `depth` einfließen: Sie laufen nie von
// selbst leer, würden den Wert also dauerhaft anheben und jeden Lastschwellwert
// unbrauchbar machen — er feuert für immer, oder er wird so hoch gesetzt, dass
// er nie feuert. Ausserdem misst ein bestehendes Zabbix-Item sonst still etwas
// anderes als vorher.
func TestQueueDepthExcludesRetryAndArchived(t *testing.T) {
	var sb strings.Builder
	writeQueueMetrics(&sb, []queueSnapshot{
		{Name: "critical", Pending: 0, Active: 0, Retry: 5, Archived: 99},
	})

	assert.Contains(t, sb.String(), `vakt_queue_depth{queue="critical"} 0`,
		"depth darf Retry/Archived nicht mitzählen")
}

// TestQueueMetricsBaselineEmptyQueues ist die Baseline-Abnahme: Eine gesunde,
// leere Warteschlange meldet weiterhin überall 0 — das Gate darf ein gesundes
// System nicht rot färben.
func TestQueueMetricsBaselineEmptyQueues(t *testing.T) {
	var sb strings.Builder
	writeQueueMetrics(&sb, []queueSnapshot{{Name: "default"}})
	out := sb.String()

	for _, want := range []string{
		`vakt_queue_depth{queue="default"} 0`,
		`vakt_queue_retry{queue="default"} 0`,
		`vakt_queue_archived{queue="default"} 0`,
	} {
		assert.Contains(t, out, want)
	}
}

// TestQueueMetricFamiliesAlwaysDeclared: Ohne konfiguriertes Redis gibt es
// keine Stichproben, aber die Familien müssen deklariert bleiben — sonst
// verschwindet die Zeitreihe ganz, und das ist für einen Alarm etwas anderes
// als eine 0.
func TestQueueMetricFamiliesAlwaysDeclared(t *testing.T) {
	var sb strings.Builder
	writeQueueMetrics(&sb, nil)
	out := sb.String()

	for _, want := range []string{
		"# TYPE vakt_queue_depth gauge",
		"# TYPE vakt_queue_retry gauge",
		"# TYPE vakt_queue_archived gauge",
	} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, `queue="`, "ohne Warteschlangen darf keine Stichprobe entstehen")
}

// TestPrometheusFamiliesAreNotInterleaved: Jede Familie braucht ihre
// Stichproben direkt hinter ihrem eigenen HELP/TYPE-Kopf. Würden die drei
// Köpfe zuerst und die Stichproben danach geschrieben, wäre die Ausgabe kein
// gültiges Prometheus-Textformat mehr.
func TestPrometheusFamiliesAreNotInterleaved(t *testing.T) {
	var sb strings.Builder
	writeQueueMetrics(&sb, []queueSnapshot{{Name: "a", Pending: 1, Retry: 2, Archived: 3}})

	lines := strings.Split(strings.TrimSpace(sb.String()), "\n")
	require.Len(t, lines, 9, "drei Familien à HELP + TYPE + eine Stichprobe")

	var family string
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "# TYPE "):
			family = strings.Fields(l)[2]
		case strings.HasPrefix(l, "# HELP "):
			continue
		default:
			assert.True(t, strings.HasPrefix(l, family+"{"),
				"Stichprobe %q steht nicht unter ihrer eigenen Familie %q", l, family)
		}
	}
}

// R1-19-02, zweiter Teil: der Fehlerzähler, dessen Zeile verschwindet.
//
// `vakt_asynq_jobs_total{result="err"}` gab es nur, wenn kürzlich etwas
// fehlgeschlagen war. Auf eine Zeitreihe, die nicht existiert, kann kein
// Zabbix-Trigger auswerten — er bleibt stumm, statt 0 zu sehen.

func TestErrCounterExistsForEveryRunningTask(t *testing.T) {
	byKind := groupAsynqEntries([]asynqMetricEntry{
		{kind: "count", task: "vaktcomply_incident_deadline_check", result: "ok", value: "42"},
	})

	var found *asynqMetricEntry
	for i, e := range byKind["count"] {
		if e.task == "vaktcomply_incident_deadline_check" && e.result == "err" {
			found = &byKind["count"][i]
		}
	}
	require.NotNil(t, found,
		"ein Task, der bisher fehlerfrei lief, hat keine err-Zeitreihe — ein Alarm "+
			"darauf bekommt gar keinen Wert und feuert nie")
	assert.Equal(t, "0", found.value, "die ergänzte Zeile muss 0 lauten, nicht geraten sein")
}

func TestExistingErrCounterIsNotOverwritten(t *testing.T) {
	byKind := groupAsynqEntries([]asynqMetricEntry{
		{kind: "count", task: "t", result: "ok", value: "10"},
		{kind: "count", task: "t", result: "err", value: "7"},
	})

	require.Len(t, byKind["count"], 2, "es darf keine dritte, erfundene Zeile entstehen")
	for _, e := range byKind["count"] {
		if e.result == "err" {
			assert.Equal(t, "7", e.value, "ein echter Fehlerstand darf nicht auf 0 gesetzt werden")
		}
	}
}

// Ein Task, von dem es überhaupt keine Spur gibt, wird nicht erfunden — sonst
// meldete /metrics Zeitreihen für Tasks, die nie liefen.
func TestNoErrCounterInventedWithoutAnyCount(t *testing.T) {
	byKind := groupAsynqEntries([]asynqMetricEntry{
		{kind: "duration_ms_max", task: "nur_dauer", result: "ok", value: "5"},
	})
	assert.Empty(t, byKind["count"], "ohne count-Eintrag darf keine err-Zeile entstehen")
}

// Die Ausgabe muss bei gleicher Eingabe gleich bleiben — SCAN liefert die
// Schlüssel in beliebiger Reihenfolge.
func TestAsynqEntriesAreSortedDeterministically(t *testing.T) {
	in := []asynqMetricEntry{
		{kind: "count", task: "zulu", result: "ok", value: "1"},
		{kind: "count", task: "alpha", result: "ok", value: "1"},
		{kind: "count", task: "alpha", result: "err", value: "2"},
	}
	got := groupAsynqEntries(in)["count"]

	var order []string
	for _, e := range got {
		order = append(order, e.task+"/"+e.result)
	}
	assert.Equal(t, []string{"alpha/err", "alpha/ok", "zulu/err", "zulu/ok"}, order)
}
