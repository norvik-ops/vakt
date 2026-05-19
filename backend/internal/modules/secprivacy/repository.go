// Package secprivacy provides DSGVO documentation: VVT, DPIA, AVV, breach notifications, and DSR tracking.
package secprivacy

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles PrivacyOps data access.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new PrivacyOps repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// --- VVT ---

// ListVVT returns all VVT entries for the organisation, ordered newest first.
func (r *Repository) ListVVT(ctx context.Context, orgID string) ([]VVTEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, name, purpose, legal_basis,
		       data_categories, data_subjects, recipients,
		       COALESCE(retention_period,''), third_country_transfer,
		       COALESCE(safeguards,''), COALESCE(responsible_person,''),
		       status, created_at, updated_at
		FROM po_vvt_entries
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list vvt: %w", err)
	}
	defer rows.Close()

	var entries []VVTEntry
	for rows.Next() {
		var e VVTEntry
		if err := rows.Scan(
			&e.ID, &e.OrgID, &e.Name, &e.Purpose, &e.LegalBasis,
			&e.DataCategories, &e.DataSubjects, &e.Recipients,
			&e.RetentionPeriod, &e.ThirdCountryTransfer,
			&e.Safeguards, &e.ResponsiblePerson,
			&e.Status, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan vvt: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// CreateVVT inserts a new VVT entry and returns the persisted record including
// database-assigned id, status ("active"), and timestamps.
func (r *Repository) CreateVVT(ctx context.Context, orgID string, in CreateVVTInput) (*VVTEntry, error) {
	var e VVTEntry
	err := r.db.QueryRow(ctx, `
		INSERT INTO po_vvt_entries
		  (org_id, name, purpose, legal_basis, data_categories, data_subjects,
		   recipients, retention_period, third_country_transfer, safeguards, responsible_person)
		VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id::text, org_id::text, name, purpose, legal_basis,
		          data_categories, data_subjects, recipients,
		          COALESCE(retention_period,''), third_country_transfer,
		          COALESCE(safeguards,''), COALESCE(responsible_person,''),
		          status, created_at, updated_at`,
		orgID, in.Name, in.Purpose, in.LegalBasis,
		in.DataCategories, in.DataSubjects, in.Recipients,
		in.RetentionPeriod, in.ThirdCountryTransfer,
		in.Safeguards, in.ResponsiblePerson,
	).Scan(
		&e.ID, &e.OrgID, &e.Name, &e.Purpose, &e.LegalBasis,
		&e.DataCategories, &e.DataSubjects, &e.Recipients,
		&e.RetentionPeriod, &e.ThirdCountryTransfer,
		&e.Safeguards, &e.ResponsiblePerson,
		&e.Status, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create vvt: %w", err)
	}
	return &e, nil
}

// --- DPIA ---

// ListDPIAs returns all DPIA records for the organisation, ordered newest first.
func (r *Repository) ListDPIAs(ctx context.Context, orgID string) ([]DPIA, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text,
		       vvt_entry_id::text, title,
		       COALESCE(description,''), COALESCE(necessity_assessment,''),
		       COALESCE(risk_assessment,''), COALESCE(mitigation_measures,''),
		       COALESCE(residual_risk,''), dpo_consultation, status,
		       reviewed_by::text, reviewed_at, created_at, updated_at
		FROM po_dpias
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC LIMIT 500`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list dpias: %w", err)
	}
	defer rows.Close()

	var dpias []DPIA
	for rows.Next() {
		var d DPIA
		if err := rows.Scan(
			&d.ID, &d.OrgID, &d.VVTEntryID, &d.Title,
			&d.Description, &d.NecessityAssessment,
			&d.RiskAssessment, &d.MitigationMeasures,
			&d.ResidualRisk, &d.DPOConsultation, &d.Status,
			&d.ReviewedBy, &d.ReviewedAt, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan dpia: %w", err)
		}
		dpias = append(dpias, d)
	}
	return dpias, rows.Err()
}

// CreateDPIA inserts a new DPIA in "draft" status and returns the full record.
func (r *Repository) CreateDPIA(ctx context.Context, orgID string, in CreateDPIAInput) (*DPIA, error) {
	var d DPIA
	err := r.db.QueryRow(ctx, `
		INSERT INTO po_dpias
		  (org_id, vvt_entry_id, title, description, necessity_assessment,
		   risk_assessment, mitigation_measures, residual_risk, dpo_consultation)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id::text, org_id::text,
		          vvt_entry_id::text, title,
		          COALESCE(description,''), COALESCE(necessity_assessment,''),
		          COALESCE(risk_assessment,''), COALESCE(mitigation_measures,''),
		          COALESCE(residual_risk,''), dpo_consultation, status,
		          reviewed_by::text, reviewed_at, created_at, updated_at`,
		orgID, in.VVTEntryID, in.Title, in.Description,
		in.NecessityAssessment, in.RiskAssessment,
		in.MitigationMeasures, in.ResidualRisk, in.DPOConsultation,
	).Scan(
		&d.ID, &d.OrgID, &d.VVTEntryID, &d.Title,
		&d.Description, &d.NecessityAssessment,
		&d.RiskAssessment, &d.MitigationMeasures,
		&d.ResidualRisk, &d.DPOConsultation, &d.Status,
		&d.ReviewedBy, &d.ReviewedAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create dpia: %w", err)
	}
	return &d, nil
}

// --- AVV ---

// ListAVVs returns all AVV records for the organisation, ordered newest first.
func (r *Repository) ListAVVs(ctx context.Context, orgID string) ([]AVV, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, processor_name, service_description,
		       contract_date, review_date, status, COALESCE(notes,''),
		       created_at, updated_at
		FROM po_avvs
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC LIMIT 500`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list avvs: %w", err)
	}
	defer rows.Close()

	var avvs []AVV
	for rows.Next() {
		var a AVV
		if err := rows.Scan(
			&a.ID, &a.OrgID, &a.ProcessorName, &a.ServiceDescription,
			&a.ContractDate, &a.ReviewDate, &a.Status, &a.Notes,
			&a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan avv: %w", err)
		}
		avvs = append(avvs, a)
	}
	return avvs, rows.Err()
}

// CreateAVV inserts a new AVV record in "active" status and returns the persisted entry.
func (r *Repository) CreateAVV(ctx context.Context, orgID string, in CreateAVVInput) (*AVV, error) {
	var a AVV
	err := r.db.QueryRow(ctx, `
		INSERT INTO po_avvs (org_id, processor_name, service_description, contract_date, review_date, notes)
		VALUES ($1::uuid, $2, $3, $4, $5, $6)
		RETURNING id::text, org_id::text, processor_name, service_description,
		          contract_date, review_date, status, COALESCE(notes,''),
		          created_at, updated_at`,
		orgID, in.ProcessorName, in.ServiceDescription,
		in.ContractDate, in.ReviewDate, in.Notes,
	).Scan(
		&a.ID, &a.OrgID, &a.ProcessorName, &a.ServiceDescription,
		&a.ContractDate, &a.ReviewDate, &a.Status, &a.Notes,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create avv: %w", err)
	}
	return &a, nil
}

// --- Breach ---

// ListBreaches returns all breach records for the organisation, ordered by discovery date descending.
func (r *Repository) ListBreaches(ctx context.Context, orgID string) ([]Breach, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, title, COALESCE(description,''), discovered_at,
		       authority_deadline_at, authority_notified_at,
		       subjects_notification_required, subjects_notified_at,
		       affected_count, data_categories, status, created_at, updated_at
		FROM po_breaches
		WHERE org_id = $1::uuid
		ORDER BY discovered_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list breaches: %w", err)
	}
	defer rows.Close()

	var breaches []Breach
	for rows.Next() {
		var b Breach
		if err := rows.Scan(
			&b.ID, &b.OrgID, &b.Title, &b.Description, &b.DiscoveredAt,
			&b.AuthorityDeadlineAt, &b.AuthorityNotifiedAt,
			&b.SubjectsNotificationRequired, &b.SubjectsNotifiedAt,
			&b.AffectedCount, &b.DataCategories, &b.Status,
			&b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan breach: %w", err)
		}
		breaches = append(breaches, b)
	}
	return breaches, rows.Err()
}

