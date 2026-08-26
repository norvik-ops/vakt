// Package vaktprivacy provides DSGVO documentation: VVT, DPIA, AVV, breach notifications, and DSR tracking.
package vaktprivacy

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/matharnica/vakt/internal/db"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
	sharedevents "github.com/matharnica/vakt/internal/shared/events"
)

// Repository handles PrivacyOps data access. VVT uses sqlc (see ADR-0005 / the
// incremental migration plan); DPIA, AVV, Breach, DSR remain on embedded SQL
// and will follow in subsequent sessions.
type Repository struct {
	db *pgxpool.Pool
	q  *db.Queries
	// subjectErasers erase module-owned PII (hr_/sr_ …) during an Art. 17 DSGVO
	// erasure. vaktprivacy must not write those prefixes itself (module isolation,
	// ADR-0079); each owner module implements the eraser over its own tables and
	// is injected here from cmd/api. Order is IRRELEVANT — every cross-module
	// identifier is pre-resolved into events.SubjectRef before any eraser runs.
	subjectErasers []sharedevents.SubjectErasure
	// subjectResolver resolves identifiers owned by another module (currently
	// hr_employees ids) before the erasers run, so no eraser depends on another
	// module's rows still existing.
	subjectResolver sharedevents.SubjectResolver
}

// requiredEraserModules lists every module known to store data-subject PII. It
// is the completeness contract for Art. 17: ExecuteErasure refuses to run
// unless an eraser is wired for EACH of these.
//
// Checking "the list is non-empty" is not enough — the dangerous case is a
// PARTIALLY wired erasure, which would delete one module's PII, leave another's
// in place, and still stamp the DSR "completed". Adding a module that stores
// subject PII means adding it here; the wiring test in erasure_note_test.go
// fails otherwise.
var requiredEraserModules = []string{"vaktaware", "vakthr"}

// NewRepository creates a new PrivacyOps repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{db: pool, q: db.New(pool)}
}

// WithSubjectErasers injects the module-owned PII erasers used by ExecuteErasure
// (Art. 17 DSGVO). Call order does NOT matter: cross-module identifiers are
// resolved up front into events.SubjectRef, so no eraser depends on another
// having run (or not yet run) first.
//
// Erasers are APPENDED, not replaced, and de-duplicated by module name. A second
// call previously discarded the first silently — which, combined with the
// completeness check, could have turned a wiring mistake into a partial erasure.
// ExecuteErasure refuses to run unless every requiredEraserModules entry is
// present: a partial Art. 17 erasure is a compliance failure, not a soft one.
func (r *Repository) WithSubjectErasers(erasers ...sharedevents.SubjectErasure) *Repository {
	for _, e := range erasers {
		dup := false
		for _, existing := range r.subjectErasers {
			if existing.ModuleName() == e.ModuleName() {
				dup = true
				break
			}
		}
		if !dup {
			r.subjectErasers = append(r.subjectErasers, e)
		}
	}
	return r
}

// WithSubjectResolver injects the resolver for identifiers owned by another
// module (hr_employees ids). ExecuteErasure refuses to run without it: without
// resolution the vaktaware enrollment delete would match nothing and silently
// leave PII behind.
func (r *Repository) WithSubjectResolver(res sharedevents.SubjectResolver) *Repository {
	r.subjectResolver = res
	return r
}

// optText collapses an empty string into a NULL pgtype.Text so that the
// generated NULLable column maps cleanly.
func optText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// textOrEmpty returns the inner string of a pgtype.Text, or "" if NULL.
func textOrEmpty(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// tsToTime collapses a NULLable Timestamptz to a zero-value time.Time.
func tsToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// uuidPtr / dateOrNil / timestampOrNil / intOrNil — small helpers for the
// pgtype <-> domain mapping. Centralised here so DPIA, AVV, Breach et al.
// share the same conversion semantics.
func uuidPtrFromUUID(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	// pgtype.UUID.String() returns the canonical 36-char form.
	s := u.String()
	return &s
}

func dateOrNilToTimePtr(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	tt := d.Time
	return &tt
}

func optDate(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

func tsToTimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	tt := t.Time
	return &tt
}

func optUUID(s *string) pgtype.UUID {
	if s == nil || *s == "" {
		return pgtype.UUID{}
	}
	var u pgtype.UUID
	_ = u.Scan(*s)
	return u
}

// vvtFromRow converts a sqlc-generated row to the domain VVTEntry.
func vvtFromRow(r db.PoVvtEntries) VVTEntry {
	return VVTEntry{
		ID:                   r.ID,
		OrgID:                r.OrgID,
		Name:                 r.Name,
		Purpose:              r.Purpose,
		LegalBasis:           r.LegalBasis,
		DataCategories:       r.DataCategories,
		DataSubjects:         r.DataSubjects,
		Recipients:           r.Recipients,
		RetentionPeriod:      textOrEmpty(r.RetentionPeriod),
		ThirdCountryTransfer: r.ThirdCountryTransfer,
		Safeguards:           textOrEmpty(r.Safeguards),
		ResponsiblePerson:    textOrEmpty(r.ResponsiblePerson),
		Status:               r.Status,
		CreatedAt:            tsToTime(r.CreatedAt),
		UpdatedAt:            tsToTime(r.UpdatedAt),
	}
}

// --- VVT (sqlc) ---

// ListVVT returns all VVT entries for the organisation, ordered newest first.
func (r *Repository) ListVVT(ctx context.Context, orgID string) ([]VVTEntry, error) {
	rows, err := r.q.ListPPVVT(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list vvt: %w", err)
	}
	out := make([]VVTEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, vvtFromRow(row))
	}
	return out, nil
}

// CreateVVT inserts a new VVT entry and returns the persisted record.
func (r *Repository) CreateVVT(ctx context.Context, orgID string, in CreateVVTInput) (*VVTEntry, error) {
	row, err := r.q.CreatePPVVT(ctx, db.CreatePPVVTParams{
		OrgID:                orgID,
		Name:                 in.Name,
		Purpose:              in.Purpose,
		LegalBasis:           in.LegalBasis,
		DataCategories:       in.DataCategories,
		DataSubjects:         in.DataSubjects,
		Recipients:           in.Recipients,
		RetentionPeriod:      optText(in.RetentionPeriod),
		ThirdCountryTransfer: in.ThirdCountryTransfer,
		Safeguards:           optText(in.Safeguards),
		ResponsiblePerson:    optText(in.ResponsiblePerson),
	})
	if err != nil {
		return nil, fmt.Errorf("create vvt: %w", err)
	}
	v := vvtFromRow(row)
	return &v, nil
}

// --- DPIA (sqlc) ---

