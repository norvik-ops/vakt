// Package secprivacy provides business logic for DSGVO documentation: VVT, DPIA, AVV,
// breach notifications (Art. 33/34), and data subject requests (Art. 15-21).
package secprivacy

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	fpdf "github.com/go-pdf/fpdf"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/services/crossevidence"
	"github.com/matharnica/vakt/internal/shared/notify"
	"github.com/matharnica/vakt/internal/shared/platform/events"
)

// Service handles PrivacyOps business logic.
type Service struct {
	db          *pgxpool.Pool
	repo        *Repository
	asynqClient *asynq.Client
}

// NewService creates a new PrivacyOps service.
// Pass a zero-value asynq.RedisClientOpt{} if Redis is not available.
func NewService(db *pgxpool.Pool, asynqOpt asynq.RedisClientOpt) *Service {
	var client *asynq.Client
	if asynqOpt.Addr != "" {
		client = asynq.NewClient(asynqOpt)
	}
	return &Service{
		db:          db,
		repo:        NewRepository(db),
		asynqClient: client,
	}
}

// --- VVT ---

// ListVVT returns all VVT entries for the organisation.
// Always returns a non-nil slice so the API response is [] rather than null.
func (s *Service) ListVVT(ctx context.Context, orgID string) ([]VVTEntry, error) {
	entries, err := s.repo.ListVVT(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list vvt: %w", err)
	}
	if entries == nil {
		entries = []VVTEntry{}
	}
	return entries, nil
}

// GetVVT fetches a single VVT entry by ID.
func (s *Service) GetVVT(ctx context.Context, orgID, id string) (*VVTEntry, error) {
	return s.repo.GetVVT(ctx, orgID, id)
}

// CreateVVT inserts a new VVT entry, normalising nil array fields to empty slices
// before persistence so the JSON API always returns arrays rather than null.
func (s *Service) CreateVVT(ctx context.Context, orgID string, in CreateVVTInput) (*VVTEntry, error) {
	if in.DataCategories == nil {
		in.DataCategories = []string{}
	}
	if in.DataSubjects == nil {
		in.DataSubjects = []string{}
	}
	if in.Recipients == nil {
		in.Recipients = []string{}
	}
	return s.repo.CreateVVT(ctx, orgID, in)
}

// UpdateVVT replaces all mutable fields of a VVT entry, normalising nil array fields
// to empty slices before persistence.
func (s *Service) UpdateVVT(ctx context.Context, orgID, id string, in UpdateVVTInput) (*VVTEntry, error) {
	if in.DataCategories == nil {
		in.DataCategories = []string{}
	}
	if in.DataSubjects == nil {
		in.DataSubjects = []string{}
	}
	if in.Recipients == nil {
		in.Recipients = []string{}
	}
	return s.repo.UpdateVVT(ctx, orgID, id, in)
}

// DeleteVVT permanently removes a VVT entry.
func (s *Service) DeleteVVT(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteVVT(ctx, orgID, id)
}

// --- DPIA ---

// ListDPIAs returns all DPIA records for the organisation.
// Always returns a non-nil slice so the API response is [] rather than null.
func (s *Service) ListDPIAs(ctx context.Context, orgID string) ([]DPIA, error) {
	dpias, err := s.repo.ListDPIAs(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list dpias: %w", err)
	}
	if dpias == nil {
		dpias = []DPIA{}
	}
	return dpias, nil
}

// GetDPIA fetches a single DPIA by ID.
func (s *Service) GetDPIA(ctx context.Context, orgID, id string) (*DPIA, error) {
	return s.repo.GetDPIA(ctx, orgID, id)
}

// CreateDPIA persists a new DPIA in "draft" status.
func (s *Service) CreateDPIA(ctx context.Context, orgID string, in CreateDPIAInput) (*DPIA, error) {
	return s.repo.CreateDPIA(ctx, orgID, in)
}

// UpdateDPIA replaces the content fields of a DPIA without changing its approval state.
func (s *Service) UpdateDPIA(ctx context.Context, orgID, id string, in UpdateDPIAInput) (*DPIA, error) {
	return s.repo.UpdateDPIA(ctx, orgID, id, in)
}