// CreateBreach inserts a breach record and derives authority_deadline_at as
// DiscoveredAt + 72 hours, reflecting the mandatory notification window under
// Art. 33 Abs. 1 DSGVO and NIS2 Art. 23.
func (r *Repository) CreateBreach(ctx context.Context, orgID string, in CreateBreachInput) (*Breach, error) {
	// Authority deadline is always 72 hours after discovery (NIS2 Art.23 + DSGVO Art.33).
	deadline := in.DiscoveredAt.Add(72 * time.Hour)

	var b Breach
	err := r.db.QueryRow(ctx, `
		INSERT INTO po_breaches
		  (org_id, title, description, discovered_at, authority_deadline_at,
		   subjects_notification_required, affected_count, data_categories)
		VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id::text, org_id::text, title, COALESCE(description,''), discovered_at,
		          authority_deadline_at, authority_notified_at,
		          subjects_notification_required, subjects_notified_at,
		          affected_count, data_categories, status, created_at, updated_at`,
		orgID, in.Title, in.Description, in.DiscoveredAt, deadline,
		in.SubjectsNotificationRequired, in.AffectedCount, in.DataCategories,
	).Scan(
		&b.ID, &b.OrgID, &b.Title, &b.Description, &b.DiscoveredAt,
		&b.AuthorityDeadlineAt, &b.AuthorityNotifiedAt,
		&b.SubjectsNotificationRequired, &b.SubjectsNotifiedAt,
		&b.AffectedCount, &b.DataCategories, &b.Status,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create breach: %w", err)
	}
	return &b, nil
}

