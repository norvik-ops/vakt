// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S21-1: Direct SAML 2.0 SP implementation using crewjam/saml.
// This supplements the Casdoor-based SAML proxy: when an org has a row in
// org_saml_configs, all SAML traffic is handled here without Casdoor.
// Casdoor-based SAML continues to work for orgs that don't configure direct SAML.
package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"time"

	"github.com/crewjam/saml"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"
	"github.com/matharnica/vakt/internal/shared/logsafe"
)

// samlMetadataMaxBytes is the maximum allowed size for an IdP metadata XML
// document. This prevents Billion-Laughs-style XML entity expansion (SEC-M03).
const samlMetadataMaxBytes = 512 * 1024 // 512 KiB

// samlAttr extracts the first value of a named attribute from a SAML assertion.
// Tries each candidateName in order; returns "" if none match.
func samlAttr(a *saml.Assertion, names ...string) string {
	for _, stmt := range a.AttributeStatements {
		for _, attr := range stmt.Attributes {
			for _, want := range names {
				if attr.Name == want || attr.FriendlyName == want {
					if len(attr.Values) > 0 {
						return attr.Values[0].Value
					}
				}
			}
		}
	}
	return ""
}

// OrgSAMLConfig holds the persisted SAML SP configuration for one org.
type OrgSAMLConfig struct {
	OrgID       string
	EntityID    string // SP entity ID URL
	ACSURL      string // Assertion Consumer Service URL
	IDPMetadata string // IdP metadata XML blob
	CertPEM     string // SP X.509 certificate PEM
	KeyPEM      string // SP RSA private key PEM (stored encrypted in DB)
	Enabled     bool
}

// samlConfigColumns is the SELECT list used when loading org_saml_configs.
const samlConfigColumns = `org_id::text, entity_id, acs_url, idp_metadata, cert_pem, key_pem, enabled`

// samlServiceKey returns the HKDF-derived sub-key used to encrypt SAML SP
// private keys at rest.  The same purpose string is used by cmd/rotate-key,
// which can migrate legacy raw-master ciphertext onto this key.  ADR-0038.
func samlServiceKey(masterKey []byte) []byte {
	if len(masterKey) != 32 {
		// Pre-derived key path: caller passed in a 32-byte buffer (test) that
		// is already a service key.  We accept it as-is to preserve the older
		// in-process call sites until they migrate.
		return masterKey
	}
	k, err := sharedcrypto.DeriveServiceKey(masterKey, "vakt-saml-v1")
	if err != nil {
		// HKDF is deterministic and only fails on bad inputs (32-byte key
		// material is the only requirement, already checked above).  Log
		// once and fall back to the raw key — this keeps the SAML flow alive
		// for already-encrypted rows.
		log.Warn().Err(err).Msg("saml: HKDF derive failed — falling back to raw master key")
		return masterKey
	}
	return k
}

// LoadOrgSAMLConfig fetches the SAML config for an org, decrypting the private key.
// Returns nil, nil when no config exists for the org.
func LoadOrgSAMLConfig(ctx context.Context, db *pgxpool.Pool, orgID string, masterKey []byte) (*OrgSAMLConfig, error) {
	if db == nil {
		return nil, nil
	}
	var c OrgSAMLConfig
	var keyEnc []byte
	err := db.QueryRow(ctx,
		`SELECT `+samlConfigColumns+` FROM org_saml_configs WHERE org_id = $1::uuid`,
		orgID,
	).Scan(&c.OrgID, &c.EntityID, &c.ACSURL, &c.IDPMetadata, &c.CertPEM, &keyEnc, &c.Enabled)
	if err != nil {
		// pgx returns pgx.ErrNoRows when no row found; treat as unconfigured
		return nil, nil //nolint:nilerr
	}

	if len(masterKey) == 32 && len(keyEnc) > 0 {
		// New rows are encrypted under the HKDF-derived saml service key.
		// Legacy rows (pre-ADR-0038) used the raw master key directly; we
		// keep the fallback so installations that have not run rotate-key
		// yet continue to work.  rotate-key migrates legacy rows on demand.
		samlKey := samlServiceKey(masterKey)
		plain, err := sharedcrypto.Decrypt(samlKey, keyEnc)
		if err != nil {
			if legacy, legErr := sharedcrypto.Decrypt(masterKey, keyEnc); legErr == nil {
				log.Warn().Str("org_id", orgID).Msg("saml: decrypted private key with legacy raw master — run cmd/rotate-key to migrate")
				c.KeyPEM = string(legacy)
				return &c, nil
			}
			return nil, fmt.Errorf("saml: decrypt private key: %w", err)
		}
		c.KeyPEM = string(plain)
	}
	return &c, nil
}