// ApproveDPIA marks a DPIA as approved by the given reviewer.
// Art. 35 DSGVO requires documented approval before high-risk processing may begin.
func (s *Service) ApproveDPIA(ctx context.Context, orgID, id, reviewerID string) (*DPIA, error) {
	return s.repo.ApproveDPIA(ctx, orgID, id, reviewerID)
}

// DeleteDPIA permanently removes a DPIA record.
func (s *Service) DeleteDPIA(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteDPIA(ctx, orgID, id)
}

// --- AVV ---

// ListAVVs returns all AVV records for the organisation.
// Always returns a non-nil slice so the API response is [] rather than null.
func (s *Service) ListAVVs(ctx context.Context, orgID string) ([]AVV, error) {
	avvs, err := s.repo.ListAVVs(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list avvs: %w", err)
	}
	if avvs == nil {
		avvs = []AVV{}
	}
	return avvs, nil
}

// GetAVV fetches a single AVV record by ID.
func (s *Service) GetAVV(ctx context.Context, orgID, id string) (*AVV, error) {
	return s.repo.GetAVV(ctx, orgID, id)
}

// CreateAVV persists a new AVV record in "active" status.
func (s *Service) CreateAVV(ctx context.Context, orgID string, in CreateAVVInput) (*AVV, error) {
	return s.repo.CreateAVV(ctx, orgID, in)
}

// UpdateAVV replaces all mutable fields of an AVV record and returns the updated entry.
func (s *Service) UpdateAVV(ctx context.Context, orgID, id string, in UpdateAVVInput) (*AVV, error) {
	return s.repo.UpdateAVV(ctx, orgID, id, in)
}

// DeleteAVV permanently removes an AVV record.
func (s *Service) DeleteAVV(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteAVV(ctx, orgID, id)
}

// ListAVVTemplates returns all built-in AVV templates.
func (s *Service) ListAVVTemplates() []AVVTemplate {
	return BuiltinAVVTemplates()
}

// ListSCCModules returns all EU Standard Contractual Clauses module descriptors.
func (s *Service) ListSCCModules() []SCCModule {
	return BuiltinSCCModules()
}

// CreateAVVFromTemplate substitutes {{vars}} in the template body, inserts a new AVV
// with the rendered body, and returns the persisted record.
func (s *Service) CreateAVVFromTemplate(ctx context.Context, orgID string, in CreateAVVFromTemplateInput) (*AVV, error) {
	templates := BuiltinAVVTemplates()
	var tpl *AVVTemplate
	for i := range templates {
		if templates[i].ID == in.TemplateID {
			tpl = &templates[i]
			break
		}
	}
	if tpl == nil {
		return nil, fmt.Errorf("template %q not found", in.TemplateID)
	}

	body := tpl.Body
	for k, v := range in.Vars {
		body = strings.ReplaceAll(body, "{{"+k+"}}", v)
	}

	// Derive a processor name from the vars if available.
	processorName := in.Vars["auftragnehmer"]
	if processorName == "" {
		processorName = tpl.Title
	}
	serviceDesc := in.Vars["zweck"]
	if serviceDesc == "" {
		serviceDesc = tpl.Description
	}

	return s.repo.CreateAVVWithBody(ctx, orgID, tpl.ID, body, processorName, serviceDesc)
}

// ExportAVVPDF generates a PDF for the given AVV and returns raw bytes and a filename.
func (s *Service) ExportAVVPDF(ctx context.Context, orgID, avvID string) ([]byte, string, error) {
	avv, err := s.repo.GetAVVWithBody(ctx, orgID, avvID)
	if err != nil {
		return nil, "", fmt.Errorf("export avv pdf: %w", err)
	}

	doc := AVVWithBody{
		Name:           avv.ProcessorName,
		ProcessorName:  avv.ProcessorName,
		ControllerName: orgID, // will be replaced by org name at handler level if available
		Purpose:        avv.ServiceDescription,
		Body:           avv.Body,
		CreatedAt:      avv.CreatedAt,
	}

	data, err := GenerateAVVPDF(doc, orgID)
	if err != nil {
		return nil, "", fmt.Errorf("generate avv pdf: %w", err)
	}
	filename := fmt.Sprintf("avv-%s.pdf", avvID)
	return data, filename, nil
}

// UpdateAVVSCC updates the SCC module and annex fields of an AVV.
func (s *Service) UpdateAVVSCC(ctx context.Context, orgID, avvID string, in UpdateAVVSCCInput) error {
	return s.repo.UpdateAVVSCC(ctx, orgID, avvID, in.SCCModule, in.AnnexI, in.AnnexII, in.AnnexIII)
}

