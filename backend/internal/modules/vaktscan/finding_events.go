// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/services/crossevidence"
	"github.com/matharnica/vakt/internal/shared/platform/events"
	queuemetrics "github.com/matharnica/vakt/internal/shared/queuemetrics"
	"github.com/matharnica/vakt/internal/shared/redisopt"
	"github.com/matharnica/vakt/internal/shared/safego"
)

// ---------------------------------------------------------------------------
// R1-14b-02: Wo das "finding-created"-Ereignis entsteht — und warum HIER
// ---------------------------------------------------------------------------
//
// CLAUDE.md verspricht woertlich: "Scanner results flow as compliance evidence
// into Vakt Comply." Die Bruecke dafuer (vaktcomply.RecordScanFindingEvidence,
// gespeist ueber crossevidence.TaskRecordEvidence) war vorbildlich gebaut —
// idempotent, mit undurchsichtiger finding_id, ohne Cross-Modul-Datenbankzugriff.
// Sie bekam nur nie ein Ereignis.
//
// Der einzige Erzeuger war Service.UpsertFinding. Diese Methode hatte im ganzen
// Baum NULL Produktiv-Aufrufer. Saemtliche echten Pfade gehen an der
// Service-Schicht vorbei direkt ins Repository:
//
//	Repository.BatchUpsertFindings   ← RunTrivyScan, RunNucleiScan, RunOpenVASScan
//	Repository.UpsertImportedFinding ← ImportSARIF, ImportCycloneDX, ImportCSV (Service),
//	                                   ImportFindingsCSV (Handler), ImportWazuh (Handler)
//	Repository.UpsertFinding         ← (der Waise selbst)
//
// Live belegt: ein CSV-Import mit severity=critical legte das Finding an
// (imported: 1), ck_scan_evidence_map blieb bei 0.
//
// WARUM DAS REPOSITORY UND NICHT DIE SERVICE-/IMPORT-SCHICHT:
//
// Die saubere Schichtenantwort waere, das Ereignis in der Import-Schicht zu
// erzeugen. Sie scheitert an den Tatsachen dieses Codes:
//
//  1. Zwei der acht Erzeugerpfade (handler_csv.go, handler_wazuh.go) greifen
//     ueber `h.service.repo` unter der Service-Schicht hindurch. Ein Emit im
//     Service erreicht sie nicht.
//  2. Die drei Scanner-Pfade laufen als Asynq-Jobs mit der Signatur
//     RunXScan(ctx, *pgxpool.Pool, payload). Sie bauen ihr Repository selbst und
//     haben weder einen Service noch einen Asynq-Client. Ein Emit im Service
//     erreicht ausgerechnet die Pfade nicht, um die es im Versprechen geht.
//  3. Genau diese Klasse ist hier schon dreimal wiedergekehrt (v0.42.25/26/27):
//     ein Erzeuger, den ein neuer Pfad vergisst. Das Repository ist der Punkt,
//     an dem KEIN Pfad vorbeikommt.
//
// VERWORFENE ALTERNATIVE — Emit in der Import-/Service-Schicht: Sie haelte die
// Schichtentrennung sauber, muesste aber an acht Aufrufstellen wiederholt werden,
// erreicht zwei davon gar nicht, und der neunte Pfad vergisst sie wieder. Das ist
// dieselbe Wiederholungs-als-Abdeckung, die der UUID-Guard schon einmal gekostet
// hat (Nachtrag 2026-07-16: ein Guard gehoert an den obersten Punkt, unter dem
// die Invariante gilt, nicht an jede Untergruppe einzeln).
//
// Die Modul-Isolation bleibt gewahrt: hier entsteht nur eine Asynq-Task mit einer
// undurchsichtigen finding_id. vaktscan liest oder schreibt keine vaktcomply-
// Tabelle; das tut allein der Worker-Consumer ueber die geteilte
// Ereignis-Schnittstelle (events.FindingCreated / crossevidence).

// findingCreatedSink nimmt die Findings entgegen, die bei einem Upsert
// TATSAECHLICH NEU ENTSTANDEN sind. Jeder Sink filtert selbst weiter.
type findingCreatedSink interface {
	FindingsCreated(ctx context.Context, orgID string, created []Finding)
}