// UpdateBreachStatus changes the status field of a breach record.
// Intended for bulk or worker-driven transitions; prefer UpdateBreach for user-initiated edits.
func (r *Repository) UpdateBreachStatus(ctx context.Context, id, orgID, status string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE po_breaches SET status=$1, updated_at=now()
		WHERE id=$2::uuid AND org_id=$3::uuid`, status, id, orgID)
	return err
}

// MarkAuthorityNotified stamps authority_notified_at to the current time,
// recording that the supervisory authority was informed as required by Art. 33 DSGVO.
func (r *Repository) MarkAuthorityNotified(ctx context.Context, id, orgID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE po_breaches SET authority_notified_at=now(), updated_at=now()
		WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID)
	return err
}

// --- VVT full CRUD ---

// GetVVT fetches a single VVT entry by ID, scoped to orgID.
func (r *Repository) GetVVT(ctx context.Context, orgID, id string) (*VVTEntry, error) {
	var e VVTEntry
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, name, purpose, legal_basis,
		       data_categories, data_subjects, recipients,
		       COALESCE(retention_period,''), third_country_transfer,
		       COALESCE(safeguards,''), COALESCE(responsible_person,''),
		       status, created_at, updated_at
		FROM po_vvt_entries
		WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID,
	).Scan(
		&e.ID, &e.OrgID, &e.Name, &e.Purpose, &e.LegalBasis,
		&e.DataCategories, &e.DataSubjects, &e.Recipients,
		&e.RetentionPeriod, &e.ThirdCountryTransfer,
		&e.Safeguards, &e.ResponsiblePerson,
		&e.Status, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get vvt %s: %w", id, err)
	}
	return &e, nil
}

// UpdateVVT replaces all mutable fields of a VVT entry and returns the updated record.
func (r *Repository) UpdateVVT(ctx context.Context, orgID, id string, in UpdateVVTInput) (*VVTEntry, error) {
	var e VVTEntry
	err := r.db.QueryRow(ctx, `
		UPDATE po_vvt_entries SET
		  name=$3, purpose=$4, legal_basis=$5, data_categories=$6,
		  data_subjects=$7, recipients=$8, retention_period=$9,
		  third_country_transfer=$10, safeguards=$11, responsible_person=$12,
		  status=$13, updated_at=now()
		WHERE id=$1::uuid AND org_id=$2::uuid
		RETURNING id::text, org_id::text, name, purpose, legal_basis,
		          data_categories, data_subjects, recipients,
		          COALESCE(retention_period,''), third_country_transfer,
		          COALESCE(safeguards,''), COALESCE(responsible_person,''),
		          status, created_at, updated_at`,
		id, orgID,
		in.Name, in.Purpose, in.LegalBasis,
		in.DataCategories, in.DataSubjects, in.Recipients,
		in.RetentionPeriod, in.ThirdCountryTransfer,
		in.Safeguards, in.ResponsiblePerson, in.Status,
	).Scan(
		&e.ID, &e.OrgID, &e.Name, &e.Purpose, &e.LegalBasis,
		&e.DataCategories, &e.DataSubjects, &e.Recipients,
		&e.RetentionPeriod, &e.ThirdCountryTransfer,
		&e.Safeguards, &e.ResponsiblePerson,
		&e.Status, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update vvt %s: %w", id, err)
	}
	return &e, nil
}

// DeleteVVT permanently removes a VVT entry. Scoped to orgID to prevent cross-tenant deletion.
func (r *Repository) DeleteVVT(ctx context.Context, orgID, id string) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM po_vvt_entries WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID)
	return err
}

// --- DPIA full CRUD ---

// GetDPIA fetches a single DPIA record by ID, scoped to orgID.
func (r *Repository) GetDPIA(ctx context.Context, orgID, id string) (*DPIA, error) {
	var d DPIA
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text,
		       vvt_entry_id::text, title,
		       COALESCE(description,''), COALESCE(necessity_assessment,''),
		       COALESCE(risk_assessment,''), COALESCE(mitigation_measures,''),
		       COALESCE(residual_risk,''), dpo_consultation, status,
		       reviewed_by::text, reviewed_at, created_at, updated_at
		FROM po_dpias
		WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID,
	).Scan(
		&d.ID, &d.OrgID, &d.VVTEntryID, &d.Title,
		&d.Description, &d.NecessityAssessment,
		&d.RiskAssessment, &d.MitigationMeasures,
		&d.ResidualRisk, &d.DPOConsultation, &d.Status,
		&d.ReviewedBy, &d.ReviewedAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get dpia %s: %w", id, err)
	}
	return &d, nil
}

// UpdateDPIA replaces the content fields of a DPIA without changing its approval state.
func (r *Repository) UpdateDPIA(ctx context.Context, orgID, id string, in UpdateDPIAInput) (*DPIA, error) {
	var d DPIA
	err := r.db.QueryRow(ctx, `
		UPDATE po_dpias SET
		  title=$3, description=$4, necessity_assessment=$5,
		  risk_assessment=$6, mitigation_measures=$7, residual_risk=$8,
		  dpo_consultation=$9, updated_at=now()
		WHERE id=$1::uuid AND org_id=$2::uuid
		RETURNING id::text, org_id::text,
		          vvt_entry_id::text, title,
		          COALESCE(description,''), COALESCE(necessity_assessment,''),
		          COALESCE(risk_assessment,''), COALESCE(mitigation_measures,''),
		          COALESCE(residual_risk,''), dpo_consultation, status,
		          reviewed_by::text, reviewed_at, created_at, updated_at`,
		id, orgID,
		in.Title, in.Description, in.NecessityAssessment,
		in.RiskAssessment, in.MitigationMeasures, in.ResidualRisk,
		in.DPOConsultation,
	).Scan(
		&d.ID, &d.OrgID, &d.VVTEntryID, &d.Title,
		&d.Description, &d.NecessityAssessment,
		&d.RiskAssessment, &d.MitigationMeasures,
		&d.ResidualRisk, &d.DPOConsultation, &d.Status,
		&d.ReviewedBy, &d.ReviewedAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update dpia %s: %w", id, err)
	}
	return &d, nil
}

// ApproveDPIA sets a DPIA's status to "approved" and records the reviewer's ID and timestamp.
// Art. 35 DSGVO requires documented approval before high-risk processing may begin.
func (r *Repository) ApproveDPIA(ctx context.Context, orgID, id, reviewerID string) (*DPIA, error) {
	var d DPIA
	err := r.db.QueryRow(ctx, `
		UPDATE po_dpias SET
		  status='approved', reviewed_by=$3::uuid, reviewed_at=now(), updated_at=now()
		WHERE id=$1::uuid AND org_id=$2::uuid
		RETURNING id::text, org_id::text,
		          vvt_entry_id::text, title,
		          COALESCE(description,''), COALESCE(necessity_assessment,''),
		          COALESCE(risk_assessment,''), COALESCE(mitigation_measures,''),
		          COALESCE(residual_risk,''), dpo_consultation, status,
		          reviewed_by::text, reviewed_at, created_at, updated_at`,
		id, orgID, reviewerID,
	).Scan(
		&d.ID, &d.OrgID, &d.VVTEntryID, &d.Title,
		&d.Description, &d.NecessityAssessment,
		&d.RiskAssessment, &d.MitigationMeasures,
		&d.ResidualRisk, &d.DPOConsultation, &d.Status,
		&d.ReviewedBy, &d.ReviewedAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("approve dpia %s: %w", id, err)
	}
	return &d, nil
}

// DeleteDPIA permanently removes a DPIA record. Scoped to orgID to prevent cross-tenant deletion.
func (r *Repository) DeleteDPIA(ctx context.Context, orgID, id string) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM po_dpias WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID)
	return err
}

// --- AVV full CRUD ---

// GetAVV fetches a single AVV record by ID, scoped to orgID.
func (r *Repository) GetAVV(ctx context.Context, orgID, id string) (*AVV, error) {
	var a AVV
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, processor_name, service_description,
		       contract_date, review_date, status, COALESCE(notes,''),
		       created_at, updated_at
		FROM po_avvs
		WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID,
	).Scan(
		&a.ID, &a.OrgID, &a.ProcessorName, &a.ServiceDescription,
		&a.ContractDate, &a.ReviewDate, &a.Status, &a.Notes,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get avv %s: %w", id, err)
	}
	return &a, nil
}

// UpdateAVV replaces all mutable fields of an AVV record and returns the updated entry.
func (r *Repository) UpdateAVV(ctx context.Context, orgID, id string, in UpdateAVVInput) (*AVV, error) {
	var a AVV
	err := r.db.QueryRow(ctx, `
		UPDATE po_avvs SET
		  processor_name=$3, service_description=$4,
		  contract_date=$5, review_date=$6, status=$7, notes=$8, updated_at=now()
		WHERE id=$1::uuid AND org_id=$2::uuid
		RETURNING id::text, org_id::text, processor_name, service_description,
		          contract_date, review_date, status, COALESCE(notes,''),
		          created_at, updated_at`,
		id, orgID,
		in.ProcessorName, in.ServiceDescription,
		in.ContractDate, in.ReviewDate, in.Status, in.Notes,
	).Scan(
		&a.ID, &a.OrgID, &a.ProcessorName, &a.ServiceDescription,
		&a.ContractDate, &a.ReviewDate, &a.Status, &a.Notes,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update avv %s: %w", id, err)
	}
	return &a, nil
}

// DeleteAVV permanently removes an AVV record. Scoped to orgID to prevent cross-tenant deletion.
func (r *Repository) DeleteAVV(ctx context.Context, orgID, id string) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM po_avvs WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID)
	return err
}

// GetAVVWithBody fetches a single AVV including template body and SCC fields.
func (r *Repository) GetAVVWithBody(ctx context.Context, orgID, id string) (*AVV, error) {
	var a AVV
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, processor_name, service_description,
		       contract_date, review_date, status, COALESCE(notes,''),
		       COALESCE(template_id,''), COALESCE(body,''),
		       COALESCE(scc_module,''), COALESCE(scc_annex_i,''),
		       COALESCE(scc_annex_ii,''), COALESCE(scc_annex_iii,''),
		       created_at, updated_at
		FROM po_avvs
		WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID,
	).Scan(
		&a.ID, &a.OrgID, &a.ProcessorName, &a.ServiceDescription,
		&a.ContractDate, &a.ReviewDate, &a.Status, &a.Notes,
		&a.TemplateID, &a.Body,
		&a.SCCModule, &a.SCCAnnexI, &a.SCCAnnexII, &a.SCCAnnexIII,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get avv with body %s: %w", id, err)
	}
	return &a, nil
}

// UpdateAVVBody sets the template_id and body fields of an AVV.
func (r *Repository) UpdateAVVBody(ctx context.Context, orgID, id, templateID, body string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE po_avvs SET template_id=$3, body=$4, updated_at=now()
		WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID, templateID, body)
	if err != nil {
		return fmt.Errorf("update avv body %s: %w", id, err)
	}
	return nil
}

// UpdateAVVSCC updates the SCC module and annex fields of an AVV.
func (r *Repository) UpdateAVVSCC(ctx context.Context, orgID, id, sccModule, annexI, annexII, annexIII string) error {
	var moduleArg interface{} = sccModule
	if sccModule == "" {
		moduleArg = nil
	}
	_, err := r.db.Exec(ctx, `
		UPDATE po_avvs SET scc_module=$3, scc_annex_i=$4, scc_annex_ii=$5, scc_annex_iii=$6, updated_at=now()
		WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID, moduleArg, annexI, annexII, annexIII)
	if err != nil {
		return fmt.Errorf("update avv scc %s: %w", id, err)
	}
	return nil
}