func dpiaFromRow(r db.PoDpias) DPIA {
	return DPIA{
		ID:                  r.ID,
		OrgID:               r.OrgID,
		VVTEntryID:          uuidPtrFromUUID(r.VvtEntryID),
		Title:               r.Title,
		Description:         textOrEmpty(r.Description),
		NecessityAssessment: textOrEmpty(r.NecessityAssessment),
		RiskAssessment:      textOrEmpty(r.RiskAssessment),
		MitigationMeasures:  textOrEmpty(r.MitigationMeasures),
		ResidualRisk:        textOrEmpty(r.ResidualRisk),
		DPOConsultation:     r.DpoConsultation,
		Status:              r.Status,
		ReviewedBy:          uuidPtrFromUUID(r.ReviewedBy),
		ReviewedAt:          tsToTimePtr(r.ReviewedAt),
		CreatedAt:           tsToTime(r.CreatedAt),
		UpdatedAt:           tsToTime(r.UpdatedAt),
	}
}

// ListDPIAs returns all DPIA records for the organisation, ordered newest first.
func (r *Repository) ListDPIAs(ctx context.Context, orgID string) ([]DPIA, error) {
	rows, err := r.q.ListPPDPIAs(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list dpias: %w", err)
	}
	out := make([]DPIA, 0, len(rows))
	for _, row := range rows {
		out = append(out, dpiaFromRow(row))
	}
	return out, nil
}

// CreateDPIA inserts a new DPIA in "draft" status and returns the full record.
func (r *Repository) CreateDPIA(ctx context.Context, orgID string, in CreateDPIAInput) (*DPIA, error) {
	row, err := r.q.CreatePPDPIA(ctx, db.CreatePPDPIAParams{
		OrgID:               orgID,
		VvtEntryID:          optUUID(in.VVTEntryID),
		Title:               in.Title,
		Description:         optText(in.Description),
		NecessityAssessment: optText(in.NecessityAssessment),
		RiskAssessment:      optText(in.RiskAssessment),
		MitigationMeasures:  optText(in.MitigationMeasures),
		ResidualRisk:        optText(in.ResidualRisk),
		DpoConsultation:     in.DPOConsultation,
	})
	if err != nil {
		return nil, fmt.Errorf("create dpia: %w", err)
	}
	d := dpiaFromRow(row)
	return &d, nil
}

// --- AVV (sqlc) ---

// avvFields is the minimal field set common to every AVV-returning sqlc row.
// sqlc emits a separate Row-type per query whose only difference is the field
// declaration order — instead of writing one mapper per type, we extract the
// fields explicitly. Keeps the per-call code tiny and the mapping logic single.
type avvFields struct {
	ID, OrgID, ProcessorName, ServiceDescription, Status string
	ContractDate, ReviewDate                             pgtype.Date
	Notes, TemplateID, Body                              pgtype.Text
	SccModule, SccAnnexI, SccAnnexIi, SccAnnexIii        pgtype.Text
	CreatedAt, UpdatedAt                                 pgtype.Timestamptz
}

func avvFromFields(f avvFields) AVV {
	return AVV{
		ID:                 f.ID,
		OrgID:              f.OrgID,
		ProcessorName:      f.ProcessorName,
		ServiceDescription: f.ServiceDescription,
		ContractDate:       dateOrNilToTimePtr(f.ContractDate),
		ReviewDate:         dateOrNilToTimePtr(f.ReviewDate),
		Status:             f.Status,
		Notes:              textOrEmpty(f.Notes),
		TemplateID:         textOrEmpty(f.TemplateID),
		Body:               textOrEmpty(f.Body),
		SCCModule:          textOrEmpty(f.SccModule),
		SCCAnnexI:          textOrEmpty(f.SccAnnexI),
		SCCAnnexII:         textOrEmpty(f.SccAnnexIi),
		SCCAnnexIII:        textOrEmpty(f.SccAnnexIii),
		CreatedAt:          tsToTime(f.CreatedAt),
		UpdatedAt:          tsToTime(f.UpdatedAt),
	}
}