// ExportSCCPDF generates a PDF for the SCC-extended AVV and returns raw bytes and a filename.
func (s *Service) ExportSCCPDF(ctx context.Context, orgID, avvID string) ([]byte, string, error) {
	avv, err := s.repo.GetAVVWithBody(ctx, orgID, avvID)
	if err != nil {
		return nil, "", fmt.Errorf("export scc pdf: %w", err)
	}

	doc := AVVWithSCC{
		AVVWithBody: AVVWithBody{
			Name:           avv.ProcessorName,
			ProcessorName:  avv.ProcessorName,
			ControllerName: orgID,
			Purpose:        avv.ServiceDescription,
			Body:           avv.Body,
			CreatedAt:      avv.CreatedAt,
		},
		SCCModule: avv.SCCModule,
		AnnexI:    avv.SCCAnnexI,
		AnnexII:   avv.SCCAnnexII,
		AnnexIII:  avv.SCCAnnexIII,
	}

	data, err := GenerateSCCPDF(doc, orgID)
	if err != nil {
		return nil, "", fmt.Errorf("generate scc pdf: %w", err)
	}
	filename := fmt.Sprintf("scc-%s.pdf", avvID)
	return data, filename, nil
}

// CheckAVVExpiry is intended to be called by the background worker on a schedule.
// It marks past-due active AVVs as "expired" and logs a warning for those
// whose review_date falls within the next 30 days, giving teams time to renew.
func (s *Service) CheckAVVExpiry(ctx context.Context) error {
	threshold := time.Now().UTC().AddDate(0, 0, 30)
	expiring, err := s.repo.ListExpiringAVVs(ctx, threshold)
	if err != nil {
		return fmt.Errorf("check avv expiry: %w", err)
	}

	expired, err := s.repo.MarkExpiredAVVs(ctx)
	if err != nil {
		return fmt.Errorf("mark expired avvs: %w", err)
	}
	if expired > 0 {
		log.Info().Int64("count", expired).Msg("secprivacy: marked AVVs as expired")
	}

	for _, avv := range expiring {
		log.Info().
			Str("avv_id", avv.ID).
			Str("processor", avv.ProcessorName).
			Msg("secprivacy: AVV review approaching")
	}
	return nil
}

// --- Breach ---

// ListBreaches returns all breach records for the organisation.
// Always returns a non-nil slice so the API response is [] rather than null.
func (s *Service) ListBreaches(ctx context.Context, orgID string) ([]Breach, error) {
	breaches, err := s.repo.ListBreaches(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list breaches: %w", err)
	}
	if breaches == nil {
		breaches = []Breach{}
	}
	return breaches, nil
}

// GetBreach fetches a single breach record by ID.
func (s *Service) GetBreach(ctx context.Context, orgID, id string) (*Breach, error) {
	return s.repo.GetBreach(ctx, orgID, id)
}

// CreateBreach persists a new breach record. authority_deadline_at is set to
// DiscoveredAt + 72 hours by the repository (Art. 33 Abs. 1 DSGVO). After a
// successful insert, an incident-creation job is enqueued in SecVitals via the
// shared Asynq queue (fire-and-forget — the breach is saved regardless of queue availability).
func (s *Service) CreateBreach(ctx context.Context, orgID string, in CreateBreachInput) (*Breach, error) {
	breach, err := s.repo.CreateBreach(ctx, orgID, in)
	if err != nil {
		return nil, err
	}
	// Enqueue incident creation in SecVitals (FR-PO05). Fire-and-forget: breach is saved regardless.
	s.publishBreachCreated(ctx, breach)
	notify.Send(ctx, s.db, orgID,
		"Neue Datenpanne erfasst",
		"Eine Datenpanne wurde dokumentiert. Bitte Art.-33-Meldepflicht prüfen (72-Stunden-Frist).",
		"error", "secprivacy")
	return breach, nil
}

// UpdateBreach replaces the editable fields of a breach record.
func (s *Service) UpdateBreach(ctx context.Context, orgID, id string, in UpdateBreachInput) (*Breach, error) {
	if in.DataCategories == nil {
		in.DataCategories = []string{}
	}
	return s.repo.UpdateBreach(ctx, orgID, id, in)
}

