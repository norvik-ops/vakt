// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/modules/vaktaware"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/modules/vakthr"
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
//	2. vakthr     — mit Noop-Nachweisschreiber: der tägliche Vertragsablauf-
//	   Job schrieb Offboarding-Nachweise ins Leere (1 Baustelle).
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

// newHRService baut den vakthr-Dienst mit echtem Nachweisschreiber.
//
// Ohne ihn steht in vakthr.Service ein Noop (service.go:74), und
// CheckContractorExpiry verwirft seinen Rückgabewert per `_ =` — der
// Offboarding-Nachweis entsteht nie, ohne Fehler und ohne Logzeile. Genau
// das ist die Zusage, die vakthr/service.go:43 als schon einmal gebrochen
// vermerkt.
//
// Der Nachweisschreiber lebt in vaktcomply. Ist das Modul abgeschaltet, bleibt
// es beim Noop — dieselbe Bedingung wie in der API (routes.go:513/537).
//
// R1-SA25-01 — zweite Abhängigkeit: der Eintritts-Auslöser.
//
// vakthr.CreateEmployee veröffentlicht seit R1-SA25-01 ein Eintritts-Ereignis,
// das vaktaware abonniert (automatische Einschreibung neuer Mitarbeiter). Ohne
// Verdrahtung steht dort der Noop, und ein künftiger Worker-Pfad, der
// Mitarbeiter anlegt (Personio-Abgleich, CSV-Import), würde die Einschreibung
// stillschweigend überspringen — dieselbe Bauform, die den Defekt überhaupt
// erst ermöglicht hat.
//
// Heute ruft kein Worker-Handler CreateEmployee auf; gemessen, nicht vermutet:
// der einzige Aufrufer ist vakthr/handler.go:125 (API). Die Verdrahtung ist
// deshalb Vorsorge, keine Reparatur — sie kostet nichts und nimmt dem nächsten
// Pfad die Gelegenheit, still danebenzugreifen.
//
// Der Auslöser ist SYNCHRON (siehe vaktaware.EnrollmentTrigger): der hier
// gebaute vaktaware-Dienst hat bewusst keinen Asynq-Client, und ein
// Warteschlangen-Weg würde an dieser Stelle genau wieder still nichts tun.
func newHRService(cfg *config.Config, pool *pgxpool.Pool) *vakthr.Service {
	evidence := vakthr.EvidenceWriter(vakthr.NoopEvidenceWriter())
	if cfg != nil && cfg.IsModuleEnabled("vaktcomply") {
		evidence = vaktcomply.NewHREvidenceWriter(pool)
	}
	var onboarding vakthr.EmployeeOnboardingTrigger = &vakthr.NoopEmployeeOnboardingTrigger{}
	if cfg != nil && cfg.IsModuleEnabled("vaktaware") {
		onboarding = vaktaware.NewEnrollmentTrigger(vaktaware.NewService(pool, vaktaware.SMTPConfig{}))
	}
	return vakthr.NewServiceFromPool(pool).
		WithEvidenceWriter(evidence).
		WithEmployeeOnboardingTrigger(onboarding)
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
	evidence := cloudintegration.NoopEvidenceWriter()
	if cfg != nil && cfg.IsModuleEnabled("vaktcomply") {
		evidence = vaktcomply.NewCloudEvidenceWriter(newComplyService(cfg, pool).Repo())
	}
	return cloudintegration.NewService(pool, workerKey(cfg, "vakt-cloud-v1"), evidence)
}