// ListAVVs returns all AVV records for the organisation, ordered newest first.
func (r *Repository) ListAVVs(ctx context.Context, orgID string) ([]AVV, error) {
	rows, err := r.q.ListPPAVVs(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list avvs: %w", err)
	}
	out := make([]AVV, 0, len(rows))
	for _, row := range rows {
		out = append(out, avvFromFields(avvFields{
			ID: row.ID, OrgID: row.OrgID,
			ProcessorName: row.ProcessorName, ServiceDescription: row.ServiceDescription,
			Status: row.Status, ContractDate: row.ContractDate, ReviewDate: row.ReviewDate,
			Notes: row.Notes, TemplateID: row.TemplateID, Body: row.Body,
			SccModule: row.SccModule, SccAnnexI: row.SccAnnexI,
			SccAnnexIi: row.SccAnnexIi, SccAnnexIii: row.SccAnnexIii,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

// CreateAVV inserts a new AVV record in "active" status and returns the persisted entry.
func (r *Repository) CreateAVV(ctx context.Context, orgID string, in CreateAVVInput) (*AVV, error) {
	row, err := r.q.CreatePPAVV(ctx, db.CreatePPAVVParams{
		OrgID:              orgID,
		ProcessorName:      in.ProcessorName,
		ServiceDescription: in.ServiceDescription,
		ContractDate:       optDate(in.ContractDate),
		ReviewDate:         optDate(in.ReviewDate),
		Notes:              optText(in.Notes),
	})
	if err != nil {
		return nil, fmt.Errorf("create avv: %w", err)
	}
	a := avvFromFields(avvFields{
		ID: row.ID, OrgID: row.OrgID,
		ProcessorName: row.ProcessorName, ServiceDescription: row.ServiceDescription,
		Status: row.Status, ContractDate: row.ContractDate, ReviewDate: row.ReviewDate,
		Notes: row.Notes, TemplateID: row.TemplateID, Body: row.Body,
		SccModule: row.SccModule, SccAnnexI: row.SccAnnexI,
		SccAnnexIi: row.SccAnnexIi, SccAnnexIii: row.SccAnnexIii,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &a, nil
}

// --- Breach ---

// breachFromRow maps the sqlc PoBreaches row to the domain Breach struct.
func breachFromRow(r db.PoBreaches) Breach {
	var affected *int
	if r.AffectedCount.Valid {
		v := int(r.AffectedCount.Int32)
		affected = &v
	}
	return Breach{
		ID:                           r.ID,
		OrgID:                        r.OrgID,
		Title:                        r.Title,
		Description:                  r.Description,
		DiscoveredAt:                 tsToTime(r.DiscoveredAt),
		AuthorityDeadlineAt:          tsToTime(r.AuthorityDeadlineAt),
		AuthorityNotifiedAt:          tsToTimePtr(r.AuthorityNotifiedAt),
		SubjectsNotificationRequired: r.SubjectsNotificationRequired,
		SubjectsNotifiedAt:           tsToTimePtr(r.SubjectsNotifiedAt),
		AffectedCount:                affected,
		DataCategories:               r.DataCategories,
		Status:                       r.Status,
		CreatedAt:                    tsToTime(r.CreatedAt),
		UpdatedAt:                    tsToTime(r.UpdatedAt),
	}
}

// ListBreaches returns all breach records for the organisation, ordered by discovery date descending.
func (r *Repository) ListBreaches(ctx context.Context, orgID string) ([]Breach, error) {
	rows, err := r.q.ListPPBreaches(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list breaches: %w", err)
	}
	out := make([]Breach, 0, len(rows))
	for _, row := range rows {
		out = append(out, breachFromRow(row))
	}
	return out, nil
}

// CreateBreach inserts a breach record and derives authority_deadline_at as
// DiscoveredAt + 72 hours, reflecting the mandatory notification window under
// Art. 33 Abs. 1 DSGVO and NIS2 Art. 23.
func (r *Repository) CreateBreach(ctx context.Context, orgID string, in CreateBreachInput) (*Breach, error) {
	// Authority deadline is always 72 hours after discovery (NIS2 Art. 23 + DSGVO Art. 33).
	deadline := in.DiscoveredAt.Add(72 * time.Hour)

	var affected pgtype.Int4
	if in.AffectedCount != nil {
		affected = pgtype.Int4{Int32: int32(*in.AffectedCount), Valid: true}
	}

	row, err := r.q.CreatePPBreach(ctx, db.CreatePPBreachParams{
		OrgID:                        orgID,
		Title:                        in.Title,
		Description:                  in.Description,
		DiscoveredAt:                 pgtype.Timestamptz{Time: in.DiscoveredAt, Valid: true},
		AuthorityDeadlineAt:          pgtype.Timestamptz{Time: deadline, Valid: true},
		SubjectsNotificationRequired: in.SubjectsNotificationRequired,
		AffectedCount:                affected,
		DataCategories:               in.DataCategories,
	})
	if err != nil {
		return nil, fmt.Errorf("create breach: %w", err)
	}
	b := breachFromRow(row)
	return &b, nil
}

// UpdateBreachStatus changes the status field of a breach record.
// Intended for bulk or worker-driven transitions; prefer UpdateBreach for user-initiated edits.
func (r *Repository) UpdateBreachStatus(ctx context.Context, id, orgID, status string) error {
	return r.q.UpdatePPBreachStatus(ctx, db.UpdatePPBreachStatusParams{
		ID:     id,
		OrgID:  orgID,
		Status: status,
	})
}

// MarkAuthorityNotified stamps authority_notified_at to the current time,
// recording that the supervisory authority was informed as required by Art. 33 DSGVO.
func (r *Repository) MarkAuthorityNotified(ctx context.Context, id, orgID string) error {
	return r.q.MarkPPBreachAuthorityNotified(ctx, db.MarkPPBreachAuthorityNotifiedParams{
		ID:    id,
		OrgID: orgID,
	})
}

// --- VVT full CRUD ---

// GetVVT fetches a single VVT entry by ID, scoped to orgID.
func (r *Repository) GetVVT(ctx context.Context, orgID, id string) (*VVTEntry, error) {
	row, err := r.q.GetPPVVT(ctx, db.GetPPVVTParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get vvt %s: %w", id, err)
	}
	v := vvtFromRow(row)
	return &v, nil
}

// UpdateVVT replaces all mutable fields of a VVT entry and returns the updated record.
func (r *Repository) UpdateVVT(ctx context.Context, orgID, id string, in UpdateVVTInput) (*VVTEntry, error) {
	row, err := r.q.UpdatePPVVT(ctx, db.UpdatePPVVTParams{
		ID:                   id,
		OrgID:                orgID,
		Name:                 in.Name,
		Purpose:              in.Purpose,
		LegalBasis:           in.LegalBasis,
		DataCategories:       in.DataCategories,
		DataSubjects:         in.DataSubjects,
		Recipients:           in.Recipients,
		RetentionPeriod:      optText(in.RetentionPeriod),
		ThirdCountryTransfer: in.ThirdCountryTransfer,
		Safeguards:           optText(in.Safeguards),
		ResponsiblePerson:    optText(in.ResponsiblePerson),
		Status:               in.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("update vvt %s: %w", id, err)
	}
	v := vvtFromRow(row)
	return &v, nil
}

// DeleteVVT permanently removes a VVT entry. Scoped to orgID to prevent cross-tenant deletion.
func (r *Repository) DeleteVVT(ctx context.Context, orgID, id string) error {
	return r.q.DeletePPVVT(ctx, db.DeletePPVVTParams{ID: id, OrgID: orgID})
}

// --- DPIA full CRUD ---

// GetDPIA fetches a single DPIA record by ID, scoped to orgID.
func (r *Repository) GetDPIA(ctx context.Context, orgID, id string) (*DPIA, error) {
	row, err := r.q.GetPPDPIA(ctx, db.GetPPDPIAParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get dpia %s: %w", id, err)
	}
	d := dpiaFromRow(row)
	return &d, nil
}

// UpdateDPIA replaces the content fields of a DPIA without changing its approval state.
func (r *Repository) UpdateDPIA(ctx context.Context, orgID, id string, in UpdateDPIAInput) (*DPIA, error) {
	row, err := r.q.UpdatePPDPIA(ctx, db.UpdatePPDPIAParams{
		ID:                  id,
		OrgID:               orgID,
		Title:               in.Title,
		Description:         optText(in.Description),
		NecessityAssessment: optText(in.NecessityAssessment),
		RiskAssessment:      optText(in.RiskAssessment),
		MitigationMeasures:  optText(in.MitigationMeasures),
		ResidualRisk:        optText(in.ResidualRisk),
		DpoConsultation:     in.DPOConsultation,
	})
	if err != nil {
		return nil, fmt.Errorf("update dpia %s: %w", id, err)
	}
	d := dpiaFromRow(row)
	return &d, nil
}

// ApproveDPIA sets a DPIA's status to "approved" and records the reviewer's ID and timestamp.
// Art. 35 DSGVO requires documented approval before high-risk processing may begin.
func (r *Repository) ApproveDPIA(ctx context.Context, orgID, id, reviewerID string) (*DPIA, error) {
	row, err := r.q.ApprovePPDPIA(ctx, db.ApprovePPDPIAParams{
		ID:         id,
		OrgID:      orgID,
		ReviewedBy: optUUID(&reviewerID),
	})
	if err != nil {
		return nil, fmt.Errorf("approve dpia %s: %w", id, err)
	}
	d := dpiaFromRow(row)
	return &d, nil
}

// DeleteDPIA permanently removes a DPIA record. Scoped to orgID to prevent cross-tenant deletion.
func (r *Repository) DeleteDPIA(ctx context.Context, orgID, id string) error {
	return r.q.DeletePPDPIA(ctx, db.DeletePPDPIAParams{ID: id, OrgID: orgID})
}

// --- AVV full CRUD (sqlc) ---

// GetAVV fetches a single AVV record by ID, scoped to orgID.
func (r *Repository) GetAVV(ctx context.Context, orgID, id string) (*AVV, error) {
	row, err := r.q.GetPPAVV(ctx, db.GetPPAVVParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get avv %s: %w", id, err)
	}
	a := avvFromFields(avvFields{
		ID: row.ID, OrgID: row.OrgID,
		ProcessorName: row.ProcessorName, ServiceDescription: row.ServiceDescription,
		Status: row.Status, ContractDate: row.ContractDate, ReviewDate: row.ReviewDate,
		Notes: row.Notes, TemplateID: row.TemplateID, Body: row.Body,
		SccModule: row.SccModule, SccAnnexI: row.SccAnnexI,
		SccAnnexIi: row.SccAnnexIi, SccAnnexIii: row.SccAnnexIii,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &a, nil
}

// UpdateAVV replaces all mutable fields of an AVV record and returns the updated entry.
func (r *Repository) UpdateAVV(ctx context.Context, orgID, id string, in UpdateAVVInput) (*AVV, error) {
	row, err := r.q.UpdatePPAVV(ctx, db.UpdatePPAVVParams{
		ID:                 id,
		OrgID:              orgID,
		ProcessorName:      in.ProcessorName,
		ServiceDescription: in.ServiceDescription,
		ContractDate:       optDate(in.ContractDate),
		ReviewDate:         optDate(in.ReviewDate),
		Status:             in.Status,
		Notes:              optText(in.Notes),
	})
	if err != nil {
		return nil, fmt.Errorf("update avv %s: %w", id, err)
	}
	a := avvFromFields(avvFields{
		ID: row.ID, OrgID: row.OrgID,
		ProcessorName: row.ProcessorName, ServiceDescription: row.ServiceDescription,
		Status: row.Status, ContractDate: row.ContractDate, ReviewDate: row.ReviewDate,
		Notes: row.Notes, TemplateID: row.TemplateID, Body: row.Body,
		SccModule: row.SccModule, SccAnnexI: row.SccAnnexI,
		SccAnnexIi: row.SccAnnexIi, SccAnnexIii: row.SccAnnexIii,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &a, nil
}

// DeleteAVV permanently removes an AVV record. Scoped to orgID to prevent cross-tenant deletion.
func (r *Repository) DeleteAVV(ctx context.Context, orgID, id string) error {
	return r.q.DeletePPAVV(ctx, db.DeletePPAVVParams{ID: id, OrgID: orgID})
}

// GetAVVWithBody fetches a single AVV including template body and SCC fields.
// Note: GetAVV already returns body + SCC since sqlc always selects all fields;
// kept as a separate method to preserve the explicit "I need the body" contract.
func (r *Repository) GetAVVWithBody(ctx context.Context, orgID, id string) (*AVV, error) {
	return r.GetAVV(ctx, orgID, id)
}

// UpdateAVVBody sets the template_id and body fields of an AVV.
func (r *Repository) UpdateAVVBody(ctx context.Context, orgID, id, templateID, body string) error {
	return r.q.UpdatePPAVVBody(ctx, db.UpdatePPAVVBodyParams{
		ID:         id,
		OrgID:      orgID,
		TemplateID: optText(templateID),
		Body:       optText(body),
	})
}

// UpdateAVVSCC updates the SCC module and annex fields of an AVV.
func (r *Repository) UpdateAVVSCC(ctx context.Context, orgID, id, sccModule, annexI, annexII, annexIII string) error {
	return r.q.UpdatePPAVVSCC(ctx, db.UpdatePPAVVSCCParams{
		ID:          id,
		OrgID:       orgID,
		SccModule:   optText(sccModule),
		SccAnnexI:   optText(annexI),
		SccAnnexIi:  optText(annexII),
		SccAnnexIii: optText(annexIII),
	})
}

// CreateAVVWithBody inserts a new AVV with a pre-rendered template body.
func (r *Repository) CreateAVVWithBody(ctx context.Context, orgID, templateID, body, processorName, serviceDesc string) (*AVV, error) {
	row, err := r.q.CreatePPAVVWithBody(ctx, db.CreatePPAVVWithBodyParams{
		OrgID:              orgID,
		ProcessorName:      processorName,
		ServiceDescription: serviceDesc,
		TemplateID:         optText(templateID),
		Body:               optText(body),
	})
	if err != nil {
		return nil, fmt.Errorf("create avv with body: %w", err)
	}
	a := avvFromFields(avvFields{
		ID: row.ID, OrgID: row.OrgID,
		ProcessorName: row.ProcessorName, ServiceDescription: row.ServiceDescription,
		Status: row.Status, ContractDate: row.ContractDate, ReviewDate: row.ReviewDate,
		Notes: row.Notes, TemplateID: row.TemplateID, Body: row.Body,
		SccModule: row.SccModule, SccAnnexI: row.SccAnnexI,
		SccAnnexIi: row.SccAnnexIi, SccAnnexIii: row.SccAnnexIii,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &a, nil
}

// ListExpiringAVVs returns AVVs whose review_date is between now and the given threshold.
func (r *Repository) ListExpiringAVVs(ctx context.Context, threshold time.Time) ([]AVV, error) {
	rows, err := r.q.ListExpiringPPAVVs(ctx, pgtype.Date{Time: threshold, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list expiring avvs: %w", err)
	}
	out := make([]AVV, 0, len(rows))
	for _, row := range rows {
		out = append(out, avvFromFields(avvFields{
			ID: row.ID, OrgID: row.OrgID,
			ProcessorName: row.ProcessorName, ServiceDescription: row.ServiceDescription,
			Status: row.Status, ContractDate: row.ContractDate, ReviewDate: row.ReviewDate,
			Notes: row.Notes, TemplateID: row.TemplateID, Body: row.Body,
			SccModule: row.SccModule, SccAnnexI: row.SccAnnexI,
			SccAnnexIi: row.SccAnnexIi, SccAnnexIii: row.SccAnnexIii,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

// MarkExpiredAVVs updates status to 'expired' for AVVs whose review_date has passed.
func (r *Repository) MarkExpiredAVVs(ctx context.Context) (int64, error) {
	n, err := r.q.MarkExpiredPPAVVs(ctx)
	if err != nil {
		return 0, fmt.Errorf("mark expired avvs: %w", err)
	}
	return n, nil
}

// --- Breach full CRUD (sqlc) ---

// GetBreach fetches a single breach record by ID, scoped to orgID.
func (r *Repository) GetBreach(ctx context.Context, orgID, id string) (*Breach, error) {
	row, err := r.q.GetPPBreach(ctx, db.GetPPBreachParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get breach %s: %w", id, err)
	}
	b := breachFromRow(row)
	return &b, nil
}

// UpdateBreach replaces the editable fields of a breach record.
// Timestamps (discovered_at, authority_deadline_at, authority_notified_at) are immutable through this method.
func (r *Repository) UpdateBreach(ctx context.Context, orgID, id string, in UpdateBreachInput) (*Breach, error) {
	var affected pgtype.Int4
	if in.AffectedCount != nil {
		affected = pgtype.Int4{Int32: int32(*in.AffectedCount), Valid: true}
	}
	row, err := r.q.UpdatePPBreach(ctx, db.UpdatePPBreachParams{
		ID:                           id,
		OrgID:                        orgID,
		Title:                        in.Title,
		Description:                  in.Description,
		SubjectsNotificationRequired: in.SubjectsNotificationRequired,
		AffectedCount:                affected,
		DataCategories:               in.DataCategories,
	})
	if err != nil {
		return nil, fmt.Errorf("update breach %s: %w", id, err)
	}
	b := breachFromRow(row)
	return &b, nil
}

// DeleteBreach permanently removes a breach record. Scoped to orgID to prevent cross-tenant deletion.
func (r *Repository) DeleteBreach(ctx context.Context, orgID, id string) error {
	return r.q.DeletePPBreach(ctx, db.DeletePPBreachParams{ID: id, OrgID: orgID})
}

// --- DSR ---

// dsrFields holds the union of all DSR-returning row columns. Same pattern
// as avvFields (ADR-0013): sqlc emits separate Row-types per query whose only
// difference is field-order; instead of 5 copy-pasted mappers we extract the
// fields once.
type dsrFields struct {
	ID, OrgID, RequesterName, RequesterEmail, Type, Status string
	Description, Notes                                     pgtype.Text
	DueDate                                                pgtype.Date
	ReceivedAt, CompletedAt, CreatedAt, UpdatedAt          pgtype.Timestamptz
	// S131-G3/R-L03: resolved_by + extension_reason were written by the resolve/extend
	// path but never selected in any read → who resolved/extended a DSR was not
	// retrievable via the API. Zero-value on reads that do not select them.
	ResolvedBy, ExtensionReason pgtype.Text
	// S9/CZ-2: channel + reference_id are accepted on create (CreateDSRInput) and
	// assigned_to is written by AssignDSR, but no read mapper ever selected them —
	// every DSR response showed an empty channel/reference and no assignee.
	Channel, ReferenceID, AssignedTo pgtype.Text
}

func dsrFromFields(f dsrFields) DSR {
	return DSR{
		ID:              f.ID,
		OrgID:           f.OrgID,
		RequesterName:   f.RequesterName,
		RequesterEmail:  f.RequesterEmail,
		Type:            f.Type,
		Description:     textOrEmpty(f.Description),
		Status:          f.Status,
		DueDate:         dateOrYMD(f.DueDate),
		ReceivedAt:      tsToTime(f.ReceivedAt),
		CompletedAt:     tsToTimePtr(f.CompletedAt),
		Notes:           textOrEmpty(f.Notes),
		CreatedAt:       tsToTime(f.CreatedAt),
		UpdatedAt:       tsToTime(f.UpdatedAt),
		ResolvedBy:      textPtr(f.ResolvedBy),
		ExtensionReason: textOrEmpty(f.ExtensionReason),
		Channel:         textOrEmpty(f.Channel),
		ReferenceID:     textOrEmpty(f.ReferenceID),
		AssignedTo:      textPtr(f.AssignedTo),
	}
}

// textPtr renders a NULLable pgtype.Text as *string, nil when NULL or empty.
func textPtr(t pgtype.Text) *string {
	if !t.Valid || t.String == "" {
		return nil
	}
	s := t.String
	return &s
}

// dateOrYMD renders a NULLable pgtype.Date as *string in YYYY-MM-DD format —
// the wire format the DSR clients expect (legacy from when the repository used
// to_char()). Returns nil when the column is NULL.
func dateOrYMD(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format("2006-01-02")
	return &s
}

// ListDSRs returns all DSRs for the given organisation, newest first.
//
// S131-G3/D27-04: raw read (not the generated ListPPDSRs) so the offset-mode
// list of GET /vaktprivacy/dsr returns the same shape as the cursor mode —
// including resolved_by/extension_reason. Otherwise the two modes of the same
// endpoint would disagree on which fields are present.
func (r *Repository) ListDSRs(ctx context.Context, orgID string) ([]DSR, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, org_id, requester_name, requester_email, type, description,
		        status, due_date, received_at, completed_at, notes,
		        created_at, updated_at, resolved_by::text, extension_reason,
		        channel, reference_id, assigned_to::text
		 FROM po_dsr
		 WHERE org_id = $1
		 ORDER BY received_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list dsrs: %w", err)
	}
	defer rows.Close()
	out := []DSR{}
	for rows.Next() {
		var f dsrFields
		if err := rows.Scan(&f.ID, &f.OrgID, &f.RequesterName, &f.RequesterEmail, &f.Type, &f.Description,
			&f.Status, &f.DueDate, &f.ReceivedAt, &f.CompletedAt, &f.Notes,
			&f.CreatedAt, &f.UpdatedAt, &f.ResolvedBy, &f.ExtensionReason,
			&f.Channel, &f.ReferenceID, &f.AssignedTo); err != nil {
			return nil, fmt.Errorf("scan dsr row: %w", err)
		}
		out = append(out, dsrFromFields(f))
	}
	return out, rows.Err()
}

// CreateDSR inserts a new data subject request and automatically sets due_date
// to now + 30 calendar days, satisfying the Art. 12 Abs. 3 DSGVO response deadline.
//
// S9/CZ-2: raw INSERT (not the generated CreatePPDSR) because in.Channel and
// in.ReferenceID are validated CreateDSRInput fields that the generated query
// never wrote — every DSR silently dropped its intake channel and ticket
// reference on create, regardless of what the caller sent.
func (r *Repository) CreateDSR(ctx context.Context, orgID string, in CreateDSRInput) (*DSR, error) {
	due := pgtype.Date{Time: time.Now().UTC().AddDate(0, 0, 30), Valid: true}
	var f dsrFields
	err := r.db.QueryRow(ctx,
		`INSERT INTO po_dsr (org_id, requester_name, requester_email, type, description, due_date, channel, reference_id)
		 VALUES ($1, $2, $3, $4, NULLIF($5,''), $6, NULLIF($7,''), NULLIF($8,''))
		 RETURNING id, org_id, requester_name, requester_email, type, description,
		           status, due_date, received_at, completed_at, notes,
		           created_at, updated_at, resolved_by::text, extension_reason,
		           channel, reference_id, assigned_to::text`,
		orgID, in.RequesterName, in.RequesterEmail, in.Type, in.Description, due, in.Channel, in.ReferenceID,
	).Scan(&f.ID, &f.OrgID, &f.RequesterName, &f.RequesterEmail, &f.Type, &f.Description,
		&f.Status, &f.DueDate, &f.ReceivedAt, &f.CompletedAt, &f.Notes,
		&f.CreatedAt, &f.UpdatedAt, &f.ResolvedBy, &f.ExtensionReason,
		&f.Channel, &f.ReferenceID, &f.AssignedTo)
	if err != nil {
		return nil, fmt.Errorf("create dsr: %w", err)
	}
	d := dsrFromFields(f)
	return &d, nil
}

// UpdateDSR updates the status and notes of an existing DSR.
// When status is "completed" or "rejected" the method stamps completed_at with
// the current UTC time, recording how long the response took relative to due_date.
func (r *Repository) UpdateDSR(ctx context.Context, orgID, id string, in UpdateDSRInput) (*DSR, error) {
	// Art. 17 DSGVO, defence in depth. Service.UpdateDSR already diverts
	// erasure-type completions to ExecuteErasure, so this branch is unreachable
	// on the normal path. It is enforced here anyway because the invariant that
	// makes "executed" readable — ExecutePPDSRErasure is the ONLY producer of a
	// completed erasure row — has to hold at the layer that owns the write, not
	// at the layer that happens to route around it today.
	existing, err := r.GetDSR(ctx, orgID, id)
	if err != nil {
		return nil, fmt.Errorf("update dsr %s: fetch for type check: %w", id, err)
	}
	if err := guardErasureCompletion(existing, in.Status); err != nil {
		return nil, err
	}

	var completedAt pgtype.Timestamptz
	if in.Status == "completed" || in.Status == "rejected" {
		completedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	}

	// UpdatePPDSR overwrites notes wholesale; keep the Art. 17 evidence block.
	notes := preserveErasureEvidence(existing, in.Notes)

	if _, err := r.q.UpdatePPDSR(ctx, db.UpdatePPDSRParams{
		ID:          id,
		OrgID:       orgID,
		Status:      in.Status,
		Notes:       optText(notes),
		CompletedAt: completedAt,
	}); err != nil {
		return nil, fmt.Errorf("update dsr %s: %w", id, err)
	}
	// S131-G3/D27-04: NC-aware-analog re-read so the generic update response carries
	// resolved_by/extension_reason (the generated UpdatePPDSR RETURNING omits them) —
	// symmetric with UpdateCAPA→GetCAPA, so PATCH and GET can never disagree on shape.
	return r.GetDSR(ctx, orgID, id)
}

// DeleteDSR permanently removes a DSR record. Callers should only invoke this
// for erroneous duplicates; completed requests should instead be archived to
// preserve the audit trail required under Art. 5 Abs. 2 DSGVO (accountability).
func (r *Repository) DeleteDSR(ctx context.Context, orgID, id string) error {
	return r.q.DeletePPDSR(ctx, db.DeletePPDSRParams{ID: id, OrgID: orgID})
}

// AssignDSR sets the assignee of a DSR (empty string clears it), scoped to org.
// S121-D5 (A2): the raw UPDATE previously lived in the HTTP handler
// (h.service.db.Exec), breaking the handler→service→repository layering rule.
// The SQL now sits in the repository like every other DSR mutation.
func (r *Repository) AssignDSR(ctx context.Context, orgID, id, assignedTo string) error {
	tag, err := r.db.Exec(ctx,
		// S121 (live write-sweep): assigned_to is uuid, so NULLIF($1,'') (text) must
		// be cast — the un-cast query 500'd with SQLSTATE 42804 (born-broken; AssignDSR
		// never worked).
		`UPDATE po_dsr SET assigned_to = NULLIF($1,'')::uuid, updated_at = NOW()
		 WHERE org_id = $2 AND id = $3`,
		assignedTo, orgID, id,
	)
	return shareddb.MustAffect(tag, err)
}

// GetDSR returns a single DSR by ID, scoped to the organisation.
//
// S9/CZ-2: raw read (not the generated GetPPDSR), same reasoning as ListDSRs —
// includes channel/reference_id/assigned_to so PATCH-then-GET and the erasure
// flow (which reads requester_email off this same row) see the full record.
func (r *Repository) GetDSR(ctx context.Context, orgID, id string) (*DSR, error) {
	var f dsrFields
	err := r.db.QueryRow(ctx,
		`SELECT id, org_id, requester_name, requester_email, type, description,
		        status, due_date, received_at, completed_at, notes,
		        created_at, updated_at, resolved_by::text, extension_reason,
		        channel, reference_id, assigned_to::text
		 FROM po_dsr
		 WHERE id = $1 AND org_id = $2`, id, orgID,
	).Scan(&f.ID, &f.OrgID, &f.RequesterName, &f.RequesterEmail, &f.Type, &f.Description,
		&f.Status, &f.DueDate, &f.ReceivedAt, &f.CompletedAt, &f.Notes,
		&f.CreatedAt, &f.UpdatedAt, &f.ResolvedBy, &f.ExtensionReason,
		&f.Channel, &f.ReferenceID, &f.AssignedTo)
	if err != nil {
		return nil, fmt.Errorf("get dsr %s: %w", id, err)
	}
	d := dsrFromFields(f)
	return &d, nil
}

// ExecuteErasure performs an Art. 17 DSGVO erasure for the given DSR:
//
//  1. Fetches the DSR to get the requester_email and verify it is an
//     erasure-type request that has not already been completed (idempotency guard).
//  2. Opens a transaction and, via the injected module erasers, deletes PII
//     records identified by that email in each owning module's own tables:
//     - sr_campaign_enrollments / sr_events / sr_targets (Vakt Aware)
//     - hr_employees (Vakt HR)
//     vaktprivacy never writes those prefixes itself (module isolation, ADR-0079).
//  3. Anonymises any matching users row in the same org (no hard-delete —
//     see DeleteUserAccount in internal/shared/account for the rationale:
//     audit-log rows stay intact, the human identity is replaced with
//     "deleted-<uuid>@vakt.local" so foreign-key joins still work). users has no
//     module prefix (platform-shared), so vaktprivacy owns this write.
//  4. Appends an evidence note to the DSR's notes field documenting how many
//     rows were affected in each table.
//  5. Marks the DSR as completed and stamps completed_at — AFTER all deletions
//     commit, so the audit trail never claims completion before the work is done.
//
// The DSR record itself is never deleted; it is compliance evidence under
// Art. 5 Abs. 2 DSGVO (accountability principle).
func (r *Repository) ExecuteErasure(ctx context.Context, orgID, id string) (*DSR, error) {
	// 1. Fetch the DSR — verify type and guard against double-execution.
	dsr, err := r.GetDSR(ctx, orgID, id)
	if err != nil {
		return nil, fmt.Errorf("execute erasure: fetch dsr: %w", err)
	}
	if dsr.Type != "erasure" {
		return nil, fmt.Errorf("execute erasure: dsr %s is type %q, not erasure", id, dsr.Type)
	}
	if dsr.Status == "completed" {
		// Already completed — return the current record without side-effects.
		return dsr, nil
	}

	// Refuse to run unless EVERY module known to hold subject PII has an eraser
	// wired. Checking only for "none wired" would let a partially wired erasure
	// through: it would delete one module's PII, leave another's in place, and
	// still stamp the DSR "completed" — a silent partial Art. 17 erasure, which
	// ADR-0079 classifies as a compliance violation rather than a soft failure.
	wired := make(map[string]bool, len(r.subjectErasers))
	for _, e := range r.subjectErasers {
		wired[e.ModuleName()] = true
	}
	var missing []string
	for _, m := range requiredEraserModules {
		if !wired[m] {
			missing = append(missing, m)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf(
			"execute erasure: no subject eraser wired for module(s) %v — refusing partial Art. 17 erasure",
			missing)
	}
	if r.subjectResolver == nil {
		return nil, fmt.Errorf("execute erasure: no subject resolver wired — refusing partial Art. 17 erasure")
	}

	requesterEmail := dsr.RequesterEmail

	// 2. Open a transaction: all deletions + the DSR status update must be atomic.
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute erasure: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 3a. Resolve cross-module identifiers BEFORE any delete, while the owning
	// rows still exist. This is what makes eraser order irrelevant: afterwards no
	// eraser needs to read another module's tables (see events.SubjectRef).
	employeeIDs, resolveErr := r.subjectResolver.ResolveEmployeeIDs(ctx, tx, orgID, requesterEmail)
	if resolveErr != nil {
		return nil, fmt.Errorf("execute erasure: resolve subject: %w", resolveErr)
	}
	subj := sharedevents.SubjectRef{
		OrgID:       orgID,
		Email:       requesterEmail,
		EmployeeIDs: employeeIDs,
	}

	// 3b. Erase module-owned PII via the injected erasers (each writes only its
	// own prefix on this tx). Order is irrelevant. Counts feed the evidence note;
	// every required module contributes a line even when it deleted nothing, so
	// an auditor can tell "nothing found" from "never looked".
	tableCounts := sharedevents.ErasureCounts{}
	for _, eraser := range r.subjectErasers {
		counts, eraseErr := eraser.EraseSubjectPII(ctx, tx, subj)
		if eraseErr != nil {
			return nil, fmt.Errorf("execute erasure: %w", eraseErr)
		}
		for table, n := range counts {
			tableCounts[table] += n
		}
	}

	// 3d. Anonymise matching users row (same org), if any.
	// We replace email with "deleted-<uuid>@vakt.local", wipe the password hash,
	// clear OIDC mappings, and mark the account inactive — identical to the
	// pattern in internal/shared/account.DeleteUserAccount.
	var anonUserID string
	_ = tx.QueryRow(ctx,
		`SELECT u.id::text FROM users u
		 JOIN org_members om ON om.user_id = u.id
		 WHERE om.org_id = $1::uuid AND lower(u.email) = lower($2)
		 LIMIT 1`,
		orgID, requesterEmail,
	).Scan(&anonUserID)

	usersAnonymised := int64(0)
	if anonUserID != "" {
		anonEmail := fmt.Sprintf("deleted-%s@vakt.local", anonUserID)
		tag, execErr := tx.Exec(ctx, `
			UPDATE users
			SET email         = $1,
			    password_hash = NULL,
			    display_name  = '[gelöscht]',
			    avatar_url    = NULL,
			    oidc_subject  = NULL,
			    oidc_provider = NULL,
			    is_active     = FALSE,
			    updated_at    = NOW()
			WHERE id = $2::uuid
		`, anonEmail, anonUserID)
		if execErr != nil {
			return nil, fmt.Errorf("execute erasure: anonymise user: %w", execErr)
		}
		usersAnonymised = tag.RowsAffected()
		// Revoke sessions + API keys. refresh_sessions has no revoked_at column and is
		// revoked by DELETE everywhere else; a 42703 here would poison the tx and abort
		// the entire erasure (Art. 17 would silently never run).
		// orgid-lint: global — scoped by user_id (global users table); Art. 17 erasure revokes
		// this user's sessions/keys everywhere, same pattern as logout/password-reset
		_, _ = tx.Exec(ctx,
			`DELETE FROM refresh_sessions WHERE user_id = $1::uuid`,
			anonUserID,
		)
		// orgid-lint: global — scoped by created_by (global users table); Art. 17 erasure
		_, _ = tx.Exec(ctx,
			`UPDATE api_keys SET revoked_at = NOW() WHERE created_by = $1::uuid AND revoked_at IS NULL`,
			anonUserID,
		)
	}

	// 4. Build evidence note summarising affected rows.
	evidenceNote := buildErasureNote(tableCounts, usersAnonymised)

	// 5. Mark the DSR as completed — AFTER all deletions, inside the same tx.
	row, err := r.q.WithTx(tx).ExecutePPDSRErasure(ctx, db.ExecutePPDSRErasureParams{
		ID:    id,
		OrgID: orgID,
		Notes: pgtype.Text{String: evidenceNote, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("execute erasure: mark completed: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("execute erasure: commit: %w", err)
	}

	d := dsrFromFields(dsrFields{
		ID: row.ID, OrgID: row.OrgID,
		RequesterName: row.RequesterName, RequesterEmail: row.RequesterEmail,
		Type: row.Type, Description: row.Description, Status: row.Status,
		DueDate: row.DueDate, ReceivedAt: row.ReceivedAt,
		CompletedAt: row.CompletedAt, Notes: row.Notes,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &d, nil
}

// buildErasureNote renders the Art. 17 evidence note from the per-table deletion
// counts reported by the module erasers plus the anonymised-users count. Table
// names are sorted so the note is deterministic regardless of eraser order.
func buildErasureNote(counts sharedevents.ErasureCounts, usersAnonymised int64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Art. 17 DSGVO erasure executed at %s.\n", time.Now().UTC().Format(time.RFC3339))

	tables := make([]string, 0, len(counts))
	for table := range counts {
		tables = append(tables, table)
	}
	sort.Strings(tables)
	for _, table := range tables {
		fmt.Fprintf(&b, "%s deleted: %d\n", table, counts[table])
	}

	fmt.Fprintf(&b, "users anonymised: %d", usersAnonymised)
	return b.String()
}

// --- DSR Portal (sqlc) ---

// CreatePortalDSR inserts a DSR submitted through the public self-service portal.
func (r *Repository) CreatePortalDSR(ctx context.Context, orgID string, in PortalDSRInput, tokenHash, verifyTokenHash, ip string) (string, error) {
	locale := in.Locale
	if locale == "" {
		locale = "de"
	}
	dsrType := in.Type
	switch dsrType {
	case "deletion":
		dsrType = "erasure"
	case "correction":
		dsrType = "rectification"
	}

	due := pgtype.Date{Time: time.Now().UTC().AddDate(0, 0, 30), Valid: true}
	id, err := r.q.CreatePortalPPDSR(ctx, db.CreatePortalPPDSRParams{
		OrgID:           orgID,
		RequesterName:   in.FirstName + " " + in.LastName,
		RequesterEmail:  in.Email,
		Type:            dsrType,
		Description:     optText(in.Description),
		DueDate:         due,
		PortalLocale:    optText(locale),
		TokenHash:       optText(tokenHash),
		VerifyTokenHash: optText(verifyTokenHash),
		SubmittedIp:     optText(ip),
	})
	if err != nil {
		return "", fmt.Errorf("create portal dsr: %w", err)
	}
	return id, nil
}

// GetDSRByTokenHash looks up a DSR by its hashed status token.
func (r *Repository) GetDSRByTokenHash(ctx context.Context, tokenHash string) (*DSR, error) {
	row, err := r.q.GetPPDSRByTokenHash(ctx, pgtype.Text{String: tokenHash, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("get dsr by token: %w", err)
	}
	d := dsrFromFields(dsrFields{
		ID: row.ID, OrgID: row.OrgID,
		RequesterName: row.RequesterName, RequesterEmail: row.RequesterEmail,
		Type: row.Type, Description: row.Description, Status: row.Status,
		DueDate: row.DueDate, ReceivedAt: row.ReceivedAt,
		CompletedAt: row.CompletedAt, Notes: row.Notes,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &d, nil
}

// GetOrgByDSRSlug looks up an organisation by its DSR portal slug.
func (r *Repository) GetOrgByDSRSlug(ctx context.Context, slug string) (orgID, orgName, dpoEmail, intro string, enabled bool, err error) {
	row, lookupErr := r.q.GetOrgByDSRSlug(ctx, pgtype.Text{String: slug, Valid: true})
	if lookupErr != nil {
		return "", "", "", "", false, fmt.Errorf("get org by dsr slug: %w", lookupErr)
	}
	return row.ID, row.Name, textOrEmpty(row.DsrDpoEmail), textOrEmpty(row.DsrPortalIntro), row.DsrPortalEnabled, nil
}

// UpdateDSRPortalSettings persists DSR portal configuration for an organisation.
func (r *Repository) UpdateDSRPortalSettings(ctx context.Context, orgID string, in UpdateDSRPortalSettingsInput) error {
	return r.q.UpdateDSRPortalSettings(ctx, db.UpdateDSRPortalSettingsParams{
		ID:               orgID,
		DsrPortalEnabled: in.Enabled,
		DsrPortalSlug:    optText(in.Slug),
		DsrDpoEmail:      optText(in.DPOEmail),
		DsrPortalIntro:   optText(in.Intro),
	})
}

// GetDSRPortalSettings fetches the current DSR portal configuration for an organisation.
func (r *Repository) GetDSRPortalSettings(ctx context.Context, orgID string) (*UpdateDSRPortalSettingsInput, error) {
	row, err := r.q.GetDSRPortalSettings(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("get dsr portal settings: %w", err)
	}
	return &UpdateDSRPortalSettingsInput{
		Enabled:  row.DsrPortalEnabled,
		Slug:     textOrEmpty(row.DsrPortalSlug),
		DPOEmail: textOrEmpty(row.DsrDpoEmail),
		Intro:    textOrEmpty(row.DsrPortalIntro),
	}, nil
}

// --- Paginated list helpers (sqlc) ---

// ListVVTPaged returns a page of VVT entries plus the total count.
func (r *Repository) ListVVTPaged(ctx context.Context, orgID string, offset, limit int) ([]VVTEntry, int, error) {
	total, err := r.q.CountPPVVT(ctx, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("count vvt: %w", err)
	}
	rows, err := r.q.ListPPVVTPaged(ctx, db.ListPPVVTPagedParams{
		OrgID: orgID, Limit: int32(limit), Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list vvt paged: %w", err)
	}
	out := make([]VVTEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, vvtFromRow(row))
	}
	return out, int(total), nil
}

// ListBreachesPaged returns a page of breach records plus the total count.
func (r *Repository) ListBreachesPaged(ctx context.Context, orgID string, offset, limit int) ([]Breach, int, error) {
	total, err := r.q.CountPPBreaches(ctx, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("count breaches: %w", err)
	}
	rows, err := r.q.ListPPBreachesPaged(ctx, db.ListPPBreachesPagedParams{
		OrgID: orgID, Limit: int32(limit), Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list breaches paged: %w", err)
	}
	out := make([]Breach, 0, len(rows))
	for _, row := range rows {
		out = append(out, breachFromRow(row))
	}
	return out, int(total), nil
}

// ListDSRsCursor returns DSRs for orgID using keyset pagination on (created_at DESC, id DESC).
func (r *Repository) ListDSRsCursor(ctx context.Context, orgID string, cursorID string, cursorTS time.Time, limit int) ([]DSR, error) {
	args := []any{orgID}
	q := `SELECT id, org_id, requester_name, requester_email, type, description,
	             status, due_date, received_at, completed_at, notes,
	             created_at, updated_at, resolved_by::text, extension_reason,
	             channel, reference_id, assigned_to::text
	      FROM po_dsr
	      WHERE org_id = $1`
	if !cursorTS.IsZero() {
		q += ` AND (created_at < $2 OR (created_at = $2 AND id::text < $3))`
		args = append(args, cursorTS, cursorID)
	}
	q += ` ORDER BY created_at DESC, id DESC LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit+1)
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list dsrs cursor: %w", err)
	}
	defer rows.Close()
	var out []DSR
	for rows.Next() {
		var f dsrFields
		if err := rows.Scan(&f.ID, &f.OrgID, &f.RequesterName, &f.RequesterEmail, &f.Type, &f.Description,
			&f.Status, &f.DueDate, &f.ReceivedAt, &f.CompletedAt, &f.Notes,
			&f.CreatedAt, &f.UpdatedAt, &f.ResolvedBy, &f.ExtensionReason,
			&f.Channel, &f.ReferenceID, &f.AssignedTo); err != nil {
			return nil, fmt.Errorf("scan dsr cursor row: %w", err)
		}
		out = append(out, dsrFromFields(f))
	}
	return out, rows.Err()
}
