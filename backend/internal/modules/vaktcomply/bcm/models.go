// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package bcm

import (
	"encoding/json"
	"time"
)

// ── S86: BIA / BCM types ──────────────────────────────────────────────────────

type BIAProcess struct {
	ID                  string    `json:"id"`
	OrgID               string    `json:"org_id"`
	Name                string    `json:"name"`
	Description         string    `json:"description"`
	ProcessOwner        string    `json:"process_owner"`
	Criticality         string    `json:"criticality"`
	Schutzbedarfsklasse int       `json:"schutzbedarfsklasse"`
	RTOHours            int       `json:"rto_hours"`
	RPOHours            int       `json:"rpo_hours"`
	MBCOPercent         int       `json:"mbco_percent"`
	Dependencies        []string  `json:"dependencies"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type CreateBIAProcessInput struct {
	Name                string   `json:"name"                 validate:"required"`
	Description         string   `json:"description"`
	ProcessOwner        string   `json:"process_owner"`
	Criticality         string   `json:"criticality"          validate:"required,oneof=high medium low"`
	Schutzbedarfsklasse int      `json:"schutzbedarfsklasse"  validate:"required,oneof=1 2 3"`
	RTOHours            int      `json:"rto_hours"            validate:"required,min=1"`
	RPOHours            int      `json:"rpo_hours"            validate:"required,min=1"`
	MBCOPercent         int      `json:"mbco_percent"         validate:"min=0,max=100"`
	Dependencies        []string `json:"dependencies"`
}

type UpdateBIAProcessInput struct {
	Name                string   `json:"name"                 validate:"required"`
	Description         string   `json:"description"`
	ProcessOwner        string   `json:"process_owner"`
	Criticality         string   `json:"criticality"          validate:"required,oneof=high medium low"`
	Schutzbedarfsklasse int      `json:"schutzbedarfsklasse"  validate:"required,oneof=1 2 3"`
	RTOHours            int      `json:"rto_hours"            validate:"required,min=1"`
	RPOHours            int      `json:"rpo_hours"            validate:"required,min=1"`
	MBCOPercent         int      `json:"mbco_percent"         validate:"min=0,max=100"`
	Dependencies        []string `json:"dependencies"`
}

type BIASummary struct {
	TotalProcesses   int         `json:"total_processes"`
	CriticalCount    int         `json:"critical_count"`
	ShortestRTOHours int         `json:"shortest_rto_hours"`
	KlasseBreakdown  map[int]int `json:"klasse_breakdown"`
}

// ── Recovery Plans ────────────────────────────────────────────────────────────

type RecoveryStep struct {
	Order       int    `json:"order"`
	Action      string `json:"action"`
	Responsible string `json:"responsible"`
	DurationMin int    `json:"duration_min"`
}

type RecoveryPlan struct {
	ID                 string         `json:"id"`
	OrgID              string         `json:"org_id"`
	BIAProcessID       *string        `json:"bia_process_id"`
	BIAProcessName     string         `json:"bia_process_name"`
	Title              string         `json:"title"`
	ActivationCriteria string         `json:"activation_criteria"`
	Responsible        string         `json:"responsible"`
	RTOHours           int            `json:"rto_hours"`
	Status             string         `json:"status"`
	Steps              []RecoveryStep `json:"steps"`
	LastTestedAt       *string        `json:"last_tested_at"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type CreateRecoveryPlanInput struct {
	BIAProcessID       *string        `json:"bia_process_id"`
	Title              string         `json:"title"              validate:"required"`
	ActivationCriteria string         `json:"activation_criteria"`
	Responsible        string         `json:"responsible"`
	RTOHours           int            `json:"rto_hours"          validate:"required,min=1"`
	Status             string         `json:"status"             validate:"required,oneof=draft active tested archived"`
	Steps              []RecoveryStep `json:"steps"`
	LastTestedAt       *string        `json:"last_tested_at"`
}

type UpdateRecoveryPlanInput struct {
	BIAProcessID       *string        `json:"bia_process_id"`
	Title              string         `json:"title"              validate:"required"`
	ActivationCriteria string         `json:"activation_criteria"`
	Responsible        string         `json:"responsible"`
	RTOHours           int            `json:"rto_hours"          validate:"required,min=1"`
	Status             string         `json:"status"             validate:"required,oneof=draft active tested archived"`
	Steps              []RecoveryStep `json:"steps"`
	LastTestedAt       *string        `json:"last_tested_at"`
}

// ── Emergency Contacts ────────────────────────────────────────────────────────

type EmergencyContact struct {
	ID              string    `json:"id"`
	OrgID           string    `json:"org_id"`
	Name            string    `json:"name"`
	Role            string    `json:"role"`
	Phone           string    `json:"phone"`
	Email           string    `json:"email"`
	EscalationLevel int       `json:"escalation_level"`
	Available247    bool      `json:"available_24_7"`
	Notes           string    `json:"notes"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type CreateEmergencyContactInput struct {
	Name            string `json:"name"             validate:"required"`
	Role            string `json:"role"`
	Phone           string `json:"phone"`
	Email           string `json:"email"            validate:"omitempty,email"`
	EscalationLevel int    `json:"escalation_level" validate:"required,oneof=1 2 3"`
	Available247    bool   `json:"available_24_7"`
	Notes           string `json:"notes"`
}

type UpdateEmergencyContactInput struct {
	Name            string `json:"name"             validate:"required"`
	Role            string `json:"role"`
	Phone           string `json:"phone"`
	Email           string `json:"email"            validate:"omitempty,email"`
	EscalationLevel int    `json:"escalation_level" validate:"required,oneof=1 2 3"`
	Available247    bool   `json:"available_24_7"`
	Notes           string `json:"notes"`
}

// ── S60: BCP / Notfallhandbuch ────────────────────────────────────────────────

// BCPPlan represents a Business Continuity Plan document.
//
// ESK-12: RTOHours/RPOHours/Schutzbedarfsklasse sind Zeiger, weil "fuer diesen
// Plan noch nicht festgelegt" ein echter Zustand ist und als `null` gemeldet
// gehoert. Ein int haette den Zustand nicht ausdruecken koennen — er haette die
// Migrations-Defaults 72/24/2 als planbezogene BSI-200-4-Angabe ausgegeben, also
// eine Zahl behauptet, die niemand entschieden hat. RTO/RPO ist Audit-Evidenz.
// LastTestedAt wird aus ck_bcp_tests ABGELEITET und ist kein Eingabefeld.
type BCPPlan struct {
	ID                  string    `json:"id"`
	OrgID               string    `json:"org_id"`
	Title               string    `json:"title"`
	Scope               string    `json:"scope"`
	Version             string    `json:"version"`
	Status              string    `json:"status"`
	Owner               string    `json:"owner"`
	RTOHours            *int      `json:"rto_hours"`
	RPOHours            *int      `json:"rpo_hours"`
	Schutzbedarfsklasse *int      `json:"schutzbedarfsklasse"`
	LastTestedAt        *string   `json:"last_tested_at"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// BCPTest represents a single BCP test record for a plan.
type BCPTest struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	PlanID    string    `json:"plan_id"`
	TestDate  string    `json:"test_date"`
	TestType  string    `json:"test_type"`
	Outcome   string    `json:"outcome"`
	Findings  string    `json:"findings"`
	CreatedAt time.Time `json:"created_at"`
}

// BCPPlanMaxRTOHours begrenzt rto_hours/rpo_hours auf ein Jahr. Eine
// Wiederanlaufzeit jenseits davon ist kein Kontinuitaetsziel mehr, sondern ein
// Tippfehler; die Untergrenze 1 haelt die 0 draussen, die in der Antwort wie
// "sofort" aussaehe, aber in Wahrheit "nicht gesetzt" hiesse (ESK-12).
const BCPPlanMaxRTOHours = 8760

// CreateBCPPlanInput is the request body for creating a BCP plan.
//
// ESK-12: Die drei BSI-200-4-Felder sind OPTIONAL (Zeiger) — ein bestehender
// Aufrufer, der sie nicht schickt, verhaelt sich unveraendert (PROCESS.md P3,
// additiv). Weggelassen heisst kuenftig `null` = "noch nicht festgelegt", nicht
// mehr 72/24/2. last_tested_at fehlt hier bewusst: es wird aus den
// ck_bcp_tests-Eintraegen abgeleitet (siehe RefreshCKBCPPlanLastTested).
//
// Die drei Felder tragen ABSICHTLICH keine `validate`-Bereichstags mehr
// (REV-ESK12 B2). Mit ihnen fing `h.validate.Struct` jede Bereichsverletzung ab
// und antwortete "Ungültige Eingabe" — der Aufrufer erfuhr also nicht, WELCHES
// Feld er korrigieren muss, und die drei benannten Sentinels aus
// validateBCPPlanTargets waren ueber HTTP unerreichbar. Die Pruefung liegt jetzt
// nur noch dort, mit Namen und verletzendem Wert im Body.
type CreateBCPPlanInput struct {
	Title               string `json:"title"                validate:"required"`
	Scope               string `json:"scope"`
	Version             string `json:"version"`
	Status              string `json:"status"               validate:"omitempty,oneof=draft active archived"`
	Owner               string `json:"owner"`
	RTOHours            *int   `json:"rto_hours"`
	RPOHours            *int   `json:"rpo_hours"`
	Schutzbedarfsklasse *int   `json:"schutzbedarfsklasse"`
}

// UpdateBCPPlanInput is the request body for updating a BCP plan (PATCH).
//
// SEMANTIK (REV-ESK12 B1, ausdruecklich entschieden und hier maschinenlesbar):
//
//	Feld fehlt im Body   -> der gespeicherte Wert bleibt UNVERAENDERT
//	Feld ist `null`      -> der gespeicherte Wert wird GELOESCHT
//	Feld traegt einen Wert -> dieser Wert wird gesetzt
//
// Das ist RFC 7386 (JSON Merge Patch) und das, was der Methodenname PATCH
// zusagt. Die Vorfassung dieses Commits hat stattdessen ersetzt: ein PATCH mit
// {"title":…,"status":…} — also genau der Body, den openapi.yaml vorher als
// vollstaendig auswies — loeschte rto_hours/rpo_hours/schutzbedarfsklasse mit
// 200 und ohne Meldung. Das ist Datenverlust an derselben Audit-Evidenz, um die
// dieser Befund geht: eine geloeschte RTO-Angabe ist in der Antwort nicht von
// "noch nicht festgelegt" zu unterscheiden. Die Konsistenz mit den
// String-Feldern, die das Argument dafuer war, ist jetzt andersherum
// hergestellt — scope/version/owner mergen ebenfalls (auch sie gingen bei einem
// PATCH ohne sie verloren; dieselbe Klasse, nur ohne eigenen Befund).
//
// title und status bleiben Pflichtfelder und damit ersetzend: was man nicht
// weglassen KANN, kann man auch nicht durch Weglassen verlieren. Ein `""` in
// scope/version/owner ist weiterhin ein zulaessiger Wert und leert das Feld —
// dafuer muss es jetzt mitgeschickt werden.
type UpdateBCPPlanInput struct {
	Title               string      `json:"title"                validate:"required"`
	Scope               *string     `json:"scope"`
	Version             *string     `json:"version"`
	Status              string      `json:"status"               validate:"required,oneof=draft active archived"`
	Owner               *string     `json:"owner"`
	RTOHours            OptionalInt `json:"rto_hours"`
	RPOHours            OptionalInt `json:"rpo_hours"`
	Schutzbedarfsklasse OptionalInt `json:"schutzbedarfsklasse"`
}

// OptionalInt unterscheidet die DREI Zustaende, die ein Feld in einem
// PATCH-Body haben kann. Ein `*int` kann nur zwei: "weggelassen" und "explizit
// null" kommen beide als nil an, encoding/json setzt einen Zeiger bei `null`
// auf nil und laesst ihn bei fehlendem Schluessel unberuehrt. Genau diese
// Ununterscheidbarkeit war der Datenverlust aus REV-ESK12 B1 — die Information
// lag im Body, wurde aber beim Dekodieren weggeworfen.
//
//	{}                     -> Set=false            -> unveraendert
//	{"rto_hours": null}    -> Set=true,  Value=nil  -> geloescht
//	{"rto_hours": 8}       -> Set=true,  Value=&8   -> gesetzt
type OptionalInt struct {
	Set   bool
	Value *int
}

// UnmarshalJSON wird von encoding/json NUR aufgerufen, wenn der Schluessel im
// Body vorkommt — das ist die Quelle der Information "Set".
func (o *OptionalInt) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.Value = nil
		return nil
	}
	var v int
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}