// DeleteBreach permanently removes a breach record.
func (s *Service) DeleteBreach(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteBreach(ctx, orgID, id)
}

// MarkAuthorityNotified stamps authority_notified_at to now, recording that the
// supervisory authority was informed within the Art. 33 DSGVO 72-hour window.
func (s *Service) MarkAuthorityNotified(ctx context.Context, id, orgID string) error {
	return s.repo.MarkAuthorityNotified(ctx, id, orgID)
}

// --- DSR ---

// ListDSRs returns all data subject requests for the organisation.
// Always returns a non-nil slice so the API response is [] rather than null.
func (s *Service) ListDSRs(ctx context.Context, orgID string) ([]DSR, error) {
	dsrs, err := s.repo.ListDSRs(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list dsrs: %w", err)
	}
	if dsrs == nil {
		dsrs = []DSR{}
	}
	return dsrs, nil
}

// CreateDSR persists a new data subject request and immediately sends a warning-level
// notification to alert the DPO. The 30-day response deadline (Art. 12 Abs. 3 DSGVO)
// is computed and stored by the repository; the notification message references it.
// The notification is fire-and-forget — a send failure does not abort the DSR creation.
func (s *Service) CreateDSR(ctx context.Context, orgID string, in CreateDSRInput) (*DSR, error) {
	dsr, err := s.repo.CreateDSR(ctx, orgID, in)
	if err != nil {
		return nil, err
	}
	notify.Send(ctx, s.db, orgID,
		"Neue Betroffenenanfrage (DSR) eingegangen",
		"Typ: "+string(in.Type)+". Bitte innerhalb von 30 Tagen bearbeiten.",
		"warning", "secprivacy")
	return dsr, nil
}

// UpdateDSR changes the status and notes of a DSR, stamping completed_at when
// the status moves to "completed" or "rejected".
func (s *Service) UpdateDSR(ctx context.Context, orgID, id string, in UpdateDSRInput) (*DSR, error) {
	dsr, err := s.repo.UpdateDSR(ctx, orgID, id, in)
	if err != nil {
		return nil, err
	}
	// Enqueue cross-module evidence when a DSR is completed.
	if in.Status == "completed" && s.asynqClient != nil {
		if task, taskErr := crossevidence.NewRecordEvidenceTask(events.DSRCompleted(orgID, id)); taskErr == nil {
			_, _ = s.asynqClient.EnqueueContext(ctx, task)
		}
	}
	return dsr, nil
}

// DeleteDSR permanently removes a DSR record. See Repository.DeleteDSR for audit-trail considerations.
func (s *Service) DeleteDSR(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteDSR(ctx, orgID, id)
}

