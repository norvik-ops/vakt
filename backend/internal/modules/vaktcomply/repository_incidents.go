package vaktcomply

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/matharnica/vakt/internal/db"
)

// --- Incident Register (FR-CK13) ---

// Domain-Wrapper für CreateCKIncident-Result; spart Tipparbeit bei jedem Mapping.
func incidentFromCreateRow(row db.CreateCKIncidentRow) Incident {
	return incidentFromFields(incidentFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Severity: row.Severity, Status: row.Status,
		DiscoveredAt: row.DiscoveredAt, ResolvedAt: row.ResolvedAt,
		AffectedSystems: row.AffectedSystems, BreachID: row.BreachID,
		IncidentType: row.IncidentType, ReportingObligation: row.ReportingObligation,
		NotificationAuthority: row.NotificationAuthority,
		Deadline4h:            row.Deadline4h, Deadline24h: row.Deadline24h,
		Deadline72h: row.Deadline72h, Deadline30d: row.Deadline30d,
		Reported4hAt: row.Reported4hAt, Reported24hAt: row.Reported24hAt,
		Reported72hAt: row.Reported72hAt, Reported30dAt: row.Reported30dAt,
		AffectedCustomers: row.AffectedCustomers, FinancialImpactEstimate: row.FinancialImpactEstimate,
		IsMajorIncident: row.IsMajorIncident, SupplierID: row.SupplierID,
		NotifiedWarn24h: row.NotifiedWarn24h, NotifiedWarn72h: row.NotifiedWarn72h,
		NotifiedWarn30d: row.NotifiedWarn30d,
		CreatedAt:       row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
}

func incidentFromGetRow(row db.GetCKIncidentRow) Incident {
	return incidentFromFields(incidentFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Severity: row.Severity, Status: row.Status,
		DiscoveredAt: row.DiscoveredAt, ResolvedAt: row.ResolvedAt,
		AffectedSystems: row.AffectedSystems, BreachID: row.BreachID,
		IncidentType: row.IncidentType, ReportingObligation: row.ReportingObligation,
		NotificationAuthority: row.NotificationAuthority,
		Deadline4h:            row.Deadline4h, Deadline24h: row.Deadline24h,
		Deadline72h: row.Deadline72h, Deadline30d: row.Deadline30d,
		Reported4hAt: row.Reported4hAt, Reported24hAt: row.Reported24hAt,
		Reported72hAt: row.Reported72hAt, Reported30dAt: row.Reported30dAt,
		AffectedCustomers: row.AffectedCustomers, FinancialImpactEstimate: row.FinancialImpactEstimate,
		IsMajorIncident: row.IsMajorIncident, SupplierID: row.SupplierID,
		NotifiedWarn24h: row.NotifiedWarn24h, NotifiedWarn72h: row.NotifiedWarn72h,
		NotifiedWarn30d: row.NotifiedWarn30d,
		CreatedAt:       row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
}

func (r *Repository) ListIncidents(ctx context.Context, orgID string) ([]Incident, error) {
	rows, err := r.q.ListCKIncidents(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	out := make([]Incident, 0, len(rows))
	for _, row := range rows {
		out = append(out, incidentFromFields(incidentFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Severity: row.Severity, Status: row.Status,
			DiscoveredAt: row.DiscoveredAt, ResolvedAt: row.ResolvedAt,
			AffectedSystems: row.AffectedSystems, BreachID: row.BreachID,
			IncidentType: row.IncidentType, ReportingObligation: row.ReportingObligation,
			NotificationAuthority: row.NotificationAuthority,
			Deadline4h:            row.Deadline4h, Deadline24h: row.Deadline24h,
			Deadline72h: row.Deadline72h, Deadline30d: row.Deadline30d,
			Reported4hAt: row.Reported4hAt, Reported24hAt: row.Reported24hAt,
			Reported72hAt: row.Reported72hAt, Reported30dAt: row.Reported30dAt,
			AffectedCustomers: row.AffectedCustomers, FinancialImpactEstimate: row.FinancialImpactEstimate,
			IsMajorIncident: row.IsMajorIncident, SupplierID: row.SupplierID,
			NotifiedWarn24h: row.NotifiedWarn24h, NotifiedWarn72h: row.NotifiedWarn72h,
			NotifiedWarn30d: row.NotifiedWarn30d,
			CreatedAt:       row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

func (r *Repository) GetIncident(ctx context.Context, orgID, id string) (*Incident, error) {
	row, err := r.q.GetCKIncident(ctx, db.GetCKIncidentParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get incident: %w", err)
	}
	inc := incidentFromGetRow(row)
	return &inc, nil
}

func (r *Repository) UpdateIncident(ctx context.Context, orgID, id string, in UpdateIncidentInput) (*Incident, error) {
	incType := in.IncidentType
	if incType == "" {
		incType = "general"
	}
	obligation := in.ReportingObligation
	if obligation == "" {
		obligation = "unknown"
	}
	row, err := r.q.UpdateCKIncident(ctx, db.UpdateCKIncidentParams{
		ID:                      id,
		OrgID:                   orgID,
		Title:                   in.Title,
		Description:             in.Description,
		Severity:                in.Severity,
		Status:                  in.Status,
		AffectedSystems:         in.AffectedSystems,
		IncidentType:            incType,
		ReportingObligation:     obligation,
		NotificationAuthority:   ckOptText(in.NotificationAuthority),
		AffectedCustomers:       ckOptIntPtr(in.AffectedCustomers),
		FinancialImpactEstimate: optTextStrPtr(in.FinancialImpactEstimate),
		IsMajorIncident:         in.IsMajorIncident,
	})
	if err != nil {
		return nil, fmt.Errorf("update incident: %w", err)
	}
	inc := incidentFromFields(incidentFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Severity: row.Severity, Status: row.Status,
		DiscoveredAt: row.DiscoveredAt, ResolvedAt: row.ResolvedAt,
		AffectedSystems: row.AffectedSystems, BreachID: row.BreachID,
		IncidentType: row.IncidentType, ReportingObligation: row.ReportingObligation,
		NotificationAuthority: row.NotificationAuthority,
		Deadline4h:            row.Deadline4h, Deadline24h: row.Deadline24h,
		Deadline72h: row.Deadline72h, Deadline30d: row.Deadline30d,
		Reported4hAt: row.Reported4hAt, Reported24hAt: row.Reported24hAt,
		Reported72hAt: row.Reported72hAt, Reported30dAt: row.Reported30dAt,
		AffectedCustomers: row.AffectedCustomers, FinancialImpactEstimate: row.FinancialImpactEstimate,
		IsMajorIncident: row.IsMajorIncident, SupplierID: row.SupplierID,
		NotifiedWarn24h: row.NotifiedWarn24h, NotifiedWarn72h: row.NotifiedWarn72h,
		NotifiedWarn30d: row.NotifiedWarn30d,
		CreatedAt:       row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &inc, nil
}

func (r *Repository) CreateIncident(ctx context.Context, orgID string, in CreateIncidentInput, deadlines map[string]*time.Time) (*Incident, error) {
	incType := in.IncidentType
	if incType == "" {
		incType = "general"
	}
	obligation := in.ReportingObligation
	if obligation == "" {
		obligation = "unknown"
	}
	var d4h, d24h, d72h, d30d *time.Time
	if deadlines != nil {
		d4h = deadlines["4h"]
		d24h = deadlines["24h"]
		d72h = deadlines["72h"]
		d30d = deadlines["30d"]
	}
	row, err := r.q.CreateCKIncident(ctx, db.CreateCKIncidentParams{
		OrgID:                   orgID,
		Title:                   in.Title,
		Description:             in.Description,
		Severity:                in.Severity,
		DiscoveredAt:            pgtype.Timestamptz{Time: in.DiscoveredAt, Valid: true},
		AffectedSystems:         in.AffectedSystems,
		BreachID:                ckOptUUIDFromPtr(in.BreachID),
		IncidentType:            incType,
		ReportingObligation:     obligation,
		NotificationAuthority:   ckOptText(in.NotificationAuthority),
		Deadline4h:              ckOptTsPtr(d4h),
		Deadline24h:             ckOptTsPtr(d24h),
		Deadline72h:             ckOptTsPtr(d72h),
		Deadline30d:             ckOptTsPtr(d30d),
		AffectedCustomers:       ckOptIntPtr(in.AffectedCustomers),
		FinancialImpactEstimate: optTextStrPtr(in.FinancialImpactEstimate),
		IsMajorIncident:         in.IsMajorIncident,
	})
	if err != nil {
		return nil, fmt.Errorf("create incident: %w", err)
	}
	inc := incidentFromCreateRow(row)
	return &inc, nil
}

// ListIncidentsByType returns all non-closed incidents of a specific type for an organisation.
func (r *Repository) ListIncidentsByType(ctx context.Context, orgID, incidentType string) ([]Incident, error) {
	rows, err := r.q.ListCKIncidentsByType(ctx, db.ListCKIncidentsByTypeParams{OrgID: orgID, IncidentType: incidentType})
	if err != nil {
		return nil, fmt.Errorf("list incidents by type: %w", err)
	}
	out := make([]Incident, 0, len(rows))
	for _, row := range rows {
		out = append(out, incidentFromFields(incidentFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Severity: row.Severity, Status: row.Status,
			DiscoveredAt: row.DiscoveredAt, ResolvedAt: row.ResolvedAt,
			AffectedSystems: row.AffectedSystems, BreachID: row.BreachID,
			IncidentType: row.IncidentType, ReportingObligation: row.ReportingObligation,
			NotificationAuthority: row.NotificationAuthority,
			Deadline4h:            row.Deadline4h, Deadline24h: row.Deadline24h,
			Deadline72h: row.Deadline72h, Deadline30d: row.Deadline30d,
			Reported4hAt: row.Reported4hAt, Reported24hAt: row.Reported24hAt,
			Reported72hAt: row.Reported72hAt, Reported30dAt: row.Reported30dAt,
			AffectedCustomers: row.AffectedCustomers, FinancialImpactEstimate: row.FinancialImpactEstimate,
			IsMajorIncident: row.IsMajorIncident, SupplierID: row.SupplierID,
			NotifiedWarn24h: row.NotifiedWarn24h, NotifiedWarn72h: row.NotifiedWarn72h,
			NotifiedWarn30d: row.NotifiedWarn30d,
			CreatedAt:       row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

func (r *Repository) MarkDeadlineReported(ctx context.Context, orgID, id, deadline string) (*Incident, error) {
	if deadline != "4h" && deadline != "24h" && deadline != "72h" && deadline != "30d" {
		return nil, fmt.Errorf("unknown deadline: %s", deadline)
	}
	row, err := r.q.MarkCKIncidentDeadlineReported(ctx, db.MarkCKIncidentDeadlineReportedParams{
		ID:       id,
		OrgID:    orgID,
		Deadline: deadline,
	})
	if err != nil {
		return nil, fmt.Errorf("mark deadline reported: %w", err)
	}
	inc := incidentFromFields(incidentFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Severity: row.Severity, Status: row.Status,
		DiscoveredAt: row.DiscoveredAt, ResolvedAt: row.ResolvedAt,
		AffectedSystems: row.AffectedSystems, BreachID: row.BreachID,
		IncidentType: row.IncidentType, ReportingObligation: row.ReportingObligation,
		NotificationAuthority: row.NotificationAuthority,
		Deadline4h:            row.Deadline4h, Deadline24h: row.Deadline24h,
		Deadline72h: row.Deadline72h, Deadline30d: row.Deadline30d,
		Reported4hAt: row.Reported4hAt, Reported24hAt: row.Reported24hAt,
		Reported72hAt: row.Reported72hAt, Reported30dAt: row.Reported30dAt,
		AffectedCustomers: row.AffectedCustomers, FinancialImpactEstimate: row.FinancialImpactEstimate,
		IsMajorIncident: row.IsMajorIncident, SupplierID: row.SupplierID,
		NotifiedWarn24h: row.NotifiedWarn24h, NotifiedWarn72h: row.NotifiedWarn72h,
		NotifiedWarn30d: row.NotifiedWarn30d,
		CreatedAt:       row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &inc, nil
}

// UpdateIncidentReportability stores the questionnaire answers and updates
// reporting_obligation, notification_authority, and gdpr_notification_required.
func (r *Repository) UpdateIncidentReportability(
	ctx context.Context,
	orgID, incidentID, obligation, authority string,
	gdprRequired bool,
	answersJSON []byte,
) error {
	if err := r.q.UpdateCKIncidentReportability(ctx, db.UpdateCKIncidentReportabilityParams{
		ID:                       incidentID,
		OrgID:                    orgID,
		ReportingObligation:      obligation,
		NotificationAuthority:    ckOptText(authority),
		GdprNotificationRequired: gdprRequired,
		ReportabilityAnswers:     answersJSON,
	}); err != nil {
		return fmt.Errorf("update incident reportability: %w", err)
	}
	return nil
}

// SaveIncidentReport archives a generated Meldungsformular with optional PDF bytes.
func (r *Repository) SaveIncidentReport(ctx context.Context, orgID, incidentID, reportType, authority string, pdfData []byte, metadata []byte) (*IncidentReport, error) {
	row, err := r.q.SaveCKIncidentReport(ctx, db.SaveCKIncidentReportParams{
		OrgID:      orgID,
		IncidentID: incidentID,
		ReportType: reportType,
		Authority:  authority,
		PdfData:    pdfData,
		Metadata:   metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("save incident report: %w", err)
	}
	return &IncidentReport{
		ID:          row.ID,
		OrgID:       row.OrgID,
		IncidentID:  row.IncidentID,
		ReportType:  row.ReportType,
		Authority:   row.Authority,
		GeneratedAt: ckTsToTime(row.GeneratedAt),
	}, nil
}

// ListIncidentReports returns all archived reports for a given incident.
func (r *Repository) ListIncidentReports(ctx context.Context, orgID, incidentID string) ([]IncidentReport, error) {
	rows, err := r.q.ListCKIncidentReports(ctx, db.ListCKIncidentReportsParams{OrgID: orgID, IncidentID: incidentID})
	if err != nil {
		return nil, fmt.Errorf("list incident reports: %w", err)
	}
	reports := make([]IncidentReport, 0, len(rows))
	for _, row := range rows {
		reports = append(reports, IncidentReport{
			ID:          row.ID,
			OrgID:       row.OrgID,
			IncidentID:  row.IncidentID,
			ReportType:  row.ReportType,
			Authority:   row.Authority,
			GeneratedAt: ckTsToTime(row.GeneratedAt),
		})
	}
	return reports, nil
}

// GetIncidentReportPDF returns the stored PDF bytes for a report entry.
func (r *Repository) GetIncidentReportPDF(ctx context.Context, orgID, reportID string) ([]byte, error) {
	data, err := r.q.GetCKIncidentReportPDF(ctx, db.GetCKIncidentReportPDFParams{ID: reportID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get incident report pdf: %w", err)
	}
	return data, nil
}

// MarkIncidentWarnNotified sets the notified_warn_* flag for a given deadline
// so the 12h-before warning is only sent once per incident + deadline pair.
func (r *Repository) MarkIncidentWarnNotified(ctx context.Context, orgID, incidentID, deadline string) error {
	if deadline != "24h" && deadline != "72h" && deadline != "30d" {
		return fmt.Errorf("unknown deadline: %s", deadline)
	}
	return r.q.MarkCKIncidentWarnNotified(ctx, db.MarkCKIncidentWarnNotifiedParams{
		ID:       incidentID,
		OrgID:    orgID,
		Deadline: deadline,
	})
}

// GetOrgSector returns the sector and federal_state for the given org.
func (r *Repository) GetOrgSector(ctx context.Context, orgID string) (*OrgSectorSettings, error) {
	row, err := r.q.GetCKOrgSector(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("get org sector: %w", err)
	}
	return &OrgSectorSettings{
		Sector:       row.Sector,
		FederalState: row.FederalState,
	}, nil
}

// UpdateOrgSector sets the sector and federal_state for the given org.
func (r *Repository) UpdateOrgSector(ctx context.Context, orgID, sector, federalState string) error {
	if _, err := r.q.UpdateCKOrgSector(ctx, db.UpdateCKOrgSectorParams{
		ID:           orgID,
		Sector:       sector,
		FederalState: federalState,
	}); err != nil {
		return fmt.Errorf("update org sector: %w", err)
	}
	return nil
}

// GetAdminEmails returns the e-mail addresses of active Admin users for the given org.
func (r *Repository) GetAdminEmails(ctx context.Context, orgID string) ([]string, error) {
	return r.q.GetCKOrgAdminEmails(ctx, orgID)
}

// ListIncidentsBySupplier returns all incidents linked to a given supplier via supplier_id FK.
func (r *Repository) ListIncidentsBySupplier(ctx context.Context, orgID, supplierID string) ([]Incident, error) {
	rows, err := r.q.ListCKIncidentsBySupplier(ctx, db.ListCKIncidentsBySupplierParams{
		OrgID:      orgID,
		SupplierID: ckOptUUIDFromStr(supplierID),
	})
	if err != nil {
		return nil, fmt.Errorf("list incidents by supplier: %w", err)
	}
	out := make([]Incident, 0, len(rows))
	for _, row := range rows {
		out = append(out, incidentFromFields(incidentFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Severity: row.Severity, Status: row.Status,
			DiscoveredAt: row.DiscoveredAt, ResolvedAt: row.ResolvedAt,
			AffectedSystems: row.AffectedSystems, BreachID: row.BreachID,
			IncidentType: row.IncidentType, ReportingObligation: row.ReportingObligation,
			NotificationAuthority: row.NotificationAuthority,
			Deadline4h:            row.Deadline4h, Deadline24h: row.Deadline24h,
			Deadline72h: row.Deadline72h, Deadline30d: row.Deadline30d,
			Reported4hAt: row.Reported4hAt, Reported24hAt: row.Reported24hAt,
			Reported72hAt: row.Reported72hAt, Reported30dAt: row.Reported30dAt,
			AffectedCustomers: row.AffectedCustomers, FinancialImpactEstimate: row.FinancialImpactEstimate,
			IsMajorIncident: row.IsMajorIncident, SupplierID: row.SupplierID,
			NotifiedWarn24h: row.NotifiedWarn24h, NotifiedWarn72h: row.NotifiedWarn72h,
			NotifiedWarn30d: row.NotifiedWarn30d,
			CreatedAt:       row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

// ListIncidentsPaged returns a page of incidents plus the total count.
func (r *Repository) ListIncidentsPaged(ctx context.Context, orgID string, offset, limit int) ([]Incident, int, error) {
	total, err := r.q.CountCKIncidents(ctx, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("count incidents: %w", err)
	}
	rows, err := r.q.ListCKIncidentsPaged(ctx, db.ListCKIncidentsPagedParams{
		OrgID:  orgID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list incidents paged: %w", err)
	}
	incidents := make([]Incident, 0, len(rows))
	for _, row := range rows {
		incidents = append(incidents, incidentFromFields(incidentFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Severity: row.Severity, Status: row.Status,
			DiscoveredAt: row.DiscoveredAt, ResolvedAt: row.ResolvedAt,
			AffectedSystems: row.AffectedSystems, BreachID: row.BreachID,
			IncidentType: row.IncidentType, ReportingObligation: row.ReportingObligation,
			NotificationAuthority: row.NotificationAuthority,
			Deadline4h:            row.Deadline4h, Deadline24h: row.Deadline24h,
			Deadline72h: row.Deadline72h, Deadline30d: row.Deadline30d,
			Reported4hAt: row.Reported4hAt, Reported24hAt: row.Reported24hAt,
			Reported72hAt: row.Reported72hAt, Reported30dAt: row.Reported30dAt,
			AffectedCustomers: row.AffectedCustomers, FinancialImpactEstimate: row.FinancialImpactEstimate,
			IsMajorIncident: row.IsMajorIncident, SupplierID: row.SupplierID,
			NotifiedWarn24h: row.NotifiedWarn24h, NotifiedWarn72h: row.NotifiedWarn72h,
			NotifiedWarn30d: row.NotifiedWarn30d,
			CreatedAt:       row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return incidents, int(total), nil
}

// UpdateIncidentDORADeadlineStatus persists the computed Ampel-Status map to
// ck_incidents.dora_deadline_status JSONB. S37-4.
func (r *Repository) UpdateIncidentDORADeadlineStatus(ctx context.Context, incidentID string, status map[string]string) error {
	b, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshal dora deadline status: %w", err)
	}
	// orgid-lint: global — UPDATE by PK; incidentID comes from a prior org-scoped query in the background job
	_, err = r.db.Exec(ctx,
		`UPDATE ck_incidents SET dora_deadline_status = $2, updated_at = NOW() WHERE id = $1::uuid`,
		incidentID, b,
	)
	return err
}

// SaveClassificationResult persists the classify-reporting wizard result to
// ck_incidents.classification_result JSONB (S39-1, Migration 140).
func (r *Repository) SaveClassificationResult(ctx context.Context, orgID, incidentID string, result ClassificationResult) error {
	b, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal classification result: %w", err)
	}
	tag, err := r.db.Exec(ctx,
		`UPDATE ck_incidents
		    SET classification_result = $3, updated_at = NOW()
		  WHERE id = $1::uuid AND org_id = $2::uuid`,
		incidentID, orgID, b,
	)
	if err != nil {
		return fmt.Errorf("save classification result: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ListNIS2ClassifiedIncidents returns all incidents where the classification
// wizard marked the obligation as "probably" — used by the NIS2 deadline check job (S39-2).
func (r *Repository) ListNIS2ClassifiedIncidents(ctx context.Context, orgID string) ([]Incident, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id::text, org_id::text, title, description, severity, status,
		        discovered_at, resolved_at, affected_systems, incident_type,
		        reporting_obligation, notification_authority,
		        deadline_24h, deadline_72h, deadline_30d,
		        reported_24h_at, reported_72h_at, reported_30d_at,
		        notified_warn_24h, notified_warn_72h, notified_warn_30d,
		        created_at, updated_at
		   FROM ck_incidents
		  WHERE org_id = $1::uuid
		    AND classification_result->>'obligation' = 'probably'
		    AND status NOT IN ('resolved','closed')`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list nis2 classified incidents: %w", err)
	}
	defer rows.Close()

	var out []Incident
	for rows.Next() {
		var inc Incident
		var desc, severity, status, incType, obligation pgtype.Text
		var authority pgtype.Text
		var resolvedAt, d24h, d72h, d30d pgtype.Timestamptz
		var r24h, r72h, r30d pgtype.Timestamptz
		var systems []string
		var warn24h, warn72h, warn30d bool
		var createdAt, updatedAt pgtype.Timestamptz

		if err := rows.Scan(
			&inc.ID, &inc.OrgID, &inc.Title, &desc, &severity, &status,
			&inc.DiscoveredAt, &resolvedAt, &systems, &incType,
			&obligation, &authority,
			&d24h, &d72h, &d30d,
			&r24h, &r72h, &r30d,
			&warn24h, &warn72h, &warn30d,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan nis2 classified incident: %w", err)
		}
		inc.Description = desc.String
		inc.Severity = severity.String
		inc.Status = status.String
		inc.AffectedSystems = systems
		inc.IncidentType = incType.String
		inc.ReportingObligation = obligation.String
		inc.NotificationAuthority = authority.String
		inc.ResolvedAt = ckTsToTimePtr(resolvedAt)
		inc.Deadline24h = ckTsToTimePtr(d24h)
		inc.Deadline72h = ckTsToTimePtr(d72h)
		inc.Deadline30d = ckTsToTimePtr(d30d)
		inc.Reported24hAt = ckTsToTimePtr(r24h)
		inc.Reported72hAt = ckTsToTimePtr(r72h)
		inc.Reported30dAt = ckTsToTimePtr(r30d)
		inc.NotifiedWarn24h = warn24h
		inc.NotifiedWarn72h = warn72h
		inc.NotifiedWarn30d = warn30d
		inc.CreatedAt = ckTsToTime(createdAt)
		inc.UpdatedAt = ckTsToTime(updatedAt)
		out = append(out, inc)
	}
	return out, rows.Err()
}

// CountRecentIncidents returns the number of incidents created at or after `since`.
func (r *Repository) CountRecentIncidents(ctx context.Context, orgID string, since time.Time) (int, error) {
	n, err := r.q.CountCKRecentIncidents(ctx, db.CountCKRecentIncidentsParams{OrgID: orgID, Since: since})
	if err != nil {
		return 0, fmt.Errorf("count recent incidents: %w", err)
	}
	return int(n), nil
}

// CountIncidentsSince returns the number of incidents created at or after `since`.
func (r *Repository) CountIncidentsSince(ctx context.Context, orgID string, since time.Time) (int, error) {
	n, err := r.q.CountCKIncidentsSince(ctx, db.CountCKIncidentsSinceParams{OrgID: orgID, Since: since})
	if err != nil {
		return 0, fmt.Errorf("count incidents since: %w", err)
	}
	return int(n), nil
}
