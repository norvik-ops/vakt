package vaktcomply

import (
	"context"
	"errors"
	"fmt"
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
	// S61-4 residual fields (Migration 164) — nil when populated via sqlc (query not regenerated)
	InherentLikelihood          pgtype.Int4
	InherentImpact              pgtype.Int4
	RiskAcceptedBy              pgtype.UUID
	RiskAcceptedAt              pgtype.Timestamptz
	RiskAcceptanceJustification pgtype.Text
	CreatedAt, UpdatedAt        pgtype.Timestamptz
}

func intPtrFromInt4(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int32)
	return &i
}

func riskFromFields(f riskFields) Risk {
	r := Risk{
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
		// S61-4 residual fields
		InherentLikelihood: intPtrFromInt4(f.InherentLikelihood),
		InherentImpact:     intPtrFromInt4(f.InherentImpact),
		CreatedAt:          ckTsToTime(f.CreatedAt),
		UpdatedAt:          ckTsToTime(f.UpdatedAt),
	}
	if f.RiskAcceptedBy.Valid {
		s := f.RiskAcceptedBy.Bytes
		str := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
			s[0:4], s[4:6], s[6:8], s[8:10], s[10:16])
		r.RiskAcceptedBy = &str
	}
	r.RiskAcceptedAt = ckTsToTimePtr(f.RiskAcceptedAt)
	if f.RiskAcceptanceJustification.Valid {
		r.RiskAcceptanceJustification = f.RiskAcceptanceJustification.String
	}
	r.ComputeScores()
	return r
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

// optTextStrPtr converts *string to pgtype.Text (nil → invalid, *"" → valid empty).
func optTextStrPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// ckOptUUIDFromPtr converts *string to pgtype.UUID; nil/empty → invalid.
func ckOptUUIDFromPtr(s *string) pgtype.UUID {
	if s == nil || *s == "" {
		return pgtype.UUID{}
	}
	return ckOptUUIDFromStr(*s)
}

// uuidStringFromPgtype returns the UUID as string ("" when invalid).
func uuidStringFromPgtype(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return u.String()
}

// policyDateFromTimePtr converts *time.Time → pgtype.Date.
func policyDateFromTimePtr(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
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
