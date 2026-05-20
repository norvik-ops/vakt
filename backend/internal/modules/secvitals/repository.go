package secvitals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/matharnica/vakt/internal/db"
)

// Repository handles ComplyKit data access. Migrating to sqlc incrementally
// (ADR-0005). Methods using r.q are sqlc-backed. Two methods bleiben bewusst
// embedded und sind oben mit „embedded SQL by design" markiert:
//   - GetMappingsForControl: UNION mit 4-stufigem JOIN-Chain (LIKE-Subqueries)
//   - RecordControlReview: dynamische UPDATE-Klausel innerhalb einer Transaktion
type Repository struct {
	db *pgxpool.Pool
	q  *db.Queries
}

// NewRepository creates a new ComplyKit repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{db: pool, q: db.New(pool)}
}

// frameworkFromCkFrameworks maps the sqlc-generated row to the Framework
// domain type. ReadinessScore is not stored in the table — it's computed
// per-call in service layer.
func frameworkFromCkFrameworks(r db.CkFrameworks) Framework {
	return Framework{
		ID:        r.ID,
		OrgID:     r.OrgID,
		Name:      r.Name,
		Version:   r.Version,
		IsBuiltin: r.IsBuiltin,
		CreatedAt: ckTsToTime(r.CreatedAt),
	}
}

// ckTsToTime converts pgtype.Timestamptz to time.Time (zero on NULL).
func ckTsToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// ckTsToTimePtr converts pgtype.Timestamptz to *time.Time (nil on NULL).
func ckTsToTimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	tm := t.Time
	return &tm
}

// ckDateToTimePtr converts pgtype.Date to *time.Time (nil on NULL).
func ckDateToTimePtr(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	tm := d.Time
	return &tm
}

// ckOptText: empty string → invalid pgtype.Text (NULL in DB).
func ckOptText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// ckOptIntPtr: nil → invalid pgtype.Int4 (NULL in DB).
func ckOptIntPtr(i *int) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*i), Valid: true}
}

// ckOptUUIDFromStr converts a string to pgtype.UUID; empty → invalid.
func ckOptUUIDFromStr(s string) pgtype.UUID {
	if s == "" {
		return pgtype.UUID{}
	}
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}

// ckOptTsPtr converts *time.Time to pgtype.Timestamptz; nil → invalid.
func ckOptTsPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// ckOptDatePtr: nil string ptr → invalid; "YYYY-MM-DD" string → pgtype.Date.
func ckOptDatePtr(s *string) pgtype.Date {
	if s == nil || *s == "" {
		return pgtype.Date{}
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

// controlFields holds all columns shared between ListCKControls and GetCKControl
// row types. ADR-0013: explicit container so a single mapper handles both.
type controlFields struct {
	ID, FrameworkID, OrgID, ControlID, Title string
	Description                              pgtype.Text
	Domain, EvidenceType                     string
	Weight                                   int32
	NotApplicable                            bool
	NotApplicableReason, ManualStatus        pgtype.Text
	MaturityScore                            int16
	Owner                                    pgtype.Text
	LastReviewedAt                           pgtype.Timestamptz
	ReviewIntervalDays                       int32
	NextReviewDue                            pgtype.Timestamptz
	LastReviewedBy, ReviewNote               string
	DueDate                                  pgtype.Date
}

// policyFields collects the columns shared by all Policy-returning sqlc rows.
type policyFields struct {
	ID, OrgID, Title, Description, Category, Status, Version string
	EffectiveDate, ReviewDate                                pgtype.Date
	Owner                                                    string
	CreatedAt, UpdatedAt                                     pgtype.Timestamptz
	VersionNum                                               int32
	VersionNote, LastUpdatedBy                               string
	ReviewedAt                                               pgtype.Timestamptz
	NextReviewDue                                            pgtype.Date
}

func policyFromFields(f policyFields) Policy {
	return Policy{
		ID:            f.ID,
		OrgID:         f.OrgID,
		Title:         f.Title,
		Description:   f.Description,
		Category:      f.Category,
		Status:        f.Status,
		Version:       f.Version,
		VersionNum:    int(f.VersionNum),
		VersionNote:   f.VersionNote,
		LastUpdatedBy: f.LastUpdatedBy,
		ReviewedAt:    ckTsToTimePtr(f.ReviewedAt),
		NextReviewDue: dateToStringPtrLocal(f.NextReviewDue),
		EffectiveDate: ckDateToTimePtr(f.EffectiveDate),
		ReviewDate:    ckDateToTimePtr(f.ReviewDate),
		Owner:         f.Owner,
		CreatedAt:     ckTsToTime(f.CreatedAt),
		UpdatedAt:     ckTsToTime(f.UpdatedAt),
	}
}

// dateToStringPtrLocal yields "YYYY-MM-DD" or nil from pgtype.Date.
func dateToStringPtrLocal(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format("2006-01-02")
	return &s
}

// incidentFields holds all columns shared between every Incident-returning
// sqlc query. ADR-0013: one mapper handles all Row-types.
type incidentFields struct {
	ID, OrgID, Title, Description, Severity, Status           string
	DiscoveredAt, ResolvedAt                                  pgtype.Timestamptz
	AffectedSystems                                           []string
	BreachID                                                  pgtype.UUID
	IncidentType, ReportingObligation                         string
	NotificationAuthority                                     pgtype.Text
	Deadline4h, Deadline24h, Deadline72h, Deadline30d         pgtype.Timestamptz
	Reported4hAt, Reported24hAt, Reported72hAt, Reported30dAt pgtype.Timestamptz
	AffectedCustomers                                         pgtype.Int4
	FinancialImpactEstimate                                   pgtype.Text
	IsMajorIncident                                           bool
	SupplierID                                                pgtype.UUID
	NotifiedWarn24h, NotifiedWarn72h, NotifiedWarn30d         bool
	CreatedAt, UpdatedAt                                      pgtype.Timestamptz
}

func uuidPtrFromPgtype(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := u.String()
	return &s
}

func incidentFromFields(f incidentFields) Incident {
	return Incident{
		ID:                      f.ID,
		OrgID:                   f.OrgID,
		Title:                   f.Title,
		Description:             f.Description,
		Severity:                f.Severity,
		Status:                  f.Status,
		DiscoveredAt:            ckTsToTime(f.DiscoveredAt),
		ResolvedAt:              ckTsToTimePtr(f.ResolvedAt),
		AffectedSystems:         f.AffectedSystems,
		BreachID:                uuidPtrFromPgtype(f.BreachID),
		IncidentType:            f.IncidentType,
		ReportingObligation:     f.ReportingObligation,
		NotificationAuthority:   f.NotificationAuthority.String,
		Deadline4h:              ckTsToTimePtr(f.Deadline4h),
		Deadline24h:             ckTsToTimePtr(f.Deadline24h),
		Deadline72h:             ckTsToTimePtr(f.Deadline72h),
		Deadline30d:             ckTsToTimePtr(f.Deadline30d),
		Reported4hAt:            ckTsToTimePtr(f.Reported4hAt),
		Reported24hAt:           ckTsToTimePtr(f.Reported24hAt),
		Reported72hAt:           ckTsToTimePtr(f.Reported72hAt),
		Reported30dAt:           ckTsToTimePtr(f.Reported30dAt),
		AffectedCustomers:       intPtrFromInt4(f.AffectedCustomers),
		FinancialImpactEstimate: textPtrOrNil(f.FinancialImpactEstimate),
		IsMajorIncident:         f.IsMajorIncident,
		SupplierID:              uuidPtrFromPgtype(f.SupplierID),
		NotifiedWarn24h:         f.NotifiedWarn24h,
		NotifiedWarn72h:         f.NotifiedWarn72h,
		NotifiedWarn30d:         f.NotifiedWarn30d,
		CreatedAt:               ckTsToTime(f.CreatedAt),
		UpdatedAt:               ckTsToTime(f.UpdatedAt),
	}
}

func textPtrOrNil(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

// riskFields collects all columns shared between every Risk-returning sqlc
// query. ADR-0013: centralise mapping in one helper.
type riskFields struct {
	ID, OrgID, Title, Description, Category  string
	Likelihood, Impact                       int16
	RiskScore                                pgtype.Int2
	Owner, Status, Treatment, TreatmentNotes string
	TreatmentOption                          pgtype.Text
	TreatmentPlan, TreatmentOwner            string
	TreatmentDueDate                         pgtype.Date
	TreatmentStatus                          string
	ResidualLikelihood                       pgtype.Int4
	ResidualImpact                           pgtype.Int4
	CreatedAt, UpdatedAt                     pgtype.Timestamptz
}

func intPtrFromInt4(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int32)
	return &i
}

func riskFromFields(f riskFields) Risk {
	return Risk{
		ID:                 f.ID,
		OrgID:              f.OrgID,
		Title:              f.Title,
		Description:        f.Description,
		Category:           f.Category,
		Likelihood:         int(f.Likelihood),
		Impact:             int(f.Impact),
		RiskScore:          int(f.RiskScore.Int16),
		Owner:              f.Owner,
		Status:             f.Status,
		Treatment:          f.Treatment,
		TreatmentNotes:     f.TreatmentNotes,
		TreatmentOption:    f.TreatmentOption.String,
		TreatmentPlan:      f.TreatmentPlan,
		TreatmentOwner:     f.TreatmentOwner,
		TreatmentDueDate:   ckDateToTimePtr(f.TreatmentDueDate),
		TreatmentStatus:    f.TreatmentStatus,
		ResidualLikelihood: intPtrFromInt4(f.ResidualLikelihood),
		ResidualImpact:     intPtrFromInt4(f.ResidualImpact),
		CreatedAt:          ckTsToTime(f.CreatedAt),
		UpdatedAt:          ckTsToTime(f.UpdatedAt),
	}
}

// evidenceFields is the union of columns returned by all Evidence-returning
// sqlc queries (Add/List/GetExpiring). Identical shape, so one container.
type evidenceFields struct {
	ID               string
	ControlID        pgtype.UUID
	OrgID            string
	Title            string
	Description      pgtype.Text
	Source           string
	FilePath         pgtype.Text
	FileSize         pgtype.Int8
	Status           string
	Version          int32
	ExpiresAt        pgtype.Timestamptz
	ExpiryNotifiedAt pgtype.Timestamptz
	CreatedAt        pgtype.Timestamptz
	UpdatedAt        pgtype.Timestamptz
}

func evidenceFromFields(f evidenceFields) Evidence {
	var controlID string
	if f.ControlID.Valid {
		controlID = f.ControlID.String()
	}
	return Evidence{
		ID:               f.ID,
		ControlID:        controlID,
		OrgID:            f.OrgID,
		Title:            f.Title,
		Description:      f.Description.String,
		Source:           f.Source,
		FilePath:         f.FilePath.String,
		FileSize:         f.FileSize.Int64,
		Status:           f.Status,
		Version:          int(f.Version),
		ExpiresAt:        ckTsToTimePtr(f.ExpiresAt),
		ExpiryNotifiedAt: ckTsToTimePtr(f.ExpiryNotifiedAt),
		CreatedAt:        ckTsToTime(f.CreatedAt),
		UpdatedAt:        ckTsToTime(f.UpdatedAt),
	}
}

func controlFromFields(f controlFields) Control {
	nextReview := ckTsToTimePtr(f.NextReviewDue)
	overdue := nextReview != nil && nextReview.Before(time.Now())
	return Control{
		ID:                  f.ID,
		FrameworkID:         f.FrameworkID,
		OrgID:               f.OrgID,
		ControlID:           f.ControlID,
		Title:               f.Title,
		Description:         f.Description.String,
		Domain:              f.Domain,
		EvidenceType:        f.EvidenceType,
		Weight:              int(f.Weight),
		NotApplicable:       f.NotApplicable,
		NotApplicableReason: f.NotApplicableReason.String,
		ManualStatus:        f.ManualStatus.String,
		MaturityScore:       int(f.MaturityScore),
		Owner:               f.Owner.String,
		LastReviewedAt:      ckTsToTimePtr(f.LastReviewedAt),
		ReviewIntervalDays:  int(f.ReviewIntervalDays),
		NextReviewDue:       nextReview,
		LastReviewedBy:      f.LastReviewedBy,
		ReviewNote:          f.ReviewNote,
		IsReviewOverdue:     overdue,
		DueDate:             ckDateToTimePtr(f.DueDate),
	}
}

// --- Frameworks ---

// CreateFramework inserts a new framework for an organisation.
func (r *Repository) CreateFramework(ctx context.Context, orgID, name, version string, isBuiltin bool) (*Framework, error) {
	row, err := r.q.CreateCKFramework(ctx, db.CreateCKFrameworkParams{
		OrgID:     orgID,
		Name:      name,
		Version:   version,
		IsBuiltin: isBuiltin,
	})
	if err != nil {
		return nil, fmt.Errorf("create framework: %w", err)
	}
	f := frameworkFromCkFrameworks(row)
	return &f, nil
}

// ListFrameworks returns all frameworks enabled for an organisation.
func (r *Repository) ListFrameworks(ctx context.Context, orgID string) ([]Framework, error) {
	rows, err := r.q.ListCKFrameworks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list frameworks: %w", err)
	}
	out := make([]Framework, 0, len(rows))
	for _, row := range rows {
		out = append(out, frameworkFromCkFrameworks(row))
	}
	return out, nil
}

// DeleteFramework removes a framework and all its controls/evidence (cascade).
func (r *Repository) DeleteFramework(ctx context.Context, orgID, frameworkID string) error {
	n, err := r.q.DeleteCKFramework(ctx, db.DeleteCKFrameworkParams{ID: frameworkID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete framework: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("framework not found")
	}
	return nil
}

// GetFramework returns a single framework by ID within an organisation.
func (r *Repository) GetFramework(ctx context.Context, orgID, frameworkID string) (*Framework, error) {
	row, err := r.q.GetCKFramework(ctx, db.GetCKFrameworkParams{ID: frameworkID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get framework: %w", err)
	}
	f := frameworkFromCkFrameworks(row)
	return &f, nil
}

// FindFrameworkByName returns a single framework by name within an organisation.
// Returns nil, nil if not found.
func (r *Repository) FindFrameworkByName(ctx context.Context, orgID, name string) (*Framework, error) {
	row, err := r.q.FindCKFrameworkByName(ctx, db.FindCKFrameworkByNameParams{OrgID: orgID, Name: name})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find framework by name: %w", err)
	}
	f := frameworkFromCkFrameworks(row)
	return &f, nil
}

// ListAllBuiltinFrameworks returns all builtin frameworks across all organisations.
// Used for startup reseeding of controls.
func (r *Repository) ListAllBuiltinFrameworks(ctx context.Context) ([]Framework, error) {
	rows, err := r.q.ListAllBuiltinCKFrameworks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list all builtin frameworks: %w", err)
	}
	out := make([]Framework, 0, len(rows))
	for _, row := range rows {
		out = append(out, frameworkFromCkFrameworks(row))
	}
	return out, nil
}

// FrameworkExists reports whether a framework with the given name already exists for the org.
func (r *Repository) FrameworkExists(ctx context.Context, orgID, name string) (bool, error) {
	exists, err := r.q.CKFrameworkExists(ctx, db.CKFrameworkExistsParams{OrgID: orgID, Name: name})
	if err != nil {
		return false, fmt.Errorf("framework exists check: %w", err)
	}
	return exists, nil
}

// --- Controls ---

// BulkInsertControls inserts a slice of controls for a framework in a single transaction.
func (r *Repository) BulkInsertControls(ctx context.Context, controls []Control) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op when Commit succeeded
	qtx := r.q.WithTx(tx)

	for _, c := range controls {
		if err := qtx.BulkInsertCKControl(ctx, db.BulkInsertCKControlParams{
			FrameworkID:  c.FrameworkID,
			OrgID:        c.OrgID,
			ControlID:    c.ControlID,
			Title:        c.Title,
			Description:  ckOptText(c.Description),
			Domain:       c.Domain,
			EvidenceType: c.EvidenceType,
			Weight:       int32(c.Weight),
		}); err != nil {
			return fmt.Errorf("insert control %s: %w", c.ControlID, err)
		}
	}

	return tx.Commit(ctx)
}