// CreateAVVWithBody inserts a new AVV with a pre-rendered template body.
func (r *Repository) CreateAVVWithBody(ctx context.Context, orgID, templateID, body, processorName, serviceDesc string) (*AVV, error) {
	var a AVV
	err := r.db.QueryRow(ctx, `
		INSERT INTO po_avvs (org_id, processor_name, service_description, template_id, body)
		VALUES ($1::uuid, $2, $3, $4, $5)
		RETURNING id::text, org_id::text, processor_name, service_description,
		          contract_date, review_date, status, COALESCE(notes,''),
		          COALESCE(template_id,''), COALESCE(body,''),
		          COALESCE(scc_module,''), COALESCE(scc_annex_i,''),
		          COALESCE(scc_annex_ii,''), COALESCE(scc_annex_iii,''),
		          created_at, updated_at`,
		orgID, processorName, serviceDesc, templateID, body,
	).Scan(
		&a.ID, &a.OrgID, &a.ProcessorName, &a.ServiceDescription,
		&a.ContractDate, &a.ReviewDate, &a.Status, &a.Notes,
		&a.TemplateID, &a.Body,
		&a.SCCModule, &a.SCCAnnexI, &a.SCCAnnexII, &a.SCCAnnexIII,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create avv with body: %w", err)
	}
	return &a, nil
}