// isNewlyCreated entscheidet, ob ein zurueckgegebenes Finding eine Neuanlage ist.
//
// ENTSCHEIDUNG ZU AKTUALISIERUNGEN: Ein Ereignis entsteht ausschliesslich bei
// echter Neuanlage, nicht wenn ein Upsert einen bestehenden Fund nur hochzaehlt.
// Begruendung:
//
//   - Fachlich ist ein wiedergesehener Fund kein neuer Nachweis. Die Evidenz
//     haengt an der finding_id; sie existiert seit der Erstanlage.
//   - Die Bruecke ist ohnehin idempotent (ON CONFLICT DO NOTHING auf
//     ck_scan_evidence_map). Ereignisse fuer Wiedersehen waeren also Last ohne
//     Wirkung — bei jedem Re-Scan einmal pro bestehendem Fund.
//
// Erkennungsmerkmal ist occurrence_count == 1. Alle drei Upsert-Wege setzen die
// Spalte beim INSERT auf 1 und erhoehen sie im ON-CONFLICT-Zweig um 1 — der Wert
// kommt also aus der Datenbank, nicht aus der Absicht des Aufrufers (GB-2).
// Bewusst NICHT ueber das Systemfeld xmax: occurrence_count steht in allen
// Queries bereits im RETURNING, xmax muesste in drei Queries nachgezogen werden,
// davon zwei sqlc-generierte (eingefroren, ADR-0078).
//
// BEKANNTE GRENZE: Funde, die VOR diesem Fix angelegt wurden, bekommen bei einem
// Re-Scan keine Evidenz mehr nachtraeglich — sie sind dann occurrence_count > 1.
// Ein Nachzieh-Lauf ueber den Bestand ist eine eigene Aufgabe (braucht einen
// Job/eine Migration ausserhalb dieses Datei-Eigentums), nicht diese hier.
func isNewlyCreated(f Finding) bool {
	return f.OccurrenceCount == 1
}

// emitFindingsCreated meldet neu entstandene Findings an alle Sinks.
//
// ENTSCHEIDUNG ZU MASSENIMPORTEN: Es gibt KEINE Obergrenze und keine
// Zusammenfassung — ein Import mit 5000 neuen kritischen Funden erzeugt 5000
// Ereignisse. Ein Deckel wuerde audit-faehige Evidenz erzeugen, die stillschweigend
// zu wenig behauptet, und genau das ist in diesem Projekt die teuerste
// Fehlerklasse (eine plausibel aussehende Null). Die Last wird stattdessen an
// drei Stellen gedaempft, ohne je etwas fallenzulassen:
//
//  1. Nur Neuanlagen (siehe isNewlyCreated) — ein Re-Scan von 5000 unveraenderten
//     Funden erzeugt NULL Ereignisse.
//  2. Nur critical/high fuer die Compliance-Bruecke (siehe crossEvidenceSink) —
//     das ist die Schwelle, die der bisherige Erzeuger schon hatte, und sie passt
//     zum Ziel A.8.8/A.8.9.
//  3. Der Versand laeuft nebenlaeufig (safego.Run, Muster von triggerWebhook), so
//     dass kein Importpfad langsamer wird. Die Nebenlaeufigkeit ist auf
//     maxEmitGoroutines begrenzt; ist das Kontingent erschoepft, laeuft der
//     Versand SYNCHRON auf dem Aufrufer-Goroutine weiter. Das bremst einen
//     Massenimport spuerbar ab — es ist Gegendruck, kein Verwerfen. Ein Batch
//     (Scanner-Pfade) belegt dabei genau ein Kontingent fuer die ganze Liste.
func (r *Repository) emitFindingsCreated(ctx context.Context, orgID string, created []Finding) {
	if len(created) == 0 || len(r.sinks) == 0 {
		return
	}
	sinks := r.sinks
	deliver := func(c context.Context) error {
		for _, s := range sinks {
			s.FindingsCreated(c, orgID, created)
		}
		return nil
	}

	select {
	case emitSlots <- struct{}{}:
		safego.Run(ctx, "vaktscan.finding.created.emit", func(parent context.Context) error {
			defer func() { <-emitSlots }()
			c, cancel := context.WithTimeout(context.WithoutCancel(parent), 30*time.Second)
			defer cancel()
			return deliver(c)
		})
	default:
		// Kontingent erschoepft: lieber langsam als unvollstaendig.
		_ = deliver(ctx)
	}
}

// emitNewFinding ist die Einzelzeilen-Form von emitFindingsCreated: sie meldet
// genau dann, wenn das Finding neu entstanden ist.
func (r *Repository) emitNewFinding(ctx context.Context, orgID string, f Finding) {
	if !isNewlyCreated(f) {
		return
	}
	r.emitFindingsCreated(ctx, orgID, []Finding{f})
}

// maxEmitGoroutines begrenzt die gleichzeitig laufenden Versand-Goroutinen.
// Der Einzel-Upsert wird pro Zeile aufgerufen; ohne Grenze spawnte ein
// 5000-Zeilen-Import 5000 Goroutinen.
const maxEmitGoroutines = 64

var emitSlots = make(chan struct{}, maxEmitGoroutines)

// ---------------------------------------------------------------------------
// Sink 1: Compliance-Bruecke nach Vakt Comply
// ---------------------------------------------------------------------------

// crossEvidenceSink stellt fuer jeden neuen critical/high-Fund eine
// crossevidence-Task ein. Der Worker-Consumer schreibt daraus die Zeile in
// ck_scan_evidence_map und haengt die Evidenz an die passenden Controls.
type crossEvidenceSink struct{}