// UpdateControl sets not_applicable, reason, manual_status, optionally maturity_score, and due_date on a control.
func (r *Repository) UpdateControl(ctx context.Context, orgID, controlID string, notApplicable bool, reason, manualStatus, owner string, maturityScore *int, dueDate *string) error {
	n, err := r.q.UpdateCKControl(ctx, db.UpdateCKControlParams{
		ID:            controlID,
		OrgID:         orgID,
		NotApplicable: notApplicable,
		Reason:        ckOptText(reason),
		ManualStatus:  ckOptText(manualStatus),
		Owner:         ckOptText(owner),
		MaturityScore: ckOptIntPtr(maturityScore),
		DueDate:       ckOptDatePtr(dueDate),
	})
	if err != nil {
		return fmt.Errorf("update control: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("control not found")
	}
	return nil
}

// ListControls returns all controls for a framework within an organisation.
func (r *Repository) ListControls(ctx context.Context, orgID, frameworkID string) ([]Control, error) {
	rows, err := r.q.ListCKControls(ctx, db.ListCKControlsParams{FrameworkID: frameworkID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("list controls: %w", err)
	}
	out := make([]Control, 0, len(rows))
	for _, row := range rows {
		out = append(out, controlFromFields(controlFields{
			ID: row.ID, FrameworkID: row.FrameworkID, OrgID: row.OrgID,
			ControlID: row.ControlID, Title: row.Title, Description: row.Description,
			Domain: row.Domain, EvidenceType: row.EvidenceType, Weight: row.Weight,
			NotApplicable: row.NotApplicable, NotApplicableReason: row.NotApplicableReason,
			ManualStatus: row.ManualStatus, MaturityScore: row.MaturityScore, Owner: row.Owner,
			LastReviewedAt: row.LastReviewedAt, ReviewIntervalDays: row.ReviewIntervalDays,
			NextReviewDue: row.NextReviewDue, LastReviewedBy: row.LastReviewedBy,
			ReviewNote: row.ReviewNote, DueDate: row.DueDate,
		}))
	}
	return out, nil
}

// GetControl returns a single control by its UUID within an organisation.
func (r *Repository) GetControl(ctx context.Context, orgID, controlID string) (*Control, error) {
	row, err := r.q.GetCKControl(ctx, db.GetCKControlParams{ID: controlID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get control: %w", err)
	}
	c := controlFromFields(controlFields{
		ID: row.ID, FrameworkID: row.FrameworkID, OrgID: row.OrgID,
		ControlID: row.ControlID, Title: row.Title, Description: row.Description,
		Domain: row.Domain, EvidenceType: row.EvidenceType, Weight: row.Weight,
		NotApplicable: row.NotApplicable, NotApplicableReason: row.NotApplicableReason,
		ManualStatus: row.ManualStatus, MaturityScore: row.MaturityScore, Owner: row.Owner,
		LastReviewedAt: row.LastReviewedAt, ReviewIntervalDays: row.ReviewIntervalDays,
		NextReviewDue: row.NextReviewDue, LastReviewedBy: row.LastReviewedBy,
		ReviewNote: row.ReviewNote, DueDate: row.DueDate,
	})
	return &c, nil
}

// UpdateSoAMetadata persists the SoA-specific fields for a single control.
func (r *Repository) UpdateSoAMetadata(ctx context.Context, orgID, controlID, justification, implementation, responsible string) error {
	n, err := r.q.UpdateCKControlSoAMetadata(ctx, db.UpdateCKControlSoAMetadataParams{
		ID:             controlID,
		OrgID:          orgID,
		Justification:  ckOptText(justification),
		Implementation: ckOptText(implementation),
		Responsible:    ckOptText(responsible),
	})
	if err != nil {
		return fmt.Errorf("update soa metadata: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("control not found")
	}
	return nil
}

// ListControlsForSoA returns all controls for a framework with SoA metadata and evidence counts,
// ordered by control_id for consistent PDF output.
func (r *Repository) ListControlsForSoA(ctx context.Context, orgID, frameworkID string) ([]SoARow, error) {
	rows, err := r.q.ListCKControlsForSoA(ctx, db.ListCKControlsForSoAParams{FrameworkID: frameworkID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("list controls for soa: %w", err)
	}
	out := make([]SoARow, 0, len(rows))
	for _, row := range rows {
		out = append(out, SoARow{
			ControlID:      row.ControlID,
			Title:          row.Title,
			Domain:         row.Domain,
			Applicable:     row.Applicable,
			Justification:  row.Justification,
			Implementation: row.Implementation,
			Responsible:    row.Responsible,
			ManualStatus:   row.ManualStatus,
			EvidenceCount:  int(row.EvidenceCount),
		})
	}
	return out, nil
}

// CountEvidenceByControl returns the number of approved evidence items per control for a framework.
// Result: map[controlUUID]count.
func (r *Repository) CountEvidenceByControl(ctx context.Context, orgID, frameworkID string) (map[string]int, error) {
	rows, err := r.q.CountCKEvidenceByControl(ctx, db.CountCKEvidenceByControlParams{OrgID: orgID, FrameworkID: frameworkID})
	if err != nil {
		return nil, fmt.Errorf("count evidence by control: %w", err)
	}
	counts := make(map[string]int, len(rows))
	for _, row := range rows {
		counts[row.ControlID] = int(row.EvidenceCount)
	}
	return counts, nil
}

// GetExpiringEvidence returns evidence items expiring within the given threshold for a framework.
func (r *Repository) GetExpiringEvidence(ctx context.Context, orgID, frameworkID string, threshold time.Time) ([]Evidence, error) {
	rows, err := r.q.GetCKExpiringEvidence(ctx, db.GetCKExpiringEvidenceParams{
		OrgID:       orgID,
		FrameworkID: frameworkID,
		ExpiresAt:   pgtype.Timestamptz{Time: threshold, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("get expiring evidence: %w", err)
	}
	out := make([]Evidence, 0, len(rows))
	for _, row := range rows {
		out = append(out, evidenceFromFields(evidenceFields{
			ID: row.ID, ControlID: row.ControlID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Source: row.Source, FilePath: row.FilePath,
			FileSize: row.FileSize, Status: row.Status, Version: row.Version,
			ExpiresAt: row.ExpiresAt, ExpiryNotifiedAt: row.ExpiryNotifiedAt,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

// GetExpiringEvidenceAllFrameworks returns evidence expiring within threshold across all frameworks for an org.
func (r *Repository) GetExpiringEvidenceAllFrameworks(ctx context.Context, orgID string, threshold time.Time) ([]Evidence, error) {
	rows, err := r.q.GetCKExpiringEvidenceAllFrameworks(ctx, db.GetCKExpiringEvidenceAllFrameworksParams{
		OrgID:     orgID,
		ExpiresAt: pgtype.Timestamptz{Time: threshold, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("get expiring evidence all frameworks: %w", err)
	}
	out := make([]Evidence, 0, len(rows))
	for _, row := range rows {
		out = append(out, evidenceFromFields(evidenceFields{
			ID: row.ID, ControlID: row.ControlID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Source: row.Source, FilePath: row.FilePath,
			FileSize: row.FileSize, Status: row.Status, Version: row.Version,
			ExpiresAt: row.ExpiresAt, ExpiryNotifiedAt: row.ExpiryNotifiedAt,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

// EvidenceExpiryNotifyRow is a minimal projection used by the expiry notification worker.
type EvidenceExpiryNotifyRow struct {
	ID           string
	OrgID        string
	Title        string
	ControlTitle string
	ExpiresAt    time.Time
}

// GetUnnotifiedExpiringEvidence returns evidence items that expire within the given
// threshold and have not yet had a notification sent (expiry_notified_at IS NULL).
// It joins ck_controls to include the control title in the notification message.
func (r *Repository) GetUnnotifiedExpiringEvidence(ctx context.Context, orgID string, threshold time.Time) ([]EvidenceExpiryNotifyRow, error) {
	rows, err := r.q.GetCKUnnotifiedExpiringEvidence(ctx, db.GetCKUnnotifiedExpiringEvidenceParams{
		OrgID:     orgID,
		ExpiresAt: pgtype.Timestamptz{Time: threshold, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("get unnotified expiring evidence: %w", err)
	}
	out := make([]EvidenceExpiryNotifyRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, EvidenceExpiryNotifyRow{
			ID:           row.ID,
			OrgID:        row.OrgID,
			Title:        row.EvidenceTitle,
			ControlTitle: row.ControlTitle,
			ExpiresAt:    ckTsToTime(row.ExpiresAt),
		})
	}
	return out, nil
}

// MarkEvidenceExpiryNotified sets expiry_notified_at = NOW() for the given evidence IDs.
func (r *Repository) MarkEvidenceExpiryNotified(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := r.q.MarkCKEvidenceExpiryNotified(ctx, ids); err != nil {
		return fmt.Errorf("mark evidence expiry notified: %w", err)
	}
	return nil
}

// --- Evidence ---

// AddEvidence inserts a new evidence item for a control.
func (r *Repository) AddEvidence(ctx context.Context, orgID, controlID, userID string, input AddEvidenceInput) (*Evidence, error) {
	row, err := r.q.AddCKEvidence(ctx, db.AddCKEvidenceParams{
		ControlID:   ckOptUUIDFromStr(controlID),
		OrgID:       orgID,
		Title:       input.Title,
		Description: ckOptText(input.Description),
		Source:      input.Source,
		FilePath:    input.FilePath,
		FileSize:    input.FileSize,
		ExpiresAt:   ckOptTsPtr(input.ExpiresAt),
		UploadedBy:  ckOptUUIDFromStr(userID),
	})
	if err != nil {
		return nil, fmt.Errorf("add evidence: %w", err)
	}
	ev := evidenceFromFields(evidenceFields{
		ID: row.ID, ControlID: row.ControlID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Source: row.Source, FilePath: row.FilePath,
		FileSize: row.FileSize, Status: row.Status, Version: row.Version,
		ExpiresAt: row.ExpiresAt, ExpiryNotifiedAt: row.ExpiryNotifiedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &ev, nil
}

// ListEvidence returns all evidence items for a control within an organisation.
func (r *Repository) ListEvidence(ctx context.Context, orgID, controlID string) ([]Evidence, error) {
	rows, err := r.q.ListCKEvidence(ctx, db.ListCKEvidenceParams{
		ControlID: ckOptUUIDFromStr(controlID),
		OrgID:     orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("list evidence: %w", err)
	}
	out := make([]Evidence, 0, len(rows))
	for _, row := range rows {
		out = append(out, evidenceFromFields(evidenceFields{
			ID: row.ID, ControlID: row.ControlID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Source: row.Source, FilePath: row.FilePath,
			FileSize: row.FileSize, Status: row.Status, Version: row.Version,
			ExpiresAt: row.ExpiresAt, ExpiryNotifiedAt: row.ExpiryNotifiedAt,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

// ListEvidenceByControls fetches all evidence for the given control IDs in a single query.
// Returns a map[controlID][]Evidence. Controls with no evidence are absent from the map.
func (r *Repository) ListEvidenceByControls(ctx context.Context, orgID string, controlIDs []string) (map[string][]Evidence, error) {
	if len(controlIDs) == 0 {
		return map[string][]Evidence{}, nil
	}
	rows, err := r.q.ListCKEvidenceByControls(ctx, db.ListCKEvidenceByControlsParams{
		Column1: controlIDs,
		OrgID:   orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("list evidence by controls: %w", err)
	}
	result := make(map[string][]Evidence, len(controlIDs))
	for _, row := range rows {
		ev := evidenceFromFields(evidenceFields{
			ID: row.ID, ControlID: row.ControlID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Source: row.Source, FilePath: row.FilePath,
			FileSize: row.FileSize, Status: row.Status, Version: row.Version,
			ExpiresAt: row.ExpiresAt, ExpiryNotifiedAt: row.ExpiryNotifiedAt,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
		result[ev.ControlID] = append(result[ev.ControlID], ev)
	}
	return result, nil
}

// GetEvidence returns a single evidence item by ID within an organisation.
func (r *Repository) GetEvidence(ctx context.Context, orgID, evidenceID string) (*Evidence, error) {
	row, err := r.q.GetCKEvidence(ctx, db.GetCKEvidenceParams{ID: evidenceID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get evidence: %w", err)
	}
	ev := evidenceFromFields(evidenceFields{
		ID: row.ID, ControlID: row.ControlID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Source: row.Source, FilePath: row.FilePath,
		FileSize: row.FileSize, Status: row.Status, Version: row.Version,
		ExpiresAt: row.ExpiresAt, ExpiryNotifiedAt: row.ExpiryNotifiedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &ev, nil
}

// ListEvidenceHistory returns the audit history for an evidence item, newest first.
func (r *Repository) ListEvidenceHistory(ctx context.Context, orgID, evidenceID string) ([]EvidenceHistoryEntry, error) {
	rows, err := r.q.ListCKEvidenceHistory(ctx, db.ListCKEvidenceHistoryParams{
		EvidenceID: evidenceID,
		OrgID:      orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("list evidence history: %w", err)
	}
	items := make([]EvidenceHistoryEntry, 0, len(rows))
	for _, row := range rows {
		items = append(items, EvidenceHistoryEntry{
			ID:         row.ID,
			EvidenceID: row.EvidenceID,
			ChangedBy:  uuidPtrFromPgtype(row.ChangedBy),
			ChangedAt:  ckTsToTime(row.ChangedAt),
			Title:      row.Title.String,
			Status:     row.Status.String,
			ChangeNote: row.ChangeNote.String,
		})
	}
	return items, nil
}

// ReviewEvidence updates the status and reviewer information on an evidence item.
func (r *Repository) ReviewEvidence(ctx context.Context, orgID, evidenceID, reviewerID, status string) error {
	n, err := r.q.ReviewCKEvidence(ctx, db.ReviewCKEvidenceParams{
		Status:     status,
		ReviewedBy: ckOptUUIDFromStr(reviewerID),
		ID:         evidenceID,
		OrgID:      orgID,
	})
	if err != nil {
		return fmt.Errorf("review evidence: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("evidence not found")
	}
	return nil
}

// AddCollectorEvidence inserts evidence produced by an automated collector.
func (r *Repository) AddCollectorEvidence(ctx context.Context, orgID, controlID, userID, source, title string, data []byte) (*Evidence, error) {
	row, err := r.q.AddCKCollectorEvidence(ctx, db.AddCKCollectorEvidenceParams{
		ControlID:     ckOptUUIDFromStr(controlID),
		OrgID:         orgID,
		Title:         title,
		Source:        source,
		CollectorData: data,
		UploadedBy:    ckOptUUIDFromStr(userID),
	})
	if err != nil {
		return nil, fmt.Errorf("add collector evidence: %w", err)
	}
	ev := evidenceFromFields(evidenceFields{
		ID: row.ID, ControlID: row.ControlID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Source: row.Source, FilePath: row.FilePath,
		FileSize: row.FileSize, Status: row.Status, Version: row.Version,
		ExpiresAt: row.ExpiresAt, ExpiryNotifiedAt: row.ExpiryNotifiedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &ev, nil
}

// --- Auditor links ---

// CreateAuditorLink inserts a new auditor link record.
func (r *Repository) CreateAuditorLink(ctx context.Context, orgID, frameworkID, userID, tokenHash string, expiresAt time.Time, maxUses *int) (*AuditorLink, error) {
	row, err := r.q.CreateCKAuditorLink(ctx, db.CreateCKAuditorLinkParams{
		OrgID:       orgID,
		FrameworkID: ckOptUUIDFromStr(frameworkID),
		TokenHash:   tokenHash,
		CreatedBy:   userID,
		ExpiresAt:   pgtype.Timestamptz{Time: expiresAt, Valid: true},
		MaxUses:     ckOptIntPtr(maxUses),
	})
	if err != nil {
		return nil, fmt.Errorf("create auditor link: %w", err)
	}
	return &AuditorLink{
		ID:          row.ID,
		OrgID:       row.OrgID,
		FrameworkID: uuidStringFromPgtype(row.FrameworkID),
		CreatedBy:   row.CreatedBy,
		ExpiresAt:   ckTsToTime(row.ExpiresAt),
		UsedCount:   int(row.UsedCount),
		MaxUses:     intPtrFromInt4(row.MaxUses),
		CreatedAt:   ckTsToTime(row.CreatedAt),
	}, nil
}

// uuidStringFromPgtype returns the UUID as string ("" when invalid).
func uuidStringFromPgtype(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return u.String()
}

// GetAuditorLinkByHash looks up an auditor link by its token hash and validates expiry.
// Returns an error if the link has been revoked.
func (r *Repository) GetAuditorLinkByHash(ctx context.Context, tokenHash string) (*AuditorLink, error) {
	row, err := r.q.GetCKAuditorLinkByHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("get auditor link: %w", err)
	}
	if row.RevokedAt.Valid {
		return nil, fmt.Errorf("auditor link revoked")
	}
	return &AuditorLink{
		ID:          row.ID,
		OrgID:       row.OrgID,
		FrameworkID: uuidStringFromPgtype(row.FrameworkID),
		CreatedBy:   row.CreatedBy,
		ExpiresAt:   ckTsToTime(row.ExpiresAt),
		UsedCount:   int(row.UsedCount),
		MaxUses:     intPtrFromInt4(row.MaxUses),
		CreatedAt:   ckTsToTime(row.CreatedAt),
	}, nil
}

// GetAuditorLinkByID returns an auditor link by UUID within an organisation.
func (r *Repository) GetAuditorLinkByID(ctx context.Context, orgID, linkID string) (*AuditorLinkListItem, error) {
	row, err := r.q.GetCKAuditorLinkByID(ctx, db.GetCKAuditorLinkByIDParams{ID: linkID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get auditor link by id: %w", err)
	}
	return &AuditorLinkListItem{
		ID:             row.ID,
		OrgID:          row.OrgID,
		FrameworkID:    uuidStringFromPgtype(row.FrameworkID),
		Label:          row.Label,
		CreatedBy:      row.CreatedBy,
		ExpiresAt:      ckTsToTime(row.ExpiresAt),
		LastAccessedAt: ckTsToTimePtr(row.LastAccessedAt),
		AccessCount:    int(row.AccessCount),
		RevokedAt:      ckTsToTimePtr(row.RevokedAt),
		CreatedAt:      ckTsToTime(row.CreatedAt),
	}, nil
}

// ListAuditorLinks returns all auditor links for an organisation.
func (r *Repository) ListAuditorLinks(ctx context.Context, orgID string) ([]AuditorLinkListItem, error) {
	rows, err := r.q.ListCKAuditorLinks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list auditor links: %w", err)
	}
	out := make([]AuditorLinkListItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, AuditorLinkListItem{
			ID:             row.ID,
			OrgID:          row.OrgID,
			FrameworkID:    uuidStringFromPgtype(row.FrameworkID),
			Label:          row.Label,
			CreatedBy:      row.CreatedBy,
			ExpiresAt:      ckTsToTime(row.ExpiresAt),
			LastAccessedAt: ckTsToTimePtr(row.LastAccessedAt),
			AccessCount:    int(row.AccessCount),
			RevokedAt:      ckTsToTimePtr(row.RevokedAt),
			CreatedAt:      ckTsToTime(row.CreatedAt),
		})
	}
	return out, nil
}

// RevokeAuditorLink sets revoked_at on an auditor link, preventing future access.
func (r *Repository) RevokeAuditorLink(ctx context.Context, orgID, linkID string) error {
	n, err := r.q.RevokeCKAuditorLink(ctx, db.RevokeCKAuditorLinkParams{ID: linkID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("revoke auditor link: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("auditor link not found or already revoked")
	}
	return nil
}

// UpdateAuditorLinkAccess bumps access_count and sets last_accessed_at.
func (r *Repository) UpdateAuditorLinkAccess(ctx context.Context, linkID string) error {
	return r.q.UpdateCKAuditorLinkAccess(ctx, linkID)
}

// IncrementAuditorLinkUsage bumps the used_count on an auditor link.
func (r *Repository) IncrementAuditorLinkUsage(ctx context.Context, linkID string) error {
	return r.q.IncrementCKAuditorLinkUsage(ctx, linkID)
}

// FindControlsByKeywords returns controls whose title or domain matches any of
// the given lowercase keywords. Used by cross-module evidence workers.
func (r *Repository) FindControlsByKeywords(ctx context.Context, orgID string, keywords []string) ([]Control, error) {
	if len(keywords) == 0 {
		return nil, nil
	}
	patterns := make([]string, len(keywords))
	for i, kw := range keywords {
		patterns[i] = "%" + strings.ToLower(kw) + "%"
	}
	rows, err := r.q.FindCKControlsByKeywords(ctx, db.FindCKControlsByKeywordsParams{
		OrgID:    orgID,
		Patterns: patterns,
	})
	if err != nil {
		return nil, fmt.Errorf("find controls by keywords: %w", err)
	}
	controls := make([]Control, 0, len(rows))
	for _, row := range rows {
		controls = append(controls, controlFromFields(controlFields{
			ID: row.ID, FrameworkID: row.FrameworkID, OrgID: row.OrgID,
			ControlID: row.ControlID, Title: row.Title, Description: row.Description,
			Domain: row.Domain, EvidenceType: row.EvidenceType, Weight: row.Weight,
			NotApplicable: row.NotApplicable, NotApplicableReason: row.NotApplicableReason,
			ManualStatus: row.ManualStatus, MaturityScore: row.MaturityScore, Owner: row.Owner,
			LastReviewedAt: row.LastReviewedAt, ReviewIntervalDays: row.ReviewIntervalDays,
			NextReviewDue: row.NextReviewDue, LastReviewedBy: row.LastReviewedBy,
			ReviewNote: row.ReviewNote, DueDate: row.DueDate,
		}))
	}
	return controls, nil
}

// FindPatchControls returns controls whose title or domain mentions patch,
// vulnerability, or update.  Used by the SecPulse auto-evidence worker to
// attach resolved-finding evidence to relevant compliance controls.
func (r *Repository) FindPatchControls(ctx context.Context, orgID string) ([]Control, error) {
	rows, err := r.q.FindCKPatchControls(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("find patch controls: %w", err)
	}
	controls := make([]Control, 0, len(rows))
	for _, row := range rows {
		controls = append(controls, controlFromFields(controlFields{
			ID: row.ID, FrameworkID: row.FrameworkID, OrgID: row.OrgID,
			ControlID: row.ControlID, Title: row.Title, Description: row.Description,
			Domain: row.Domain, EvidenceType: row.EvidenceType, Weight: row.Weight,
			NotApplicable: row.NotApplicable, NotApplicableReason: row.NotApplicableReason,
			ManualStatus: row.ManualStatus, MaturityScore: row.MaturityScore, Owner: row.Owner,
			LastReviewedAt: row.LastReviewedAt, ReviewIntervalDays: row.ReviewIntervalDays,
			NextReviewDue: row.NextReviewDue, LastReviewedBy: row.LastReviewedBy,
			ReviewNote: row.ReviewNote, DueDate: row.DueDate,
		}))
	}
	return controls, nil
}

// --- Risk Assessment (FR-CK12) ---

func (r *Repository) ListRisks(ctx context.Context, orgID string) ([]Risk, error) {
	rows, err := r.q.ListCKRisks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list risks: %w", err)
	}
	out := make([]Risk, 0, len(rows))
	for _, row := range rows {
		out = append(out, riskFromFields(riskFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Category: row.Category,
			Likelihood: row.Likelihood, Impact: row.Impact, RiskScore: row.RiskScore,
			Owner: row.Owner, Status: row.Status, Treatment: row.Treatment,
			TreatmentNotes:  row.TreatmentNotes,
			TreatmentOption: row.TreatmentOption, TreatmentPlan: row.TreatmentPlan,
			TreatmentOwner: row.TreatmentOwner, TreatmentDueDate: row.TreatmentDueDate,
			TreatmentStatus:    row.TreatmentStatus,
			ResidualLikelihood: row.ResidualLikelihood, ResidualImpact: row.ResidualImpact,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

func (r *Repository) GetRisk(ctx context.Context, orgID, id string) (*Risk, error) {
	row, err := r.q.GetCKRisk(ctx, db.GetCKRiskParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get risk: %w", err)
	}
	risk := riskFromFields(riskFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Category: row.Category,
		Likelihood: row.Likelihood, Impact: row.Impact, RiskScore: row.RiskScore,
		Owner: row.Owner, Status: row.Status, Treatment: row.Treatment,
		TreatmentNotes:  row.TreatmentNotes,
		TreatmentOption: row.TreatmentOption, TreatmentPlan: row.TreatmentPlan,
		TreatmentOwner: row.TreatmentOwner, TreatmentDueDate: row.TreatmentDueDate,
		TreatmentStatus:    row.TreatmentStatus,
		ResidualLikelihood: row.ResidualLikelihood, ResidualImpact: row.ResidualImpact,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &risk, nil
}

func (r *Repository) UpdateRisk(ctx context.Context, orgID, id string, in UpdateRiskInput) (*Risk, error) {
	row, err := r.q.UpdateCKRisk(ctx, db.UpdateCKRiskParams{
		ID:             id,
		OrgID:          orgID,
		Title:          in.Title,
		Description:    in.Description,
		Category:       in.Category,
		Likelihood:     int16(in.Likelihood),
		Impact:         int16(in.Impact),
		Owner:          in.Owner,
		Status:         in.Status,
		Treatment:      in.Treatment,
		TreatmentNotes: in.TreatmentNotes,
	})
	if err != nil {
		return nil, fmt.Errorf("update risk: %w", err)
	}
	risk := riskFromFields(riskFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Category: row.Category,
		Likelihood: row.Likelihood, Impact: row.Impact, RiskScore: row.RiskScore,
		Owner: row.Owner, Status: row.Status, Treatment: row.Treatment,
		TreatmentNotes:  row.TreatmentNotes,
		TreatmentOption: row.TreatmentOption, TreatmentPlan: row.TreatmentPlan,
		TreatmentOwner: row.TreatmentOwner, TreatmentDueDate: row.TreatmentDueDate,
		TreatmentStatus:    row.TreatmentStatus,
		ResidualLikelihood: row.ResidualLikelihood, ResidualImpact: row.ResidualImpact,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &risk, nil
}

// UpdateRiskTreatment patches only the treatment workflow fields for a risk.
// Tri-state TreatmentDueDate:
//   - nil      → keep existing (read current value first)
//   - *""      → set NULL
//   - *"date"  → set the parsed date
func (r *Repository) UpdateRiskTreatment(ctx context.Context, orgID, id string, in UpdateRiskTreatmentInput) (*Risk, error) {
	var dueDate pgtype.Date
	if in.TreatmentDueDate == nil {
		// keep current: read first
		cur, err := r.GetRisk(ctx, orgID, id)
		if err != nil {
			return nil, fmt.Errorf("read risk for due_date keep: %w", err)
		}
		if cur.TreatmentDueDate != nil {
			dueDate = pgtype.Date{Time: *cur.TreatmentDueDate, Valid: true}
		}
	} else if *in.TreatmentDueDate != "" {
		t, err := time.Parse("2006-01-02", *in.TreatmentDueDate)
		if err != nil {
			return nil, fmt.Errorf("parse treatment_due_date: %w", err)
		}
		dueDate = pgtype.Date{Time: t, Valid: true}
	}
	row, err := r.q.UpdateCKRiskTreatment(ctx, db.UpdateCKRiskTreatmentParams{
		ID:                 id,
		OrgID:              orgID,
		TreatmentOption:    ckOptText(in.TreatmentOption),
		TreatmentPlan:      in.TreatmentPlan,
		TreatmentOwner:     in.TreatmentOwner,
		TreatmentStatus:    in.TreatmentStatus,
		ResidualLikelihood: ckOptIntPtr(in.ResidualLikelihood),
		ResidualImpact:     ckOptIntPtr(in.ResidualImpact),
		TreatmentDueDate:   dueDate,
	})
	if err != nil {
		return nil, fmt.Errorf("update risk treatment: %w", err)
	}
	risk := riskFromFields(riskFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Category: row.Category,
		Likelihood: row.Likelihood, Impact: row.Impact, RiskScore: row.RiskScore,
		Owner: row.Owner, Status: row.Status, Treatment: row.Treatment,
		TreatmentNotes:  row.TreatmentNotes,
		TreatmentOption: row.TreatmentOption, TreatmentPlan: row.TreatmentPlan,
		TreatmentOwner: row.TreatmentOwner, TreatmentDueDate: row.TreatmentDueDate,
		TreatmentStatus:    row.TreatmentStatus,
		ResidualLikelihood: row.ResidualLikelihood, ResidualImpact: row.ResidualImpact,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &risk, nil
}

func (r *Repository) CreateRisk(ctx context.Context, orgID string, in CreateRiskInput) (*Risk, error) {
	row, err := r.q.CreateCKRisk(ctx, db.CreateCKRiskParams{
		OrgID:          orgID,
		Title:          in.Title,
		Description:    in.Description,
		Category:       in.Category,
		Likelihood:     int16(in.Likelihood),
		Impact:         int16(in.Impact),
		Owner:          in.Owner,
		Treatment:      in.Treatment,
		TreatmentNotes: in.TreatmentNotes,
	})
	if err != nil {
		return nil, fmt.Errorf("create risk: %w", err)
	}
	risk := riskFromFields(riskFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Category: row.Category,
		Likelihood: row.Likelihood, Impact: row.Impact, RiskScore: row.RiskScore,
		Owner: row.Owner, Status: row.Status, Treatment: row.Treatment,
		TreatmentNotes:  row.TreatmentNotes,
		TreatmentOption: row.TreatmentOption, TreatmentPlan: row.TreatmentPlan,
		TreatmentOwner: row.TreatmentOwner, TreatmentDueDate: row.TreatmentDueDate,
		TreatmentStatus:    row.TreatmentStatus,
		ResidualLikelihood: row.ResidualLikelihood, ResidualImpact: row.ResidualImpact,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &risk, nil
}

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

// optTextStrPtr converts *string to pgtype.Text (nil → invalid, *"" → valid empty).
func optTextStrPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
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

// ckOptUUIDFromPtr converts *string to pgtype.UUID; nil/empty → invalid.
func ckOptUUIDFromPtr(s *string) pgtype.UUID {
	if s == nil || *s == "" {
		return pgtype.UUID{}
	}
	return ckOptUUIDFromStr(*s)
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

// --- Supplier Register (NIS2 Art. 21 / DORA Art. 28) ---

// supplierFields holds the shared columns of every Supplier-returning sqlc Row.
// All Row-Types (Create/Get/List/Update) haben identische Shape.
type supplierFields struct {
	ID, OrgID, Name                        string
	ContactName, ContactEmail, ServiceType pgtype.Text
	Criticality                            string
	Nis2Relevant, DoraRelevant             bool
	ContractEnd                            pgtype.Date
	Notes                                  pgtype.Text
	SubSuppliers                           []string
	DataLocation                           pgtype.Text
	ExitStrategyExists                     bool
	AssessmentStatus                       string
	LastAssessmentAt                       pgtype.Timestamptz
	CreatedAt, UpdatedAt                   pgtype.Timestamptz
}

func supplierFromFields(f supplierFields) Supplier {
	return Supplier{
		ID:                 f.ID,
		OrgID:              f.OrgID,
		Name:               f.Name,
		ContactName:        f.ContactName.String,
		ContactEmail:       f.ContactEmail.String,
		ServiceType:        f.ServiceType.String,
		Criticality:        f.Criticality,
		NIS2Relevant:       f.Nis2Relevant,
		DORARelevant:       f.DoraRelevant,
		ContractEnd:        ckDateToTimePtr(f.ContractEnd),
		Notes:              f.Notes.String,
		SubSuppliers:       f.SubSuppliers,
		DataLocation:       f.DataLocation.String,
		ExitStrategyExists: f.ExitStrategyExists,
		AssessmentStatus:   f.AssessmentStatus,
		LastAssessmentAt:   ckTsToTimePtr(f.LastAssessmentAt),
		CreatedAt:          ckTsToTime(f.CreatedAt),
		UpdatedAt:          ckTsToTime(f.UpdatedAt),
	}
}

func (r *Repository) ListSuppliers(ctx context.Context, orgID string, filter *SupplierFilter) ([]Supplier, error) {
	params := db.ListCKSuppliersParams{OrgID: orgID}
	if filter != nil {
		params.Criticality = ckOptText(filter.Criticality)
		params.AssessmentStatus = ckOptText(filter.AssessmentStatus)
	}
	rows, err := r.q.ListCKSuppliers(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list suppliers: %w", err)
	}
	out := make([]Supplier, 0, len(rows))
	for _, row := range rows {
		out = append(out, supplierFromFields(supplierFields{
			ID: row.ID, OrgID: row.OrgID, Name: row.Name,
			ContactName: row.ContactName, ContactEmail: row.ContactEmail,
			ServiceType: row.ServiceType, Criticality: row.Criticality,
			Nis2Relevant: row.Nis2Relevant, DoraRelevant: row.DoraRelevant,
			ContractEnd: row.ContractEnd, Notes: row.Notes,
			SubSuppliers: row.SubSuppliers, DataLocation: row.DataLocation,
			ExitStrategyExists: row.ExitStrategyExists,
			AssessmentStatus:   row.AssessmentStatus,
			LastAssessmentAt:   row.LastAssessmentAt,
			CreatedAt:          row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

func (r *Repository) GetSupplier(ctx context.Context, orgID, id string) (*Supplier, error) {
	row, err := r.q.GetCKSupplier(ctx, db.GetCKSupplierParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get supplier: %w", err)
	}
	s := supplierFromFields(supplierFields{
		ID: row.ID, OrgID: row.OrgID, Name: row.Name,
		ContactName: row.ContactName, ContactEmail: row.ContactEmail,
		ServiceType: row.ServiceType, Criticality: row.Criticality,
		Nis2Relevant: row.Nis2Relevant, DoraRelevant: row.DoraRelevant,
		ContractEnd: row.ContractEnd, Notes: row.Notes,
		SubSuppliers: row.SubSuppliers, DataLocation: row.DataLocation,
		ExitStrategyExists: row.ExitStrategyExists,
		AssessmentStatus:   row.AssessmentStatus,
		LastAssessmentAt:   row.LastAssessmentAt,
		CreatedAt:          row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &s, nil
}

func (r *Repository) CreateSupplier(ctx context.Context, orgID string, in CreateSupplierInput) (*Supplier, error) {
	crit := in.Criticality
	if crit == "" {
		crit = "standard"
	}
	subSuppliers := in.SubSuppliers
	if subSuppliers == nil {
		subSuppliers = []string{}
	}
	assessmentStatus := in.AssessmentStatus
	if assessmentStatus == "" {
		assessmentStatus = "none"
	}
	row, err := r.q.CreateCKSupplier(ctx, db.CreateCKSupplierParams{
		OrgID:              orgID,
		Name:               in.Name,
		ContactName:        in.ContactName,
		ContactEmail:       in.ContactEmail,
		ServiceType:        in.ServiceType,
		Criticality:        crit,
		Nis2Relevant:       in.NIS2Relevant,
		DoraRelevant:       in.DORARelevant,
		ContractEnd:        policyDateFromTimePtr(in.ContractEnd),
		Notes:              in.Notes,
		SubSuppliers:       subSuppliers,
		DataLocation:       in.DataLocation,
		ExitStrategyExists: in.ExitStrategyExists,
		AssessmentStatus:   assessmentStatus,
		LastAssessmentAt:   ckOptTsPtr(in.LastAssessmentAt),
	})
	if err != nil {
		return nil, fmt.Errorf("create supplier: %w", err)
	}
	s := supplierFromFields(supplierFields{
		ID: row.ID, OrgID: row.OrgID, Name: row.Name,
		ContactName: row.ContactName, ContactEmail: row.ContactEmail,
		ServiceType: row.ServiceType, Criticality: row.Criticality,
		Nis2Relevant: row.Nis2Relevant, DoraRelevant: row.DoraRelevant,
		ContractEnd: row.ContractEnd, Notes: row.Notes,
		SubSuppliers: row.SubSuppliers, DataLocation: row.DataLocation,
		ExitStrategyExists: row.ExitStrategyExists,
		AssessmentStatus:   row.AssessmentStatus,
		LastAssessmentAt:   row.LastAssessmentAt,
		CreatedAt:          row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &s, nil
}

func (r *Repository) UpdateSupplier(ctx context.Context, orgID, id string, in UpdateSupplierInput) (*Supplier, error) {
	crit := in.Criticality
	if crit == "" {
		crit = "standard"
	}
	subSuppliers := in.SubSuppliers
	if subSuppliers == nil {
		subSuppliers = []string{}
	}
	assessmentStatus := in.AssessmentStatus
	if assessmentStatus == "" {
		assessmentStatus = "none"
	}
	row, err := r.q.UpdateCKSupplier(ctx, db.UpdateCKSupplierParams{
		ID:                 id,
		OrgID:              orgID,
		Name:               in.Name,
		ContactName:        in.ContactName,
		ContactEmail:       in.ContactEmail,
		ServiceType:        in.ServiceType,
		Criticality:        crit,
		Nis2Relevant:       in.NIS2Relevant,
		DoraRelevant:       in.DORARelevant,
		ContractEnd:        policyDateFromTimePtr(in.ContractEnd),
		Notes:              in.Notes,
		SubSuppliers:       subSuppliers,
		DataLocation:       in.DataLocation,
		ExitStrategyExists: in.ExitStrategyExists,
		AssessmentStatus:   assessmentStatus,
		LastAssessmentAt:   ckOptTsPtr(in.LastAssessmentAt),
	})
	if err != nil {
		return nil, fmt.Errorf("update supplier: %w", err)
	}
	s := supplierFromFields(supplierFields{
		ID: row.ID, OrgID: row.OrgID, Name: row.Name,
		ContactName: row.ContactName, ContactEmail: row.ContactEmail,
		ServiceType: row.ServiceType, Criticality: row.Criticality,
		Nis2Relevant: row.Nis2Relevant, DoraRelevant: row.DoraRelevant,
		ContractEnd: row.ContractEnd, Notes: row.Notes,
		SubSuppliers: row.SubSuppliers, DataLocation: row.DataLocation,
		ExitStrategyExists: row.ExitStrategyExists,
		AssessmentStatus:   row.AssessmentStatus,
		LastAssessmentAt:   row.LastAssessmentAt,
		CreatedAt:          row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &s, nil
}

func (r *Repository) DeleteSupplier(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteCKSupplier(ctx, db.DeleteCKSupplierParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete supplier: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("supplier not found")
	}
	return nil
}

// supplierExists ensures the supplier belongs to the org; reused by link/unlink/list.
func (r *Repository) supplierExists(ctx context.Context, supplierID, orgID string) error {
	exists, err := r.q.CKSupplierExists(ctx, db.CKSupplierExistsParams{ID: supplierID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("verify supplier: %w", err)
	}
	if !exists {
		return fmt.Errorf("supplier not found")
	}
	return nil
}

// LinkSupplierRisk links a risk to a supplier. Idempotent (ON CONFLICT DO NOTHING).
func (r *Repository) LinkSupplierRisk(ctx context.Context, orgID, supplierID, riskID string) error {
	if err := r.supplierExists(ctx, supplierID, orgID); err != nil {
		return err
	}
	if err := r.q.LinkCKSupplierRisk(ctx, db.LinkCKSupplierRiskParams{SupplierID: supplierID, RiskID: riskID}); err != nil {
		return fmt.Errorf("link supplier risk: %w", err)
	}
	return nil
}

// UnlinkSupplierRisk removes a risk link from a supplier.
func (r *Repository) UnlinkSupplierRisk(ctx context.Context, orgID, supplierID, riskID string) error {
	if err := r.supplierExists(ctx, supplierID, orgID); err != nil {
		return err
	}
	if err := r.q.UnlinkCKSupplierRisk(ctx, db.UnlinkCKSupplierRiskParams{SupplierID: supplierID, RiskID: riskID}); err != nil {
		return fmt.Errorf("unlink supplier risk: %w", err)
	}
	return nil
}

// ListSupplierRisks returns all risks linked to the given supplier.
func (r *Repository) ListSupplierRisks(ctx context.Context, orgID, supplierID string) ([]Risk, error) {
	if err := r.supplierExists(ctx, supplierID, orgID); err != nil {
		return nil, err
	}
	rows, err := r.q.ListCKSupplierRisks(ctx, db.ListCKSupplierRisksParams{SupplierID: supplierID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("list supplier risks: %w", err)
	}
	out := make([]Risk, 0, len(rows))
	for _, row := range rows {
		out = append(out, riskFromFields(riskFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Category: row.Category,
			Likelihood: row.Likelihood, Impact: row.Impact, RiskScore: row.RiskScore,
			Owner: row.Owner, Status: row.Status, Treatment: row.Treatment,
			TreatmentNotes:  row.TreatmentNotes,
			TreatmentOption: row.TreatmentOption, TreatmentPlan: row.TreatmentPlan,
			TreatmentOwner: row.TreatmentOwner, TreatmentDueDate: row.TreatmentDueDate,
			TreatmentStatus:    row.TreatmentStatus,
			ResidualLikelihood: row.ResidualLikelihood, ResidualImpact: row.ResidualImpact,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
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

// --- AI System Inventory (EU AI Act) ---

// aiSystemFromCkAiSystems maps the sqlc Table-Row to the AISystem domain.
// All Row-types (Create/Get/List/Update) share the identical shape — one mapper.
func aiSystemFromCkAiSystems(r db.CkAiSystems) AISystem {
	return AISystem{
		ID:                      r.ID,
		OrgID:                   r.OrgID,
		Name:                    r.Name,
		Description:             r.Description.String,
		Provider:                r.Provider.String,
		UseCase:                 r.UseCase.String,
		AffectedGroups:          r.AffectedGroups.String,
		AutonomyLevel:           r.AutonomyLevel,
		InProductionSince:       ckDateToTimePtr(r.InProductionSince),
		Status:                  r.Status,
		RiskClass:               r.RiskClass.String,
		ClassificationRationale: r.ClassificationRationale.String,
		ClassifiedAt:            ckTsToTimePtr(r.ClassifiedAt),
		ClassifiedBy:            r.ClassifiedBy.String,
		CreatedAt:               ckTsToTime(r.CreatedAt),
		UpdatedAt:               ckTsToTime(r.UpdatedAt),
	}
}

func (r *Repository) ListAISystems(ctx context.Context, orgID string, filters AISystemFilters) ([]AISystem, error) {
	rows, err := r.q.ListCKAISystems(ctx, db.ListCKAISystemsParams{
		OrgID:     orgID,
		RiskClass: ckOptText(filters.RiskClass),
		Status:    ckOptText(filters.Status),
	})
	if err != nil {
		return nil, fmt.Errorf("list ai systems: %w", err)
	}
	out := make([]AISystem, 0, len(rows))
	for _, row := range rows {
		out = append(out, aiSystemFromCkAiSystems(db.CkAiSystems(row)))
	}
	return out, nil
}

func (r *Repository) DeleteAISystem(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteCKAISystem(ctx, db.DeleteCKAISystemParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete ai system: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) GetAISystem(ctx context.Context, orgID, id string) (*AISystem, error) {
	row, err := r.q.GetCKAISystem(ctx, db.GetCKAISystemParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get ai system: %w", err)
	}
	a := aiSystemFromCkAiSystems(db.CkAiSystems(row))
	return &a, nil
}

func (r *Repository) CreateAISystem(ctx context.Context, orgID string, in CreateAISystemInput) (*AISystem, error) {
	al := in.AutonomyLevel
	if al == "" {
		al = "assistive"
	}
	row, err := r.q.CreateCKAISystem(ctx, db.CreateCKAISystemParams{
		OrgID:                   orgID,
		Name:                    in.Name,
		Description:             in.Description,
		Provider:                in.Provider,
		UseCase:                 in.UseCase,
		AffectedGroups:          in.AffectedGroups,
		AutonomyLevel:           al,
		InProductionSince:       policyDateFromTimePtr(in.InProductionSince),
		RiskClass:               in.RiskClass,
		ClassificationRationale: in.ClassificationRationale,
	})
	if err != nil {
		return nil, fmt.Errorf("create ai system: %w", err)
	}
	a := aiSystemFromCkAiSystems(db.CkAiSystems(row))
	return &a, nil
}

func (r *Repository) UpdateAISystem(ctx context.Context, orgID, id string, in UpdateAISystemInput) (*AISystem, error) {
	al := in.AutonomyLevel
	if al == "" {
		al = "assistive"
	}
	st := in.Status
	if st == "" {
		st = "under_review"
	}
	var classifiedAt pgtype.Timestamptz
	if in.ClassifiedBy != "" && in.RiskClass != "" {
		classifiedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}
	row, err := r.q.UpdateCKAISystem(ctx, db.UpdateCKAISystemParams{
		ID:                      id,
		OrgID:                   orgID,
		Name:                    in.Name,
		Description:             in.Description,
		Provider:                in.Provider,
		UseCase:                 in.UseCase,
		AffectedGroups:          in.AffectedGroups,
		AutonomyLevel:           al,
		InProductionSince:       policyDateFromTimePtr(in.InProductionSince),
		Status:                  st,
		RiskClass:               in.RiskClass,
		ClassificationRationale: in.ClassificationRationale,
		ClassifiedAt:            classifiedAt,
		ClassifiedBy:            in.ClassifiedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("update ai system: %w", err)
	}
	a := aiSystemFromCkAiSystems(db.CkAiSystems(row))
	return &a, nil
}

// --- Policy Management (FR-CK14) ---

func (r *Repository) ListPolicies(ctx context.Context, orgID string) ([]Policy, error) {
	rows, err := r.q.ListCKPolicies(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list policies: %w", err)
	}
	out := make([]Policy, 0, len(rows))
	for _, row := range rows {
		out = append(out, policyFromFields(policyFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Category: row.Category,
			Status: row.Status, Version: row.Version,
			EffectiveDate: row.EffectiveDate, ReviewDate: row.ReviewDate,
			Owner: row.Owner, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			VersionNum: row.VersionNum, VersionNote: row.VersionNote,
			LastUpdatedBy: row.LastUpdatedBy,
			ReviewedAt:    row.ReviewedAt, NextReviewDue: row.NextReviewDue,
		}))
	}
	return out, nil
}

func (r *Repository) GetPolicy(ctx context.Context, orgID, id string) (*Policy, error) {
	row, err := r.q.GetCKPolicy(ctx, db.GetCKPolicyParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get policy: %w", err)
	}
	p := policyFromFields(policyFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Category: row.Category,
		Status: row.Status, Version: row.Version,
		EffectiveDate: row.EffectiveDate, ReviewDate: row.ReviewDate,
		Owner: row.Owner, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		VersionNum: row.VersionNum, VersionNote: row.VersionNote,
		LastUpdatedBy: row.LastUpdatedBy,
		ReviewedAt:    row.ReviewedAt, NextReviewDue: row.NextReviewDue,
	})
	return &p, nil
}

// UpdatePolicy snapshots the current policy version into ck_policy_versions, then increments
// version_num and applies the update fields. All steps run in a single transaction.
func (r *Repository) UpdatePolicy(ctx context.Context, orgID, id string, in UpdatePolicyInput) (*Policy, error) {
	versionLabel := in.Version
	if versionLabel == "" {
		versionLabel = "1.0"
	}
	versionNote := ""
	if in.VersionNote != nil {
		versionNote = *in.VersionNote
	}
	updatedBy := ""
	if in.UpdatedBy != nil {
		updatedBy = *in.UpdatedBy
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op when Commit succeeded
	qtx := r.q.WithTx(tx)

	// Snapshot the current state into ck_policy_versions before updating.
	if err := qtx.SnapshotCKPolicyVersion(ctx, db.SnapshotCKPolicyVersionParams{ID: id, OrgID: orgID}); err != nil {
		return nil, fmt.Errorf("snapshot policy version: %w", err)
	}

	row, err := qtx.UpdateCKPolicy(ctx, db.UpdateCKPolicyParams{
		ID:            id,
		OrgID:         orgID,
		Title:         in.Title,
		Description:   in.Description,
		Category:      in.Category,
		Status:        in.Status,
		Version:       versionLabel,
		EffectiveDate: policyDateFromTimePtr(in.EffectiveDate),
		ReviewDate:    policyDateFromTimePtr(in.ReviewDate),
		Owner:         in.Owner,
		VersionNote:   versionNote,
		LastUpdatedBy: updatedBy,
		RefreshReview: updatedBy != "",
		NextReviewDue: ckOptDatePtr(in.NextReviewDue),
	})
	if err != nil {
		return nil, fmt.Errorf("update policy: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit policy update: %w", err)
	}
	p := policyFromFields(policyFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Category: row.Category,
		Status: row.Status, Version: row.Version,
		EffectiveDate: row.EffectiveDate, ReviewDate: row.ReviewDate,
		Owner: row.Owner, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		VersionNum: row.VersionNum, VersionNote: row.VersionNote,
		LastUpdatedBy: row.LastUpdatedBy,
		ReviewedAt:    row.ReviewedAt, NextReviewDue: row.NextReviewDue,
	})
	return &p, nil
}

// policyDateFromTimePtr converts *time.Time → pgtype.Date.
func policyDateFromTimePtr(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

func (r *Repository) CreatePolicy(ctx context.Context, orgID string, in CreatePolicyInput) (*Policy, error) {
	version := in.Version
	if version == "" {
		version = "1.0"
	}
	row, err := r.q.CreateCKPolicy(ctx, db.CreateCKPolicyParams{
		OrgID:         orgID,
		Title:         in.Title,
		Description:   in.Description,
		Category:      in.Category,
		Version:       version,
		EffectiveDate: policyDateFromTimePtr(in.EffectiveDate),
		ReviewDate:    policyDateFromTimePtr(in.ReviewDate),
		Owner:         in.Owner,
	})
	if err != nil {
		return nil, fmt.Errorf("create policy: %w", err)
	}
	p := policyFromFields(policyFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Category: row.Category,
		Status: row.Status, Version: row.Version,
		EffectiveDate: row.EffectiveDate, ReviewDate: row.ReviewDate,
		Owner: row.Owner, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		VersionNum: row.VersionNum, VersionNote: row.VersionNote,
		LastUpdatedBy: row.LastUpdatedBy,
		ReviewedAt:    row.ReviewedAt, NextReviewDue: row.NextReviewDue,
	})
	return &p, nil
}

// ListPolicyVersions returns all historical version snapshots for a policy, newest first.
func (r *Repository) ListPolicyVersions(ctx context.Context, orgID, policyID string) ([]PolicyVersion, error) {
	rows, err := r.q.ListCKPolicyVersions(ctx, db.ListCKPolicyVersionsParams{PolicyID: policyID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("list policy versions: %w", err)
	}
	versions := make([]PolicyVersion, 0, len(rows))
	for _, row := range rows {
		versions = append(versions, PolicyVersion{
			ID: row.ID, OrgID: row.OrgID, PolicyID: row.PolicyID, Version: int(row.Version),
			Title: row.Title, Content: row.Content, Status: row.Status,
			VersionNote: row.VersionNote, UpdatedBy: row.UpdatedBy,
			CreatedAt: ckTsToTime(row.CreatedAt),
		})
	}
	return versions, nil
}

// GetPolicyVersion returns a single historical version snapshot.
func (r *Repository) GetPolicyVersion(ctx context.Context, orgID, policyID string, version int) (PolicyVersion, error) {
	row, err := r.q.GetCKPolicyVersion(ctx, db.GetCKPolicyVersionParams{
		PolicyID: policyID, OrgID: orgID, Version: int32(version),
	})
	if err != nil {
		return PolicyVersion{}, fmt.Errorf("get policy version: %w", err)
	}
	return PolicyVersion{
		ID: row.ID, OrgID: row.OrgID, PolicyID: row.PolicyID, Version: int(row.Version),
		Title: row.Title, Content: row.Content, Status: row.Status,
		VersionNote: row.VersionNote, UpdatedBy: row.UpdatedBy,
		CreatedAt: ckTsToTime(row.CreatedAt),
	}, nil
}

// --- Internal Audit Records (FR-CK15) ---

// auditRecordFields is shared between Create/Get/List/Update Row types.
type auditRecordFields struct {
	ID, OrgID, Title, Scope, Auditor, Status, Findings, Recommendations string
	AuditDate                                                           pgtype.Date
	CreatedAt, UpdatedAt                                                pgtype.Timestamptz
}

func auditRecordFromFields(f auditRecordFields) AuditRecord {
	rec := AuditRecord{
		ID:              f.ID,
		OrgID:           f.OrgID,
		Title:           f.Title,
		Scope:           f.Scope,
		Auditor:         f.Auditor,
		Status:          f.Status,
		Findings:        f.Findings,
		Recommendations: f.Recommendations,
		CreatedAt:       ckTsToTime(f.CreatedAt),
		UpdatedAt:       ckTsToTime(f.UpdatedAt),
	}
	if f.AuditDate.Valid {
		rec.AuditDate = f.AuditDate.Time
	}
	return rec
}

func (r *Repository) ListAuditRecords(ctx context.Context, orgID string) ([]AuditRecord, error) {
	rows, err := r.q.ListCKAuditRecords(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list audit records: %w", err)
	}
	out := make([]AuditRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, auditRecordFromFields(auditRecordFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title, Scope: row.Scope,
			Auditor: row.Auditor, AuditDate: row.AuditDate, Status: row.Status,
			Findings: row.Findings, Recommendations: row.Recommendations,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

func (r *Repository) GetAuditRecord(ctx context.Context, orgID, id string) (*AuditRecord, error) {
	row, err := r.q.GetCKAuditRecord(ctx, db.GetCKAuditRecordParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get audit record: %w", err)
	}
	rec := auditRecordFromFields(auditRecordFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title, Scope: row.Scope,
		Auditor: row.Auditor, AuditDate: row.AuditDate, Status: row.Status,
		Findings: row.Findings, Recommendations: row.Recommendations,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &rec, nil
}

func (r *Repository) UpdateAuditRecord(ctx context.Context, orgID, id string, in UpdateAuditRecordInput) (*AuditRecord, error) {
	row, err := r.q.UpdateCKAuditRecord(ctx, db.UpdateCKAuditRecordParams{
		ID:              id,
		OrgID:           orgID,
		Title:           in.Title,
		Scope:           in.Scope,
		Auditor:         in.Auditor,
		AuditDate:       pgtype.Date{Time: in.AuditDate, Valid: true},
		Status:          in.Status,
		Findings:        in.Findings,
		Recommendations: in.Recommendations,
	})
	if err != nil {
		return nil, fmt.Errorf("update audit record: %w", err)
	}
	rec := auditRecordFromFields(auditRecordFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title, Scope: row.Scope,
		Auditor: row.Auditor, AuditDate: row.AuditDate, Status: row.Status,
		Findings: row.Findings, Recommendations: row.Recommendations,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &rec, nil
}

func (r *Repository) CreateAuditRecord(ctx context.Context, orgID string, in CreateAuditRecordInput) (*AuditRecord, error) {
	row, err := r.q.CreateCKAuditRecord(ctx, db.CreateCKAuditRecordParams{
		OrgID:           orgID,
		Title:           in.Title,
		Scope:           in.Scope,
		Auditor:         in.Auditor,
		AuditDate:       pgtype.Date{Time: in.AuditDate, Valid: true},
		Findings:        in.Findings,
		Recommendations: in.Recommendations,
	})
	if err != nil {
		return nil, fmt.Errorf("create audit record: %w", err)
	}
	rec := auditRecordFromFields(auditRecordFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title, Scope: row.Scope,
		Auditor: row.Auditor, AuditDate: row.AuditDate, Status: row.Status,
		Findings: row.Findings, Recommendations: row.Recommendations,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &rec, nil
}

// --- Control Tasks ---

// controlTaskFromCkControlTasks maps the sqlc row to the domain ControlTask.
func controlTaskFromCkControlTasks(r db.CkControlTasks) ControlTask {
	return ControlTask{
		ID:        r.ID,
		ControlID: r.ControlID,
		OrgID:     r.OrgID,
		Text:      r.Text,
		Completed: r.Completed,
		CreatedAt: ckTsToTime(r.CreatedAt),
		UpdatedAt: ckTsToTime(r.UpdatedAt),
	}
}

func (r *Repository) ListControlTasks(ctx context.Context, orgID, controlID string) ([]ControlTask, error) {
	rows, err := r.q.ListCKControlTasks(ctx, db.ListCKControlTasksParams{ControlID: controlID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("list control tasks: %w", err)
	}
	out := make([]ControlTask, 0, len(rows))
	for _, row := range rows {
		out = append(out, controlTaskFromCkControlTasks(row))
	}
	return out, nil
}

func (r *Repository) CreateControlTask(ctx context.Context, orgID, controlID string, in CreateControlTaskInput) (*ControlTask, error) {
	row, err := r.q.CreateCKControlTask(ctx, db.CreateCKControlTaskParams{
		ControlID: controlID,
		OrgID:     orgID,
		Text:      in.Text,
	})
	if err != nil {
		return nil, fmt.Errorf("create control task: %w", err)
	}
	t := controlTaskFromCkControlTasks(row)
	return &t, nil
}

func (r *Repository) UpdateControlTask(ctx context.Context, orgID, controlID, taskID string, in UpdateControlTaskInput) (*ControlTask, error) {
	row, err := r.q.UpdateCKControlTask(ctx, db.UpdateCKControlTaskParams{
		Completed: in.Completed,
		ID:        taskID,
		ControlID: controlID,
		OrgID:     orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("update control task: %w", err)
	}
	t := controlTaskFromCkControlTasks(row)
	return &t, nil
}

func (r *Repository) DeleteControlTask(ctx context.Context, orgID, controlID, taskID string) error {
	n, err := r.q.DeleteCKControlTask(ctx, db.DeleteCKControlTaskParams{
		ID:        taskID,
		ControlID: controlID,
		OrgID:     orgID,
	})
	if err != nil {
		return fmt.Errorf("delete control task: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("task not found")
	}
	return nil
}

// --- Risk ↔ Control Links ---

// LinkRiskControl creates a link between a risk and a control within an organisation.
func (r *Repository) LinkRiskControl(ctx context.Context, orgID, riskID, controlID string) error {
	if err := r.q.LinkCKRiskControl(ctx, db.LinkCKRiskControlParams{
		RiskID: riskID, ControlID: controlID, OrgID: orgID,
	}); err != nil {
		return fmt.Errorf("link risk control: %w", err)
	}
	return nil
}

// UnlinkRiskControl removes the link between a risk and a control within an organisation.
func (r *Repository) UnlinkRiskControl(ctx context.Context, orgID, riskID, controlID string) error {
	n, err := r.q.UnlinkCKRiskControl(ctx, db.UnlinkCKRiskControlParams{
		RiskID: riskID, ControlID: controlID, OrgID: orgID,
	})
	if err != nil {
		return fmt.Errorf("unlink risk control: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("link not found")
	}
	return nil
}

// --- Resilience Tests (DORA Art. 24-27) ---

func resilienceTestFromCkResilienceTests(r db.CkResilienceTests) ResilienceTest {
	t := ResilienceTest{
		ID:                r.ID,
		OrgID:             r.OrgID,
		Type:              r.Type,
		Scope:             r.Scope.String,
		Provider:          r.Provider.String,
		Summary:           r.Summary.String,
		RemediationStatus: r.RemediationStatus,
		AttachmentURL:     r.AttachmentUrl.String,
		CreatedAt:         ckTsToTime(r.CreatedAt),
		UpdatedAt:         ckTsToTime(r.UpdatedAt),
	}
	if r.TestDate.Valid {
		t.TestDate = r.TestDate.Time
	}
	return t
}

// ListResilienceTests returns all resilience tests for an organisation, sorted by test_date DESC.
func (r *Repository) ListResilienceTests(ctx context.Context, orgID string) ([]ResilienceTest, error) {
	rows, err := r.q.ListCKResilienceTests(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list resilience tests: %w", err)
	}
	out := make([]ResilienceTest, 0, len(rows))
	for _, row := range rows {
		out = append(out, resilienceTestFromCkResilienceTests(row))
	}
	return out, nil
}

// GetResilienceTest returns a single resilience test by ID within an organisation.
// Returns an error containing "not found" if the test does not exist.
func (r *Repository) GetResilienceTest(ctx context.Context, orgID, id string) (*ResilienceTest, error) {
	row, err := r.q.GetCKResilienceTest(ctx, db.GetCKResilienceTestParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("resilience test not found: %w", err)
	}
	t := resilienceTestFromCkResilienceTests(row)
	return &t, nil
}

// CreateResilienceTest inserts a new resilience test entry and returns it.
func (r *Repository) CreateResilienceTest(ctx context.Context, orgID string, in CreateResilienceTestInput) (*ResilienceTest, error) {
	remStatus := in.RemediationStatus
	if remStatus == "" {
		remStatus = "open"
	}
	row, err := r.q.CreateCKResilienceTest(ctx, db.CreateCKResilienceTestParams{
		OrgID:             orgID,
		Type:              in.Type,
		Scope:             in.Scope,
		Provider:          in.Provider,
		TestDate:          pgtype.Date{Time: in.TestDate, Valid: true},
		Summary:           in.Summary,
		RemediationStatus: remStatus,
	})
	if err != nil {
		return nil, fmt.Errorf("create resilience test: %w", err)
	}
	t := resilienceTestFromCkResilienceTests(row)
	return &t, nil
}

// UpdateResilienceTest updates an existing resilience test entry and returns it.
func (r *Repository) UpdateResilienceTest(ctx context.Context, orgID, id string, in UpdateResilienceTestInput) (*ResilienceTest, error) {
	row, err := r.q.UpdateCKResilienceTest(ctx, db.UpdateCKResilienceTestParams{
		ID:                id,
		OrgID:             orgID,
		Type:              in.Type,
		Scope:             in.Scope,
		Provider:          in.Provider,
		TestDate:          pgtype.Date{Time: in.TestDate, Valid: true},
		Summary:           in.Summary,
		RemediationStatus: in.RemediationStatus,
	})
	if err != nil {
		return nil, fmt.Errorf("update resilience test: %w", err)
	}
	t := resilienceTestFromCkResilienceTests(row)
	return &t, nil
}

// DeleteResilienceTest removes a resilience test entry.
func (r *Repository) DeleteResilienceTest(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteCKResilienceTest(ctx, db.DeleteCKResilienceTestParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete resilience test: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("resilience test not found")
	}
	return nil
}

// UpdateResilienceTestAttachment sets the attachment_url on a resilience test entry.
func (r *Repository) UpdateResilienceTestAttachment(ctx context.Context, orgID, id, url string) error {
	n, err := r.q.UpdateCKResilienceTestAttachment(ctx, db.UpdateCKResilienceTestAttachmentParams{
		ID:            id,
		OrgID:         orgID,
		AttachmentUrl: ckOptText(url),
	})
	if err != nil {
		return fmt.Errorf("update resilience test attachment: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("resilience test not found")
	}
	return nil
}

// --- Framework Mappings (Story 28.2) ---

func frameworkMappingFromCk(r db.CkFrameworkMappings) FrameworkMapping {
	return FrameworkMapping{
		ID:              r.ID,
		OrgID:           r.OrgID,
		SourceControlID: r.SourceControlID,
		TargetControlID: r.TargetControlID,
		CreatedAt:       ckTsToTime(r.CreatedAt),
	}
}

// CreateMapping inserts a new cross-framework control mapping.
// Returns nil, nil (no error) when the mapping already exists (ON CONFLICT DO NOTHING).
func (r *Repository) CreateMapping(ctx context.Context, orgID, sourceControlID, targetControlID string) (*FrameworkMapping, error) {
	row, err := r.q.CreateCKMapping(ctx, db.CreateCKMappingParams{
		OrgID:           orgID,
		SourceControlID: sourceControlID,
		TargetControlID: targetControlID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// ON CONFLICT DO NOTHING — mapping already exists, not an error.
			return nil, nil
		}
		return nil, fmt.Errorf("create mapping: %w", err)
	}
	m := frameworkMappingFromCk(db.CkFrameworkMappings(row))
	return &m, nil
}

// ListMappingsByOrg returns all framework mappings for an organisation.
func (r *Repository) ListMappingsByOrg(ctx context.Context, orgID string) ([]FrameworkMapping, error) {
	rows, err := r.q.ListCKMappingsByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list mappings: %w", err)
	}
	out := make([]FrameworkMapping, 0, len(rows))
	for _, row := range rows {
		out = append(out, frameworkMappingFromCk(db.CkFrameworkMappings(row)))
	}
	return out, nil
}

// DeleteMapping removes a framework mapping by ID within an organisation.
func (r *Repository) DeleteMapping(ctx context.Context, orgID, mappingID string) error {
	n, err := r.q.DeleteCKMapping(ctx, db.DeleteCKMappingParams{ID: mappingID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete mapping: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("mapping not found")
	}
	return nil
}

// GetMappingsBySourceControlIDs returns mappings keyed by source_control_id for a set of source UUIDs.
func (r *Repository) GetMappingsBySourceControlIDs(ctx context.Context, orgID string, sourceIDs []string) (map[string]FrameworkMapping, error) {
	if len(sourceIDs) == 0 {
		return map[string]FrameworkMapping{}, nil
	}
	rows, err := r.q.GetCKMappingsBySourceControlIDs(ctx, db.GetCKMappingsBySourceControlIDsParams{
		OrgID:   orgID,
		Column2: sourceIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("get mappings by source ids: %w", err)
	}
	result := make(map[string]FrameworkMapping, len(rows))
	for _, row := range rows {
		m := frameworkMappingFromCk(db.CkFrameworkMappings(row))
		result[m.SourceControlID] = m
	}
	return result, nil
}

// ListRiskControls returns all controls linked to a risk within an organisation.
func (r *Repository) ListRiskControls(ctx context.Context, orgID, riskID string) ([]Control, error) {
	rows, err := r.q.ListCKRiskControls(ctx, db.ListCKRiskControlsParams{RiskID: riskID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("list risk controls: %w", err)
	}
	out := make([]Control, 0, len(rows))
	for _, row := range rows {
		out = append(out, controlFromFields(controlFields{
			ID: row.ID, FrameworkID: row.FrameworkID, OrgID: row.OrgID,
			ControlID: row.ControlID, Title: row.Title, Description: row.Description,
			Domain: row.Domain, EvidenceType: row.EvidenceType, Weight: row.Weight,
			NotApplicable: row.NotApplicable, NotApplicableReason: row.NotApplicableReason,
			ManualStatus: row.ManualStatus, MaturityScore: row.MaturityScore, Owner: row.Owner,
			LastReviewedAt: row.LastReviewedAt, ReviewIntervalDays: row.ReviewIntervalDays,
			NextReviewDue: row.NextReviewDue, LastReviewedBy: row.LastReviewedBy,
			ReviewNote: row.ReviewNote, DueDate: row.DueDate,
		}))
	}
	return out, nil
}

// --- Cross-Framework Mappings (global reference table) ---

// GetMappingsForControl returns all framework controls that map to/from the given control UUID.
// It resolves the global text-code table (ck_framework_control_mappings) to org-specific UUIDs via JOIN.
//
// embedded SQL by design — see Sitzung F-Wrap-Up commit. Diese UNION mit 4-stufigem
// JOIN-Chain (jeweils mit LIKE-Subquery zur Framework-Auflösung) ist sqlc-machbar,
// aber das resultierende Query-File würde ~50 Zeilen Aliase + Casts brauchen und
// der generierte Go-Code würde keine Lesbarkeit gewinnen. Diese Query ist
// stabil seit Sprint 3 und wird höchstens als „read-once" pro Page-Render aufgerufen.
func (r *Repository) GetMappingsForControl(ctx context.Context, orgID, controlID string) ([]ControlMapping, error) {
	rows, err := r.db.Query(ctx, `
		SELECT m.id::text, m.source_framework, m.source_control_code,
		       m.target_framework, m.target_control_code, m.mapping_type,
		       c2.id::text, c2.title, f2.name
		FROM ck_framework_control_mappings m
		JOIN ck_controls c1 ON c1.control_id = m.source_control_code
		    AND c1.framework_id IN (
		        SELECT id FROM ck_frameworks
		        WHERE lower(name) LIKE '%' || lower(m.source_framework) || '%'
		          AND org_id = $1::uuid
		    )
		    AND c1.org_id = $1::uuid
		    AND c1.id = $2::uuid
		JOIN ck_controls c2 ON c2.control_id = m.target_control_code
		    AND c2.framework_id IN (
		        SELECT id FROM ck_frameworks
		        WHERE lower(name) LIKE '%' || lower(m.target_framework) || '%'
		          AND org_id = $1::uuid
		    )
		    AND c2.org_id = $1::uuid
		JOIN ck_frameworks f2 ON f2.id = c2.framework_id

		UNION

		SELECT m.id::text, m.target_framework, m.target_control_code,
		       m.source_framework, m.source_control_code, m.mapping_type,
		       c1.id::text, c1.title, f1.name
		FROM ck_framework_control_mappings m
		JOIN ck_controls c2 ON c2.control_id = m.target_control_code
		    AND c2.framework_id IN (
		        SELECT id FROM ck_frameworks
		        WHERE lower(name) LIKE '%' || lower(m.target_framework) || '%'
		          AND org_id = $1::uuid
		    )
		    AND c2.org_id = $1::uuid
		    AND c2.id = $2::uuid
		JOIN ck_controls c1 ON c1.control_id = m.source_control_code
		    AND c1.framework_id IN (
		        SELECT id FROM ck_frameworks
		        WHERE lower(name) LIKE '%' || lower(m.source_framework) || '%'
		          AND org_id = $1::uuid
		    )
		    AND c1.org_id = $1::uuid
		JOIN ck_frameworks f1 ON f1.id = c1.framework_id

		ORDER BY 4, 5`,
		orgID, controlID,
	)
	if err != nil {
		return nil, fmt.Errorf("get control mappings: %w", err)
	}
	defer rows.Close()

	var mappings []ControlMapping
	for rows.Next() {
		var m ControlMapping
		if err := rows.Scan(
			&m.ID, &m.SourceFramework, &m.SourceControlCode,
			&m.TargetFramework, &m.TargetControlCode, &m.MappingType,
			&m.TargetControlID, &m.TargetControlTitle, &m.TargetFrameworkName,
		); err != nil {
			return nil, fmt.Errorf("scan control mapping: %w", err)
		}
		mappings = append(mappings, m)
	}
	return mappings, rows.Err()
}

// SeedGlobalControlMapping inserts a single row into ck_framework_control_mappings,
// silently ignoring duplicates (ON CONFLICT DO NOTHING).
func (r *Repository) SeedGlobalControlMapping(ctx context.Context, srcFW, srcCode, tgtFW, tgtCode, mappingType string) error {
	err := r.q.SeedCKGlobalControlMapping(ctx, db.SeedCKGlobalControlMappingParams{
		SourceFramework:   srcFW,
		SourceControlCode: srcCode,
		TargetFramework:   tgtFW,
		TargetControlCode: tgtCode,
		MappingType:       mappingType,
	})
	if err != nil {
		return fmt.Errorf("seed global control mapping %s/%s→%s/%s: %w", srcFW, srcCode, tgtFW, tgtCode, err)
	}
	return nil
}

// --- Questionnaire Builder (Story 29.2) ---

func questionnaireFromCk(r db.CkQuestionnaires) Questionnaire {
	return Questionnaire{
		ID:          r.ID,
		OrgID:       r.OrgID,
		Name:        r.Name,
		Description: r.Description.String,
		IsTemplate:  r.IsTemplate,
		CreatedAt:   ckTsToTime(r.CreatedAt),
		UpdatedAt:   ckTsToTime(r.UpdatedAt),
	}
}

func questionFromCk(r db.CkQuestionnaireQuestions) Question {
	q := Question{
		ID:              r.ID,
		QuestionnaireID: r.QuestionnaireID,
		OrderIdx:        int(r.OrderIdx),
		QuestionText:    r.QuestionText,
		QuestionType:    r.QuestionType,
		Required:        r.Required,
		ControlID:       uuidPtrFromPgtype(r.ControlID),
		CreatedAt:       ckTsToTime(r.CreatedAt),
		UpdatedAt:       ckTsToTime(r.UpdatedAt),
	}
	if len(r.Options) > 0 {
		_ = json.Unmarshal(r.Options, &q.Options)
	}
	return q
}

// CreateQuestionnaire inserts a new questionnaire for an organisation.
func (r *Repository) CreateQuestionnaire(ctx context.Context, orgID, name, description string, isTemplate bool) (*Questionnaire, error) {
	row, err := r.q.CreateCKQuestionnaire(ctx, db.CreateCKQuestionnaireParams{
		OrgID:       orgID,
		Name:        name,
		Description: ckOptText(description),
		IsTemplate:  isTemplate,
	})
	if err != nil {
		return nil, fmt.Errorf("create questionnaire: %w", err)
	}
	q := questionnaireFromCk(db.CkQuestionnaires(row))
	return &q, nil
}

// GetQuestionnaire returns a questionnaire with its questions ordered by order_idx.
func (r *Repository) GetQuestionnaire(ctx context.Context, orgID, id string) (*Questionnaire, error) {
	row, err := r.q.GetCKQuestionnaireBase(ctx, db.GetCKQuestionnaireBaseParams{ID: id, OrgID: orgID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("questionnaire not found")
		}
		return nil, fmt.Errorf("get questionnaire: %w", err)
	}
	q := questionnaireFromCk(db.CkQuestionnaires(row))
	questions, err := r.ListQuestions(ctx, id)
	if err != nil {
		return nil, err
	}
	q.Questions = questions
	return &q, nil
}

// ListQuestionnaires returns questionnaires for an org, optionally filtered by is_template.
func (r *Repository) ListQuestionnaires(ctx context.Context, orgID string, isTemplate *bool) ([]Questionnaire, error) {
	params := db.ListCKQuestionnairesParams{OrgID: orgID}
	if isTemplate != nil {
		params.IsTemplate = pgtype.Bool{Bool: *isTemplate, Valid: true}
	}
	rows, err := r.q.ListCKQuestionnaires(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list questionnaires: %w", err)
	}
	out := make([]Questionnaire, 0, len(rows))
	for _, row := range rows {
		out = append(out, questionnaireFromCk(db.CkQuestionnaires(row)))
	}
	return out, nil
}

// UpdateQuestionnaire updates name/description/is_template of a questionnaire.
func (r *Repository) UpdateQuestionnaire(ctx context.Context, orgID, id, name, description string, isTemplate bool) (*Questionnaire, error) {
	row, err := r.q.UpdateCKQuestionnaire(ctx, db.UpdateCKQuestionnaireParams{
		ID:          id,
		OrgID:       orgID,
		Name:        name,
		Description: ckOptText(description),
		IsTemplate:  isTemplate,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("questionnaire not found")
		}
		return nil, fmt.Errorf("update questionnaire: %w", err)
	}
	q := questionnaireFromCk(db.CkQuestionnaires(row))
	return &q, nil
}

// DeleteQuestionnaire removes a questionnaire and its questions (cascade).
func (r *Repository) DeleteQuestionnaire(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteCKQuestionnaire(ctx, db.DeleteCKQuestionnaireParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete questionnaire: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("questionnaire not found")
	}
	return nil
}

// CreateQuestion inserts a new question into a questionnaire.
func (r *Repository) CreateQuestion(ctx context.Context, questionnaireID, questionText, questionType string, options []string, required bool, controlID *string) (*Question, error) {
	maxIdx, err := r.q.NextCKQuestionOrderIdx(ctx, questionnaireID)
	if err != nil {
		return nil, fmt.Errorf("next order_idx: %w", err)
	}
	var optionsJSON []byte
	if len(options) > 0 {
		var err error
		optionsJSON, err = json.Marshal(options)
		if err != nil {
			return nil, fmt.Errorf("marshal options: %w", err)
		}
	}
	row, err := r.q.CreateCKQuestion(ctx, db.CreateCKQuestionParams{
		QuestionnaireID: questionnaireID,
		OrderIdx:        maxIdx,
		QuestionText:    questionText,
		QuestionType:    questionType,
		Options:         optionsJSON,
		Required:        required,
		ControlID:       ckOptUUIDFromPtr(controlID),
	})
	if err != nil {
		return nil, fmt.Errorf("create question: %w", err)
	}
	q := questionFromCk(db.CkQuestionnaireQuestions(row))
	return &q, nil
}

// GetQuestion returns a single question by ID.
func (r *Repository) GetQuestion(ctx context.Context, questionnaireID, questionID string) (*Question, error) {
	row, err := r.q.GetCKQuestion(ctx, db.GetCKQuestionParams{ID: questionID, QuestionnaireID: questionnaireID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("question not found")
		}
		return nil, fmt.Errorf("get question: %w", err)
	}
	q := questionFromCk(db.CkQuestionnaireQuestions(row))
	return &q, nil
}

// UpdateQuestion updates an existing question.
func (r *Repository) UpdateQuestion(ctx context.Context, questionnaireID, questionID, questionText, questionType string, options []string, required bool, controlID *string) (*Question, error) {
	var optionsJSON []byte
	if len(options) > 0 {
		var err error
		optionsJSON, err = json.Marshal(options)
		if err != nil {
			return nil, fmt.Errorf("marshal options: %w", err)
		}
	}
	row, err := r.q.UpdateCKQuestion(ctx, db.UpdateCKQuestionParams{
		ID:              questionID,
		QuestionnaireID: questionnaireID,
		QuestionText:    questionText,
		QuestionType:    questionType,
		Options:         optionsJSON,
		Required:        required,
		ControlID:       ckOptUUIDFromPtr(controlID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("question not found")
		}
		return nil, fmt.Errorf("update question: %w", err)
	}
	q := questionFromCk(db.CkQuestionnaireQuestions(row))
	return &q, nil
}

// DeleteQuestion removes a question.
func (r *Repository) DeleteQuestion(ctx context.Context, questionnaireID, questionID string) error {
	n, err := r.q.DeleteCKQuestion(ctx, db.DeleteCKQuestionParams{ID: questionID, QuestionnaireID: questionnaireID})
	if err != nil {
		return fmt.Errorf("delete question: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("question not found")
	}
	return nil
}

// ListQuestions returns all questions for a questionnaire ordered by order_idx.
func (r *Repository) ListQuestions(ctx context.Context, questionnaireID string) ([]Question, error) {
	rows, err := r.q.ListCKQuestions(ctx, questionnaireID)
	if err != nil {
		return nil, fmt.Errorf("list questions: %w", err)
	}
	out := make([]Question, 0, len(rows))
	for _, row := range rows {
		out = append(out, questionFromCk(db.CkQuestionnaireQuestions(row)))
	}
	return out, nil
}

// ReorderQuestions updates order_idx for each question ID in the provided slice.
// Original used pgx.Batch; sqlc-Variante iteriert sequentiell. Bei kleinen Listen
// (typischerweise <20 Questions) keine messbare Performance-Differenz.
func (r *Repository) ReorderQuestions(ctx context.Context, questionnaireID string, order []string) error {
	for i, qID := range order {
		if err := r.q.ReorderCKQuestion(ctx, db.ReorderCKQuestionParams{
			OrderIdx:        int32(i),
			ID:              qID,
			QuestionnaireID: questionnaireID,
		}); err != nil {
			return fmt.Errorf("reorder questions: %w", err)
		}
	}
	return nil
}

// CloneQuestionnaire copies a questionnaire and all its questions with new UUIDs.
func (r *Repository) CloneQuestionnaire(ctx context.Context, orgID, sourceID, name string) (*Questionnaire, error) {
	src, err := r.GetQuestionnaire(ctx, orgID, sourceID)
	if err != nil {
		return nil, fmt.Errorf("clone: get source: %w", err)
	}

	newQ, err := r.CreateQuestionnaire(ctx, orgID, name, src.Description, false)
	if err != nil {
		return nil, fmt.Errorf("clone: create questionnaire: %w", err)
	}

	for _, sq := range src.Questions {
		if _, err := r.CreateQuestion(ctx, newQ.ID, sq.QuestionText, sq.QuestionType, sq.Options, sq.Required, sq.ControlID); err != nil {
			return nil, fmt.Errorf("clone: copy question: %w", err)
		}
	}

	return r.GetQuestionnaire(ctx, orgID, newQ.ID)
}

// --- Supplier Portal Assessments (Story 29.3) ---

func assessmentFromCk(r db.CkSupplierAssessments) Assessment {
	return Assessment{
		ID:              r.ID,
		OrgID:           r.OrgID,
		SupplierID:      r.SupplierID,
		QuestionnaireID: r.QuestionnaireID,
		TokenHash:       r.TokenHash,
		ExpiresAt:       ckTsToTime(r.ExpiresAt),
		Status:          r.Status,
		SubmittedAt:     ckTsToTimePtr(r.SubmittedAt),
		SubmittedByIP:   r.SubmittedByIp.String,
		UserAgent:       r.UserAgent.String,
		CreatedAt:       ckTsToTime(r.CreatedAt),
	}
}

// CreateAssessment inserts a new supplier assessment record.
func (r *Repository) CreateAssessment(ctx context.Context, a Assessment) error {
	if err := r.q.CreateCKAssessment(ctx, db.CreateCKAssessmentParams{
		OrgID:           a.OrgID,
		SupplierID:      a.SupplierID,
		QuestionnaireID: a.QuestionnaireID,
		TokenHash:       a.TokenHash,
		ExpiresAt:       pgtype.Timestamptz{Time: a.ExpiresAt, Valid: true},
		Status:          a.Status,
	}); err != nil {
		return fmt.Errorf("create assessment: %w", err)
	}
	return nil
}

// GetAssessmentByTokenHash looks up an assessment by its SHA-256 token hash.
func (r *Repository) GetAssessmentByTokenHash(ctx context.Context, hash string) (*Assessment, error) {
	row, err := r.q.GetCKAssessmentByTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("assessment not found")
		}
		return nil, fmt.Errorf("get assessment: %w", err)
	}
	a := assessmentFromCk(db.CkSupplierAssessments(row))
	return &a, nil
}

// UpdateAssessmentStatus updates status and optional submission metadata.
// For terminal transitions (submitted/reviewed), the UPDATE is conditional on
// the current status not already being terminal — preventing double-submit races.
func (r *Repository) UpdateAssessmentStatus(ctx context.Context, id, status string, submittedAt *time.Time, submittedByIP, userAgent string) error {
	n, err := r.q.UpdateCKAssessmentStatus(ctx, db.UpdateCKAssessmentStatusParams{
		ID:            id,
		Status:        status,
		SubmittedAt:   ckOptTsPtr(submittedAt),
		SubmittedByIp: ckOptText(submittedByIP),
		UserAgent:     ckOptText(userAgent),
	})
	if err != nil {
		return fmt.Errorf("update assessment status: %w", err)
	}
	if n == 0 && (status == "submitted" || status == "reviewed") {
		return fmt.Errorf("assessment already submitted")
	}
	return nil
}

// UpsertAnswers upserts answers sequentially via UpsertCKAnswer. Original code
// nutzte pgx.Batch — sqlc-Variante macht den Trade-off Batch-Performance vs.
// Type-Safety. Da Supplier-Antworten typischerweise einzeln über das Portal
// gesendet werden (selten >50 in einem Batch), ist sequentiell akzeptabel.
func (r *Repository) UpsertAnswers(ctx context.Context, assessmentID string, answers []AnswerInput) error {
	if len(answers) == 0 {
		return nil
	}
	for _, ans := range answers {
		var optionsJSON []byte
		if len(ans.AnswerOptions) > 0 {
			var jsonErr error
			optionsJSON, jsonErr = json.Marshal(ans.AnswerOptions)
			if jsonErr != nil {
				return fmt.Errorf("marshal answer_options: %w", jsonErr)
			}
		}
		if err := r.q.UpsertCKAnswer(ctx, db.UpsertCKAnswerParams{
			AssessmentID:  assessmentID,
			QuestionID:    ans.QuestionID,
			AnswerText:    ckOptText(ans.AnswerText),
			AnswerBool:    pgtype.Bool{Bool: ans.AnswerBool != nil && *ans.AnswerBool, Valid: ans.AnswerBool != nil},
			AnswerOptions: optionsJSON,
			FileUrl:       ckOptText(ans.FileURL),
		}); err != nil {
			return fmt.Errorf("upsert answer: %w", err)
		}
	}
	return nil
}

// GetAssessmentWithQuestionnaire returns an assessment joined with its questionnaire and questions.
func (r *Repository) GetAssessmentWithQuestionnaire(ctx context.Context, id string) (*AssessmentWithQuestionnaire, error) {
	row, err := r.q.GetCKAssessmentBase(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("assessment not found")
		}
		return nil, fmt.Errorf("get assessment: %w", err)
	}
	a := assessmentFromCk(db.CkSupplierAssessments(row))
	qnr, err := r.GetQuestionnaire(ctx, a.OrgID, a.QuestionnaireID)
	if err != nil {
		return nil, fmt.Errorf("get questionnaire for assessment: %w", err)
	}
	return &AssessmentWithQuestionnaire{
		Assessment:    a,
		Questionnaire: qnr,
	}, nil
}

// ListAssessmentsForSupplier returns all assessments for a given supplier within an org.
func (r *Repository) ListAssessmentsForSupplier(ctx context.Context, orgID, supplierID string) ([]Assessment, error) {
	rows, err := r.q.ListCKAssessmentsForSupplier(ctx, db.ListCKAssessmentsForSupplierParams{
		OrgID:      orgID,
		SupplierID: supplierID,
	})
	if err != nil {
		return nil, fmt.Errorf("list assessments: %w", err)
	}
	out := make([]Assessment, 0, len(rows))
	for _, row := range rows {
		out = append(out, assessmentFromCk(db.CkSupplierAssessments(row)))
	}
	return out, nil
}

// UpdateSupplierAssessmentStatus sets assessment_status and last_assessment_at on a supplier row.
func (r *Repository) UpdateSupplierAssessmentStatus(ctx context.Context, orgID, supplierID, status string, at *time.Time) error {
	if err := r.q.UpdateCKSupplierAssessmentStatus(ctx, db.UpdateCKSupplierAssessmentStatusParams{
		ID:               supplierID,
		AssessmentStatus: status,
		LastAssessmentAt: ckOptTsPtr(at),
		OrgID:            orgID,
	}); err != nil {
		return fmt.Errorf("update supplier assessment status: %w", err)
	}
	return nil
}

// --- Assessment Review (Story 29.4) ---

// UpdateAnswerReview sets review_status and rework_note on a single answer.
// org_id wird via JOIN auf ck_supplier_assessments validiert (ck_supplier_answers
// hat keine eigene org_id-Spalte — Schema-Lücke aus Migration 048; existierender
// Code referenzierte sa.org_id was zur Laufzeit gefehlt hätte).
func (r *Repository) UpdateAnswerReview(ctx context.Context, orgID, assessmentID, answerID, reviewStatus, reworkNote string) error {
	n, err := r.q.UpdateCKAnswerReview(ctx, db.UpdateCKAnswerReviewParams{
		ReviewStatus: ckOptText(reviewStatus),
		Column2:      reworkNote,
		ID:           answerID,
		AssessmentID: assessmentID,
		OrgID:        orgID,
	})
	if err != nil {
		return fmt.Errorf("update answer review: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetAnswerWithQuestion fetches a single answer joined with its question (for evidence creation).
func (r *Repository) GetAnswerWithQuestion(ctx context.Context, orgID, answerID string) (*AnswerWithQuestion, error) {
	row, err := r.q.GetCKAnswerWithQuestion(ctx, db.GetCKAnswerWithQuestionParams{ID: answerID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get answer with question: %w", err)
	}
	return &AnswerWithQuestion{
		AnswerID:       row.AnswerID,
		AssessmentID:   row.AssessmentID,
		OrgID:          row.OrgID,
		QuestionID:     row.QuestionID,
		QuestionText:   row.QuestionText,
		ControlID:      uuidPtrFromPgtype(row.ControlID),
		AnswerText:     row.AnswerText,
		FileURL:        row.FileUrl,
		ReviewStatus:   textPtrOrNil(row.ReviewStatus),
		ReworkNote:     textPtrOrNil(row.ReworkNote),
		CertExpiryDate: ckDateToTimePtr(row.CertExpiryDate),
	}, nil
}

// GetAnswersForAssessment returns all answers for an assessment with question info.
func (r *Repository) GetAnswersForAssessment(ctx context.Context, orgID, assessmentID string) ([]AnswerWithReview, error) {
	rows, err := r.q.GetCKAnswersForAssessment(ctx, db.GetCKAnswersForAssessmentParams{AssessmentID: assessmentID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get answers for assessment: %w", err)
	}
	out := make([]AnswerWithReview, 0, len(rows))
	for _, row := range rows {
		out = append(out, AnswerWithReview{
			ID:             row.ID,
			QuestionText:   row.QuestionText,
			AnswerText:     row.AnswerText,
			FileURL:        row.FileUrl,
			ReviewStatus:   textPtrOrNil(row.ReviewStatus),
			ReworkNote:     textPtrOrNil(row.ReworkNote),
			ControlID:      uuidPtrFromPgtype(row.ControlID),
			CertExpiryDate: ckDateToTimePtr(row.CertExpiryDate),
		})
	}
	return out, nil
}

// MarkAssessmentReviewed atomically sets status=reviewed and updates the supplier's assessment_status.
func (r *Repository) MarkAssessmentReviewed(ctx context.Context, orgID, assessmentID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("mark assessment reviewed: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op when Commit succeeded
	qtx := r.q.WithTx(tx)

	supplierID, err := qtx.MarkCKAssessmentReviewed(ctx, db.MarkCKAssessmentReviewedParams{ID: assessmentID, OrgID: orgID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("mark assessment reviewed: update assessment: %w", err)
	}
	if err := qtx.UpdateCKSupplierAssessmentStatus(ctx, db.UpdateCKSupplierAssessmentStatusParams{
		ID:               supplierID,
		AssessmentStatus: "completed",
		LastAssessmentAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		OrgID:            orgID,
	}); err != nil {
		return fmt.Errorf("mark assessment reviewed: update supplier: %w", err)
	}
	return tx.Commit(ctx)
}

// GetAssessmentsForSupplier returns all assessments for a supplier, newest first.
func (r *Repository) GetAssessmentsForSupplier(ctx context.Context, orgID, supplierID string) ([]Assessment, error) {
	rows, err := r.q.ListCKAssessmentsForSupplier(ctx, db.ListCKAssessmentsForSupplierParams{
		OrgID:      orgID,
		SupplierID: supplierID,
	})
	if err != nil {
		return nil, fmt.Errorf("get assessments for supplier: %w", err)
	}
	out := make([]Assessment, 0, len(rows))
	for _, row := range rows {
		out = append(out, assessmentFromCk(db.CkSupplierAssessments(row)))
	}
	return out, nil
}

// FindExpiringCerts returns certificate answers whose cert_expiry_date is on or before the threshold.
func (r *Repository) FindExpiringCerts(ctx context.Context, orgID string, before time.Time) ([]CertExpiryWarning, error) {
	rows, err := r.q.FindCKExpiringCerts(ctx, db.FindCKExpiringCertsParams{
		OrgID:          orgID,
		CertExpiryDate: pgtype.Date{Time: before, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("find expiring certs: %w", err)
	}
	results := make([]CertExpiryWarning, 0, len(rows))
	for _, row := range rows {
		w := CertExpiryWarning{
			AnswerID:     row.AnswerID,
			SupplierID:   row.SupplierID,
			SupplierName: row.SupplierName,
			QuestionText: row.QuestionText,
			FileURL:      row.FileUrl.String,
		}
		if row.CertExpiryDate.Valid {
			w.CertExpiryDate = row.CertExpiryDate.Time
		}
		results = append(results, w)
	}
	return results, nil
}

// InsertAIClassification saves a new classification event and returns its ID.
func (r *Repository) InsertAIClassification(ctx context.Context, orgID, systemID string, in ClassifyAISystemInput) (string, error) {
	var wizardJSON []byte
	if in.WizardAnswers != nil {
		var err error
		wizardJSON, err = json.Marshal(in.WizardAnswers)
		if err != nil {
			return "", fmt.Errorf("marshal wizard answers: %w", err)
		}
	}
	id, err := r.q.InsertCKAIClassification(ctx, db.InsertCKAIClassificationParams{
		OrgID:         orgID,
		AiSystemID:    systemID,
		RiskClass:     in.RiskClass,
		Rationale:     ckOptText(in.Rationale),
		ClassifiedBy:  ckOptText(in.ClassifiedBy),
		WizardAnswers: wizardJSON,
	})
	if err != nil {
		return "", fmt.Errorf("insert ai classification: %w", err)
	}
	return id, nil
}

// UpdateAISystemClassification sets the denormalized classification fields on the AI system row.
func (r *Repository) UpdateAISystemClassification(ctx context.Context, orgID, systemID string, in ClassifyAISystemInput) error {
	n, err := r.q.UpdateCKAISystemClassification(ctx, db.UpdateCKAISystemClassificationParams{
		ID:                      systemID,
		OrgID:                   orgID,
		RiskClass:               ckOptText(in.RiskClass),
		ClassificationRationale: ckOptText(in.Rationale),
		ClassifiedBy:            ckOptText(in.ClassifiedBy),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListAIClassifications returns the classification history for an AI system, newest first.
func (r *Repository) ListAIClassifications(ctx context.Context, orgID, systemID string) ([]AIClassification, error) {
	rows, err := r.q.ListCKAIClassifications(ctx, db.ListCKAIClassificationsParams{
		OrgID:      orgID,
		AiSystemID: systemID,
	})
	if err != nil {
		return nil, fmt.Errorf("list ai classifications: %w", err)
	}
	results := make([]AIClassification, 0, len(rows))
	for _, row := range rows {
		c := AIClassification{
			ID:           row.ID,
			OrgID:        row.OrgID,
			AISystemID:   row.AiSystemID,
			RiskClass:    row.RiskClass,
			Rationale:    row.Rationale.String,
			ClassifiedBy: row.ClassifiedBy.String,
			ClassifiedAt: ckTsToTime(row.ClassifiedAt),
		}
		if len(row.WizardAnswers) > 0 {
			_ = json.Unmarshal(row.WizardAnswers, &c.WizardAnswers)
		}
		results = append(results, c)
	}
	return results, nil
}

// aiDocFromCk maps the sqlc CkAiDocumentation row to the domain AIDocumentation type.
func aiDocFromCk(r db.CkAiDocumentation) AIDocumentation {
	return AIDocumentation{
		ID:                 r.ID,
		OrgID:              r.OrgID,
		AISystemID:         r.AiSystemID,
		Version:            int(r.Version),
		SystemDescription:  r.SystemDescription.String,
		IntendedPurpose:    r.IntendedPurpose.String,
		TrainingData:       r.TrainingData.String,
		DataQuality:        r.DataQuality.String,
		PerformanceMetrics: r.PerformanceMetrics.String,
		SystemLimits:       r.SystemLimits.String,
		RiskManagement:     r.RiskManagement.String,
		HumanOversight:     r.HumanOversight.String,
		LoggingAuditTrail:  r.LoggingAuditTrail.String,
		AuthoredBy:         r.AuthoredBy.String,
		Status:             r.Status,
		CreatedAt:          ckTsToTime(r.CreatedAt),
		UpdatedAt:          ckTsToTime(r.UpdatedAt),
	}
}

// UpsertAIDocumentation inserts or updates (creates a new version) the technical documentation for an AI system.
// Each save creates a new version row; returns the saved document.
func (r *Repository) UpsertAIDocumentation(ctx context.Context, orgID, systemID string, in UpsertAIDocumentationInput) (*AIDocumentation, error) {
	nextVer, err := r.q.NextCKAIDocumentationVersion(ctx, db.NextCKAIDocumentationVersionParams{
		OrgID:      orgID,
		AiSystemID: systemID,
	})
	if err != nil {
		return nil, fmt.Errorf("next ai documentation version: %w", err)
	}
	status := in.Status
	if status == "" {
		status = "draft"
	}
	row, err := r.q.InsertCKAIDocumentation(ctx, db.InsertCKAIDocumentationParams{
		OrgID:              orgID,
		AiSystemID:         systemID,
		Version:            nextVer,
		SystemDescription:  ckOptText(in.SystemDescription),
		IntendedPurpose:    ckOptText(in.IntendedPurpose),
		TrainingData:       ckOptText(in.TrainingData),
		DataQuality:        ckOptText(in.DataQuality),
		PerformanceMetrics: ckOptText(in.PerformanceMetrics),
		SystemLimits:       ckOptText(in.SystemLimits),
		RiskManagement:     ckOptText(in.RiskManagement),
		HumanOversight:     ckOptText(in.HumanOversight),
		LoggingAuditTrail:  ckOptText(in.LoggingAuditTrail),
		AuthoredBy:         ckOptText(in.AuthoredBy),
		Status:             status,
	})
	if err != nil {
		return nil, fmt.Errorf("insert ai documentation: %w", err)
	}
	doc := aiDocFromCk(row)
	return &doc, nil
}

// GetLatestAIDocumentation returns the most recent documentation version for an AI system.
func (r *Repository) GetLatestAIDocumentation(ctx context.Context, orgID, systemID string) (*AIDocumentation, error) {
	row, err := r.q.GetLatestCKAIDocumentation(ctx, db.GetLatestCKAIDocumentationParams{
		OrgID:      orgID,
		AiSystemID: systemID,
	})
	if err != nil {
		return nil, err
	}
	doc := aiDocFromCk(row)
	return &doc, nil
}

// ListAIDocumentationVersions returns all versions of a system's documentation, newest first.
func (r *Repository) ListAIDocumentationVersions(ctx context.Context, orgID, systemID string) ([]AIDocumentation, error) {
	rows, err := r.q.ListCKAIDocumentationVersions(ctx, db.ListCKAIDocumentationVersionsParams{
		OrgID:      orgID,
		AiSystemID: systemID,
	})
	if err != nil {
		return nil, fmt.Errorf("list ai doc versions: %w", err)
	}
	results := make([]AIDocumentation, 0, len(rows))
	for _, row := range rows {
		results = append(results, aiDocFromCk(row))
	}
	return results, nil
}

// GetEUAIActStats returns aggregate counts needed for the EU AI Act dashboard.
func (r *Repository) GetEUAIActStats(ctx context.Context, orgID string) (total int, byRisk map[string]int, byStatus map[string]int, withoutDocs int, err error) {
	byRisk = map[string]int{}
	byStatus = map[string]int{}

	rows, err := r.q.ListCKAISystemsForStats(ctx, orgID)
	if err != nil {
		return 0, nil, nil, 0, fmt.Errorf("get eu ai act stats: %w", err)
	}
	for _, row := range rows {
		total++
		byRisk[row.RiskClass]++
		byStatus[row.Status]++
	}

	count, err := r.q.CountCKAISystemsWithoutDocs(ctx, orgID)
	if err != nil {
		return 0, nil, nil, 0, fmt.Errorf("count ai systems without docs: %w", err)
	}
	withoutDocs = int(count)
	return total, byRisk, byStatus, withoutDocs, nil
}

// EvidenceForExport is a flattened view of evidence joined with its control,
// used exclusively by the audit-package ZIP generator.
type EvidenceForExport struct {
	ControlID        string
	ControlTitle     string
	ControlDomain    string // e.g. "A.5" from the control code
	EvidenceID       string
	EvidenceTitle    string
	EvidenceSource   string // 'manual', 'github', 'aws', etc.
	EvidenceDesc     string
	EvidenceFilePath string
	CollectedAt      time.Time
}

// ListEvidenceForFramework returns all evidence for all controls of a framework
// joined with control metadata. Controls without evidence are included with
// empty evidence fields so every control appears in the index PDF.
func (r *Repository) ListEvidenceForFramework(ctx context.Context, orgID, frameworkID string) ([]EvidenceForExport, error) {
	rows, err := r.q.ListCKEvidenceForFramework(ctx, db.ListCKEvidenceForFrameworkParams{
		OrgID:       orgID,
		FrameworkID: frameworkID,
	})
	if err != nil {
		return nil, fmt.Errorf("list evidence for framework: %w", err)
	}
	result := make([]EvidenceForExport, 0, len(rows))
	now := time.Now()
	for _, row := range rows {
		evID := ""
		if row.EvidenceID.Valid {
			evID = row.EvidenceID.String()
		}
		collectedAt := now
		if row.EvidenceCreatedAt.Valid {
			collectedAt = row.EvidenceCreatedAt.Time
		}
		result = append(result, EvidenceForExport{
			ControlID:        row.ControlUuid,
			ControlTitle:     row.ControlTitle,
			ControlDomain:    row.ControlCode,
			EvidenceID:       evID,
			EvidenceTitle:    row.EvidenceTitle.String,
			EvidenceSource:   row.EvidenceSource.String,
			EvidenceDesc:     row.EvidenceDesc.String,
			EvidenceFilePath: row.EvidenceFilePath.String,
			CollectedAt:      collectedAt,
		})
	}
	return result, nil
}

// --- Maßnahmen-Katalog (control measures) ---

// measureFromCkControlMeasures maps the sqlc Table-Row to the domain ControlMeasure.
func measureFromCkControlMeasures(r db.CkControlMeasures) ControlMeasure {
	return ControlMeasure{
		ID:          r.ID,
		ControlID:   r.ControlID,
		OrgID:       r.OrgID,
		Title:       r.Title,
		Description: r.Description,
		Difficulty:  r.Difficulty,
		StepOrder:   int(r.StepOrder),
		IsBuiltin:   r.IsBuiltin,
		CreatedAt:   ckTsToTime(r.CreatedAt),
	}
}

// ListMeasures returns all measures for a control within an organisation, ordered by step_order.
func (r *Repository) ListMeasures(ctx context.Context, orgID, controlID string) ([]ControlMeasure, error) {
	rows, err := r.q.ListCKMeasures(ctx, db.ListCKMeasuresParams{OrgID: orgID, ControlID: controlID})
	if err != nil {
		return nil, fmt.Errorf("list measures: %w", err)
	}
	out := make([]ControlMeasure, 0, len(rows))
	for _, row := range rows {
		out = append(out, measureFromCkControlMeasures(row))
	}
	return out, nil
}

// CreateMeasure inserts a new measure for a control.
func (r *Repository) CreateMeasure(ctx context.Context, orgID, controlID string, in CreateMeasureInput) (ControlMeasure, error) {
	row, err := r.q.CreateCKMeasure(ctx, db.CreateCKMeasureParams{
		ControlID:   controlID,
		OrgID:       orgID,
		Title:       in.Title,
		Description: in.Description,
		Difficulty:  in.Difficulty,
		StepOrder:   int32(in.StepOrder),
	})
	if err != nil {
		return ControlMeasure{}, fmt.Errorf("create measure: %w", err)
	}
	return measureFromCkControlMeasures(row), nil
}

// UpdateMeasure updates editable fields of a measure.
func (r *Repository) UpdateMeasure(ctx context.Context, orgID, measureID string, in UpdateMeasureInput) (ControlMeasure, error) {
	row, err := r.q.UpdateCKMeasure(ctx, db.UpdateCKMeasureParams{
		ID:          measureID,
		OrgID:       orgID,
		Title:       optTextStrPtr(in.Title),
		Description: optTextStrPtr(in.Description),
		Difficulty:  optTextStrPtr(in.Difficulty),
		StepOrder:   ckOptIntPtr(in.StepOrder),
	})
	if err != nil {
		return ControlMeasure{}, fmt.Errorf("update measure: %w", err)
	}
	return measureFromCkControlMeasures(row), nil
}

// DeleteMeasure removes a non-builtin measure by ID.
func (r *Repository) DeleteMeasure(ctx context.Context, orgID, measureID string) error {
	n, err := r.q.DeleteCKMeasure(ctx, db.DeleteCKMeasureParams{ID: measureID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete measure: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("measure not found or is builtin")
	}
	return nil
}

// SeedMeasuresForControl inserts builtin measures for a control, skipping duplicates by title.
func (r *Repository) SeedMeasuresForControl(ctx context.Context, orgID, controlID string, measures []CreateMeasureInput) error {
	for i, m := range measures {
		if err := r.q.SeedCKMeasure(ctx, db.SeedCKMeasureParams{
			ControlID:   controlID,
			OrgID:       orgID,
			Title:       m.Title,
			Description: m.Description,
			Difficulty:  m.Difficulty,
			StepOrder:   int32(i),
		}); err != nil {
			return fmt.Errorf("seed measure %q: %w", m.Title, err)
		}
	}
	return nil
}

// FindControlByCode looks up a control UUID by its text control_id code within an org.
// Returns an empty string if not found.
func (r *Repository) FindControlByCode(ctx context.Context, orgID, code string) (string, error) {
	id, err := r.q.FindCKControlByCode(ctx, db.FindCKControlByCodeParams{
		OrgID:     orgID,
		ControlID: code,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("find control by code %q: %w", code, err)
	}
	return id, nil
}

// ListAllOrgs returns the IDs of all organisations.
// Used for cross-org seeding on startup.
func (r *Repository) ListAllOrgs(ctx context.Context) ([]string, error) {
	ids, err := r.q.ListAllOrgIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list all orgs: %w", err)
	}
	return ids, nil
}

// --- CAPA (Corrective and Preventive Actions) ---

// capaFromCkCapas maps the sqlc Table-Row to the domain CAPA type.
func capaFromCkCapas(r db.CkCapas) CAPA {
	return CAPA{
		ID:               r.ID,
		OrgID:            r.OrgID,
		SourceType:       r.SourceType,
		SourceID:         r.SourceID,
		Title:            r.Title,
		Description:      r.Description,
		RootCause:        r.RootCause,
		ActionPlan:       r.ActionPlan,
		AssigneeEmail:    r.AssigneeEmail,
		DueDate:          ckDateToTimePtr(r.DueDate),
		Priority:         r.Priority,
		Status:           r.Status,
		VerificationNote: r.VerificationNote,
		ClosedAt:         ckTsToTimePtr(r.ClosedAt),
		CreatedAt:        ckTsToTime(r.CreatedAt),
		UpdatedAt:        ckTsToTime(r.UpdatedAt),
	}
}

// ListCAPAs returns CAPAs for an organisation, optionally filtered by status.
func (r *Repository) ListCAPAs(ctx context.Context, orgID string, statusFilter string) ([]CAPA, error) {
	rows, err := r.q.ListCKCAPAs(ctx, db.ListCKCAPAsParams{
		OrgID:  orgID,
		Status: ckOptText(statusFilter),
	})
	if err != nil {
		return nil, fmt.Errorf("list capas: %w", err)
	}
	out := make([]CAPA, 0, len(rows))
	for _, row := range rows {
		out = append(out, capaFromCkCapas(row))
	}
	return out, nil
}

// ListCAPAsForSource returns CAPAs linked to a specific source (audit/incident/risk).
func (r *Repository) ListCAPAsForSource(ctx context.Context, orgID, sourceType, sourceID string) ([]CAPA, error) {
	rows, err := r.q.ListCKCAPAsForSource(ctx, db.ListCKCAPAsForSourceParams{
		OrgID:      orgID,
		SourceType: sourceType,
		SourceID:   sourceID,
	})
	if err != nil {
		return nil, fmt.Errorf("list capas for source: %w", err)
	}
	out := make([]CAPA, 0, len(rows))
	for _, row := range rows {
		out = append(out, capaFromCkCapas(row))
	}
	return out, nil
}

// GetCAPA returns a single CAPA by ID within an organisation.
func (r *Repository) GetCAPA(ctx context.Context, orgID, capaID string) (CAPA, error) {
	row, err := r.q.GetCKCAPA(ctx, db.GetCKCAPAParams{ID: capaID, OrgID: orgID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CAPA{}, ErrNotFound
		}
		return CAPA{}, fmt.Errorf("get capa: %w", err)
	}
	return capaFromCkCapas(row), nil
}

// CreateCAPA inserts a new CAPA record.
func (r *Repository) CreateCAPA(ctx context.Context, orgID string, in CreateCAPAInput) (CAPA, error) {
	priority := in.Priority
	if priority == "" {
		priority = "medium"
	}
	row, err := r.q.CreateCKCAPA(ctx, db.CreateCKCAPAParams{
		OrgID:         orgID,
		SourceType:    in.SourceType,
		SourceID:      in.SourceID,
		Title:         in.Title,
		Description:   in.Description,
		AssigneeEmail: in.AssigneeEmail,
		DueDate:       ckOptDatePtr(in.DueDate),
		Priority:      priority,
	})
	if err != nil {
		return CAPA{}, fmt.Errorf("create capa: %w", err)
	}
	return capaFromCkCapas(row), nil
}

// UpdateCAPA applies partial updates to a CAPA using COALESCE.
// When status transitions to 'closed', closed_at is set to NOW().
func (r *Repository) UpdateCAPA(ctx context.Context, orgID, capaID string, in UpdateCAPAInput) (CAPA, error) {
	row, err := r.q.UpdateCKCAPA(ctx, db.UpdateCKCAPAParams{
		ID:               capaID,
		OrgID:            orgID,
		Title:            optTextStrPtr(in.Title),
		Description:      optTextStrPtr(in.Description),
		RootCause:        optTextStrPtr(in.RootCause),
		ActionPlan:       optTextStrPtr(in.ActionPlan),
		AssigneeEmail:    optTextStrPtr(in.AssigneeEmail),
		DueDate:          ckOptDatePtr(in.DueDate),
		Priority:         optTextStrPtr(in.Priority),
		Status:           optTextStrPtr(in.Status),
		VerificationNote: optTextStrPtr(in.VerificationNote),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CAPA{}, ErrNotFound
		}
		return CAPA{}, fmt.Errorf("update capa: %w", err)
	}
	return capaFromCkCapas(row), nil
}

// DeleteCAPA removes a CAPA record.
func (r *Repository) DeleteCAPA(ctx context.Context, orgID, capaID string) error {
	n, err := r.q.DeleteCKCAPA(ctx, db.DeleteCKCAPAParams{ID: capaID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete capa: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Collaborative Tasks ---

func taskFromCk(r db.CkTasks) Task {
	return Task{
		ID:            r.ID,
		OrgID:         r.OrgID,
		EntityType:    r.EntityType,
		EntityID:      r.EntityID,
		Title:         r.Title,
		Description:   r.Description,
		AssigneeEmail: r.AssigneeEmail,
		DueDate:       ckDateToTimePtr(r.DueDate),
		Status:        r.Status,
		Priority:      r.Priority,
		CreatedBy:     r.CreatedBy,
		CreatedAt:     ckTsToTime(r.CreatedAt),
		UpdatedAt:     ckTsToTime(r.UpdatedAt),
	}
}

// ListTasks returns all tasks for the given entity, ordered newest first.
func (r *Repository) ListTasks(ctx context.Context, orgID, entityType, entityID string) ([]Task, error) {
	rows, err := r.q.ListCKTasks(ctx, db.ListCKTasksParams{
		OrgID:      orgID,
		EntityType: entityType,
		EntityID:   entityID,
	})
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	out := make([]Task, 0, len(rows))
	for _, row := range rows {
		out = append(out, taskFromCk(row))
	}
	return out, nil
}

// CreateTask inserts a new task and returns the created row.
func (r *Repository) CreateTask(ctx context.Context, orgID, entityType, entityID string, in CreateTaskInput) (Task, error) {
	dueDate := pgtype.Date{}
	if in.DueDate != nil && *in.DueDate != "" {
		t, err := time.Parse("2006-01-02", *in.DueDate)
		if err != nil {
			return Task{}, fmt.Errorf("invalid due_date format (expected YYYY-MM-DD): %w", err)
		}
		dueDate = pgtype.Date{Time: t, Valid: true}
	}
	status := in.Status
	if status == "" {
		status = "open"
	}
	priority := in.Priority
	if priority == "" {
		priority = "medium"
	}
	row, err := r.q.CreateCKTask(ctx, db.CreateCKTaskParams{
		OrgID:         orgID,
		EntityType:    entityType,
		EntityID:      entityID,
		Title:         in.Title,
		Description:   in.Description,
		AssigneeEmail: in.AssigneeEmail,
		DueDate:       dueDate,
		Status:        status,
		Priority:      priority,
	})
	if err != nil {
		return Task{}, fmt.Errorf("create task: %w", err)
	}
	return taskFromCk(row), nil
}

// UpdateTask applies partial updates to a task via COALESCE.
func (r *Repository) UpdateTask(ctx context.Context, orgID, taskID string, in UpdateTaskInput) (Task, error) {
	dueDate := pgtype.Date{}
	if in.DueDate != nil && *in.DueDate != "" {
		t, err := time.Parse("2006-01-02", *in.DueDate)
		if err != nil {
			return Task{}, fmt.Errorf("invalid due_date format (expected YYYY-MM-DD): %w", err)
		}
		dueDate = pgtype.Date{Time: t, Valid: true}
	}
	row, err := r.q.UpdateCKTask(ctx, db.UpdateCKTaskParams{
		ID:            taskID,
		OrgID:         orgID,
		Title:         optTextStrPtr(in.Title),
		Description:   optTextStrPtr(in.Description),
		AssigneeEmail: optTextStrPtr(in.AssigneeEmail),
		DueDate:       dueDate,
		Status:        optTextStrPtr(in.Status),
		Priority:      optTextStrPtr(in.Priority),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Task{}, fmt.Errorf("task not found")
		}
		return Task{}, fmt.Errorf("update task: %w", err)
	}
	return taskFromCk(row), nil
}

// DeleteTask removes a task.
func (r *Repository) DeleteTask(ctx context.Context, orgID, taskID string) error {
	n, err := r.q.DeleteCKTask(ctx, db.DeleteCKTaskParams{ID: taskID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("task not found")
	}
	return nil
}

// ListOverdueTasks returns tasks with due_date in the past that are not done.
func (r *Repository) ListOverdueTasks(ctx context.Context, orgID string) ([]Task, error) {
	rows, err := r.q.ListCKOverdueTasks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list overdue tasks: %w", err)
	}
	out := make([]Task, 0, len(rows))
	for _, row := range rows {
		out = append(out, taskFromCk(row))
	}
	return out, nil
}

// --- Comments ---

func commentFromCk(r db.CkComments) Comment {
	return Comment{
		ID:          r.ID,
		OrgID:       r.OrgID,
		EntityType:  r.EntityType,
		EntityID:    r.EntityID,
		AuthorEmail: r.AuthorEmail,
		Body:        r.Body,
		CreatedAt:   ckTsToTime(r.CreatedAt),
	}
}

// ListComments returns all comments for an entity ordered chronologically.
func (r *Repository) ListComments(ctx context.Context, orgID, entityType, entityID string) ([]Comment, error) {
	rows, err := r.q.ListCKComments(ctx, db.ListCKCommentsParams{
		OrgID:      orgID,
		EntityType: entityType,
		EntityID:   entityID,
	})
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	out := make([]Comment, 0, len(rows))
	for _, row := range rows {
		out = append(out, commentFromCk(row))
	}
	return out, nil
}

// CreateComment inserts a new comment and returns the created row.
func (r *Repository) CreateComment(ctx context.Context, orgID, entityType, entityID string, in CreateCommentInput) (Comment, error) {
	row, err := r.q.CreateCKComment(ctx, db.CreateCKCommentParams{
		OrgID:       orgID,
		EntityType:  entityType,
		EntityID:    entityID,
		AuthorEmail: in.AuthorEmail,
		Body:        in.Body,
	})
	if err != nil {
		return Comment{}, fmt.Errorf("create comment: %w", err)
	}
	return commentFromCk(row), nil
}

// DeleteComment removes a comment.
func (r *Repository) DeleteComment(ctx context.Context, orgID, commentID string) error {
	n, err := r.q.DeleteCKComment(ctx, db.DeleteCKCommentParams{ID: commentID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("comment not found")
	}
	return nil
}

// --- Evidence Files (Migration 074) ---

// evidenceFileFromCk maps the sqlc CkEvidenceFiles row to the domain EvidenceFile.
func evidenceFileFromCk(r db.CkEvidenceFiles) EvidenceFile {
	evID := ""
	if r.EvidenceID.Valid {
		evID = r.EvidenceID.String()
	}
	return EvidenceFile{
		ID:           r.ID,
		OrgID:        r.OrgID,
		EvidenceID:   evID,
		ControlID:    r.ControlID,
		OriginalName: r.OriginalName,
		StoredName:   r.StoredName,
		MimeType:     r.MimeType,
		SizeBytes:    r.SizeBytes,
		UploadedBy:   r.UploadedBy,
		CreatedAt:    ckTsToTime(r.CreatedAt),
	}
}

// CreateEvidenceFile inserts a new evidence file record.
func (r *Repository) CreateEvidenceFile(ctx context.Context, f EvidenceFile) (EvidenceFile, error) {
	row, err := r.q.CreateCKEvidenceFile(ctx, db.CreateCKEvidenceFileParams{
		OrgID:        f.OrgID,
		EvidenceID:   ckOptUUIDFromStr(f.EvidenceID),
		ControlID:    f.ControlID,
		OriginalName: f.OriginalName,
		StoredName:   f.StoredName,
		MimeType:     f.MimeType,
		SizeBytes:    f.SizeBytes,
		UploadedBy:   f.UploadedBy,
	})
	if err != nil {
		return EvidenceFile{}, fmt.Errorf("create evidence file: %w", err)
	}
	return evidenceFileFromCk(row), nil
}

// ListEvidenceFiles returns all files attached to an evidence record.
func (r *Repository) ListEvidenceFiles(ctx context.Context, orgID, evidenceID string) ([]EvidenceFile, error) {
	rows, err := r.q.ListCKEvidenceFiles(ctx, db.ListCKEvidenceFilesParams{
		OrgID:      orgID,
		EvidenceID: ckOptUUIDFromStr(evidenceID),
	})
	if err != nil {
		return nil, fmt.Errorf("list evidence files: %w", err)
	}
	items := make([]EvidenceFile, 0, len(rows))
	for _, row := range rows {
		items = append(items, evidenceFileFromCk(row))
	}
	return items, nil
}

// ListEvidenceFilesByControl returns all files attached to any evidence for a given control.
func (r *Repository) ListEvidenceFilesByControl(ctx context.Context, orgID, controlID string) ([]EvidenceFile, error) {
	rows, err := r.q.ListCKEvidenceFilesByControl(ctx, db.ListCKEvidenceFilesByControlParams{
		OrgID:     orgID,
		ControlID: controlID,
	})
	if err != nil {
		return nil, fmt.Errorf("list evidence files by control: %w", err)
	}
	items := make([]EvidenceFile, 0, len(rows))
	for _, row := range rows {
		items = append(items, evidenceFileFromCk(row))
	}
	return items, nil
}

// GetEvidenceFile returns a single evidence file by ID within an organisation.
func (r *Repository) GetEvidenceFile(ctx context.Context, orgID, fileID string) (EvidenceFile, error) {
	row, err := r.q.GetCKEvidenceFile(ctx, db.GetCKEvidenceFileParams{ID: fileID, OrgID: orgID})
	if err != nil {
		return EvidenceFile{}, fmt.Errorf("get evidence file: %w", err)
	}
	return evidenceFileFromCk(row), nil
}

// DeleteEvidenceFile removes an evidence file record and returns its metadata for disk deletion.
func (r *Repository) DeleteEvidenceFile(ctx context.Context, orgID, fileID string) (EvidenceFile, error) {
	row, err := r.q.DeleteCKEvidenceFile(ctx, db.DeleteCKEvidenceFileParams{ID: fileID, OrgID: orgID})
	if err != nil {
		return EvidenceFile{}, fmt.Errorf("delete evidence file: %w", err)
	}
	return evidenceFileFromCk(row), nil
}

// --- Control Review Cycles (Migration 075) ---

// scanControl is a helper that scans the standard control SELECT columns including review fields.
func scanControl(row interface {
	Scan(dest ...any) error
}) (Control, error) {
	var c Control
	var nextReviewDue *time.Time
	err := row.Scan(
		&c.ID, &c.FrameworkID, &c.OrgID, &c.ControlID, &c.Title,
		&c.Description, &c.Domain, &c.EvidenceType, &c.Weight,
		&c.NotApplicable, &c.NotApplicableReason, &c.ManualStatus, &c.MaturityScore,
		&c.LastReviewedAt, &c.ReviewIntervalDays, &nextReviewDue,
		&c.LastReviewedBy, &c.ReviewNote,
	)
	if err != nil {
		return Control{}, err
	}
	c.NextReviewDue = nextReviewDue
	c.IsReviewOverdue = nextReviewDue != nil && nextReviewDue.Before(time.Now())
	return c, nil
}

// RecordControlReview records a review event for a control:
//   - Updates last_reviewed_at, review_interval_days, last_reviewed_by, review_note on ck_controls.
//   - Inserts a row into ck_control_reviews.
//   - Returns the updated control.
//
// embedded SQL by design — see Sitzung F-Wrap-Up commit. Die UPDATE-Query ist
// dynamisch (interval_expr wechselt zwischen "$5" und "review_interval_days"),
// das wäre in sqlc nur über zwei separate Queries machbar (with/without interval
// override). Da das Verhalten in einer Transaktion atomar bleiben muss und
// sqlc-WithTx-Pattern bereits etabliert ist, wäre die Aufteilung mehr Code
// für wenig Gewinn.
func (r *Repository) RecordControlReview(ctx context.Context, orgID, controlID string, in RecordReviewInput, statusAtReview string) (Control, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Control{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Determine interval: use provided value or keep existing.
	// Use a parameterised placeholder ($5) when a new interval is given to avoid SQL injection
	// via integer interpolation, even though in.ReviewInterval is typed as int.
	var (
		intervalExpr string
		queryArgs    []any
	)
	if in.ReviewInterval > 0 {
		intervalExpr = "$5"
		queryArgs = []any{controlID, orgID, in.ReviewedBy, in.ReviewNote, in.ReviewInterval}
	} else {
		intervalExpr = "review_interval_days"
		queryArgs = []any{controlID, orgID, in.ReviewedBy, in.ReviewNote}
	}

	q := fmt.Sprintf(`
		UPDATE ck_controls
		SET last_reviewed_at      = NOW(),
		    review_interval_days  = %s,
		    last_reviewed_by      = $3,
		    review_note           = $4
		WHERE id = $1::uuid AND org_id = $2::uuid
		RETURNING id::text, framework_id::text, org_id::text, control_id, title,
		          COALESCE(description,''), domain, evidence_type, weight,
		          not_applicable, COALESCE(not_applicable_reason,''),
		          COALESCE(manual_status,''), maturity_score,
		          last_reviewed_at, review_interval_days, next_review_due,
		          last_reviewed_by, review_note`, intervalExpr)

	c, err := scanControl(tx.QueryRow(ctx, q, queryArgs...))
	if err != nil {
		return Control{}, fmt.Errorf("update control for review: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO ck_control_reviews (org_id, control_id, reviewed_by, review_note, status_at_review)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5)`,
		orgID, controlID, in.ReviewedBy, in.ReviewNote, statusAtReview,
	)
	if err != nil {
		return Control{}, fmt.Errorf("insert control review: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Control{}, fmt.Errorf("commit review tx: %w", err)
	}
	return c, nil
}

// ListControlReviews returns the review history for a control, newest first.
func (r *Repository) ListControlReviews(ctx context.Context, orgID, controlID string) ([]ControlReview, error) {
	rows, err := r.q.ListCKControlReviews(ctx, db.ListCKControlReviewsParams{
		ControlID: controlID,
		OrgID:     orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("list control reviews: %w", err)
	}
	reviews := make([]ControlReview, 0, len(rows))
	for _, row := range rows {
		reviews = append(reviews, ControlReview{
			ID:             row.ID,
			ControlID:      row.ControlID,
			ReviewedBy:     row.ReviewedBy,
			ReviewNote:     row.ReviewNote,
			StatusAtReview: row.StatusAtReview,
			ReviewedAt:     ckTsToTime(row.ReviewedAt),
		})
	}
	return reviews, nil
}

// ListOverdueControls returns controls whose next_review_due is in the past, ordered by urgency.
func (r *Repository) ListOverdueControls(ctx context.Context, orgID string) ([]Control, error) {
	rows, err := r.q.ListCKOverdueControls(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list overdue controls: %w", err)
	}
	controls := make([]Control, 0, len(rows))
	for _, row := range rows {
		controls = append(controls, controlFromFields(controlFields{
			ID: row.ID, FrameworkID: row.FrameworkID, OrgID: row.OrgID,
			ControlID: row.ControlID, Title: row.Title, Description: row.Description,
			Domain: row.Domain, EvidenceType: row.EvidenceType, Weight: row.Weight,
			NotApplicable: row.NotApplicable, NotApplicableReason: row.NotApplicableReason,
			ManualStatus: row.ManualStatus, MaturityScore: row.MaturityScore, Owner: row.Owner,
			LastReviewedAt: row.LastReviewedAt, ReviewIntervalDays: row.ReviewIntervalDays,
			NextReviewDue: row.NextReviewDue, LastReviewedBy: row.LastReviewedBy,
			ReviewNote: row.ReviewNote, DueDate: row.DueDate,
		}))
	}
	return controls, nil
}

// --- Paginated list helpers (used by pagination-aware handlers) ---

// ListControlsPaged returns a page of controls plus the total count.
func (r *Repository) ListControlsPaged(ctx context.Context, orgID, frameworkID string, offset, limit int) ([]Control, int, error) {
	total, err := r.q.CountCKControls(ctx, db.CountCKControlsParams{
		FrameworkID: frameworkID,
		OrgID:       orgID,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count controls: %w", err)
	}
	rows, err := r.q.ListCKControlsPaged(ctx, db.ListCKControlsPagedParams{
		FrameworkID: frameworkID,
		OrgID:       orgID,
		Limit:       int32(limit),
		Offset:      int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list controls paged: %w", err)
	}
	controls := make([]Control, 0, len(rows))
	for _, row := range rows {
		controls = append(controls, controlFromFields(controlFields{
			ID: row.ID, FrameworkID: row.FrameworkID, OrgID: row.OrgID,
			ControlID: row.ControlID, Title: row.Title, Description: row.Description,
			Domain: row.Domain, EvidenceType: row.EvidenceType, Weight: row.Weight,
			NotApplicable: row.NotApplicable, NotApplicableReason: row.NotApplicableReason,
			ManualStatus: row.ManualStatus, MaturityScore: row.MaturityScore, Owner: row.Owner,
			LastReviewedAt: row.LastReviewedAt, ReviewIntervalDays: row.ReviewIntervalDays,
			NextReviewDue: row.NextReviewDue, LastReviewedBy: row.LastReviewedBy,
			ReviewNote: row.ReviewNote, DueDate: row.DueDate,
		}))
	}
	return controls, int(total), nil
}

// ListRisksPaged returns a page of risks plus the total count.
func (r *Repository) ListRisksPaged(ctx context.Context, orgID string, offset, limit int) ([]Risk, int, error) {
	total, err := r.q.CountCKRisks(ctx, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("count risks: %w", err)
	}
	rows, err := r.q.ListCKRisksPaged(ctx, db.ListCKRisksPagedParams{
		OrgID:  orgID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list risks paged: %w", err)
	}
	risks := make([]Risk, 0, len(rows))
	for _, row := range rows {
		risks = append(risks, riskFromFields(riskFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Category: row.Category,
			Likelihood: row.Likelihood, Impact: row.Impact, RiskScore: row.RiskScore,
			Owner: row.Owner, Status: row.Status, Treatment: row.Treatment,
			TreatmentNotes:  row.TreatmentNotes,
			TreatmentOption: row.TreatmentOption, TreatmentPlan: row.TreatmentPlan,
			TreatmentOwner: row.TreatmentOwner, TreatmentDueDate: row.TreatmentDueDate,
			TreatmentStatus:    row.TreatmentStatus,
			ResidualLikelihood: row.ResidualLikelihood, ResidualImpact: row.ResidualImpact,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return risks, int(total), nil
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

// ListPoliciesPaged returns a page of policies plus the total count.
func (r *Repository) ListPoliciesPaged(ctx context.Context, orgID string, offset, limit int) ([]Policy, int, error) {
	total, err := r.q.CountCKPolicies(ctx, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("count policies: %w", err)
	}
	rows, err := r.q.ListCKPoliciesPaged(ctx, db.ListCKPoliciesPagedParams{
		OrgID:  orgID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list policies paged: %w", err)
	}
	policies := make([]Policy, 0, len(rows))
	for _, row := range rows {
		policies = append(policies, policyFromFields(policyFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Category: row.Category,
			Status: row.Status, Version: row.Version,
			EffectiveDate: row.EffectiveDate, ReviewDate: row.ReviewDate,
			Owner: row.Owner, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			VersionNum: row.VersionNum, VersionNote: row.VersionNote,
			LastUpdatedBy: row.LastUpdatedBy,
			ReviewedAt:    row.ReviewedAt, NextReviewDue: row.NextReviewDue,
		}))
	}
	return policies, int(total), nil
}

// ListCAPAsPaged returns a page of CAPAs plus the total count.
func (r *Repository) ListCAPAsPaged(ctx context.Context, orgID string, statusFilter string, offset, limit int) ([]CAPA, int, error) {
	statusArg := ckOptText(statusFilter)
	total, err := r.q.CountCKCAPAs(ctx, db.CountCKCAPAsParams{OrgID: orgID, Status: statusArg})
	if err != nil {
		return nil, 0, fmt.Errorf("count capas: %w", err)
	}
	rows, err := r.q.ListCKCAPAsPaged(ctx, db.ListCKCAPAsPagedParams{
		OrgID:  orgID,
		Status: statusArg,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list capas paged: %w", err)
	}
	capas := make([]CAPA, 0, len(rows))
	for _, row := range rows {
		capas = append(capas, capaFromCkCapas(row))
	}
	return capas, int(total), nil
}

// --- Score History ---

// InsertScoreSnapshot inserts a compliance score snapshot for an organisation.
// frameworkID is optional (pass empty string for the org-wide snapshot).
func (r *Repository) InsertScoreSnapshot(ctx context.Context, orgID string, frameworkID *string, score float64, total, implemented int) error {
	var fwID pgtype.UUID
	if frameworkID != nil && *frameworkID != "" {
		fwID = ckOptUUIDFromStr(*frameworkID)
	}
	return r.q.InsertCKScoreSnapshot(ctx, db.InsertCKScoreSnapshotParams{
		OrgID:               orgID,
		FrameworkID:         fwID,
		Score:               float64PtrToNumericCK(&score),
		ControlsTotal:       int32(total),
		ControlsImplemented: int32(implemented),
	})
}

// float64PtrToNumericCK is the secvitals-local copy of the secpulse helper.
func float64PtrToNumericCK(f *float64) pgtype.Numeric {
	if f == nil {
		return pgtype.Numeric{}
	}
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(*f, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}
	}
	return n
}

// ScoreHistoryEntry is a single data point for the score trend chart.
type ScoreHistoryEntry struct {
	Date                string  `json:"date"`
	Score               float64 `json:"score"`
	ControlsTotal       int     `json:"controls_total"`
	ControlsImplemented int     `json:"controls_implemented"`
}

// GetScoreHistory returns aggregated daily score history for an organisation.
// framework_id is nil to query the org-wide score. Days is the look-back window.
func (r *Repository) GetScoreHistory(ctx context.Context, orgID string, days int) ([]ScoreHistoryEntry, error) {
	rows, err := r.q.GetCKScoreHistory(ctx, db.GetCKScoreHistoryParams{
		OrgID: orgID,
		Days:  int32(days),
	})
	if err != nil {
		return nil, fmt.Errorf("get score history: %w", err)
	}
	out := make([]ScoreHistoryEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, ScoreHistoryEntry{
			Date:                row.Date,
			Score:               row.Score,
			ControlsTotal:       int(row.ControlsTotal),
			ControlsImplemented: int(row.ControlsImplemented),
		})
	}
	return out, nil
}

// BulkUpdateControlStatus sets manual_status for all controls in ids that belong to the org.
func (r *Repository) BulkUpdateControlStatus(ctx context.Context, orgID string, ids []string, status string) error {
	if err := r.q.BulkUpdateCKControlStatus(ctx, db.BulkUpdateCKControlStatusParams{
		ManualStatus: ckOptText(status),
		Ids:          ids,
		OrgID:        orgID,
	}); err != nil {
		return fmt.Errorf("bulk update control status: %w", err)
	}
	return nil
}

// BulkUpdateCAPAStatus sets status for all CAPAs in ids that belong to the org.
// Behavior unchanged from original embedded query but jetzt setzt der Query
// auch closed_at = NOW() bei Übergang in 'closed' (Audit-Trail-Konsistenz mit
// UpdateCAPA).
func (r *Repository) BulkUpdateCAPAStatus(ctx context.Context, orgID string, ids []string, status string) error {
	_, err := r.q.BulkUpdateCKCAPAStatus(ctx, db.BulkUpdateCKCAPAStatusParams{
		OrgID:  orgID,
		Status: status,
		Ids:    ids,
	})
	if err != nil {
		return fmt.Errorf("bulk update capa status: %w", err)
	}
	return nil
}