// GenerateBreachNotificationPDF streams an Art. 33 DSGVO notification letter PDF to w.
func (s *Service) GenerateBreachNotificationPDF(ctx context.Context, orgID, breachID string, w io.Writer) error {
	b, err := s.repo.GetBreach(ctx, orgID, breachID)
	if err != nil {
		return fmt.Errorf("get breach for pdf: %w", err)
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	date := time.Now().Format("02.01.2006")

	pdf.SetTitle(tr("Meldung einer Datenschutzverletzung (Art. 33 DSGVO)"), false)
	pdf.SetAuthor("Vakt", false)

	pdf.SetHeaderFunc(func() {
		pdf.SetFont("Helvetica", "I", 8)
		pdf.SetTextColor(140, 140, 140)
		pdf.CellFormat(0, 10, tr("Datenschutzverletzung · Art. 33 DSGVO · "+date), "", 0, "R", false, 0, "")
		pdf.Ln(-1)
		pdf.SetDrawColor(200, 200, 200)
		x := pdf.GetX()
		y := pdf.GetY()
		pdf.Line(x, y, x+190, y)
		pdf.Ln(3)
	})

	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 8)
		pdf.SetTextColor(140, 140, 140)
		pdf.CellFormat(0, 10, tr("Diese Meldung wurde gemäß Art. 33 DSGVO erstellt."), "", 0, "C", false, 0, "")
	})

	// Cover page
	pdf.AddPage()
	pdf.SetY(80)
	pdf.SetFont("Helvetica", "B", 22)
	pdf.SetTextColor(15, 23, 42)
	pdf.MultiCell(0, 12, tr("Meldung einer Datenschutzverletzung"), "", "C", false)
	pdf.SetFont("Helvetica", "", 13)
	pdf.SetTextColor(60, 60, 60)
	pdf.CellFormat(0, 8, tr("gemäß Art. 33 DSGVO"), "", 1, "C", false, 0, "")
	pdf.Ln(10)
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(100, 100, 100)
	pdf.CellFormat(0, 6, tr("Erstellt am: "+date), "", 1, "C", false, 0, "")

	// Detail page
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetTextColor(29, 78, 216)
	pdf.MultiCell(0, 8, tr(b.Title), "", "L", false)
	pdf.SetDrawColor(200, 200, 200)
	pdf.Line(10, pdf.GetY(), 200, pdf.GetY())
	pdf.Ln(4)

	field := func(label, value string) {
		if value == "" {
			return
		}
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(80, 80, 80)
		pdf.CellFormat(65, 5.5, tr(label+":"), "", 0, "LT", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(15, 23, 42)
		pdf.MultiCell(0, 5.5, tr(value), "", "L", false)
	}

	field("Datum der Verletzung", b.DiscoveredAt.Format("02.01.2006 15:04 Uhr"))
	field("Entdeckungsdatum", b.DiscoveredAt.Format("02.01.2006 15:04 Uhr"))
	field("Meldefrist Aufsichtsbehörde", b.AuthorityDeadlineAt.Format("02.01.2006 15:04 Uhr"))

	switch b.Status {
	case "authority_notified":
		field("Status", "Behörde informiert")
	case "closed":
		field("Status", "Geschlossen")
	default:
		field("Status", "Offen")
	}

	field("Beschreibung", b.Description)

	if len(b.DataCategories) > 0 {
		field("Betroffene Datenkategorien", strings.Join(b.DataCategories, ", "))
	}

	if b.AffectedCount != nil {
		field("Anzahl betroffener Personen", fmt.Sprintf("%d", *b.AffectedCount))
	}

	if b.SubjectsNotificationRequired {
		field("Benachrichtigung Betroffene (Art. 34 DSGVO)", "Ja — erforderlich")
		if b.SubjectsNotifiedAt != nil {
			field("Betroffene informiert am", b.SubjectsNotifiedAt.Format("02.01.2006"))
		}
	} else {
		field("Benachrichtigung Betroffene (Art. 34 DSGVO)", "Nein — nicht erforderlich")
	}

	if b.AuthorityNotifiedAt != nil {
		field("Aufsichtsbehörde informiert am", b.AuthorityNotifiedAt.Format("02.01.2006"))
	}

	return pdf.Output(w)
}