// ListExpiringAVVs returns AVVs whose review_date is between now and the given threshold.
func (r *Repository) ListExpiringAVVs(ctx context.Context, threshold time.Time) ([]AVV, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, processor_name, service_description,
		       contract_date, review_date, status, COALESCE(notes,''),
		       created_at, updated_at
		FROM po_avvs
		WHERE status='active' AND review_date IS NOT NULL
		  AND review_date <= $1::date AND review_date >= CURRENT_DATE
		ORDER BY review_date ASC`, threshold)
	if err != nil {
		return nil, fmt.Errorf("list expiring avvs: %w", err)
	}
	defer rows.Close()

	var avvs []AVV
	for rows.Next() {
		var a AVV
		if err := rows.Scan(
			&a.ID, &a.OrgID, &a.ProcessorName, &a.ServiceDescription,
			&a.ContractDate, &a.ReviewDate, &a.Status, &a.Notes,
			&a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan expiring avv: %w", err)
		}
		avvs = append(avvs, a)
	}
	return avvs, rows.Err()
}

// MarkExpiredAVVs updates status to 'expired' for AVVs whose review_date has passed.
func (r *Repository) MarkExpiredAVVs(ctx context.Context) (int64, error) {
	res, err := r.db.Exec(ctx, `
		UPDATE po_avvs SET status='expired', updated_at=now()
		WHERE status='active' AND review_date IS NOT NULL AND review_date < CURRENT_DATE`)
	if err != nil {
		return 0, fmt.Errorf("mark expired avvs: %w", err)
	}
	return res.RowsAffected(), nil
}

// --- Breach full CRUD ---

// GetBreach fetches a single breach record by ID, scoped to orgID.
func (r *Repository) GetBreach(ctx context.Context, orgID, id string) (*Breach, error) {
	var b Breach
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, title, COALESCE(description,''), discovered_at,
		       authority_deadline_at, authority_notified_at,
		       subjects_notification_required, subjects_notified_at,
		       affected_count, data_categories, status, created_at, updated_at
		FROM po_breaches
		WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID,
	).Scan(
		&b.ID, &b.OrgID, &b.Title, &b.Description, &b.DiscoveredAt,
		&b.AuthorityDeadlineAt, &b.AuthorityNotifiedAt,
		&b.SubjectsNotificationRequired, &b.SubjectsNotifiedAt,
		&b.AffectedCount, &b.DataCategories, &b.Status,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get breach %s: %w", id, err)
	}
	return &b, nil
}

// UpdateBreach replaces the editable fields of a breach record.
// Timestamps (discovered_at, authority_deadline_at, authority_notified_at) are immutable through this method.
func (r *Repository) UpdateBreach(ctx context.Context, orgID, id string, in UpdateBreachInput) (*Breach, error) {
	var b Breach
	err := r.db.QueryRow(ctx, `
		UPDATE po_breaches SET
		  title=$3, description=$4,
		  subjects_notification_required=$5, affected_count=$6,
		  data_categories=$7, updated_at=now()
		WHERE id=$1::uuid AND org_id=$2::uuid
		RETURNING id::text, org_id::text, title, COALESCE(description,''), discovered_at,
		          authority_deadline_at, authority_notified_at,
		          subjects_notification_required, subjects_notified_at,
		          affected_count, data_categories, status, created_at, updated_at`,
		id, orgID,
		in.Title, in.Description,
		in.SubjectsNotificationRequired, in.AffectedCount,
		in.DataCategories,
	).Scan(
		&b.ID, &b.OrgID, &b.Title, &b.Description, &b.DiscoveredAt,
		&b.AuthorityDeadlineAt, &b.AuthorityNotifiedAt,
		&b.SubjectsNotificationRequired, &b.SubjectsNotifiedAt,
		&b.AffectedCount, &b.DataCategories, &b.Status,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update breach %s: %w", id, err)
	}
	return &b, nil
}

// DeleteBreach permanently removes a breach record. Scoped to orgID to prevent cross-tenant deletion.
func (r *Repository) DeleteBreach(ctx context.Context, orgID, id string) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM po_breaches WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID)
	return err
}

// --- DSR ---

// ListDSRs returns all DSRs for the given organisation, newest first.
// Results include the due_date formatted as YYYY-MM-DD for JSON serialisation.
func (r *Repository) ListDSRs(ctx context.Context, orgID string) ([]DSR, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, requester_name, requester_email, type,
		       COALESCE(description,''), status,
		       to_char(due_date, 'YYYY-MM-DD'),
		       received_at, completed_at,
		       COALESCE(notes,''), created_at, updated_at
		FROM po_dsr
		WHERE org_id = $1::uuid
		ORDER BY received_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list dsrs: %w", err)
	}
	defer rows.Close()

	var dsrs []DSR
	for rows.Next() {
		var d DSR
		if err := rows.Scan(
			&d.ID, &d.OrgID, &d.RequesterName, &d.RequesterEmail, &d.Type,
			&d.Description, &d.Status, &d.DueDate,
			&d.ReceivedAt, &d.CompletedAt,
			&d.Notes, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan dsr: %w", err)
		}
		dsrs = append(dsrs, d)
	}
	return dsrs, rows.Err()
}

// CreateDSR inserts a new data subject request and automatically sets due_date
// to now + 30 calendar days in the database, satisfying the Art. 12 Abs. 3 DSGVO
// response deadline without requiring the caller to supply it.
func (r *Repository) CreateDSR(ctx context.Context, orgID string, in CreateDSRInput) (*DSR, error) {
	var d DSR
	err := r.db.QueryRow(ctx, `
		INSERT INTO po_dsr
		  (org_id, requester_name, requester_email, type, description, notes,
		   due_date)
		VALUES ($1::uuid, $2, $3, $4, $5, $6,
		        (now() + interval '30 days')::date)
		RETURNING id::text, org_id::text, requester_name, requester_email, type,
		          COALESCE(description,''), status,
		          to_char(due_date, 'YYYY-MM-DD'),
		          received_at, completed_at,
		          COALESCE(notes,''), created_at, updated_at`,
		orgID, in.RequesterName, in.RequesterEmail, in.Type,
		in.Description, in.Notes,
	).Scan(
		&d.ID, &d.OrgID, &d.RequesterName, &d.RequesterEmail, &d.Type,
		&d.Description, &d.Status, &d.DueDate,
		&d.ReceivedAt, &d.CompletedAt,
		&d.Notes, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create dsr: %w", err)
	}
	return &d, nil
}

// UpdateDSR updates the status and notes of an existing DSR.
// When status is "completed" or "rejected" the method stamps completed_at with
// the current UTC time, recording how long the response took relative to due_date.
func (r *Repository) UpdateDSR(ctx context.Context, orgID, id string, in UpdateDSRInput) (*DSR, error) {
	var completedAt *time.Time
	if in.Status == "completed" || in.Status == "rejected" {
		now := time.Now().UTC()
		completedAt = &now
	}
	var d DSR
	err := r.db.QueryRow(ctx, `
		UPDATE po_dsr SET
		  status=$3, notes=$4, completed_at=$5, updated_at=now()
		WHERE id=$1::uuid AND org_id=$2::uuid
		RETURNING id::text, org_id::text, requester_name, requester_email, type,
		          COALESCE(description,''), status,
		          to_char(due_date, 'YYYY-MM-DD'),
		          received_at, completed_at,
		          COALESCE(notes,''), created_at, updated_at`,
		id, orgID, in.Status, in.Notes, completedAt,
	).Scan(
		&d.ID, &d.OrgID, &d.RequesterName, &d.RequesterEmail, &d.Type,
		&d.Description, &d.Status, &d.DueDate,
		&d.ReceivedAt, &d.CompletedAt,
		&d.Notes, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update dsr %s: %w", id, err)
	}
	return &d, nil
}

// DeleteDSR permanently removes a DSR record. Callers should only invoke this
// for erroneous duplicates; completed requests should instead be archived to
// preserve the audit trail required under Art. 5 Abs. 2 DSGVO (accountability).
func (r *Repository) DeleteDSR(ctx context.Context, orgID, id string) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM po_dsr WHERE id=$1::uuid AND org_id=$2::uuid`, id, orgID)
	return err
}

