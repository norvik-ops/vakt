// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"

	ldaplib "github.com/go-ldap/ldap/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

const ldapSource = "ldap-collector"

// LDAPEvidenceCollector collects compliance evidence from LDAP/Active Directory.
type LDAPEvidenceCollector struct {
	db       *pgxpool.Pool
	evidence EvidenceWriter
}

// NewLDAPEvidenceCollector creates a new LDAPEvidenceCollector.
func NewLDAPEvidenceCollector(db *pgxpool.Pool, evidence EvidenceWriter) *LDAPEvidenceCollector {
	return &LDAPEvidenceCollector{
		db:       db,
		evidence: evidence,
	}
}

// Collect runs all LDAP/AD evidence collectors. Returns the number of evidence items created.
func (c *LDAPEvidenceCollector) Collect(ctx context.Context, orgID string, cfg LDAPConfig) (int, error) {
	conn, err := c.connect(cfg)
	if err != nil {
		return 0, fmt.Errorf("ldap connect: %w", err)
	}
	defer conn.Close()

	bindPw := cfg.BindPassword
	if err := conn.Bind(cfg.BindDN, bindPw); err != nil {
		return 0, fmt.Errorf("ldap bind: %w", err)
	}

	accessControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"access", "identity", "rights", "account"})
	authControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"password", "authentication", "credential"})
	adminControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"privileged", "admin", "access"})

	total := 0

	if n, err := c.collectInactiveUsers(ctx, orgID, conn, cfg, accessControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("ldap_collector: inactive users failed")
	} else {
		total += n
	}

	if n, err := c.collectPasswordNeverExpires(ctx, orgID, conn, cfg, authControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("ldap_collector: password never expires failed")
	} else {
		total += n
	}

	if n, err := c.collectPrivilegedGroups(ctx, orgID, conn, cfg, adminControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("ldap_collector: privileged groups failed")
	} else {
		total += n
	}

	if n, err := c.collectDisabledUsers(ctx, orgID, conn, cfg, accessControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("ldap_collector: disabled users failed")
	} else {
		total += n
	}

	if n, err := c.collectActiveUserCount(ctx, orgID, conn, cfg, accessControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("ldap_collector: active user count failed")
	} else {
		total += n
	}

	return total, nil
}

// connect dials the LDAP server using the config (plain, STARTTLS, or LDAPS).
func (c *LDAPEvidenceCollector) connect(cfg LDAPConfig) (*ldaplib.Conn, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	if cfg.UseTLS {
		tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
		conn, err := ldaplib.DialURL("ldaps://"+addr, ldaplib.DialWithTLSConfig(tlsCfg))
		if err != nil {
			return nil, fmt.Errorf("ldaps dial: %w", err)
		}
		return conn, nil
	}

	conn, err := ldaplib.DialURL("ldap://" + addr)
	if err != nil {
		return nil, fmt.Errorf("ldap dial: %w", err)
	}
	return conn, nil
}

// windowsFiletimeToTime converts a Windows FILETIME (100-ns ticks since 1601-01-01) to time.Time.
// Conversion via Unix epoch avoids int64 overflow: the offset from Windows epoch (1601-01-01)
// to Unix epoch (1970-01-01) is 116,444,736,000,000,000 100-ns ticks.
func windowsFiletimeToTime(fileTime int64) time.Time {
	if fileTime <= 0 {
		return time.Time{}
	}
	const windowsToUnixOffset int64 = 116_444_736_000_000_000
	unixTicks := fileTime - windowsToUnixOffset
	unixSec := unixTicks / 10_000_000
	unixNsec := (unixTicks % 10_000_000) * 100
	return time.Unix(unixSec, unixNsec).UTC()
}