// MarshalJSON haelt den Rundlauf dicht: ein geloeschtes Feld muss als `null`
// wieder herauskommen, nicht als 0.
func (o OptionalInt) MarshalJSON() ([]byte, error) {
	if o.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*o.Value)
}

// SetInt / ClearInt bauen die beiden gesetzten Zustaende im Go-Code (Tests,
// interne Aufrufer) so, wie sie ueber die Leitung ankaemen.
func SetInt(v int) OptionalInt { return OptionalInt{Set: true, Value: &v} }

// ClearInt entspricht `"feld": null` im Body: ausdruecklich loeschen.
func ClearInt() OptionalInt { return OptionalInt{Set: true} }

// resolve liefert den Wert, der nach diesem PATCH gelten soll.
func (o OptionalInt) resolve(current *int) *int {
	if !o.Set {
		return current
	}
	return o.Value
}

// resolveString liefert den String-Wert, der nach diesem PATCH gelten soll.
func resolveString(in *string, current string) string {
	if in == nil {
		return current
	}
	return *in
}

// MergeInto legt den Requestbody ueber den gespeicherten Plan und gibt den
// Zustand zurueck, der geschrieben werden soll. Sie ist der eine Ort, an dem die
// oben dokumentierte Semantik steht — der Rundlauftest
// TestBCPPlanPatch_OmittedFieldsSurvive prueft sie durch die volle Naht.
func (in UpdateBCPPlanInput) MergeInto(cur BCPPlan) BCPPlan {
	next := cur
	next.Title = in.Title
	next.Status = in.Status
	next.Scope = resolveString(in.Scope, cur.Scope)
	next.Version = resolveString(in.Version, cur.Version)
	next.Owner = resolveString(in.Owner, cur.Owner)
	next.RTOHours = in.RTOHours.resolve(cur.RTOHours)
	next.RPOHours = in.RPOHours.resolve(cur.RPOHours)
	next.Schutzbedarfsklasse = in.Schutzbedarfsklasse.resolve(cur.Schutzbedarfsklasse)
	return next
}

// CreateBCPTestInput is the request body for logging a BCP test.
type CreateBCPTestInput struct {
	TestDate string `json:"test_date" validate:"required"`
	TestType string `json:"test_type" validate:"required,oneof=tabletop walkthrough fulltest"`
	Outcome  string `json:"outcome"   validate:"required,oneof=passed failed partial"`
	Findings string `json:"findings"`
}

// LinkBCPPlanEvidenceInput optionally carries a control_id to link the plan as evidence.
type LinkBCPPlanEvidenceInput struct {
	ControlID string `json:"control_id"`
}

// ── BCM Score ─────────────────────────────────────────────────────────────────

type BCMReadinessScore struct {
	Score    int            `json:"score"`
	Criteria []BCMCriterion `json:"criteria"`
}

type BCMCriterion struct {
	Key    string `json:"key"`
	Met    bool   `json:"met"`
	Points int    `json:"points"`
}