// ExecuteErasure marks an erasure-type DSR as completed, stamps completed_at,
// and appends an evidence note documenting the deletion actions taken.
// Only affects DSRs of type "erasure" that are not yet completed, providing a
// guard against double-execution.
func (r *Repository) ExecuteErasure(ctx context.Context, orgID, id, evidenceNote string) (*DSR, error) {
	var d DSR
	err := r.db.QueryRow(ctx, `
		UPDATE po_dsr SET
		  status='completed', completed_at=now(),
		  notes=CASE WHEN notes IS NULL OR notes='' THEN $3 ELSE notes || E'\n\n' || $3 END,
		  updated_at=now()
		WHERE id=$1::uuid AND org_id=$2::uuid
		  AND type='erasure' AND status != 'completed'
		RETURNING id::text, org_id::text, requester_name, requester_email, type,
		          COALESCE(description,''), status,
		          to_char(due_date, 'YYYY-MM-DD'),
		          received_at, completed_at,
		          COALESCE(notes,''), created_at, updated_at`,
		id, orgID, evidenceNote,
	).Scan(
		&d.ID, &d.OrgID, &d.RequesterName, &d.RequesterEmail, &d.Type,
		&d.Description, &d.Status, &d.DueDate,
		&d.ReceivedAt, &d.CompletedAt,
		&d.Notes, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("execute erasure dsr %s: %w", id, err)
	}
	return &d, nil
}

