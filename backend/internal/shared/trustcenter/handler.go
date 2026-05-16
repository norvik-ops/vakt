package trustcenter

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

// ─── Public response types ────────────────────────────────────────────────────

type FrameworkStatus struct {
	Name              string `json:"name"`
	Version           string `json:"version"`
	CompliancePercent int    `json:"compliance_percent"`
	TotalControls     int    `json:"total_controls"`
	CompliantControls int    `json:"compliant_controls"`
}

type Certificate struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Issuer    string  `json:"issuer,omitempty"`
	IssuedAt  *string `json:"issued_at,omitempty"`
	ExpiresAt *string `json:"expires_at,omitempty"`
}

type PublicPolicy struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

type TrustCenterResponse struct {
	OrgName         string            `json:"org_name"`
	Description     string            `json:"description"`
	Contact         string            `json:"contact"`
	LogoURL         string            `json:"logo_url,omitempty"`
	Frameworks      []FrameworkStatus `json:"frameworks"`
	Certificates    []Certificate     `json:"certificates,omitempty"`
	PublicPolicies  []PublicPolicy    `json:"public_policies,omitempty"`
	SubprocessorsMd string            `json:"subprocessors_md,omitempty"`
	ShowFrameworks  bool              `json:"show_frameworks"`
	ShowPolicies    bool              `json:"show_policies"`
	ShowCerts       bool              `json:"show_certs"`
	PublishedAt     time.Time         `json:"published_at"`
	PoweredBy       string            `json:"powered_by"`
}

// ─── Admin types ──────────────────────────────────────────────────────────────

type TrustCenterSettings struct {
	Enabled         bool   `json:"enabled"`
	Description     string `json:"description"`
	Contact         string `json:"contact"`
	LogoURL         string `json:"logo_url"`
	ShowFrameworks  bool   `json:"show_frameworks"`
	ShowPolicies    bool   `json:"show_policies"`
	ShowCerts       bool   `json:"show_certs"`
	SubprocessorsMd string `json:"subprocessors_md"`
}

type CreateCertificateInput struct {
	Name      string `json:"name"`
	Issuer    string `json:"issuer"`
	IssuedAt  string `json:"issued_at"`
	ExpiresAt string `json:"expires_at"`
}

// ─── Public handler ───────────────────────────────────────────────────────────

func (h *Handler) GetTrustCenter(c echo.Context) error {
	slug := c.Param("slug")

	// Look up org by slug, check trust center is enabled
	var orgID, orgName string
	var description, contact, logoURL, subprocessorsMd *string
	var enabled, showFrameworks, showPolicies, showCerts bool
	err := h.db.QueryRow(c.Request().Context(), `
		SELECT id::text, name,
		       trust_center_description, trust_center_contact,
		       trust_center_enabled,
		       COALESCE(trust_center_logo_url, ''),
		       COALESCE(trust_center_show_frameworks, TRUE),
		       COALESCE(trust_center_show_policies, FALSE),
		       COALESCE(trust_center_show_certs, TRUE),
		       trust_center_subprocessors_md
		FROM organizations
		WHERE slug = $1`,
		slug,
	).Scan(&orgID, &orgName, &description, &contact, &enabled,
		&logoURL, &showFrameworks, &showPolicies, &showCerts, &subprocessorsMd)
	if err != nil || !enabled {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "trust center not found"})
	}

	resp := TrustCenterResponse{
		OrgName:        orgName,
		ShowFrameworks: showFrameworks,
		ShowPolicies:   showPolicies,
		ShowCerts:      showCerts,
		PublishedAt:    time.Now().UTC(),
		PoweredBy:      "Vakt",
	}
	if description != nil {
		resp.Description = *description
	}
	if contact != nil {
		resp.Contact = *contact
	}
	if logoURL != nil {
		resp.LogoURL = *logoURL
	}
	if subprocessorsMd != nil {
		resp.SubprocessorsMd = *subprocessorsMd
	}

	// Frameworks (only if show_frameworks)
	if showFrameworks {
		rows, err := h.db.Query(c.Request().Context(), `
			SELECT f.name, f.version,
			       COUNT(c.id)                                                           AS total,
			       COUNT(c.id) FILTER (WHERE c.not_applicable = false
			                             AND (c.manual_status = 'compliant'
			                                  OR EXISTS (
			                                      SELECT 1 FROM ck_evidence e
			                                      WHERE e.control_id = c.id
			                                        AND e.status = 'approved'
			                                  )))                                        AS compliant
			FROM ck_frameworks f
			JOIN ck_controls c ON c.framework_id = f.id
			WHERE f.org_id = $1::uuid
			GROUP BY f.id, f.name, f.version
			ORDER BY f.name`,
			orgID,
		)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		defer rows.Close()

		var frameworks []FrameworkStatus
		for rows.Next() {
			var fs FrameworkStatus
			if err := rows.Scan(&fs.Name, &fs.Version, &fs.TotalControls, &fs.CompliantControls); err != nil {
				continue
			}
			if fs.TotalControls > 0 {
				fs.CompliancePercent = (fs.CompliantControls * 100) / fs.TotalControls
			}
			frameworks = append(frameworks, fs)
		}
		resp.Frameworks = frameworks
	}

	// Certificates (only if show_certs)
	if showCerts {
		certRows, err := h.db.Query(c.Request().Context(), `
			SELECT id::text, name, COALESCE(issuer,''),
			       issued_at::text, expires_at::text
			FROM tc_certificates
			WHERE org_id = $1::uuid AND is_public = TRUE
			ORDER BY display_order, created_at`,
			orgID,
		)
		if err == nil {
			defer certRows.Close()
			for certRows.Next() {
				var cert Certificate
				var issuedAt, expiresAt *string
				if err := certRows.Scan(&cert.ID, &cert.Name, &cert.Issuer, &issuedAt, &expiresAt); err != nil {
					continue
				}
				cert.IssuedAt = issuedAt
				cert.ExpiresAt = expiresAt
				resp.Certificates = append(resp.Certificates, cert)
			}
		}
	}

	// Public policies (only if show_policies)
	if showPolicies {
		polRows, err := h.db.Query(c.Request().Context(), `
			SELECT p.id::text, p.title, COALESCE(p.description,'')
			FROM tc_public_policies tp
			JOIN ck_policies p ON p.id = tp.policy_id
			WHERE tp.org_id = $1::uuid
			ORDER BY p.title`,
			orgID,
		)
		if err == nil {
			defer polRows.Close()
			for polRows.Next() {
				var pol PublicPolicy
				if err := polRows.Scan(&pol.ID, &pol.Title, &pol.Body); err != nil {
					continue
				}
				resp.PublicPolicies = append(resp.PublicPolicies, pol)
			}
		}
	}

	return c.JSON(http.StatusOK, resp)
}