// GenerateVVTPDF streams a DSGVO Art. 30-compliant PDF of all active VVT entries to w.
func (s *Service) GenerateVVTPDF(ctx context.Context, orgID string, w io.Writer) error {
	entries, err := s.repo.ListVVT(ctx, orgID)
	if err != nil {
		return fmt.Errorf("list vvt for pdf: %w", err)
	}

	var active []VVTEntry
	for _, e := range entries {
		if e.Status == "active" {
			active = append(active, e)
		}
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	date := time.Now().Format("02.01.2006")

	pdf.SetTitle(tr("Verzeichnis von Verarbeitungstätigkeiten (Art. 30 DSGVO)"), false)
	pdf.SetAuthor("Vakt", false)

	pdf.SetHeaderFunc(func() {
		pdf.SetFont("Helvetica", "I", 8)
		pdf.SetTextColor(140, 140, 140)
		pdf.CellFormat(0, 10, tr("Verzeichnis von Verarbeitungstätigkeiten · Art. 30 DSGVO · "+date), "", 0, "R", false, 0, "")
		pdf.Ln(-1)
		pdf.SetDrawColor(200, 200, 200)
		x := pdf.GetX()
		y := pdf.GetY()
		pdf.Line(x, y, x+190, y)
		pdf.Ln(3)
	})

	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 8)
		pdf.SetTextColor(140, 140, 140)
		pdf.CellFormat(0, 10, fmt.Sprintf("Seite %d", pdf.PageNo()), "", 0, "C", false, 0, "")
	})

	// ── Cover page ──────────────────────────────────────────────────────────────
	pdf.AddPage()
	pdf.SetY(80)
	pdf.SetFont("Helvetica", "B", 22)
	pdf.SetTextColor(15, 23, 42)
	pdf.MultiCell(0, 12, tr("Verzeichnis von Verarbeitungstätigkeiten"), "", "C", false)
	pdf.SetFont("Helvetica", "", 13)
	pdf.SetTextColor(60, 60, 60)
	pdf.CellFormat(0, 8, tr("gemäß Art. 30 DSGVO"), "", 1, "C", false, 0, "")
	pdf.Ln(10)
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(100, 100, 100)
	pdf.CellFormat(0, 6, tr("Erstellt am: "+date), "", 1, "C", false, 0, "")
	pdf.CellFormat(0, 6, tr(fmt.Sprintf("Anzahl aktiver Einträge: %d", len(active))), "", 1, "C", false, 0, "")

	// ── One page per entry ───────────────────────────────────────────────────────
	for i, e := range active {
		pdf.AddPage()

		// Entry heading
		pdf.SetFont("Helvetica", "B", 14)
		pdf.SetTextColor(29, 78, 216)
		pdf.MultiCell(0, 8, tr(fmt.Sprintf("%d. %s", i+1, e.Name)), "", "L", false)

		pdf.SetDrawColor(200, 200, 200)
		pdf.Line(10, pdf.GetY(), 200, pdf.GetY())
		pdf.Ln(4)

		// Field renderer
		field := func(label, value string) {
			if value == "" {
				return
			}
			pdf.SetFont("Helvetica", "B", 9)
			pdf.SetTextColor(80, 80, 80)
			pdf.CellFormat(55, 5.5, tr(label+":"), "", 0, "LT", false, 0, "")
			pdf.SetFont("Helvetica", "", 9)
			pdf.SetTextColor(15, 23, 42)
			pdf.MultiCell(0, 5.5, tr(value), "", "L", false)
		}

		field("Zweck der Verarbeitung", e.Purpose)
		field("Rechtsgrundlage", e.LegalBasis)
		if len(e.DataCategories) > 0 {
			field("Datenkategorien", strings.Join(e.DataCategories, ", "))
		}
		if len(e.DataSubjects) > 0 {
			field("Betroffene Personen", strings.Join(e.DataSubjects, ", "))
		}
		if len(e.Recipients) > 0 {
			field("Empfänger", strings.Join(e.Recipients, ", "))
		}
		field("Löschfrist", e.RetentionPeriod)
		if e.ThirdCountryTransfer {
			field("Drittlandtransfer", "Ja (Übermittlung in Drittland außerhalb EU/EWR)")
			field("Schutzmaßnahmen (Art. 46 DSGVO)", e.Safeguards)
		} else {
			field("Drittlandtransfer", "Nein")
		}
		field("Verantwortliche Person", e.ResponsiblePerson)
		field("Status", "Aktiv")
		field("Erstellt am", e.CreatedAt.Format("02.01.2006"))
	}

	return pdf.Output(w)
}