func (crossEvidenceSink) FindingsCreated(ctx context.Context, orgID string, created []Finding) {
	client := crossEvidenceClient()
	if client == nil {
		return
	}
	var enqueued, failed int
	for _, f := range created {
		if f.Severity != "critical" && f.Severity != "high" {
			continue
		}
		task, err := crossevidence.NewRecordEvidenceTask(
			events.FindingCreated(orgID, f.ID, f.Title, f.Severity),
		)
		if err != nil {
			failed++
			continue
		}
		if _, err := client.EnqueueContext(ctx, task, asynq.Queue(crossevidence.Queue)); err != nil {
			queuemetrics.RecordError(crossevidence.Queue)
			failed++
			continue
		}
		enqueued++
	}
	if enqueued == 0 && failed == 0 {
		return
	}
	ev := log.Info()
	if failed > 0 {
		// Ein fehlgeschlagenes Enqueue heisst: fuer diesen Fund entsteht KEINE
		// Evidenz. Das darf nicht still passieren.
		ev = log.Warn()
	}
	ev.Str("org_id", orgID).
		Int("enqueued", enqueued).
		Int("failed", failed).
		Int("created_total", len(created)).
		Msg("vaktscan: finding-created Ereignisse eingestellt")
}

var (
	crossEvidenceMu     sync.Mutex
	crossEvidenceCached *asynq.Client
	crossEvidenceURL    string
	crossEvidenceWarn   sync.Once
)

// crossEvidenceClient baut den Enqueue-Client einmal pro Prozess.
//
// Die Adresse kommt aus VAKT_REDIS_URL ueber redisopt.AsynqFromURL — dieselbe
// und einzige Ableitung, die auch API und Worker benutzen. Direkt ein
// asynq.RedisClientOpt zu bauen waere genau der Defekt, den R1-14b-01 gerade
// geschlossen hat: eine Datenbanknummer in der URL laesst sonst Erzeuger und
// Verbraucher in verschiedenen Redis-Datenbanken landen, lautlos.
//
// Warum aus der Umgebung und nicht durchgereicht: Die drei Scanner-Jobs haben die
// Signatur RunXScan(ctx, *pgxpool.Pool, payload) und bekommen weder Config noch
// Client. Sie explizit zu verdrahten hiesse, cmd/worker zu aendern — das gehoert
// einer anderen Spur. Der Prozess liest hier dieselbe Variable, aus der auch
// config.RedisUrl stammt, es kann also nicht auseinanderlaufen.
// Der Client wird auf die URL geschluesselt zwischengespeichert, nicht per
// sync.Once. Zwei Gruende, beide real:
//
//   - Der leere Fall darf sich nicht festsetzen. Ein einziger Aufruf vor dem
//     Setzen der Variable haette den Client sonst fuer die ganze Prozesslaufzeit
//     auf nil festgenagelt — im Betrieb eine tote Kette nach einem spaeten
//     Config-Load, ohne dass irgendwo etwas fehlschlaegt.
//   - Aendert sich die URL, muss der Client mitziehen. Ein Client, der noch auf
//     die alte Redis-Datenbank zeigt, ist genau der Defekt aus R1-14b-01: Erzeuger
//     und Verbraucher reden lautlos aneinander vorbei.
//
// Gewarnt wird trotzdem nur einmal, sonst flutet jeder Import das Log.
func crossEvidenceClient() *asynq.Client {
	crossEvidenceMu.Lock()
	defer crossEvidenceMu.Unlock()

	raw := getEnv("VAKT_REDIS_URL")
	if raw == "" {
		crossEvidenceWarn.Do(func() {
			log.Warn().Msg("vaktscan: VAKT_REDIS_URL ist leer — es entstehen KEINE finding-created Ereignisse und damit keine Scan-Evidenz in Vakt Comply")
		})
		return nil
	}
	if crossEvidenceCached != nil && crossEvidenceURL == raw {
		return crossEvidenceCached
	}
	if crossEvidenceCached != nil {
		_ = crossEvidenceCached.Close()
	}
	crossEvidenceCached = asynq.NewClient(redisopt.AsynqFromURL(raw))
	crossEvidenceURL = raw
	return crossEvidenceCached
}

// ---------------------------------------------------------------------------
// Sink 2: ausgehender Webhook finding.created
// ---------------------------------------------------------------------------

// webhookSink feuert den ausgehenden Webhook "finding.created". Er hing am selben
// verwaisten Erzeuger und war damit ebenso tot.
//
// Anders als die Compliance-Bruecke filtert er NICHT auf critical/high: ein
// Abonnent von "finding.created" erwartet jeden neuen Fund. Das entspricht dem
// bisherigen (unerreichbaren) Verhalten in Service.UpsertFinding.
type webhookSink struct{ svc *Service }

func (w webhookSink) FindingsCreated(ctx context.Context, orgID string, created []Finding) {
	for _, f := range created {
		w.svc.triggerWebhook(ctx, orgID, "finding.created", map[string]any{
			"id":       f.ID,
			"title":    f.Title,
			"severity": f.Severity,
			"org_id":   orgID,
		})
	}
}