// UpsertOrgSAMLConfig writes a SAML config row (insert or update).
// keyPEM is encrypted with masterKey before storage.
func UpsertOrgSAMLConfig(ctx context.Context, db *pgxpool.Pool, cfg *OrgSAMLConfig, masterKey []byte) error {
	var keyEnc []byte
	if len(masterKey) == 32 && cfg.KeyPEM != "" {
		// Encrypt new rows with the HKDF-derived saml key (ADR-0038).
		samlKey := samlServiceKey(masterKey)
		enc, err := sharedcrypto.Encrypt(samlKey, []byte(cfg.KeyPEM))
		if err != nil {
			return fmt.Errorf("saml: encrypt private key: %w", err)
		}
		keyEnc = enc
	}
	_, err := db.Exec(ctx, `
		INSERT INTO org_saml_configs (org_id, entity_id, acs_url, idp_metadata, cert_pem, key_pem, enabled, updated_at)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (org_id) DO UPDATE SET
			entity_id    = EXCLUDED.entity_id,
			acs_url      = EXCLUDED.acs_url,
			idp_metadata = EXCLUDED.idp_metadata,
			cert_pem     = EXCLUDED.cert_pem,
			key_pem      = EXCLUDED.key_pem,
			enabled      = EXCLUDED.enabled,
			updated_at   = NOW()`,
		cfg.OrgID, cfg.EntityID, cfg.ACSURL, cfg.IDPMetadata, cfg.CertPEM, keyEnc, cfg.Enabled,
	)
	return err
}