// ─── Admin handlers ───────────────────────────────────────────────────────────

// GetTrustCenterSettings handles GET /api/v1/trust-center/settings
func (h *Handler) GetTrustCenterSettings(c echo.Context) error {
	orgID := c.Get("org_id").(string)

	var s TrustCenterSettings
	var description, contact, logoURL, subprocessorsMd *string
	err := h.db.QueryRow(c.Request().Context(), `
		SELECT trust_center_enabled,
		       trust_center_description,
		       trust_center_contact,
		       trust_center_logo_url,
		       COALESCE(trust_center_show_frameworks, TRUE),
		       COALESCE(trust_center_show_policies, FALSE),
		       COALESCE(trust_center_show_certs, TRUE),
		       trust_center_subprocessors_md
		FROM organizations
		WHERE id = $1::uuid`, orgID,
	).Scan(&s.Enabled, &description, &contact, &logoURL,
		&s.ShowFrameworks, &s.ShowPolicies, &s.ShowCerts, &subprocessorsMd)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get trust center settings failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve trust center settings",
			"code":  "TC_GET_SETTINGS_ERROR",
		})
	}
	if description != nil {
		s.Description = *description
	}
	if contact != nil {
		s.Contact = *contact
	}
	if logoURL != nil {
		s.LogoURL = *logoURL
	}
	if subprocessorsMd != nil {
		s.SubprocessorsMd = *subprocessorsMd
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"data": s})
}

// UpdateTrustCenterSettings handles PATCH /api/v1/trust-center/settings
func (h *Handler) UpdateTrustCenterSettings(c echo.Context) error {
	orgID := c.Get("org_id").(string)

	var in TrustCenterSettings
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "TC_BAD_REQUEST",
		})
	}

	_, err := h.db.Exec(c.Request().Context(), `
		UPDATE organizations
		SET trust_center_enabled        = $2,
		    trust_center_description    = NULLIF($3, ''),
		    trust_center_contact        = NULLIF($4, ''),
		    trust_center_logo_url       = NULLIF($5, ''),
		    trust_center_show_frameworks = $6,
		    trust_center_show_policies  = $7,
		    trust_center_show_certs     = $8,
		    trust_center_subprocessors_md = NULLIF($9, ''),
		    updated_at                  = NOW()
		WHERE id = $1::uuid`,
		orgID, in.Enabled, in.Description, in.Contact, in.LogoURL,
		in.ShowFrameworks, in.ShowPolicies, in.ShowCerts, in.SubprocessorsMd,
	)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("update trust center settings failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update trust center settings",
			"code":  "TC_UPDATE_SETTINGS_ERROR",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ListCertificates handles GET /api/v1/trust-center/certificates
func (h *Handler) ListCertificates(c echo.Context) error {
	orgID := c.Get("org_id").(string)

	rows, err := h.db.Query(c.Request().Context(), `
		SELECT id::text, name, COALESCE(issuer,''),
		       issued_at::text, expires_at::text
		FROM tc_certificates
		WHERE org_id = $1::uuid
		ORDER BY display_order, created_at`,
		orgID,
	)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("list certificates failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve certificates",
			"code":  "TC_LIST_CERTS_ERROR",
		})
	}
	defer rows.Close()

	certs := []Certificate{}
	for rows.Next() {
		var cert Certificate
		var issuedAt, expiresAt *string
		if err := rows.Scan(&cert.ID, &cert.Name, &cert.Issuer, &issuedAt, &expiresAt); err != nil {
			continue
		}
		cert.IssuedAt = issuedAt
		cert.ExpiresAt = expiresAt
		certs = append(certs, cert)
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"data": certs})
}