// collectInactiveUsers searches for accounts not logged in for more than 90 days.
func (c *LDAPEvidenceCollector) collectInactiveUsers(ctx context.Context, orgID string, conn *ldaplib.Conn, cfg LDAPConfig, controls []ControlMatch) (int, error) {
	threshold := time.Now().UTC().AddDate(0, 0, -90)

	var filter string
	if cfg.IsActiveDirectory {
		// Windows FILETIME: 100-ns intervals since 1601-01-01
		windowsEpoch := time.Date(1601, 1, 1, 0, 0, 0, 0, time.UTC)
		ticks := threshold.Sub(windowsEpoch).Nanoseconds() / 100
		// Enabled accounts with lastLogon older than threshold (or never logged in: lastLogon=0)
		filter = fmt.Sprintf(
			"(&(objectClass=user)(!(userAccountControl:1.2.840.113556.1.4.803:=2))(|(lastLogon=0)(lastLogon<=%d)))",
			ticks,
		)
	} else {
		// OpenLDAP: shadowLastChange in days since Unix epoch
		thresholdDays := threshold.Unix() / 86400
		filter = fmt.Sprintf(
			"(&(objectClass=inetOrgPerson)(|(!(shadowLastChange=*))(shadowLastChange<=%d)))",
			thresholdDays,
		)
	}

	req := ldaplib.NewSearchRequest(
		cfg.BaseDN,
		ldaplib.ScopeWholeSubtree,
		ldaplib.NeverDerefAliases,
		0, 0, false,
		filter,
		[]string{"dn", "cn", "sAMAccountName"},
		nil,
	)

	result, err := conn.SearchWithPaging(req, 500)
	if err != nil {
		return 0, fmt.Errorf("ldap search inactive users: %w", err)
	}

	count := len(result.Entries)
	status := "ok"
	if count > 0 {
		status = "warning"
	}

	details := map[string]any{
		"collected_at":   time.Now().UTC().Format(time.RFC3339),
		"inactive_users": count,
		"threshold_days": 90,
		"status":         status,
	}

	title := fmt.Sprintf("LDAP/AD Inaktive Accounts: %d User seit >90 Tagen nicht eingeloggt", count)
	if err := c.addEvidence(ctx, orgID, firstControlID(controls), title, details); err != nil {
		return 0, err
	}
	return 1, nil
}

// collectPasswordNeverExpires finds accounts with the "password never expires" flag set (AD-only).
func (c *LDAPEvidenceCollector) collectPasswordNeverExpires(ctx context.Context, orgID string, conn *ldaplib.Conn, cfg LDAPConfig, controls []ControlMatch) (int, error) {
	var filter string
	if cfg.IsActiveDirectory {
		// userAccountControl bit 65536 (0x10000) = DONT_EXPIRE_PASSWD
		filter = "(&(objectClass=user)(userAccountControl:1.2.840.113556.1.4.803:=65536))"
	} else {
		// OpenLDAP: shadowMax=99999 or not set typically means no expiry
		filter = "(&(objectClass=inetOrgPerson)(shadowMax=99999))"
	}

	req := ldaplib.NewSearchRequest(
		cfg.BaseDN,
		ldaplib.ScopeWholeSubtree,
		ldaplib.NeverDerefAliases,
		0, 0, false,
		filter,
		[]string{"dn", "cn"},
		nil,
	)

	result, err := conn.SearchWithPaging(req, 500)
	if err != nil {
		return 0, fmt.Errorf("ldap search password never expires: %w", err)
	}

	count := len(result.Entries)
	status := "ok"
	if count > 0 {
		status = "warning"
	}

	details := map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"count":        count,
		"status":       status,
	}

	title := fmt.Sprintf("LDAP/AD Password-Hygiene: %d Accounts mit 'Passwort läuft nie ab'", count)
	if err := c.addEvidence(ctx, orgID, firstControlID(controls), title, details); err != nil {
		return 0, err
	}
	return 1, nil
}