// --- DSR Portal ---

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

	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO po_dsr
		  (org_id, requester_name, requester_email, type, description,
		   due_date, source, portal_locale, submitted_ip,
		   token_hash, verify_token_hash)
		VALUES ($1::uuid, $2, $3, $4, $5,
		        (now() + interval '30 days')::date,
		        'portal', $6, $7, $8, $9)
		RETURNING id::text`,
		orgID, in.FirstName+" "+in.LastName, in.Email,
		dsrType, in.Description,
		locale, ip, tokenHash, verifyTokenHash,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create portal dsr: %w", err)
	}
	return id, nil
}

// GetDSRByTokenHash looks up a DSR by its hashed status token.
func (r *Repository) GetDSRByTokenHash(ctx context.Context, tokenHash string) (*DSR, error) {
	var d DSR
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, requester_name, requester_email, type,
		       COALESCE(description,''), status,
		       to_char(due_date, 'YYYY-MM-DD'),
		       received_at, completed_at,
		       COALESCE(notes,''), created_at, updated_at
		FROM po_dsr
		WHERE token_hash = $1`, tokenHash,
	).Scan(
		&d.ID, &d.OrgID, &d.RequesterName, &d.RequesterEmail, &d.Type,
		&d.Description, &d.Status, &d.DueDate,
		&d.ReceivedAt, &d.CompletedAt,
		&d.Notes, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get dsr by token: %w", err)
	}
	return &d, nil
}

