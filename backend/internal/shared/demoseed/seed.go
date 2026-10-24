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
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// randomPassword returns n random URL-safe base64 characters suitable for use
// as a temporary password. Uses crypto/rand for cryptographic randomness.
func randomPassword(n int) string {
	// Each base64 character encodes 6 bits; we need ceil(n*6/8) raw bytes.
	raw := make([]byte, (n*6+7)/8)
	if _, err := rand.Read(raw); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	s := hex.EncodeToString(raw) // hex gives us only [0-9a-f], always URL-safe
	if len(s) > n {
		s = s[:n]
	}
	return s
}

// Run seeds the shared "demo" org (idempotent — skips if slug "demo" exists).
//
// Hinweis: Diese statische Variante wird primär für lokale Entwicklung und
// Tests genutzt. Bei lokalem Demo-Betrieb (VAKT_DEMO=true) werden pro Visitor ephemere
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

	// users.role mirrors the org_members role assigned below (D24-1): the admin
	// seed is Admin ('admin'), the analyst is SecurityAnalyst ('editor'). Set
	// explicitly — the default is 'viewer' since migration 249.
	var analystID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name, role)
		VALUES ($1, $2, 'Max Mustermann', 'admin')
		RETURNING id::text`, adminEmail, string(adminHash)).Scan(&adminID); err != nil {
		return "", "", fmt.Errorf("demoseed: admin user: %w", err)
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name, role)
		VALUES ($1, $2, 'Anna Analyst', 'editor')
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

	// Scan findings, Privacy (VVT, DSR, breach, AVV, DPIA), Vault, Aware and HR
	// demo data went with the archived modules (tag archiv/module-vollstaendig).
	now := time.Now()

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

	// ── SecVitals: BSI IT-Grundschutz Framework (S73-3a) ─────────────────────
	var bsiFrameworkID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO ck_frameworks (org_id, name, version, is_builtin)
		VALUES ($1::uuid, 'BSI IT-Grundschutz', '200-2:2021', true)
		RETURNING id::text`, orgID).Scan(&bsiFrameworkID); err != nil {
		return "", "", fmt.Errorf("demoseed: bsi framework: %w", err)
	}
	bsiSeedControls := []struct {
		id, title, domain, desc string
		manualStatus            string // "" | "in_progress" | "implemented"
	}{
		{"BSI-ISMS.1.A1", "Sicherheitsleitlinie", "Sicherheitsmanagement",
			"Informationssicherheitsleitlinie erstellen, verabschieden und mindestens jährlich überprüfen.", "implemented"},
		{"BSI-ISMS.1.A6", "Sicherheitskonzept", "Sicherheitsmanagement",
			"Dokumentiertes Sicherheitskonzept nach BSI-Standard 200-2 (Basis- oder Kern-Absicherung).", "in_progress"},
		{"BSI-ORP.3.A1", "Sensibilisierung und Schulung", "Organisation",
			"Mindestens jährliche Security-Awareness-Trainings für alle Mitarbeitenden.", "implemented"},
		{"BSI-OPS.1.1.3.A1", "Patch- und Änderungsmanagement", "Betrieb",
			"Kritische Sicherheitsupdates innerhalb von 7 Tagen, sonstige innerhalb von 30 Tagen.", "implemented"},
		{"BSI-OPS.1.1.4.A1", "Schutz vor Schadprogrammen", "Betrieb",
			"Zentral verwaltete Antimalware-Lösung auf allen Endpunkten und Servern.", "implemented"},
		{"BSI-DER.1.A1", "Detektion sicherheitsrelevanter Ereignisse", "Detektion und Reaktion",
			"Verfahren zur Erkennung von Sicherheitsvorfällen (SIEM, IDS/IPS, Anomalie-Erkennung).", ""},
		{"BSI-CON.3.A1", "Datensicherungskonzept", "Konzeption",
			"Backup-Konzept nach 3-2-1-Regel mit halbjährlichen Restore-Tests.", "in_progress"},
		{"BSI-NET.1.1.A1", "Sicherheitsrichtlinie Netzarchitektur", "Netz und Kommunikation",
			"Dokumentierte Netzarchitektur mit Segmentierungskonzept und Sicherheitszonen.", ""},
	}
	var bsiControlIDs []string
	for _, ctrl := range bsiSeedControls {
		var ctrlID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO ck_controls (framework_id, org_id, control_id, title, description, domain)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)
			RETURNING id::text`,
			bsiFrameworkID, orgID, ctrl.id, ctrl.title, ctrl.desc, ctrl.domain).Scan(&ctrlID); err != nil {
			return "", "", fmt.Errorf("demoseed: bsi control %s: %w", ctrl.id, err)
		}
		bsiControlIDs = append(bsiControlIDs, ctrlID)
		if ctrl.manualStatus != "" {
			// orgid-lint: global — demo seed updates a control it just inserted by PK within the same transaction
			if _, err := tx.Exec(ctx, `
				UPDATE ck_controls SET manual_status = $1 WHERE id = $2::uuid`,
				ctrl.manualStatus, ctrlID); err != nil {
				return "", "", fmt.Errorf("demoseed: bsi manual_status %s: %w", ctrl.id, err)
			}
		}
	}

	// BSI Modeling entries: map two assets to key Bausteine.
	if len(assetIDs) >= 2 && len(bsiControlIDs) >= 5 {
		bsiModelingEntries := []struct {
			assetIdx, ctrlIdx int
			priority, status  string
		}{
			{0, 0, "R1", "yes"},     // Webserver → Sicherheitsleitlinie
			{0, 3, "R1", "yes"},     // Webserver → Patch-Management
			{0, 4, "R1", "yes"},     // Webserver → Schutz vor Schadprogrammen
			{1, 0, "R1", "yes"},     // DB-Cluster → Sicherheitsleitlinie
			{1, 1, "R1", "partial"}, // DB-Cluster → Sicherheitskonzept
			{1, 6, "R1", "partial"}, // DB-Cluster → Datensicherungskonzept
		}
		for _, me := range bsiModelingEntries {
			if _, err := tx.Exec(ctx, `
				INSERT INTO ck_bsi_modeling (org_id, asset_id, control_id, priority, check_status, created_by)
				VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6::uuid)`,
				orgID, assetIDs[me.assetIdx], bsiControlIDs[me.ctrlIdx],
				me.priority, me.status, adminID); err != nil {
				return "", "", fmt.Errorf("demoseed: bsi modeling: %w", err)
			}
		}
	}

	// ── Notifications ─────────────────────────────────────────────────────────
	notifications := []struct{ title, body, typ, module string }{
		{"Willkommen bei Vakt", "Demo-Daten wurden erfolgreich geladen. Erkunde das ISMS über die linke Navigation.", "info", "system"},
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
		{"Datenschutzaufsichtsbehörde", "regulator", "DSGVO-Compliance, Art. 30 VVT, Löschkonzept", "Unzureichende Dokumentation"},
		{"Geschäftsführung / CISO", "employee", "Nachweisbare ISO 27001-Konformität, Auditberichte", "Unklare Verantwortlichkeiten"},
		{"Kunden (B2B)", "customer", "Datensicherheit, AVV-Abschluss, Transparenz", "Datenpannen, mangelnde Transparenz"},
		{"IT-Lieferanten", "supplier", "Klare Sicherheitsanforderungen, AVV", "Zu hohe Anforderungen"},
		{"Mitarbeitende", "employee", "Klare Richtlinien, Schulungen, sichere Arbeitsumgebung", "Überwachung ohne Betriebsvereinbarung"},
		{"BSI / ENISA", "regulator", "NIS2-Umsetzung, Meldepflichten, Grundschutz", "Meldung von Sicherheitsvorfällen"},
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
		VALUES ($1::uuid, 2026, 'Gesamtes ISMS (ISO 27001:2022)', 'in_progress')
		RETURNING id`, orgID).Scan(&auditPlanID); err != nil {
		return "", "", fmt.Errorf("demoseed: audit plan: %w", err)
	}
	auditSeeds := []struct {
		title, auditType, status, scope, plannedDate string
	}{
		{"Internes ISMS-Audit Q1 2026", "isms_internal", "completed", "ISO 27001 ISMS", now.AddDate(0, -3, 0).Format("2006-01-02")},
		{"Lieferantenaudit IT-Dienstleister", "supplier_audit", "planned", "Kritische IT-Lieferanten", now.AddDate(0, 2, 0).Format("2006-01-02")},
		{"Compliance-Prüfung DSGVO/NIS2", "compliance_check", "planned", "DSGVO & NIS2", now.AddDate(0, 4, 0).Format("2006-01-02")},
	}
	for _, a := range auditSeeds {
		if _, err := tx.Exec(ctx, `
			INSERT INTO ck_audit_program_audits (org_id, audit_plan_id, title, audit_type, scope, planned_date, status)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::date, $7)`,
			orgID, auditPlanID, a.title, a.auditType, a.scope, a.plannedDate, a.status); err != nil {
			return "", "", fmt.Errorf("demoseed: audit %q: %w", a.title, err)
		}
	}

	// ── S76-2: BSI Strukturanalyse — Beispielgraph Schutzbedarfsvererbung ────
	// 2 Anwendungen → 1 Server → 1 Raum (Smoke-Plan aus der Story)
	var toHRID, toCRMID, toSrvID, toRoomID string
	_ = tx.QueryRow(ctx, `
		INSERT INTO ck_bsi_target_objects (org_id, name, type, protection_c, protection_i, protection_a, absicherungsniveau, description)
		VALUES ($1::uuid, 'HR-Portal', 'application', 'hoch', 'hoch', 'normal', 'standard',
		        'Personalverwaltung (Fehlzeiten, Stammdaten)')
		RETURNING id::text`, orgID).Scan(&toHRID)
	_ = tx.QueryRow(ctx, `
		INSERT INTO ck_bsi_target_objects (org_id, name, type, protection_c, protection_i, protection_a, absicherungsniveau, description)
		VALUES ($1::uuid, 'CRM-System', 'application', 'normal', 'normal', 'hoch', 'standard',
		        'Kundenverwaltung (Kontakte, Aufträge)')
		RETURNING id::text`, orgID).Scan(&toCRMID)
	_ = tx.QueryRow(ctx, `
		INSERT INTO ck_bsi_target_objects (org_id, name, type, protection_c, protection_i, protection_a, absicherungsniveau, description)
		VALUES ($1::uuid, 'Applikationsserver app-01', 'it_system', 'normal', 'normal', 'normal', 'standard',
		        'Primärer Applikationsserver im Rechenzentrum')
		RETURNING id::text`, orgID).Scan(&toSrvID)
	_ = tx.QueryRow(ctx, `
		INSERT INTO ck_bsi_target_objects (org_id, name, type, protection_c, protection_i, protection_a, absicherungsniveau, description)
		VALUES ($1::uuid, 'Serverraum EG', 'room', 'normal', 'normal', 'normal', 'basis',
		        'Serverraum Erdgeschoss (Zutritt Ausweis)')
		RETURNING id::text`, orgID).Scan(&toRoomID)
	// Edges: HR-Portal runs_on srv-01 ; CRM runs_on srv-01 ; srv-01 located_in serverraum
	if toHRID != "" && toSrvID != "" {
		_, _ = tx.Exec(ctx, `
			INSERT INTO ck_bsi_target_object_dependencies (org_id, source_id, target_id, dependency_type)
			VALUES ($1::uuid, $2::uuid, $3::uuid, 'runs_on')
			ON CONFLICT DO NOTHING`, orgID, toHRID, toSrvID)
	}
	if toCRMID != "" && toSrvID != "" {
		_, _ = tx.Exec(ctx, `
			INSERT INTO ck_bsi_target_object_dependencies (org_id, source_id, target_id, dependency_type)
			VALUES ($1::uuid, $2::uuid, $3::uuid, 'runs_on')
			ON CONFLICT DO NOTHING`, orgID, toCRMID, toSrvID)
	}
	if toSrvID != "" && toRoomID != "" {
		_, _ = tx.Exec(ctx, `
			INSERT INTO ck_bsi_target_object_dependencies (org_id, source_id, target_id, dependency_type)
			VALUES ($1::uuid, $2::uuid, $3::uuid, 'located_in')
			ON CONFLICT DO NOTHING`, orgID, toSrvID, toRoomID)
	}

	// ── S81-6: ISO 27001:2022 Demo-Framework ─────────────────────────────────
	// Show the central Vanta-comparison framework with realistic control statuses
	// so sales prospects can compare Vakt against competing tools.
	var iso27001FrameworkID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO ck_frameworks (org_id, name, version, is_builtin)
		VALUES ($1::uuid, 'ISO/IEC 27001:2022', '2022', true)
		RETURNING id::text`, orgID).Scan(&iso27001FrameworkID); err != nil {
		return "", "", fmt.Errorf("demoseed: iso27001 framework: %w", err)
	}
	type isoCtrl struct {
		id, title, domain, desc, status string
	}
	isoControls := []isoCtrl{
		{"A.5.1", "Policies for information security", "Organisational Controls",
			"A set of information security policies shall be defined, approved by management, published, communicated, and reviewed.", "implemented"},
		{"A.5.2", "Information security roles and responsibilities", "Organisational Controls",
			"Information security roles and responsibilities shall be defined and allocated.", "implemented"},
		{"A.5.15", "Access control", "Organisational Controls",
			"Rules to control physical and logical access to information and other assets shall be established and implemented.", "in_progress"},
		{"A.5.23", "Information security for use of cloud services", "Organisational Controls",
			"Processes for acquisition, use, management and exit from cloud services shall be established.", "in_progress"},
		{"A.6.3", "Information security awareness, education and training", "People Controls",
			"Personnel and relevant interested parties shall receive information security awareness, education and training.", "implemented"},
		{"A.6.8", "Information security event reporting", "People Controls",
			"The organisation shall provide a mechanism for personnel to report observed or suspected events.", "implemented"},
		{"A.7.1", "Physical security perimeters", "Physical Controls",
			"Security perimeters shall be defined and used to protect areas that contain information.", "implemented"},
		{"A.7.7", "Clear desk and clear screen", "Physical Controls",
			"Clear desk rules for papers and removable storage and clear screen rules shall be defined.", "implemented"},
		{"A.7.10", "Storage media", "Physical Controls",
			"Storage media shall be managed through their acquisition, use, transportation and disposal.", "in_progress"},
		{"A.8.1", "User end point devices", "Technological Controls",
			"Information stored on, processed by, or accessible via user end point devices shall be protected.", "in_progress"},
		{"A.8.5", "Secure authentication", "Technological Controls",
			"Secure authentication technologies and procedures shall be implemented based on information access restrictions.", "implemented"},
		{"A.8.8", "Management of technical vulnerabilities", "Technological Controls",
			"Information about technical vulnerabilities of information systems in use shall be obtained and evaluated.", "not_started"},
		{"A.8.16", "Monitoring activities", "Technological Controls",
			"Networks, systems and applications shall be monitored for anomalous behaviour.", "not_started"},
		{"A.8.24", "Use of cryptography", "Technological Controls",
			"Rules for the effective use of cryptography, including cryptographic key management, shall be defined and implemented.", "implemented"},
	}
	for _, ctrl := range isoControls {
		var ctrlID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO ck_controls (framework_id, org_id, control_id, title, description, domain)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)
			RETURNING id::text`,
			iso27001FrameworkID, orgID, ctrl.id, ctrl.title, ctrl.desc, ctrl.domain).Scan(&ctrlID); err != nil {
			return "", "", fmt.Errorf("demoseed: iso27001 control %s: %w", ctrl.id, err)
		}
		if ctrl.status != "" && ctrl.status != "not_started" {
			// orgid-lint: global — UPDATE by PK ctrlID from the RETURNING clause of the INSERT just above
			if _, err := tx.Exec(ctx, `
				UPDATE ck_controls SET manual_status = $1 WHERE id = $2::uuid`,
				ctrl.status, ctrlID); err != nil {
				return "", "", fmt.Errorf("demoseed: iso27001 control status %s: %w", ctrl.id, err)
			}
		}
		// S88-5: seed applied physical-control checklists as evidence for A.7.7 + A.7.10.
		if ctrl.id == "A.7.7" || ctrl.id == "A.7.10" {
			// ON CONFLICT DO NOTHING deckt einen Wiederholungslauf des Seeds ab:
			// seit Migration 257 traegt ck_evidence einen Teilindex ueber
			// (org_id, control_id, source, title), den diese Zeile trifft.
			if _, err := tx.Exec(ctx, `
				INSERT INTO ck_evidence (control_id, org_id, title, description, source, collector_data, status)
				VALUES ($1::uuid, $2::uuid, $3, $4, 'checklist', $5::jsonb, 'pending')
				ON CONFLICT (org_id, control_id, source, title)
				  WHERE control_id IS NOT NULL
				    AND source <> 'manual'
				    AND auto_source_type IS NULL
				DO NOTHING`,
				ctrlID, orgID,
				fmt.Sprintf("Checkliste: %s (%s)", ctrl.title, ctrl.id),
				"Geführte Checkliste für physische Maßnahme angewendet (ISO A.7.x)",
				fmt.Sprintf(`{"source":"physical_control_template","control_code":%q}`, ctrl.id),
			); err != nil {
				return "", "", fmt.Errorf("demoseed: physical checklist %s: %w", ctrl.id, err)
			}
		}
	}

	// ── S86: BSI-200-4 BCM Demo-Seed ──────────────────────────────────────────
	var itInfraID string
	biaProcesses := []struct {
		name   string
		crit   string
		klasse int
		rto    int
		rpo    int
		mbco   int
		owner  string
	}{
		{"IT-Infrastruktur-Betrieb", "high", 3, 4, 1, 80, "IT-Leitung"},
		{"Kundendaten-Verarbeitung", "high", 2, 24, 4, 60, "Datenschutzbeauftragter"},
		{"Personalverwaltung", "medium", 1, 72, 24, 40, "HR-Manager"},
	}
	for i, p := range biaProcesses {
		var id string
		if err := tx.QueryRow(ctx, `
			INSERT INTO ck_bia_processes (org_id, name, criticality, schutzbedarfsklasse, rto_hours, rpo_hours, mbco_percent, process_owner, dependencies)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, '{}')
			RETURNING id::text`,
			orgID, p.name, p.crit, p.klasse, p.rto, p.rpo, p.mbco, p.owner,
		).Scan(&id); err != nil {
			return "", "", fmt.Errorf("demoseed: bia_process %d: %w", i, err)
		}
		if i == 0 {
			itInfraID = id
		}
	}

	// WAP — IT-Infrastruktur-Betrieb
	if itInfraID != "" {
		wapSteps, _ := json.Marshal([]map[string]any{
			{"order": 1, "action": "Incident-Alarm auslösen", "responsible": "IT-Leiter", "duration_min": 15},
			{"order": 2, "action": "Backup-Systeme aktivieren", "responsible": "SysAdmin", "duration_min": 30},
			{"order": 3, "action": "Dienste sequenziell starten", "responsible": "DevOps", "duration_min": 60},
			{"order": 4, "action": "Funktionstests durchführen", "responsible": "QA", "duration_min": 30},
			{"order": 5, "action": "Management informieren", "responsible": "IT-Leiter", "duration_min": 15},
		})
		lastTested := now.AddDate(0, -3, 0).Format("2006-01-02")
		if _, err := tx.Exec(ctx, `
			INSERT INTO ck_recovery_plans (org_id, bia_process_id, title, activation_criteria, responsible, rto_hours, status, steps, last_tested_at)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9::date)`,
			orgID, itInfraID,
			"WAP IT-Infrastruktur",
			"Ausfall von mehr als 2 kritischen Servern oder Netzwerkausfall > 30 Minuten",
			"IT-Leitung",
			4, "tested", string(wapSteps), lastTested,
		); err != nil {
			return "", "", fmt.Errorf("demoseed: recovery_plan: %w", err)
		}
	}

	// Alarmierungsplan (3 Kontakte, 3 Eskalationsstufen)
	emergencyContacts := []struct {
		name  string
		role  string
		phone string
		email string
		level int
		avail bool
	}{
		{"Max Mustermann", "IT-Notfall-Koordinator", "+49 89 123456-10", "it-notfall@example.com", 1, true},
		{"Anna Schmidt", "IT-Leitung", "+49 89 123456-20", "it-leitung@example.com", 2, false},
		{"BSI CERT-Bund", "Externer Dienstleister", "+49 228 99 9582-5555", "certbund@bsi.bund.de", 3, false},
	}
	for _, c := range emergencyContacts {
		if _, err := tx.Exec(ctx, `
			INSERT INTO ck_emergency_contacts (org_id, name, role, phone, email, escalation_level, available_24_7)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)`,
			orgID, c.name, c.role, c.phone, c.email, c.level, c.avail,
		); err != nil {
			return "", "", fmt.Errorf("demoseed: emergency_contact: %w", err)
		}
	}

	// ── S88-2: Backup-/Restore-Nachweis Demo-Seed ─────────────────────────────
	var backupJobID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO ck_backup_jobs
		  (org_id, name, source, destination, frequency, encrypted,
		   last_success_at, last_status, restore_max_age_days, notes)
		VALUES ($1::uuid, 'Postgres Nightly Dump', 'vakt-db', 'Hetzner Storage Box (verschlüsselt)',
		        'daily', TRUE, NOW() - INTERVAL '8 hours', 'success', 365,
		        'Automatisiertes pg_dump, AES-256-verschlüsselt, Off-Site repliziert')
		RETURNING id::text`, orgID).Scan(&backupJobID); err != nil {
		return "", "", fmt.Errorf("demoseed: backup_job: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ck_backup_restore_tests
		  (org_id, job_id, tested_at, result, rto_target_hours, rto_actual_hours, tester, notes)
		VALUES ($1::uuid, $2::uuid, CURRENT_DATE - INTERVAL '45 days', 'success', 4, 3, 'IT-Leitung',
		        'Vollständiger Restore-Test in Staging, RTO eingehalten')`,
		orgID, backupJobID); err != nil {
		return "", "", fmt.Errorf("demoseed: restore_test: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", "", fmt.Errorf("demoseed: commit: %w", err)
	}

	log.Info().Str("org_id", orgID).Str("slug", orgSlug).Msg("demoseed: done")
	return orgID, adminID, nil
}
