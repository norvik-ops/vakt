// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S21-1: Direct SAML 2.0 SP implementation using crewjam/saml.
// This supplements the Casdoor-based SAML proxy: when an org has a row in
// org_saml_configs, all SAML traffic is handled here without Casdoor.
// Casdoor-based SAML continues to work for orgs that don't configure direct SAML.
package auth

import (
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
)

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
		plain, err := sharedcrypto.Decrypt(masterKey, keyEnc)
		if err != nil {
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
		enc, err := sharedcrypto.Encrypt(masterKey, []byte(cfg.KeyPEM))
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

	// Parse IdP metadata
	var idpMetadata saml.EntityDescriptor
	if err := xml.Unmarshal([]byte(cfg.IDPMetadata), &idpMetadata); err != nil {
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

	// Use samlsp helper to build the redirect URL.
	redirectURL, err := sp.MakeRedirectAuthenticationRequest("")
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("saml_initiate: make AuthnRequest failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to generate AuthnRequest",
			"code":  "AUTH_SAML_AUTHN_ERROR",
		})
	}

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

	assertion, err := sp.ParseResponse(c.Request(), nil)
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
		log.Error().Err(err).Str("org_id", orgID).Str("email", email).Msg("saml_acs: provision user failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "SAML user provisioning failed",
			"code":  "AUTH_SAML_PROVISION_FAILED",
		})
	}

	return c.JSON(http.StatusOK, authResp)
}