// GenerateSAMLCert creates a self-signed RSA-2048 certificate valid for 10 years.
// Returns certPEM and keyPEM strings.
func GenerateSAMLCert(orgID string) (certPEM, keyPEM string, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("saml cert: generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", fmt.Errorf("saml cert: serial: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "vakt-saml-sp-" + orgID,
			Organization: []string{"Vakt"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return "", "", fmt.Errorf("saml cert: create: %w", err)
	}

	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	return certPEM, keyPEM, nil
}

// buildSAMLSP constructs a crewjam/saml ServiceProvider from an OrgSAMLConfig.
func buildSAMLSP(cfg *OrgSAMLConfig) (*saml.ServiceProvider, error) {
	// Parse private key
	keyBlock, _ := pem.Decode([]byte(cfg.KeyPEM))
	if keyBlock == nil {
		return nil, fmt.Errorf("saml: invalid key PEM")
	}
	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("saml: parse private key: %w", err)
	}

	// Parse certificate
	certBlock, _ := pem.Decode([]byte(cfg.CertPEM))
	if certBlock == nil {
		return nil, fmt.Errorf("saml: invalid cert PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("saml: parse certificate: %w", err)
	}

	// Parse IdP metadata — limit input size before decoding to mitigate
	// Billion-Laughs XML entity expansion attacks (SEC-M03 / CWE-776).
	if len(cfg.IDPMetadata) > samlMetadataMaxBytes {
		return nil, fmt.Errorf("saml: IdP metadata exceeds %d byte limit", samlMetadataMaxBytes)
	}
	var idpMetadata saml.EntityDescriptor
	dec := xml.NewDecoder(bytes.NewReader([]byte(cfg.IDPMetadata)))
	dec.Entity = map[string]string{} // disable entity expansion
	if err := dec.Decode(&idpMetadata); err != nil {
		return nil, fmt.Errorf("saml: parse IdP metadata: %w", err)
	}

	entityURL, err := url.Parse(cfg.EntityID)
	if err != nil {
		return nil, fmt.Errorf("saml: parse entity_id URL: %w", err)
	}
	acsURL, err := url.Parse(cfg.ACSURL)
	if err != nil {
		return nil, fmt.Errorf("saml: parse acs_url: %w", err)
	}

	sp := saml.ServiceProvider{
		Key:         key,
		Certificate: cert,
		MetadataURL: *entityURL,
		AcsURL:      *acsURL,
		IDPMetadata: &idpMetadata,
	}
	return &sp, nil
}

// masterKeyFromCfg returns the raw 32-byte master key from cfg.SecretKey (hex).
// Returns nil when SecretKey is not set (dev mode).
func masterKeyFromHex(hexKey string) []byte {
	if hexKey == "" {
		return nil
	}
	b, _ := hex.DecodeString(hexKey)
	if len(b) != 32 {
		return nil
	}
	return b
}

// SAMLDirectMetadata serves GET /api/v1/auth/saml/metadata using the org's
// direct SAML SP config. Falls back to Casdoor proxy if no direct config.
func (h *Handler) SAMLDirectMetadata(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		// Unauthenticated access: try Casdoor fallback
		return h.SAMLMetadata(c)
	}

	masterKey := masterKeyFromHex(h.cfg.SecretKey)
	cfg, err := LoadOrgSAMLConfig(c.Request().Context(), h.db, orgID, masterKey)
	if err != nil || cfg == nil || !cfg.Enabled {
		return h.SAMLMetadata(c)
	}

	sp, err := buildSAMLSP(cfg)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("saml_direct: build SP failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "SAML SP configuration error",
			"code":  "AUTH_SAML_CONFIG_ERROR",
		})
	}

	metadataXML, err := xml.MarshalIndent(sp.Metadata(), "", "  ")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to generate SP metadata",
			"code":  "AUTH_SAML_METADATA_ERROR",
		})
	}

	return c.Blob(http.StatusOK, "application/xml", metadataXML)
}

// SAMLInitiate handles GET /api/v1/auth/saml/initiate — SP-initiated login.
// Generates an AuthnRequest and redirects the user to the IdP.
func (h *Handler) SAMLInitiate(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "org context required",
			"code":  "AUTH_SAML_NO_ORG",
		})
	}

	masterKey := masterKeyFromHex(h.cfg.SecretKey)
	cfg, err := LoadOrgSAMLConfig(c.Request().Context(), h.db, orgID, masterKey)
	if err != nil || cfg == nil || !cfg.Enabled {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "SAML not configured for this organisation",
			"code":  "AUTH_SAML_NOT_CONFIGURED",
		})
	}

	sp, err := buildSAMLSP(cfg)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("saml_initiate: build SP failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "SAML SP configuration error",
			"code":  "AUTH_SAML_CONFIG_ERROR",
		})
	}

	// Build the AuthnRequest directly so we can capture its ID and bind it to
	// a short-lived cookie. The ID is matched against assertion.InResponseTo
	// in SAMLDirectACS — without this binding, a SAML response captured from
	// one user could be replayed against another's session (ADR-0036).
	authReq, err := sp.MakeAuthenticationRequest(
		sp.GetSSOBindingLocation(saml.HTTPRedirectBinding),
		saml.HTTPRedirectBinding,
		saml.HTTPPostBinding,
	)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("saml_initiate: make AuthnRequest failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to generate AuthnRequest",
			"code":  "AUTH_SAML_AUTHN_ERROR",
		})
	}
	redirectURL, err := authReq.Redirect("", sp)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("saml_initiate: redirect URL failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to encode AuthnRequest redirect",
			"code":  "AUTH_SAML_AUTHN_ERROR",
		})
	}

	// Sign the request ID with HMAC derived from the master key, set it as a
	// short-lived HttpOnly cookie. SameSite=None because the SAML IdP will
	// POST back from a foreign origin; Secure flag depends on the request
	// scheme.
	signed, err := signSAMLRequestID(masterKey, authReq.ID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("saml_initiate: sign request id failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "internal error", "code": "AUTH_SAML_AUTHN_ERROR",
		})
	}
	secure := CookieSecure(c)
	c.SetCookie(&http.Cookie{ // nosemgrep: cookie-missing-secure -- Secure is set via variable; static analysis can't resolve it
		Name:     samlRequestIDCookieName,
		Value:    signed,
		Path:     "/api/v1/auth/saml",
		MaxAge:   samlRequestIDTTLSeconds,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteNoneMode,
	})

	// Return redirect URL for SPA (frontend handles the redirect)
	return c.JSON(http.StatusOK, map[string]string{"redirect_url": redirectURL.String()})
}