// GenerateDPIAPDF streams a DSGVO Art. 35-compliant PDF of all DPIA records to w.
// Approved DPIAs are listed first to make the most audit-relevant entries immediately
// visible; the remainder follow in creation-date descending order.
// Each DPIA page includes necessity/proportionality assessment, risk assessment,
// mitigation measures, residual risk, and DPO consultation status as required by Art. 35 Abs. 7.
func (s *Service) GenerateDPIAPDF(ctx context.Context, orgID string, w io.Writer) error {
	all, err := s.repo.ListDPIAs(ctx, orgID)
	if err != nil {
		return fmt.Errorf("list dpias for pdf: %w", err)
	}

	// Sort: approved first, rest by created_at DESC (ListDPIAs already returns DESC order)
	var approved, rest []DPIA
	for _, d := range all {
		if d.Status == "approved" {
			approved = append(approved, d)
		} else {
			rest = append(rest, d)
		}
	}
	entries := append(approved, rest...)

	statusLabel := map[string]string{
		"draft":     "Entwurf",
		"in_review": "In Prüfung",
		"approved":  "Freigegeben",
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	date := time.Now().Format("02.01.2006")

	pdf.SetTitle(tr("Datenschutz-Folgenabschätzung (Art. 35 DSGVO)"), false)
	pdf.SetAuthor("Vakt", false)

	pdf.SetHeaderFunc(func() {
		pdf.SetFont("Helvetica", "I", 8)
		pdf.SetTextColor(140, 140, 140)
		pdf.CellFormat(0, 10, tr("Datenschutz-Folgenabschätzung · Art. 35 DSGVO · "+date), "", 0, "R", false, 0, "")
		pdf.Ln(-1)
		pdf.SetDrawColor(200, 200, 200)
		x := pdf.GetX()
		y := pdf.GetY()
		pdf.Line(x, y, x+190, y)
		pdf.Ln(3)
	})

	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 8)
		pdf.SetTextColor(140, 140, 140)
		pdf.CellFormat(0, 10, fmt.Sprintf("Seite %d", pdf.PageNo()), "", 0, "C", false, 0, "")
	})

	// ── Cover page ──────────────────────────────────────────────────────────────
	pdf.AddPage()
	pdf.SetY(80)
	pdf.SetFont("Helvetica", "B", 22)
	pdf.SetTextColor(15, 23, 42)
	pdf.MultiCell(0, 12, tr("Datenschutz-Folgenabschätzung"), "", "C", false)
	pdf.SetFont("Helvetica", "", 13)
	pdf.SetTextColor(60, 60, 60)
	pdf.CellFormat(0, 8, tr("gemäß Art. 35 DSGVO"), "", 1, "C", false, 0, "")
	pdf.Ln(10)
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(100, 100, 100)
	pdf.CellFormat(0, 6, tr("Erstellt am: "+date), "", 1, "C", false, 0, "")
	pdf.CellFormat(0, 6, tr(fmt.Sprintf("Anzahl DSFAs: %d", len(entries))), "", 1, "C", false, 0, "")

	// ── One page per DPIA ───────────────────────────────────────────────────────
	for i, d := range entries {
		pdf.AddPage()

		// Entry heading
		pdf.SetFont("Helvetica", "B", 14)
		pdf.SetTextColor(29, 78, 216)
		pdf.MultiCell(0, 8, tr(fmt.Sprintf("%d. %s", i+1, d.Title)), "", "L", false)

		pdf.SetDrawColor(200, 200, 200)
		pdf.Line(10, pdf.GetY(), 200, pdf.GetY())
		pdf.Ln(4)

		// Field renderer
		field := func(label, value string) {
			if value == "" {
				return
			}
			pdf.SetFont("Helvetica", "B", 9)
			pdf.SetTextColor(80, 80, 80)
			pdf.CellFormat(60, 5.5, tr(label+":"), "", 0, "LT", false, 0, "")
			pdf.SetFont("Helvetica", "", 9)
			pdf.SetTextColor(15, 23, 42)
			pdf.MultiCell(0, 5.5, tr(value), "", "L", false)
		}

		label, ok := statusLabel[d.Status]
		if !ok {
			label = d.Status
		}
		field("Status", label)
		if d.VVTEntryID != nil && *d.VVTEntryID != "" {
			field("Verknüpfter VVT-Eintrag (ID)", *d.VVTEntryID)
		}
		field("Beschreibung", d.Description)
		field("Erforderlichkeit & Verhältnismäßigkeit", d.NecessityAssessment)
		field("Risikobewertung", d.RiskAssessment)
		field("Abhilfemaßnahmen", d.MitigationMeasures)
		field("Restrisiko", d.ResidualRisk)
		if d.DPOConsultation {
			field("DSB konsultiert", "Ja")
		} else {
			field("DSB konsultiert", "Nein")
		}
		field("Erstellt am", d.CreatedAt.Format("02.01.2006"))
		if d.ReviewedAt != nil {
			field("Freigegeben am", d.ReviewedAt.Format("02.01.2006"))
		}
	}

	return pdf.Output(w)
}

// publishBreachCreated enqueues a TaskBreachIncidentCreate job so the worker
// can create a linked incident in SecVitals without a cross-module import.
func (s *Service) publishBreachCreated(ctx context.Context, b *Breach) {
	payload := BreachIncidentPayload{
		OrgID:        b.OrgID,
		BreachID:     b.ID,
		Title:        b.Title,
		Description:  b.Description,
		DiscoveredAt: b.DiscoveredAt,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Str("breach_id", b.ID).Msg("secprivacy: failed to marshal breach incident payload")
		return
	}
	if s.asynqClient != nil {
		task := asynq.NewTask(TaskBreachIncidentCreate, data)
		if _, err := s.asynqClient.EnqueueContext(ctx, task, asynq.Queue(Queue)); err != nil {
			log.Error().Err(err).Str("breach_id", b.ID).Msg("secprivacy: failed to enqueue breach incident job")
		}
	} else {
		log.Info().
			Str("event_type", "secprivacy.breach.created").
			Str("breach_id", b.ID).
			Str("org_id", b.OrgID).
			Msg("secprivacy: breach created (Redis unavailable — incident not auto-created)")
	}
}

