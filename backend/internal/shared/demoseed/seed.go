// Package demoseed populates a fresh database with realistic demo data so the
// platform can be explored immediately after `docker compose up`.
//
// Activate with: VAKT_DEMO=true
package demoseed

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"
)

// EphemeralSession holds the credentials of a freshly seeded ephemeral demo org.
//
// Wichtig: Die Klartext-Passwörter sind hier NUR enthalten weil der Server
// sie unmittelbar nach Erzeugung an das Frontend zurückgeben muss (sonst
// kann sich niemand einloggen — die Bcrypt-Hashes sind nicht reversibel).
// Nach dem Response werden die Klartext-Werte verworfen. In der DB
// liegen nur die Hashes (Cost 12).
type EphemeralSession struct {
	OrgID           string
	AdminID         string
	Roles           []string
	AdminPassword   string
	AnalystPassword string
}

// randomHex returns n random bytes encoded as a hex string (2n characters).
func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// randomPassword returns n random URL-safe base64 characters suitable for use
// as a temporary password. Uses crypto/rand for cryptographic randomness.
func randomPassword(n int) string {
	// Each base64 character encodes 6 bits; we need ceil(n*6/8) raw bytes.
	raw := make([]byte, (n*6+7)/8)
	rand.Read(raw)
	s := hex.EncodeToString(raw) // hex gives us only [0-9a-f], always URL-safe
	if len(s) > n {
		s = s[:n]
	}
	return s
}

