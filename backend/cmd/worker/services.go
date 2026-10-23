// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/shared/notify"
	cloudintegration "github.com/matharnica/vakt/internal/shared/platform/integrations/cloud"
)

// Diese Datei ist die einzige Stelle im Worker, an der Modul-Dienste gebaut
// werden.
//
// R1-19-04 / R1-19-W02 — das Muster dahinter:
//
// Der Worker und die API bauen dieselben Dienste, aber an verschiedenen
// Stellen. Die API verdrahtet in cmd/api/routes.go ihre Abhängigkeiten; der
// Worker baute jeden Dienst dort, wo er ihn brauchte, und ließ die
// Verdrahtung weg. Ein Dienst ohne seine Abhängigkeit ist dabei nie kaputt
// gegangen — er hat still nichts getan, und der Aufrufer hat Erfolg geloggt.
//
// Gemessen wurden 24 Baustellen über 9 Diensttypen; 22 davon wichen von der
// API ab, drei davon folgenreich (die übrigen betrafen Abhängigkeiten, die
// auf keinem Worker-Pfad gelesen werden — siehe Commit-Text):
//
//	1. vaktcomply — ohne Benachrichtigungsdienst: NIS2-/DORA-Meldefrist-
//	   Mails gingen nie raus (12 Baustellen).
//	2. vakthr     — mit Noop-Nachweisschreiber (Modul inzwischen archiviert).
//	3. cloud      — mit Noop-Nachweisschreiber: die tägliche Cloud-
//	   Nachweiserhebung fand nichts und meldete Erfolg (1 Baustelle).
//
// Die Bauform gegen den Rückfall ist nicht „jede Stelle einmal richtig
// machen" — genau das wäre der Fix, der bei der dreizehnten Stelle wieder
// fehlt. Stattdessen: EINE Datei baut, alle anderen rufen nur ab, und ein
// Deckungsgate (services_wiring_test.go) weist jede Konstruktion außerhalb
// dieser Datei mit Datei und Zeile zurück. Wer einen neuen Handler schreibt,
// kann die Verdrahtung nicht vergessen, weil er gar nicht erst selbst baut.

// newComplyService baut den vaktcomply-Dienst so, wie der Worker ihn braucht.
//
// Bewusst NICHT verdrahtet: WithRedis, WithWebhooks, WithAIClient und
// WithAssetProtectionLinker. Alle vier sind auf der API-Seite gesetzt, werden
// aber von keinem Worker-Pfad gelesen (nachgemessen: die Leser liegen
// ausschließlich in Policy-Schreiboperationen und in CreateIncident, die der
// Worker nicht aufruft). Sie hier mitzunehmen wäre eine Verhaltensänderung
// ohne Befund.
func newComplyService(cfg *config.Config, pool *pgxpool.Pool) *vaktcomply.Service {
	svc := vaktcomply.NewService(pool)
	svc.WithNotifyService(notify.NewService(pool, cfg))
	return svc
}

// newCloudService baut den Cloud-Integrationsdienst mit echtem
// Nachweisschreiber.
//
// Mit dem Noop liefert FindControlsByKeywords nil und AddCollectorEvidence
// verwirft stillschweigend (cloud/evidence.go:27). Die Kollektoren melden dann
// `total == 0` ohne Fehler, was azure_collector.go:87 ausdrücklich als Erfolg
// wertet — last_sync_status='success' bei null erhobenen Nachweisen. Der
// Noop ist für den Fall gedacht, dass vaktcomply abgeschaltet ist; der Worker
// benutzte ihn unbedingt.
func newCloudService(cfg *config.Config, pool *pgxpool.Pool) *cloudintegration.Service {
	evidence := vaktcomply.NewCloudEvidenceWriter(newComplyService(cfg, pool).Repo())
	return cloudintegration.NewService(pool, workerKey(cfg, "vakt-cloud-v1"), evidence)
}