// generateToken creates a cryptographically random 32-byte token.
// Returns (rawToken hex, tokenHash hex, error).
func generateToken() (rawToken, tokenHash string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	rawToken = hex.EncodeToString(raw)
	h := sha256.Sum256([]byte(rawToken))
	tokenHash = hex.EncodeToString(h[:])
	return rawToken, tokenHash, nil
}

// GetDSRPortalInfo returns public info about a DSR portal identified by its slug.
func (s *Service) GetDSRPortalInfo(ctx context.Context, slug string) (*DSRPortalInfo, error) {
	_, orgName, _, intro, enabled, err := s.repo.GetOrgByDSRSlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("get dsr portal info: %w", err)
	}
	return &DSRPortalInfo{OrgName: orgName, Slug: slug, Intro: intro, Enabled: enabled}, nil
}

// SubmitPortalDSR validates the slug, generates tokens, and persists the DSR.
func (s *Service) SubmitPortalDSR(ctx context.Context, slug string, in PortalDSRInput, ip string) (string, error) {
	orgID, _, dpoEmail, _, enabled, err := s.repo.GetOrgByDSRSlug(ctx, slug)
	if err != nil {
		return "", fmt.Errorf("portal submit: org not found for slug %q: %w", slug, err)
	}
	if !enabled {
		return "", errors.New("portal submit: DSR portal is not enabled")
	}
	rawStatus, statusHash, err := generateToken()
	if err != nil {
		return "", err
	}
	_, verifyHash, err := generateToken()
	if err != nil {
		return "", err
	}
	if _, err = s.repo.CreatePortalDSR(ctx, orgID, in, statusHash, verifyHash, ip); err != nil {
		return "", err
	}
	notify.Send(ctx, s.db, orgID,
		"Neue DSR-Anfrage über Self-Service-Portal",
		"Typ: "+in.Type+". Antragsteller: "+in.FirstName+" "+in.LastName+" ("+in.Email+"). DPO: "+dpoEmail,
		"warning", "secprivacy")
	return rawStatus, nil
}

// GetPortalDSR looks up a DSR using the raw status token.
func (s *Service) GetPortalDSR(ctx context.Context, rawToken string) (*DSR, error) {
	h := sha256.Sum256([]byte(rawToken))
	return s.repo.GetDSRByTokenHash(ctx, hex.EncodeToString(h[:]))
}

// GetDSRPortalSettings returns the org's portal configuration.
func (s *Service) GetDSRPortalSettings(ctx context.Context, orgID string) (*UpdateDSRPortalSettingsInput, error) {
	return s.repo.GetDSRPortalSettings(ctx, orgID)
}

// UpdateDSRPortalSettings persists new portal settings.
func (s *Service) UpdateDSRPortalSettings(ctx context.Context, orgID string, in UpdateDSRPortalSettingsInput) error {
	return s.repo.UpdateDSRPortalSettings(ctx, orgID, in)
}

// --- Paginated list methods ---

// ListVVTPaged returns a page of VVT entries plus the total count.
func (s *Service) ListVVTPaged(ctx context.Context, orgID string, offset, limit int) ([]VVTEntry, int, error) {
	return s.repo.ListVVTPaged(ctx, orgID, offset, limit)
}

// ListBreachesPaged returns a page of breach records plus the total count.
func (s *Service) ListBreachesPaged(ctx context.Context, orgID string, offset, limit int) ([]Breach, int, error) {
	return s.repo.ListBreachesPaged(ctx, orgID, offset, limit)
}

// ListDSRsCursor returns DSRs using keyset pagination.
func (s *Service) ListDSRsCursor(ctx context.Context, orgID string, cursorID string, cursorTS time.Time, limit int) ([]DSR, error) {
	return s.repo.ListDSRsCursor(ctx, orgID, cursorID, cursorTS, limit)
}