// SAMLDirectACS handles POST /api/v1/auth/saml/acs using crewjam/saml for
// assertion validation. Falls back to Casdoor when no direct config.
func (h *Handler) SAMLDirectACS(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	var masterKey []byte
	var cfg *OrgSAMLConfig
	if orgID != "" {
		masterKey = masterKeyFromHex(h.cfg.SecretKey)
		var err error
		cfg, err = LoadOrgSAMLConfig(c.Request().Context(), h.db, orgID, masterKey)
		if err != nil {
			log.Error().Err(err).Str("org_id", orgID).Msg("saml_acs: load config failed")
		}
	}

	if cfg == nil || !cfg.Enabled {
		// No direct config — fall through to Casdoor-based handler
		return h.SAMLCallback(c)
	}

	sp, err := buildSAMLSP(cfg)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("saml_acs: build SP failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "SAML SP configuration error",
			"code":  "AUTH_SAML_CONFIG_ERROR",
		})
	}

	if err := c.Request().ParseForm(); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid form data",
			"code":  "AUTH_SAML_BAD_REQUEST",
		})
	}

	// Recover the AuthnRequest ID from the signed cookie set at /saml/initiate.
	// The ID is passed to ParseResponse so crewjam/saml can verify the
	// assertion's InResponseTo binding. Without this step any signed assertion
	// from the IdP would be accepted, enabling replay attacks (ADR-0036).
	var allowedRequestIDs []string
	if cookie, cerr := c.Cookie(samlRequestIDCookieName); cerr == nil && cookie.Value != "" {
		masterKey := masterKeyFromHex(h.cfg.SecretKey)
		if id, vErr := verifySAMLRequestID(masterKey, cookie.Value); vErr == nil {
			allowedRequestIDs = []string{id}
		} else {
			log.Warn().Err(vErr).Str("org_id", orgID).Msg("saml_acs: rejecting invalid saml_req_id cookie")
		}
	}
	// Always expire the cookie immediately — it is single-use by design.
	secure := CookieSecure(c)
	c.SetCookie(&http.Cookie{ // nosemgrep: cookie-missing-secure -- Secure is set via variable; static analysis can't resolve it
		Name:     samlRequestIDCookieName,
		Value:    "",
		Path:     "/api/v1/auth/saml",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteNoneMode,
	})

	assertion, err := sp.ParseResponse(c.Request(), allowedRequestIDs)
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("saml_acs: assertion validation failed")
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "SAML assertion validation failed",
			"code":  "AUTH_SAML_INVALID_ASSERTION",
		})
	}

	// Extract user attributes from assertion
	email := samlAttr(assertion, "email", "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress")
	if email == "" {
		email = assertion.Subject.NameID.Value
	}
	name := samlAttr(assertion, "displayName", "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname")

	if email == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "SAML assertion missing email claim",
			"code":  "AUTH_SAML_MISSING_EMAIL",
		})
	}

	deviceHint := c.Request().Header.Get("User-Agent")
	if len(deviceHint) > 120 {
		deviceHint = deviceHint[:120]
	}

	authResp, err := h.service.provisionSAMLUser(c.Request().Context(), orgID, assertion.Subject.NameID.Value, email, name, deviceHint)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("email_redacted", logsafe.RedactEmail(email)).Msg("saml_acs: provision user failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "SAML user provisioning failed",
			"code":  "AUTH_SAML_PROVISION_FAILED",
		})
	}

	return c.JSON(http.StatusOK, authResp)
}