// collectPrivilegedGroups finds members of privileged groups (Domain Admins, Administrators etc.).
func (c *LDAPEvidenceCollector) collectPrivilegedGroups(ctx context.Context, orgID string, conn *ldaplib.Conn, cfg LDAPConfig, controls []ControlMatch) (int, error) {
	groups := cfg.PrivilegedGroups
	if len(groups) == 0 {
		groups = []string{"Domain Admins", "Administrators"}
	}

	totalMembers := 0
	groupCounts := map[string]int{}

	for _, groupName := range groups {
		filter := fmt.Sprintf("(&(objectClass=user)(memberOf=CN=%s,%s))", ldaplib.EscapeFilter(groupName), cfg.BaseDN)

		req := ldaplib.NewSearchRequest(
			cfg.BaseDN,
			ldaplib.ScopeWholeSubtree,
			ldaplib.NeverDerefAliases,
			0, 0, false,
			filter,
			[]string{"dn"},
			nil,
		)

		result, err := conn.SearchWithPaging(req, 500)
		if err != nil {
			log.Warn().Err(err).Str("group", groupName).Msg("ldap_collector: privileged group search failed")
			continue
		}
		n := len(result.Entries)
		groupCounts[groupName] = n
		totalMembers += n
	}

	details := map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"total":        totalMembers,
		"by_group":     groupCounts,
	}

	title := fmt.Sprintf("LDAP/AD Privilegierte Accounts: %d Mitglieder in privilegierten Gruppen", totalMembers)
	if err := c.addEvidence(ctx, orgID, firstControlID(controls), title, details); err != nil {
		return 0, err
	}
	return 1, nil
}

// collectDisabledUsers finds accounts deactivated in the last 30 days as offboarding evidence.
func (c *LDAPEvidenceCollector) collectDisabledUsers(ctx context.Context, orgID string, conn *ldaplib.Conn, cfg LDAPConfig, controls []ControlMatch) (int, error) {
	var filter string
	if cfg.IsActiveDirectory {
		// Accounts with userAccountControl bit 2 (disabled) set
		filter = "(&(objectClass=user)(userAccountControl:1.2.840.113556.1.4.803:=2))"
	} else {
		// OpenLDAP: pwdAccountLockedTime is set
		filter = "(&(objectClass=inetOrgPerson)(pwdAccountLockedTime=*))"
	}

	req := ldaplib.NewSearchRequest(
		cfg.BaseDN,
		ldaplib.ScopeWholeSubtree,
		ldaplib.NeverDerefAliases,
		0, 0, false,
		filter,
		[]string{"dn", "cn"},
		nil,
	)

	result, err := conn.SearchWithPaging(req, 500)
	if err != nil {
		return 0, fmt.Errorf("ldap search disabled users: %w", err)
	}

	count := len(result.Entries)
	details := map[string]any{
		"collected_at":   time.Now().UTC().Format(time.RFC3339),
		"disabled_count": count,
		"status":         "ok",
	}

	title := fmt.Sprintf("LDAP/AD Offboarding-Nachweis: %d Accounts deaktiviert", count)
	if err := c.addEvidence(ctx, orgID, firstControlID(controls), title, details); err != nil {
		return 0, err
	}
	return 1, nil
}

// collectActiveUserCount collects the total number of active enabled user accounts.
func (c *LDAPEvidenceCollector) collectActiveUserCount(ctx context.Context, orgID string, conn *ldaplib.Conn, cfg LDAPConfig, controls []ControlMatch) (int, error) {
	var filter string
	if cfg.IsActiveDirectory {
		filter = "(&(objectClass=user)(!(userAccountControl:1.2.840.113556.1.4.803:=2)))"
	} else {
		filter = "(&(objectClass=inetOrgPerson)(!(pwdAccountLockedTime=*)))"
	}

	req := ldaplib.NewSearchRequest(
		cfg.BaseDN,
		ldaplib.ScopeWholeSubtree,
		ldaplib.NeverDerefAliases,
		0, 0, false,
		filter,
		[]string{"dn"},
		nil,
	)

	result, err := conn.SearchWithPaging(req, 500)
	if err != nil {
		return 0, fmt.Errorf("ldap count active users: %w", err)
	}

	count := len(result.Entries)
	details := map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"active_users": count,
		"status":       "ok",
	}

	title := fmt.Sprintf("LDAP/AD Asset-Übersicht: %d aktive User-Accounts im Verzeichnis", count)
	if err := c.addEvidence(ctx, orgID, firstControlID(controls), title, details); err != nil {
		return 0, err
	}
	return 1, nil
}

func (c *LDAPEvidenceCollector) addEvidence(ctx context.Context, orgID, controlID, title string, details map[string]any) error {
	data, _ := json.Marshal(details)
	if controlID == "" {
		return nil
	}
	return c.evidence.AddCollectorEvidence(ctx, orgID, controlID, "", ldapSource, title, data)
}