// Run seeds the shared "demo" org (idempotent — skips if slug "demo" exists).
//
// Hinweis: Diese statische Variante wird primär für lokale Entwicklung und
// Tests genutzt. Auf `secdemo.norvikops.de` werden pro Visitor ephemere
// Sessions per `RunEphemeral` erzeugt (Random-Passwörter, 4h-Lifetime).
// Passwörter müssen ≥ 10 Zeichen sein (auth-Validierung in service.go).
func Run(ctx context.Context, db *pgxpool.Pool, masterKeyHex string) error {
	var exists bool
	if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM organizations WHERE slug = 'demo')`).Scan(&exists); err != nil {
		return fmt.Errorf("demoseed: check: %w", err)
	}
	if exists {
		log.Info().Msg("demoseed: demo org already exists, skipping")
		return nil
	}
	_, _, err := runSeed(ctx, db, masterKeyHex, "Musterfirma GmbH", "demo", "admin@vakt.local", "analyst@vakt.local", "admin1234demo", "analyst1234demo")
	return err
}

// RunEphemeral creates a new isolated demo org with a unique slug and returns
// the org/user IDs needed to issue an auth token. Passwords are randomly
// generated (16 hex chars) so that ephemeral sessions are not guessable.
func RunEphemeral(ctx context.Context, db *pgxpool.Pool, masterKeyHex string) (*EphemeralSession, error) {
	slug := "demo-" + randomHex(4) // 8 hex chars, e.g. "demo-a3f2b1c9"
	adminEmail := "admin@" + slug + ".demo"
	analystEmail := "analyst@" + slug + ".demo"
	adminPwd := randomPassword(16)
	analystPwd := randomPassword(16)
	orgID, adminID, err := runSeed(ctx, db, masterKeyHex, "Demo-Umgebung", slug, adminEmail, analystEmail, adminPwd, analystPwd)
	if err != nil {
		return nil, err
	}
	return &EphemeralSession{
		OrgID:           orgID,
		AdminID:         adminID,
		Roles:           []string{"admin"},
		AdminPassword:   adminPwd,
		AnalystPassword: analystPwd,
	}, nil
}

// runSeed creates an org and seeds all demo data inside a single transaction.
// Returns the created orgID and adminID.
func runSeed(ctx context.Context, db *pgxpool.Pool, masterKeyHex, orgName, orgSlug, adminEmail, analystEmail, adminPwd, analystPwd string) (orgID, adminID string, err error) {
	log.Info().Str("slug", orgSlug).Msg("demoseed: seeding demo data...")

	tx, err := db.Begin(ctx)
	if err != nil {
		return "", "", fmt.Errorf("demoseed: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// ── Organisation ──────────────────────────────────────────────────────────
	if err := tx.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ($1, $2)
		RETURNING id::text`, orgName, orgSlug).Scan(&orgID); err != nil {
		return "", "", fmt.Errorf("demoseed: org: %w", err)
	}

	// ── Roles ─────────────────────────────────────────────────────────────────
	var adminRoleID, analystRoleID string
	tx.QueryRow(ctx, `SELECT id::text FROM roles WHERE name = 'Admin'`).Scan(&adminRoleID)
	tx.QueryRow(ctx, `SELECT id::text FROM roles WHERE name = 'SecurityAnalyst'`).Scan(&analystRoleID)

	// ── Users ─────────────────────────────────────────────────────────────────
	adminHash, _ := bcrypt.GenerateFromPassword([]byte(adminPwd), 12)
	analystHash, _ := bcrypt.GenerateFromPassword([]byte(analystPwd), 12)

	var analystID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ($1, $2, 'Max Mustermann')
		RETURNING id::text`, adminEmail, string(adminHash)).Scan(&adminID); err != nil {
		return "", "", fmt.Errorf("demoseed: admin user: %w", err)
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ($1, $2, 'Anna Analyst')
		RETURNING id::text`, analystEmail, string(analystHash)).Scan(&analystID); err != nil {
		return "", "", fmt.Errorf("demoseed: analyst user: %w", err)
	}

	// Org memberships.
	if _, err := tx.Exec(ctx, `
		INSERT INTO org_members (org_id, user_id, role_id) VALUES
		($1::uuid, $2::uuid, $3::uuid),
		($1::uuid, $4::uuid, $5::uuid)`,
		orgID, adminID, adminRoleID, analystID, analystRoleID); err != nil {
		return "", "", fmt.Errorf("demoseed: org_members: %w", err)
	}

	// ── SLA config ────────────────────────────────────────────────────────────
	if _, err := tx.Exec(ctx, `
		INSERT INTO vb_sla_config (org_id, critical_days, high_days, medium_days, low_days)
		VALUES ($1::uuid, 7, 30, 90, 180)`, orgID); err != nil {
		return "", "", fmt.Errorf("demoseed: sla_config: %w", err)
	}

	// ── Assets ────────────────────────────────────────────────────────────────
	assetIDs := make([]string, 0, 5)
	type asset struct {
		name, typ, crit string
		tags            []string
	}
	assets := []asset{
		{"Produktions-Webserver", "server", "critical", []string{"prod", "extern", "nis2"}},
		{"Datenbank-Cluster", "server", "critical", []string{"prod", "intern", "pii"}},
		{"API-Gateway", "container", "high", []string{"prod", "extern"}},
		{"Dev-Server", "server", "medium", []string{"intern", "dev"}},
		{"Haupt-Repository", "repo", "high", []string{"intern", "sourcecode"}},
	}
	for _, a := range assets {
		var id string
		if err := tx.QueryRow(ctx, `
			INSERT INTO vb_assets (org_id, name, type, criticality, tags, owner_id)
			VALUES ($1::uuid, $2, $3, $4, $5, $6::uuid)
			RETURNING id::text`,
			orgID, a.name, a.typ, a.crit, a.tags, adminID).Scan(&id); err != nil {
			return "", "", fmt.Errorf("demoseed: asset %s: %w", a.name, err)
		}
		assetIDs = append(assetIDs, id)
	}

	// ── Scan ──────────────────────────────────────────────────────────────────
	var scanID string
	tx.QueryRow(ctx, `
		INSERT INTO vb_scans (org_id, asset_id, scanner, status, started_at, completed_at)
		VALUES ($1::uuid, $2::uuid, 'trivy', 'completed', now()-interval '2 days', now()-interval '2 days'+interval '12 minutes')
		RETURNING id::text`, orgID, assetIDs[0]).Scan(&scanID)

	// ── Findings ──────────────────────────────────────────────────────────────
	type finding struct {
		assetIdx   int
		cve, title string
		sev        string
		cvss       float64
		status     string
		daysAgo    int
		slaDays    int
	}
	findings := []finding{
		{0, "CVE-2024-1234", "OpenSSL Heap-Buffer-Overflow (kritisch)", "critical", 9.8, "open", 10, 7},
		{0, "CVE-2024-5678", "Apache Log4j RCE via JNDI Lookup", "critical", 10.0, "in_progress", 5, 7},
		{1, "CVE-2024-2222", "PostgreSQL Privilege Escalation", "high", 8.1, "open", 20, 30},
		{1, "CVE-2023-9999", "Unsichere Backup-Konfiguration", "high", 7.5, "accepted_risk", 45, 30},
		{2, "CVE-2024-3333", "JWT Algorithmus-Verwechslung (none-Angriff)", "critical", 9.1, "open", 3, 7},
		{2, "", "Veraltete nginx Version (1.18)", "medium", 5.3, "open", 15, 90},
		{3, "CVE-2023-8888", "SSH Brute-Force kein Rate-Limit", "medium", 6.5, "resolved", 30, 90},
		{3, "", "Veraltete Python-Abhängigkeiten (requests 2.26)", "low", 3.1, "open", 25, 180},
		{4, "", "Hartcodierte AWS-Zugangsdaten in Commit-History", "critical", 9.5, "in_progress", 2, 7},
		{4, "", "Fehlende .gitignore für .env-Dateien", "medium", 5.0, "resolved", 60, 90},
		{0, "CVE-2024-6666", "TLS 1.0/1.1 noch aktiviert", "medium", 5.9, "open", 8, 90},
		{1, "", "Standard-Passwörter in Test-DB", "high", 8.0, "resolved", 90, 30},
	}

	now := time.Now()
	for _, f := range findings {
		createdAt := now.AddDate(0, 0, -f.daysAgo)
		slaDeadline := createdAt.AddDate(0, 0, f.slaDays)
		var cvePtr *string
		if f.cve != "" {
			cvePtr = &f.cve
		}
		var scanPtr *string
		if scanID != "" && f.assetIdx == 0 {
			scanPtr = &scanID
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO vb_findings
				(org_id, asset_id, scan_id, cve_id, title, severity, cvss_score, status,
				 scanner, risk_score, sla_due_at, created_at, updated_at, last_seen_at)
			VALUES ($1::uuid,$2::uuid,$3::uuid,$4,$5,$6,$7,$8,'trivy',$9,$10,$11,$11,$11)`,
			orgID, assetIDs[f.assetIdx], scanPtr, cvePtr, f.title, f.sev, f.cvss,
			f.status, int(f.cvss*10), slaDeadline, createdAt); err != nil {
			return "", "", fmt.Errorf("demoseed: finding %q: %w", f.title, err)
		}
	}

	// ── SecPrivacy: VVT ───────────────────────────────────────────────────────
	vvtEntries := []struct{ name, purpose, basis string }{
		{"Kundendaten CRM", "Verwaltung von Kundenbeziehungen und Vertragsdaten", "Art. 6 Abs. 1 lit. b DSGVO (Vertragserfüllung)"},
		{"Mitarbeiterdaten HR", "Personalverwaltung und Gehaltsabrechnung", "Art. 6 Abs. 1 lit. c DSGVO (rechtliche Verpflichtung)"},
		{"Website-Analytics", "Analyse des Nutzerverhaltens zur Produktverbesserung", "Art. 6 Abs. 1 lit. a DSGVO (Einwilligung)"},
	}
	for _, v := range vvtEntries {
		if _, err := tx.Exec(ctx, `
			INSERT INTO po_vvt_entries
				(org_id, name, purpose, legal_basis, data_categories, data_subjects,
				 recipients, retention_period, responsible_person, status)
			VALUES ($1::uuid,$2,$3,$4,
				ARRAY['Stammdaten','Kontaktdaten'],
				ARRAY['Kunden','Mitarbeiter'],
				ARRAY['Steuerberater','IT-Dienstleister'],
				'3 Jahre nach Vertragsende', 'Datenschutzbeauftragter', 'active')`,
			orgID, v.name, v.purpose, v.basis); err != nil {
			return "", "", fmt.Errorf("demoseed: vvt %q: %w", v.name, err)
		}
	}

	// ── SecPrivacy: DSRs ──────────────────────────────────────────────────────
	dsrs := []struct {
		name, email, typ, status string
		daysAgo                  int
	}{
		{"Hans Müller", "h.mueller@example.de", "access", "in_progress", 15},
		{"Maria Schmidt", "m.schmidt@example.de", "erasure", "open", 5},
		{"Klaus Weber", "k.weber@example.com", "portability", "completed", 35},
		{"Petra Bauer", "p.bauer@example.de", "objection", "open", 2},
	}
	for _, d := range dsrs {
		receivedAt := now.AddDate(0, 0, -d.daysAgo)
		dueDate := receivedAt.AddDate(0, 0, 30)
		var completedAt *time.Time
		if d.status == "completed" {
			t := receivedAt.AddDate(0, 0, 20)
			completedAt = &t
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO po_dsr (org_id, requester_name, requester_email, type, status,
				due_date, received_at, completed_at)
			VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8)`,
			orgID, d.name, d.email, d.typ, d.status,
			dueDate.Format("2006-01-02"), receivedAt, completedAt); err != nil {
			return "", "", fmt.Errorf("demoseed: dsr %q: %w", d.name, err)
		}
	}

	// ── SecPrivacy: Breach ────────────────────────────────────────────────────
	if _, err := tx.Exec(ctx, `
		INSERT INTO po_breaches
			(org_id, title, description, discovered_at, authority_deadline_at,
			 authority_notified_at, affected_count, data_categories, status)
		VALUES ($1::uuid,
			'Unbefugter Datenbankzugriff (Test-System)',
			'Ein falsch konfiguriertes Test-System war 48 Stunden ohne Authentifizierung erreichbar. Kontaktdaten von ca. 230 Testnutzern waren einsehbar.',
			now()-interval '60 days',
			now()-interval '57 days',
			now()-interval '58 days',
			230,
			ARRAY['Kontaktdaten','E-Mail-Adressen'],
			'closed')`, orgID); err != nil {
		return "", "", fmt.Errorf("demoseed: breach: %w", err)
	}

	// ── SecPrivacy: AVV ───────────────────────────────────────────────────────
	if _, err := tx.Exec(ctx, `
		INSERT INTO po_avvs (org_id, processor_name, service_description, contract_date, review_date, status)
		VALUES
		($1::uuid, 'Cloudflare Inc.', 'CDN und DDoS-Schutz für Web-Präsenz', '2023-01-15', '2025-01-15', 'active'),
		($1::uuid, 'Mailchimp (Intuit)', 'E-Mail-Marketing für Newsletter-Versand', '2022-06-01', '2024-06-01', 'expired')`,
		orgID); err != nil {
		return "", "", fmt.Errorf("demoseed: avv: %w", err)
	}

	// ── SecVitals: Risks ──────────────────────────────────────────────────────
	risks := []struct {
		title, category    string
		likelihood, impact int
		status, treatment  string
	}{
		{"Datenverlust durch Ransomware", "Informationssicherheit", 3, 5, "open", "mitigate"},
		{"Ausfall Produktionsdatenbank", "Verfügbarkeit", 2, 5, "mitigated", "mitigate"},
		{"Phishing-Angriff auf Mitarbeiter", "Awareness", 4, 3, "open", "mitigate"},
		{"Compliance-Verstoß DSGVO Art. 32", "Datenschutz", 2, 4, "open", "transfer"},
	}
	for _, r := range risks {
		if _, err := tx.Exec(ctx, `
			INSERT INTO ck_risks (org_id, title, description, category, likelihood, impact,
				owner, status, treatment, created_by)
			VALUES ($1::uuid,$2,'Identifiziert im jährlichen Risikoassessment.',$3,$4,$5,'CISO',$6,$7,$8::uuid)`,
			orgID, r.title, r.category, r.likelihood, r.impact, r.status, r.treatment, adminID); err != nil {
			return "", "", fmt.Errorf("demoseed: risk %q: %w", r.title, err)
		}
	}

	// ── SecVitals: Incidents ──────────────────────────────────────────────────
	incidents := []struct {
		title, sev, status string
		daysAgo            int
	}{
		{"Phishing-Mail: Zugangsdaten eines Mitarbeiters kompromittiert", "high", "resolved", 45},
		{"DDoS-Angriff auf Web-Präsenz (30 Min. Ausfall)", "medium", "resolved", 120},
		{"Fehlkonfiguration S3-Bucket — Daten kurzzeitig öffentlich", "critical", "closed", 200},
	}
	for _, inc := range incidents {
		discoveredAt := time.Now().AddDate(0, 0, -inc.daysAgo)
		if _, err := tx.Exec(ctx, `
			INSERT INTO ck_incidents (org_id, title, description, severity, status, discovered_at, created_by)
			VALUES ($1::uuid, $2, 'Entdeckt durch internes Monitoring. Sofortmaßnahmen wurden eingeleitet.', $3, $4, $5, $6::uuid)`,
			orgID, inc.title, inc.sev, inc.status, discoveredAt, adminID); err != nil {
			return "", "", fmt.Errorf("demoseed: incident %q: %w", inc.title, err)
		}
	}

	// ── SecVitals: Policies ───────────────────────────────────────────────────
	policies := []struct{ title, category, status, owner string }{
		{"Informationssicherheits-Richtlinie", "Informationssicherheit", "active", "CISO"},
		{"Passwort- und Zugangsverwaltung", "Zugriffskontrolle", "active", "IT-Leiter"},
		{"Mobiles Arbeiten und Homeoffice", "Betrieb", "active", "HR"},
		{"Incident Response Plan", "Notfallmanagement", "active", "CISO"},
		{"Datenschutzrichtlinie (DSGVO)", "Datenschutz", "draft", "Datenschutzbeauftragter"},
	}
	for _, p := range policies {
		if _, err := tx.Exec(ctx, `
			INSERT INTO ck_policies (org_id, title, description, category, status, version, effective_date, owner, created_by)
			VALUES ($1::uuid, $2, 'Verbindliche Regelung für alle Mitarbeiter und externen Dienstleister.', $3, $4, '1.2', CURRENT_DATE - INTERVAL '180 days', $5, $6::uuid)`,
			orgID, p.title, p.category, p.status, p.owner, adminID); err != nil {
			return "", "", fmt.Errorf("demoseed: policy %q: %w", p.title, err)
		}
	}

	// ── SecVitals: Audits ─────────────────────────────────────────────────────
	audits := []struct {
		title, auditor, status string
		daysAgo                int
	}{
		{"Internes Audit ISO 27001 Annex A", "Interne Revision", "completed", 90},
		{"NIS2-Readiness-Assessment", "Extern: SecAudit GmbH", "completed", 180},
		{"Penetrationstest Web-Applikationen", "Extern: RedTeam AG", "in_progress", 10},
	}
	for _, a := range audits {
		auditDate := time.Now().AddDate(0, 0, -a.daysAgo)
		if _, err := tx.Exec(ctx, `
			INSERT INTO ck_audit_records (org_id, title, scope, auditor, audit_date, status, findings, recommendations, created_by)
			VALUES ($1::uuid, $2, 'Gesamte IT-Infrastruktur und Prozesse', $3, $4, $5,
				'Mehrere Kontrollen mit Handlungsbedarf identifiziert.',
				'Priorisierung der offenen Maßnahmen bis Q2.', $6::uuid)`,
			orgID, a.title, a.auditor, auditDate, a.status, adminID); err != nil {
			return "", "", fmt.Errorf("demoseed: audit %q: %w", a.title, err)
		}
	}

	// ── SecPrivacy: DPIA ──────────────────────────────────────────────────────
	dpias := []struct{ title, necessity, risk, mitigation, residual, status string }{
		{
			"DPIA: KI-gestützte Bewerberauswahl",
			"Der Einsatz von KI-Algorithmen zur Vorauswahl von Bewerbungen verarbeitet sensible Profildaten und birgt Diskriminierungsrisiken gemäß Art. 22 DSGVO.",
			"Hohes Risiko durch automatisierte Entscheidungsfindung: mögliche Benachteiligung aufgrund von Alter, Geschlecht oder Herkunft.",
			"Einsatz erklärbarer KI-Modelle, regelmäßige Bias-Audits, Opt-out-Möglichkeit für Bewerber, Dokumentation aller Entscheidungen.",
			"Restrisiko gering nach Umsetzung der Maßnahmen. Quartalsweise Überprüfung durch DPO.",
			"approved",
		},
		{
			"DPIA: Videoüberwachung Betriebsgelände",
			"Überwachung des Eingangsbereichs und Lagers zur Einbruchprävention. Verarbeitung von Bildaufnahmen von Mitarbeitern und Besuchern.",
			"Mittleres Risiko: Eingriff in die Persönlichkeitsrechte der Mitarbeiter, mögliche verdeckte Überwachung.",
			"Hinweisschilder, Speicherdauer auf 72 h begrenzt, Zugriff nur für Sicherheitsverantwortliche, Betriebsvereinbarung abgeschlossen.",
			"Restrisiko akzeptabel. Nächste Überprüfung in 12 Monaten.",
			"approved",
		},
	}
	for _, d := range dpias {
		if _, err := tx.Exec(ctx, `
			INSERT INTO po_dpias (org_id, title, description, necessity_assessment, risk_assessment,
				mitigation_measures, residual_risk, status)
			VALUES ($1::uuid, $2, 'Durchgeführt gemäß Art. 35 DSGVO.', $3, $4, $5, $6, $7)`,
			orgID, d.title, d.necessity, d.risk, d.mitigation, d.residual, d.status); err != nil {
			return "", "", fmt.Errorf("demoseed: dpia %q: %w", d.title, err)
		}
	}

	// ── SecVitals: NIS2-Framework ─────────────────────────────────────────────
	var frameworkID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO ck_frameworks (org_id, name, version, is_builtin)
		VALUES ($1::uuid, 'NIS2-Richtlinie (EU 2022/2555)', '2022', true)
		RETURNING id::text`, orgID).Scan(&frameworkID); err != nil {
		return "", "", fmt.Errorf("demoseed: framework: %w", err)
	}
	controls := []struct{ id, title, domain, desc string }{
		{"NIS2-5.1", "Risikomanagement-Richtlinie", "Risikomanagement",
			"Einführung und Umsetzung einer dokumentierten Richtlinie für das IT-Risikomanagement."},
		{"NIS2-5.2", "Risikoidentifikation und -bewertung", "Risikomanagement",
			"Systematische Identifikation, Analyse und Bewertung von Risiken für Netz- und Informationssysteme."},
		{"NIS2-6.1", "Meldepflicht: Erhebliche Sicherheitsvorfälle", "Incident Management",
			"Meldung erheblicher Vorfälle an die zuständige Behörde innerhalb von 24 h (Erstmeldung) und 72 h (Folgemeldung)."},
		{"NIS2-6.2", "Incident-Response-Plan", "Incident Management",
			"Dokumentierter und getesteter Plan zur Reaktion auf Sicherheitsvorfälle inkl. Kommunikationsketten."},
		{"NIS2-7.1", "Business Continuity Management", "Betriebskontinuität",
			"Backup-Strategien, Notfallwiederherstellung und Krisenmanagement für kritische Systeme."},
		{"NIS2-8.1", "Sicherheit der Lieferkette", "Lieferkette",
			"Bewertung und Überwachung von Sicherheitsrisiken durch Drittanbieter und Dienstleister."},
		{"NIS2-9.1", "Zugriffskontrolle und Least Privilege", "Zugriffskontrolle",
			"Rollenbasierte Zugriffskontrolle, Minimalprinzip und privilegierte Zugänge mit MFA."},
		{"NIS2-9.2", "Multi-Faktor-Authentifizierung", "Zugriffskontrolle",
			"Verpflichtende MFA für alle privilegierten Konten und Remote-Zugänge."},
		{"NIS2-10.1", "Kryptographie und Schlüsselverwaltung", "Kryptographie",
			"Einsatz geeigneter Verschlüsselung für Daten in Ruhe und in Übertragung. Dokumentierte Schlüsselverwaltung."},
		{"NIS2-11.1", "Security Awareness Training", "Personalmaßnahmen",
			"Regelmäßige Schulungen aller Mitarbeiter zu Phishing, Social Engineering und sicherem Umgang mit Daten."},
		{"NIS2-12.1", "Schwachstellenmanagement", "Technische Sicherheit",
			"Systematische Identifikation, Priorisierung und Behebung von Schwachstellen in IT-Systemen."},
		{"NIS2-12.2", "Netzwerksegmentierung", "Technische Sicherheit",
			"Segmentierung kritischer Netzwerkbereiche zur Begrenzung der Ausbreitung von Angriffen."},
	}
	for _, ctrl := range controls {
		if _, err := tx.Exec(ctx, `
			INSERT INTO ck_controls (framework_id, org_id, control_id, title, description, domain)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)`,
			frameworkID, orgID, ctrl.id, ctrl.title, ctrl.desc, ctrl.domain); err != nil {
			return "", "", fmt.Errorf("demoseed: control %s: %w", ctrl.id, err)
		}
	}

	// ── SecVault ──────────────────────────────────────────────────────────────
	masterKey, _ := hex.DecodeString(masterKeyHex)
	if len(masterKey) > 0 {
		vaultProjects := []struct{ name, slug, desc string }{
			{"Haupt-Applikation", "main-app", "Produktionsgeheimnisse für die Vakt"},
			{"CI/CD Pipeline", "cicd", "Deployment-Keys und Registry-Tokens für GitHub Actions"},
		}
		for _, vp := range vaultProjects {
			var projID string
			if err := tx.QueryRow(ctx, `
				INSERT INTO so_projects (org_id, name, slug, description, created_by)
				VALUES ($1::uuid, $2, $3, $4, $5::uuid)
				RETURNING id::text`, orgID, vp.name, vp.slug, vp.desc, adminID).Scan(&projID); err != nil {
				return "", "", fmt.Errorf("demoseed: vault project %q: %w", vp.name, err)
			}
			projectKey, err := sharedcrypto.DeriveProjectKey(masterKey, projID)
			if err != nil {
				return "", "", fmt.Errorf("demoseed: derive project key: %w", err)
			}
			envs := []string{"production", "staging", "development"}
			for _, envName := range envs {
				var envID string
				if err := tx.QueryRow(ctx, `
					INSERT INTO so_environments (project_id, org_id, name)
					VALUES ($1::uuid, $2::uuid, $3) RETURNING id::text`,
					projID, orgID, envName).Scan(&envID); err != nil {
					return "", "", fmt.Errorf("demoseed: vault env %s: %w", envName, err)
				}
				if envName == "production" {
					secrets := []struct{ k, v string }{
						{"DATABASE_URL", "postgres://app:s3cr3t@db.internal:5432/sechealth"},
						{"REDIS_URL", "redis://:r3dis_pass@redis.internal:6379"},
						{"SECRET_KEY", "a7f3e2b9c4d1f8e5a2b6c9d3f7e4a1b8c5d2f9e6a3b7c4d8f2e5a9b1c6d3f7"},
					}
					for _, s := range secrets {
						enc, err := sharedcrypto.Encrypt(projectKey, []byte(s.v))
						if err != nil {
							return "", "", fmt.Errorf("demoseed: encrypt secret: %w", err)
						}
						if _, err := tx.Exec(ctx, `
							INSERT INTO so_secrets (environment_id, org_id, key, encrypted_value, created_by)
							VALUES ($1::uuid, $2::uuid, $3, $4, $5::uuid)`,
							envID, orgID, s.k, enc, adminID); err != nil {
							return "", "", fmt.Errorf("demoseed: vault secret %s: %w", s.k, err)
						}
					}
				}
			}
		}
	}

	// ── SecReflex ─────────────────────────────────────────────────────────────
	var templateID, landingPageID, groupID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO sr_templates (org_id, name, subject, from_name, from_email, html_body, attack_type, is_preset, created_by)
		VALUES ($1::uuid,
			'IT-Support: Dringende Passwort-Zurücksetzung',
			'[DRINGEND] Ihr Account wird in 24h gesperrt',
			'IT-Helpdesk', 'helpdesk@it-support-intern.de',
			'<h2>Wichtige Sicherheitsmitteilung</h2><p>Ihr Passwort muss dringend zurückgesetzt werden. Klicken Sie auf den Link um Ihren Account zu schützen.</p><p><a href="{{.TrackingURL}}">Jetzt Passwort zurücksetzen</a></p>',
			'phishing', true, $2::uuid)
		RETURNING id::text`, orgID, adminID).Scan(&templateID); err != nil {
		return "", "", fmt.Errorf("demoseed: reflex template: %w", err)
	}
	// Additional phishing template library (7 more presets)
	extraTemplates := []struct{ name, subject, fromName, fromEmail, body string }{
		{
			"CEO-Fraud: Dringende IT-Bestätigung",
			"Vertraulich: Sofortige Bestätigung erforderlich",
			"Max Mustermann (CEO)", "ceo@mustermann-ceo.de",
			`<h2>Wichtige Anweisung</h2><p>Ich befinde mich gerade in einem Meeting und benötige Ihre sofortige Hilfe. Bitte bestätigen Sie über den untenstehenden Link Ihre IT-Zugangsberechtigungen für das neue System.</p><p><a href="{{.TrackingURL}}">Zugang jetzt bestätigen</a></p>`,
		},
		{
			"Paketlieferung: DHL Sendungsbestätigung",
			"Ihre DHL-Sendung wartet auf Bestätigung",
			"DHL Paketservice", "noreply@dhl-pakete-service.de",
			`<h2>&#128230; Ihre Sendung</h2><p>Ihre Sendung konnte nicht zugestellt werden. Bitte bestätigen Sie Ihre Lieferadresse innerhalb von 24 Stunden.</p><p><a href="{{.TrackingURL}}">Jetzt Lieferung bestätigen</a></p>`,
		},
		{
			"MFA: Unbekanntes Gerät erkannt",
			"Microsoft: Neues Gerät an Ihrem Konto erkannt",
			"Microsoft Sicherheit", "security@microsoft-konto-de.com",
			`<h2>&#128274; Sicherheitswarnung</h2><p>Ein unbekanntes Gerät hat versucht, sich mit Ihrem Microsoft-Konto anzumelden. Falls Sie das nicht waren, bestätigen Sie jetzt Ihre Identität.</p><p><a href="{{.TrackingURL}}">Jetzt verifizieren</a></p>`,
		},
		{
			"Software-Update: Adobe Sicherheitspatch",
			"Kritisches Sicherheits-Update für Adobe Acrobat",
			"Adobe Update Service", "updates@adobe-sicherheit.net",
			`<h2>&#128297; Kritisches Update verfügbar</h2><p>Eine kritische Sicherheitslücke in Adobe Acrobat wurde entdeckt. Bitte installieren Sie das Update sofort, um Ihr System zu schützen.</p><p><a href="{{.TrackingURL}}">Update jetzt installieren</a></p>`,
		},
		{
			"HR-Onboarding: Profil vervollständigen",
			"Willkommen bei Musterfirma GmbH — Ihr Profil wartet",
			"HR Team Musterfirma", "hr@musterfirma-onboarding.de",
			`<h2>Herzlich Willkommen!</h2><p>Ihr Mitarbeiterprofil ist noch nicht vollständig. Bitte füllen Sie Ihre Daten bis Ende der Woche aus, um Ihren IT-Zugang zu erhalten.</p><p><a href="{{.TrackingURL}}">Profil jetzt vervollständigen</a></p>`,
		},
		{
			"IT-Support: Ticket-Update",
			"Ihr Support-Ticket #4821 wurde bearbeitet",
			"IT-Helpdesk Support", "tickets@it-support-system.de",
			`<h2>&#128203; Ticket Update</h2><p>Ihr Support-Ticket #4821 wurde von unserem Team bearbeitet. Bitte bestätigen Sie die vorgeschlagene Lösung oder kommentieren Sie den Status.</p><p><a href="{{.TrackingURL}}">Ticket jetzt ansehen</a></p>`,
		},
		{
			"Bewerbungsbestätigung",
			"Ihre Bewerbung ist eingegangen — nächste Schritte",
			"Karriere Team", "karriere@bewerbungsportal-de.com",
			`<h2>Bewerbung erhalten</h2><p>Vielen Dank für Ihre Bewerbung. Für den nächsten Schritt im Auswahlverfahren bitten wir Sie, Ihre Identität zu bestätigen.</p><p><a href="{{.TrackingURL}}">Bewerbung jetzt abschließen</a></p>`,
		},
	}
	for _, et := range extraTemplates {
		if _, err := tx.Exec(ctx, `
			INSERT INTO sr_templates (org_id, name, subject, from_name, from_email, html_body, attack_type, is_preset, created_by)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, 'phishing', true, $7::uuid)`,
			orgID, et.name, et.subject, et.fromName, et.fromEmail, et.body, adminID); err != nil {
			return "", "", fmt.Errorf("demoseed: reflex template %q: %w", et.name, err)
		}
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO sr_landing_pages (org_id, name, html_content)
		VALUES ($1::uuid, 'Awareness-Seite: Gut gemacht!',
			'<div style="font-family:sans-serif;max-width:600px;margin:40px auto;text-align:center"><h1>&#128994; Gut gemacht!</h1><p>Das war ein <strong>Phishing-Test</strong> Ihres IT-Sicherheitsteams. Sie haben auf einen simulierten Angriff reagiert.</p><p>Bitte nehmen Sie an unserer Pflichtschulung teil, um sich für echte Angriffe zu wappnen.</p></div>')
		RETURNING id::text`, orgID).Scan(&landingPageID); err != nil {
		return "", "", fmt.Errorf("demoseed: reflex landing page: %w", err)
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO sr_target_groups (org_id, name, source)
		VALUES ($1::uuid, 'Alle Mitarbeiter', 'manual')
		RETURNING id::text`, orgID).Scan(&groupID); err != nil {
		return "", "", fmt.Errorf("demoseed: reflex group: %w", err)
	}
	targets := []struct{ email, first, last, dept string }{
		{"m.mueller@musterfirma.de", "Max", "Müller", "Vertrieb"},
		{"a.schmidt@musterfirma.de", "Anna", "Schmidt", "HR"},
		{"t.fischer@musterfirma.de", "Thomas", "Fischer", "IT"},
		{"s.weber@musterfirma.de", "Sandra", "Weber", "Buchhaltung"},
		{"k.meyer@musterfirma.de", "Klaus", "Meyer", "Geschäftsführung"},
	}
	var targetIDs []string
	for _, t := range targets {
		var tID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO sr_targets (org_id, group_id, email, first_name, last_name, department)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)
			RETURNING id::text`, orgID, groupID, t.email, t.first, t.last, t.dept).Scan(&tID); err != nil {
			return "", "", fmt.Errorf("demoseed: reflex target %s: %w", t.email, err)
		}
		targetIDs = append(targetIDs, tID)
	}
	campaigns := []struct {
		name, status string
		daysAgo      int
	}{
		{"Phishing-Test Q1 2026", "completed", 60},
		{"Awareness-Kampagne: CEO-Fraud", "completed", 150},
		{"Quartalstest Q2 2026", "scheduled", -14},
	}
	var completedCampIDs []string
	for _, camp := range campaigns {
		startedAt := time.Now().AddDate(0, 0, -camp.daysAgo)
		completedAt := startedAt.Add(7 * 24 * time.Hour)
		var campID string
		if camp.status == "scheduled" {
			if err := tx.QueryRow(ctx, `
				INSERT INTO sr_campaigns (org_id, name, status, template_id, group_id, landing_page_id,
					from_name, from_email, subject, scheduled_at, recurrence, betriebsrat_mode, created_by)
				VALUES ($1::uuid, $2, $3, $4::uuid, $5::uuid, $6::uuid,
					'IT-Helpdesk', 'helpdesk@it-support-intern.de', '[DRINGEND] Ihr Account wird in 24h gesperrt',
					$7, 'none', true, $8::uuid)
				RETURNING id::text`,
				orgID, camp.name, camp.status, templateID, groupID, landingPageID,
				completedAt, adminID).Scan(&campID); err != nil {
				return "", "", fmt.Errorf("demoseed: campaign %q: %w", camp.name, err)
			}
		} else {
			if err := tx.QueryRow(ctx, `
				INSERT INTO sr_campaigns (org_id, name, status, template_id, group_id, landing_page_id,
					from_name, from_email, subject, started_at, completed_at, recurrence, betriebsrat_mode, created_by)
				VALUES ($1::uuid, $2, $3, $4::uuid, $5::uuid, $6::uuid,
					'IT-Helpdesk', 'helpdesk@it-support-intern.de', '[DRINGEND] Ihr Account wird in 24h gesperrt',
					$7, $8, 'none', true, $9::uuid)
				RETURNING id::text`,
				orgID, camp.name, camp.status, templateID, groupID, landingPageID,
				startedAt, completedAt, adminID).Scan(&campID); err != nil {
				return "", "", fmt.Errorf("demoseed: campaign %q: %w", camp.name, err)
			}
			completedCampIDs = append(completedCampIDs, campID)
		}
	}
	// Click events for completed campaigns: Q1 2026 → 3 clicks, CEO-Fraud → 2 clicks
	clickCounts := []int{3, 2}
	for i, cid := range completedCampIDs {
		n := clickCounts[i]
		for j := 0; j < n && j < len(targetIDs); j++ {
			occurredAt := time.Now().AddDate(0, 0, -(i*10 + j + 2))
			if _, err := tx.Exec(ctx, `
				INSERT INTO sr_events (org_id, campaign_id, target_id, type, tracking_token, occurred_at)
				VALUES ($1::uuid, $2::uuid, $3::uuid, 'click', $4, $5)`,
				orgID, cid, targetIDs[j], fmt.Sprintf("demo-%s-%d", cid[:8], j), occurredAt); err != nil {
				return "", "", fmt.Errorf("demoseed: event campaign %d target %d: %w", i, j, err)
			}
		}
	}

	// ── Training Modules ─────────────────────────────────────────────────────────
	type tmQuestion struct {
		Text    string   `json:"text"`
		Options []string `json:"options"`
		Answer  int      `json:"answer"`
	}
	type tmDef struct {
		title      string
		attackType string
		contentURL string
		duration   int
		questions  []tmQuestion
	}
	trainingDefs := []tmDef{
		{
			"Phishing erkennen", "phishing",
			"Lernziel: Phishing-Mails anhand Absender, Dringlichkeit und verdächtiger Links erkennen.",
			600,
			[]tmQuestion{
				{"Was ist ein typisches Merkmal einer Phishing-Mail?",
					[]string{"Bekannter Absender", "Unbekannte Domain im Absender", "Kein Link enthalten", "Korrekte Rechtschreibung"}, 1},
				{"Sie erhalten: 'Ihr Konto wird gesperrt — sofort handeln!'. Was tun Sie?",
					[]string{"Link sofort klicken", "IT-Abteilung informieren", "Passwort sofort ändern", "E-Mail weiterleiten"}, 1},
				{"Woran erkennen Sie eine sichere URL?",
					[]string{"Sie beginnt mit http://", "Sie enthält viele Sonderzeichen", "Sie beginnt mit https:// und zeigt die echte Domain", "Lange URLs sind immer sicher"}, 2},
			},
		},
		{
			"Passwort-Hygiene", "phishing",
			"Lernziel: Starke Passwörter erstellen und einen Passwort-Manager sicher verwenden.",
			480,
			[]tmQuestion{
				{"Welches Passwort ist am sichersten?",
					[]string{"password123", "Max1990!", "xK#9mP!2vL@qR7", "qwerty"}, 2},
				{"Wie oft sollten Sie dasselbe Passwort verwenden?",
					[]string{"Für alle Dienste", "Für Arbeit und Privat getrennt", "Nie — jeder Dienst ein eigenes Passwort", "Monatlich wechseln und wiederverwenden"}, 2},
				{"Was ist ein Passwort-Manager?",
					[]string{"Ein Kollege, der Passwörter verwaltet", "Eine Software, die Passwörter sicher speichert", "Eine Excel-Tabelle", "Ein Post-it am Monitor"}, 1},
			},
		},
		{
			"Clean Desk Policy", "usb",
			"Lernziel: Arbeitsbereiche absichern und physische Sicherheitsrisiken im Büro vermeiden.",
			360,
			[]tmQuestion{
				{"Was tun Sie beim Verlassen des Arbeitsplatzes?",
					[]string{"Passwörter auf Post-its sichtbar lassen", "Alle vertraulichen Unterlagen wegschließen", "USB-Sticks offen liegenlassen", "Monitor eingeschaltet lassen"}, 1},
				{"Sie finden einen unbekannten USB-Stick. Was tun Sie?",
					[]string{"Einstecken und Inhalt prüfen", "IT-Abteilung abgeben ohne einzustecken", "Mit nach Hause nehmen", "In einen Drucker stecken"}, 1},
			},
		},
		{
			"Multi-Faktor-Authentifizierung (MFA)", "phishing",
			"Lernziel: MFA verstehen, korrekt einrichten und gegen Social Engineering absichern.",
			420,
			[]tmQuestion{
				{"Was bietet MFA als zweiten Faktor?",
					[]string{"Ein weiteres Passwort", "Einen Einmalcode (TOTP) oder Hardware-Token", "Eine Sicherheitsfrage", "Einen festen PIN"}, 1},
				{"Sie erhalten einen MFA-Code, den Sie nicht angefordert haben. Was bedeutet das?",
					[]string{"Das System testet Sie automatisch", "Jemand versucht sich in Ihr Konto einzuloggen", "Technischer Fehler — ignorieren", "Ihr Code läuft bald ab"}, 1},
				{"Warum MFA-Codes niemals per Telefon weitergeben?",
					[]string{"Weil Codes zu kurz sind", "Social Engineering — Angreifer nutzen Codes sofort aus", "Codes gelten dann nicht mehr", "DSGVO verbietet es"}, 1},
			},
		},
		{
			"Social Engineering erkennen", "vishing",
			"Lernziel: Manipulationsversuche per Telefon, E-Mail und persönlich erkennen und abwehren.",
			540,
			[]tmQuestion{
				{"Ein Anrufer gibt sich als IT aus und fragt nach Ihrem Passwort. Was tun Sie?",
					[]string{"Passwort mitteilen — man muss der IT vertrauen", "Auflegen und IT über die bekannte interne Nummer zurückrufen", "Passwort buchstabieren", "E-Mail mit Passwort senden"}, 1},
				{"Was ist 'Pretexting' im Kontext von Social Engineering?",
					[]string{"Textvorlagen für Phishing-Mails", "Eine erfundene Geschichte, um Vertrauen zu erschleichen", "Zeitdruck als Manipulationsmittel", "Gefälschte E-Mail-Absender"}, 1},
			},
		},
	}
	for _, m := range trainingDefs {
		qJSON, err := json.Marshal(m.questions)
		if err != nil {
			return "", "", fmt.Errorf("demoseed: training module marshal: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO sr_training_modules
				(org_id, title, type, attack_type, content_url, duration_seconds, passing_score, questions, created_by)
			VALUES ($1::uuid, $2, 'quiz', $3, $4, $5, 80, $6, $7::uuid)`,
			orgID, m.title, m.attackType, m.contentURL, m.duration, qJSON, adminID); err != nil {
			return "", "", fmt.Errorf("demoseed: training module %q: %w", m.title, err)
		}
	}

	// ── Notifications ─────────────────────────────────────────────────────────
	notifications := []struct{ title, body, typ, module string }{
		{"2 kritische Findings offen", "OpenSSL- und Log4j-Schwachstellen überschreiten SLA-Frist in 4 Tagen.", "error", "vaktscan"},
		{"DSR-Frist läuft ab", "Die Auskunftsanfrage von Hans Müller muss in 15 Tagen beantwortet sein.", "warning", "vaktprivacy"},
		{"AVV abgelaufen", "Der Auftragsverarbeitungsvertrag mit Mailchimp ist seit 11 Monaten abgelaufen.", "warning", "vaktprivacy"},
		{"Hardcodierte Credentials gefunden", "Im Haupt-Repository wurden potenzielle Zugangsdaten in der Commit-History entdeckt.", "error", "vaktvault"},
		{"Willkommen bei Vakt", "Demo-Daten wurden erfolgreich geladen. Erkunde alle Module über die linke Navigation.", "info", "system"},
	}
	for _, n := range notifications {
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_notifications (org_id, title, body, type, module)
			VALUES ($1::uuid, $2, $3, $4, $5)`,
			orgID, n.title, n.body, n.typ, n.module); err != nil {
			return "", "", fmt.Errorf("demoseed: notification %q: %w", n.title, err)
		}
	}

	// ── Sprint 68: Interessierte Parteien (Clause 4.2) ────────────────────────
	interestedParties := []struct {
		name, category, requirements, concerns string
	}{
		{"Datenschutzaufsichtsbehörde", "regulatory", "DSGVO-Compliance, Art. 30 VVT, Löschkonzept", "Unzureichende Dokumentation"},
		{"Geschäftsführung / CISO", "internal", "Nachweisbare ISO 27001-Konformität, Auditberichte", "Unklare Verantwortlichkeiten"},
		{"Kunden (B2B)", "customer", "Datensicherheit, AVV-Abschluss, Transparenz", "Datenpannen, mangelnde Transparenz"},
		{"IT-Lieferanten", "supplier", "Klare Sicherheitsanforderungen, AVV", "Zu hohe Anforderungen"},
		{"Mitarbeitende", "internal", "Klare Richtlinien, Schulungen, sichere Arbeitsumgebung", "Überwachung ohne Betriebsvereinbarung"},
		{"BSI / ENISA", "regulatory", "NIS2-Umsetzung, Meldepflichten, Grundschutz", "Meldung von Sicherheitsvorfällen"},
	}
	for _, p := range interestedParties {
		if _, err := tx.Exec(ctx, `
			INSERT INTO ck_interested_parties (org_id, name, category, requirements, concerns, is_system_default)
			VALUES ($1::uuid, $2, $3, $4, $5, true)`,
			orgID, p.name, p.category, p.requirements, p.concerns); err != nil {
			return "", "", fmt.Errorf("demoseed: interested party %q: %w", p.name, err)
		}
	}

	// ── Sprint 68: Audit-Programm (Clause 9.2) ────────────────────────────────
	var auditPlanID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO ck_audit_plans (org_id, year, scope, status)
		VALUES ($1::uuid, 2026, 'Gesamtes ISMS (ISO 27001:2022)', 'active')
		RETURNING id`, orgID).Scan(&auditPlanID); err != nil {
		return "", "", fmt.Errorf("demoseed: audit plan: %w", err)
	}
	auditSeeds := []struct {
		title, auditType, status, scheduledDate string
	}{
		{"Internes ISMS-Audit Q1 2026", "isms_internal", "completed", now.AddDate(0, -3, 0).Format("2006-01-02")},
		{"Lieferantenaudit IT-Dienstleister", "supplier_audit", "planned", now.AddDate(0, 2, 0).Format("2006-01-02")},
		{"Compliance-Prüfung DSGVO/NIS2", "compliance_check", "planned", now.AddDate(0, 4, 0).Format("2006-01-02")},
	}
	for _, a := range auditSeeds {
		if _, err := tx.Exec(ctx, `
			INSERT INTO ck_audit_program_audits (org_id, plan_id, title, audit_type, scheduled_date, status)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5::date, $6)`,
			orgID, auditPlanID, a.title, a.auditType, a.scheduledDate, a.status); err != nil {
			return "", "", fmt.Errorf("demoseed: audit %q: %w", a.title, err)
		}
	}

	// ── Sprint 68: Löscherinnerungen (Art. 5 Abs. 1 lit. e DSGVO) ─────────────
	deletionReminders := []struct {
		description, dataCategory, dueDate string
	}{
		{"Bewerbungsunterlagen löschen (Absagen älter als 6 Monate)", "Bewerbungsunterlagen",
			now.AddDate(0, 0, 12).Format("2006-01-02")},
		{"Kundendaten inaktiver Accounts (> 2 Jahre) löschen", "Kundenstammdaten",
			now.AddDate(0, 0, 30).Format("2006-01-02")},
		{"Protokolldaten älter als 90 Tage löschen", "Server-Logs",
			now.AddDate(0, 0, 7).Format("2006-01-02")},
	}
	for _, r := range deletionReminders {
		if _, err := tx.Exec(ctx, `
			INSERT INTO po_deletion_reminders (org_id, description, data_category, deletion_due_date)
			VALUES ($1::uuid, $2, $3, $4::date)`,
			orgID, r.description, r.dataCategory, r.dueDate); err != nil {
			return "", "", fmt.Errorf("demoseed: deletion reminder %q: %w", r.description, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", "", fmt.Errorf("demoseed: commit: %w", err)
	}

	log.Info().Str("org_id", orgID).Str("slug", orgSlug).Msg("demoseed: done")
	return orgID, adminID, nil
}