// GetOrgByDSRSlug looks up an organisation by its DSR portal slug.
func (r *Repository) GetOrgByDSRSlug(ctx context.Context, slug string) (orgID, orgName, dpoEmail, intro string, enabled bool, err error) {
	var dpoEmailPtr, introPtr *string
	err = r.db.QueryRow(ctx, `
		SELECT id::text, name,
		       dsr_dpo_email, dsr_portal_intro, dsr_portal_enabled
		FROM organizations
		WHERE dsr_portal_slug = $1`, slug,
	).Scan(&orgID, &orgName, &dpoEmailPtr, &introPtr, &enabled)
	if err != nil {
		return "", "", "", "", false, fmt.Errorf("get org by dsr slug: %w", err)
	}
	if dpoEmailPtr != nil {
		dpoEmail = *dpoEmailPtr
	}
	if introPtr != nil {
		intro = *introPtr
	}
	return orgID, orgName, dpoEmail, intro, enabled, nil
}

// UpdateDSRPortalSettings persists DSR portal configuration for an organisation.
func (r *Repository) UpdateDSRPortalSettings(ctx context.Context, orgID string, in UpdateDSRPortalSettingsInput) error {
	_, err := r.db.Exec(ctx, `
		UPDATE organizations
		SET dsr_portal_enabled = $2,
		    dsr_portal_slug    = NULLIF($3, ''),
		    dsr_dpo_email      = NULLIF($4, ''),
		    dsr_portal_intro   = NULLIF($5, '')
		WHERE id = $1::uuid`,
		orgID, in.Enabled, in.Slug, in.DPOEmail, in.Intro,
	)
	if err != nil {
		return fmt.Errorf("update dsr portal settings: %w", err)
	}
	return nil
}

// GetDSRPortalSettings fetches the current DSR portal configuration for an organisation.
func (r *Repository) GetDSRPortalSettings(ctx context.Context, orgID string) (*UpdateDSRPortalSettingsInput, error) {
	var out UpdateDSRPortalSettingsInput
	var slugPtr, dpoPtr, introPtr *string
	err := r.db.QueryRow(ctx, `
		SELECT dsr_portal_enabled,
		       dsr_portal_slug,
		       dsr_dpo_email,
		       dsr_portal_intro
		FROM organizations
		WHERE id = $1::uuid`, orgID,
	).Scan(&out.Enabled, &slugPtr, &dpoPtr, &introPtr)
	if err != nil {
		return nil, fmt.Errorf("get dsr portal settings: %w", err)
	}
	if slugPtr != nil {
		out.Slug = *slugPtr
	}
	if dpoPtr != nil {
		out.DPOEmail = *dpoPtr
	}
	if introPtr != nil {
		out.Intro = *introPtr
	}
	return &out, nil
}

// --- Paginated list helpers ---

// ListVVTPaged returns a page of VVT entries plus the total count.
func (r *Repository) ListVVTPaged(ctx context.Context, orgID string, offset, limit int) ([]VVTEntry, int, error) {
	var total int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM po_vvt_entries WHERE org_id = $1::uuid`, orgID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count vvt: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, name, purpose, legal_basis,
		       data_categories, data_subjects, recipients,
		       COALESCE(retention_period,''), third_country_transfer,
		       COALESCE(safeguards,''), COALESCE(responsible_person,''),
		       status, created_at, updated_at
		FROM po_vvt_entries
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list vvt paged: %w", err)
	}
	defer rows.Close()

	var entries []VVTEntry
	for rows.Next() {
		var e VVTEntry
		if err := rows.Scan(
			&e.ID, &e.OrgID, &e.Name, &e.Purpose, &e.LegalBasis,
			&e.DataCategories, &e.DataSubjects, &e.Recipients,
			&e.RetentionPeriod, &e.ThirdCountryTransfer,
			&e.Safeguards, &e.ResponsiblePerson,
			&e.Status, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan vvt paged: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}

// ListBreachesPaged returns a page of breach records plus the total count.
func (r *Repository) ListBreachesPaged(ctx context.Context, orgID string, offset, limit int) ([]Breach, int, error) {
	var total int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM po_breaches WHERE org_id = $1::uuid`, orgID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count breaches: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, title, COALESCE(description,''), discovered_at,
		       authority_deadline_at, authority_notified_at,
		       subjects_notification_required, subjects_notified_at,
		       affected_count, data_categories, status, created_at, updated_at
		FROM po_breaches
		WHERE org_id = $1::uuid
		ORDER BY discovered_at DESC
		LIMIT $2 OFFSET $3`, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list breaches paged: %w", err)
	}
	defer rows.Close()

	var breaches []Breach
	for rows.Next() {
		var b Breach
		if err := rows.Scan(
			&b.ID, &b.OrgID, &b.Title, &b.Description, &b.DiscoveredAt,
			&b.AuthorityDeadlineAt, &b.AuthorityNotifiedAt,
			&b.SubjectsNotificationRequired, &b.SubjectsNotifiedAt,
			&b.AffectedCount, &b.DataCategories, &b.Status,
			&b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan breach paged: %w", err)
		}
		breaches = append(breaches, b)
	}
	return breaches, total, rows.Err()
}