// CreateCertificate handles POST /api/v1/trust-center/certificates
func (h *Handler) CreateCertificate(c echo.Context) error {
	orgID := c.Get("org_id").(string)

	var in CreateCertificateInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "TC_BAD_REQUEST",
		})
	}
	if in.Name == "" {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "name is required",
			"code":  "TC_VALIDATION_ERROR",
		})
	}

	var issuedAt, expiresAt *string
	if in.IssuedAt != "" {
		issuedAt = &in.IssuedAt
	}
	if in.ExpiresAt != "" {
		expiresAt = &in.ExpiresAt
	}

	var cert Certificate
	var dbIssuedAt, dbExpiresAt *string
	err := h.db.QueryRow(c.Request().Context(), `
		INSERT INTO tc_certificates (org_id, name, issuer, issued_at, expires_at)
		VALUES ($1::uuid, $2, NULLIF($3,''), $4::date, $5::date)
		RETURNING id::text, name, COALESCE(issuer,''), issued_at::text, expires_at::text`,
		orgID, in.Name, in.Issuer, issuedAt, expiresAt,
	).Scan(&cert.ID, &cert.Name, &cert.Issuer, &dbIssuedAt, &dbExpiresAt)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("create certificate failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create certificate",
			"code":  "TC_CREATE_CERT_ERROR",
		})
	}
	cert.IssuedAt = dbIssuedAt
	cert.ExpiresAt = dbExpiresAt
	return c.JSON(http.StatusCreated, cert)
}

// DeleteCertificate handles DELETE /api/v1/trust-center/certificates/:id
func (h *Handler) DeleteCertificate(c echo.Context) error {
	orgID := c.Get("org_id").(string)
	certID := c.Param("id")

	tag, err := h.db.Exec(c.Request().Context(), `
		DELETE FROM tc_certificates WHERE id = $1::uuid AND org_id = $2::uuid`,
		certID, orgID,
	)
	if err != nil {
		log.Error().Err(err).Str("cert_id", certID).Msg("delete certificate failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to delete certificate",
			"code":  "TC_DELETE_CERT_ERROR",
		})
	}
	if tag.RowsAffected() == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "certificate not found",
			"code":  "TC_CERT_NOT_FOUND",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

// PublishPolicy handles POST /api/v1/trust-center/policies/:policyId/publish
func (h *Handler) PublishPolicy(c echo.Context) error {
	orgID := c.Get("org_id").(string)
	policyID := c.Param("policyId")

	_, err := h.db.Exec(c.Request().Context(), `
		INSERT INTO tc_public_policies (org_id, policy_id)
		VALUES ($1::uuid, $2::uuid)
		ON CONFLICT DO NOTHING`,
		orgID, policyID,
	)
	if err != nil {
		log.Error().Err(err).Str("policy_id", policyID).Msg("publish policy failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to publish policy",
			"code":  "TC_PUBLISH_POLICY_ERROR",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "published"})
}

// UnpublishPolicy handles DELETE /api/v1/trust-center/policies/:policyId/publish
func (h *Handler) UnpublishPolicy(c echo.Context) error {
	orgID := c.Get("org_id").(string)
	policyID := c.Param("policyId")

	_, err := h.db.Exec(c.Request().Context(), `
		DELETE FROM tc_public_policies WHERE org_id = $1::uuid AND policy_id = $2::uuid`,
		orgID, policyID,
	)
	if err != nil {
		log.Error().Err(err).Str("policy_id", policyID).Msg("unpublish policy failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to unpublish policy",
			"code":  "TC_UNPUBLISH_POLICY_ERROR",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "unpublished"})
}

// ListPublishedPolicies handles GET /api/v1/trust-center/policies (admin — returns published policy IDs)
func (h *Handler) ListPublishedPolicies(c echo.Context) error {
	orgID := c.Get("org_id").(string)

	rows, err := h.db.Query(c.Request().Context(), `
		SELECT policy_id::text FROM tc_public_policies WHERE org_id = $1::uuid`,
		orgID,
	)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("list published policies failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to list published policies",
			"code":  "TC_LIST_POLICIES_ERROR",
		})
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"data": ids})
}
