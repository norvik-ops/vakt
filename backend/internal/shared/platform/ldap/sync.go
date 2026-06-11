package ldap

import (
	"context"
	"crypto/tls"
	"fmt"
	"regexp"
	"strings"

	ldaplib "github.com/go-ldap/ldap/v3"
)

// ldapFilterSafeRe validates that a user-supplied LDAP filter only contains
// characters that are safe in an LDAP filter expression. Filters that pass
// this check may still be syntactically invalid, but cannot inject
// metacharacters not already present in RFC 4515 filter syntax.
//
// Decision: we validate the whole filter string rather than escaping it because
// the configured UserFilter is a complete LDAP filter expression
// (e.g. "(&(objectClass=person)(uid=*))"), not a bare attribute value. Calling
// ldaplib.EscapeFilter on such a string would double-escape the parentheses and
// break the filter entirely. Instead we apply an allowlist that permits only
// chars used in RFC 4515 filter syntax plus common attribute-value characters.
var ldapFilterSafeRe = regexp.MustCompile(`^[\(\)&|!=<>~*a-zA-Z0-9=,._ @\-\/:]+$`)

// LDAPUser holds normalised user data retrieved from the directory.
type LDAPUser struct {
	DN          string
	Email       string
	DisplayName string
	Groups      []string
}

// Syncer connects to an LDAP directory and retrieves user records.
type Syncer struct {
	cfg Config
}

// NewSyncer creates a new Syncer for the given Config.
func NewSyncer(cfg Config) *Syncer {
	return &Syncer{cfg: cfg}
}

// ListUsers connects to LDAP, binds with the service account, and returns all
// user entries that match the configured UserFilter.
func (s *Syncer) ListUsers(ctx context.Context) ([]LDAPUser, error) {
	conn, err := s.dial()
	if err != nil {
		return nil, fmt.Errorf("ldap dial: %w", err)
	}
	defer conn.Close()

	if err := conn.Bind(s.cfg.BindDN, s.cfg.BindPass); err != nil {
		return nil, fmt.Errorf("ldap bind: %w", err)
	}

	filter := s.cfg.UserFilter
	if filter == "" {
		filter = "(objectClass=person)"
	}
	// Validate the filter against the allowlist to guard against injection of
	// unexpected metacharacters. The filter is administrator-supplied, but
	// defence-in-depth is worthwhile given that LDAP injection is a known attack
	// vector (CWE-90). Reject rather than silently sanitize so the operator is
	// alerted to a misconfiguration.
	if filter != "(objectClass=person)" && !ldapFilterSafeRe.MatchString(filter) {
		return nil, fmt.Errorf("ldap user filter contains disallowed characters — only RFC 4515 filter syntax is permitted")
	}

	req := ldaplib.NewSearchRequest(
		s.cfg.BaseDN,
		ldaplib.ScopeWholeSubtree,
		ldaplib.NeverDerefAliases,
		0, // sizeLimit (0 = server default)
		0, // timeLimit (0 = server default)
		false,
		filter,
		[]string{"dn", "mail", "displayName", "cn", "memberOf"},
		nil,
	)

	result, err := conn.SearchWithPaging(req, 500)
	if err != nil {
		return nil, fmt.Errorf("ldap search: %w", err)
	}

	users := make([]LDAPUser, 0, len(result.Entries))
	for _, entry := range result.Entries {
		user := LDAPUser{
			DN:     entry.DN,
			Email:  entry.GetAttributeValue("mail"),
			Groups: entry.GetAttributeValues("memberOf"),
		}

		// Prefer displayName; fall back to cn.
		if dn := entry.GetAttributeValue("displayName"); dn != "" {
			user.DisplayName = dn
		} else {
			user.DisplayName = entry.GetAttributeValue("cn")
		}

		// Simplify group DNs to their CN component for readability.
		for i, g := range user.Groups {
			user.Groups[i] = extractCN(g)
		}

		users = append(users, user)
	}

	return users, nil
}

// dial opens an LDAP connection, upgrading to TLS when configured.
func (s *Syncer) dial() (*ldaplib.Conn, error) {
	url := s.cfg.URL

	// ldaps:// scheme implies implicit TLS regardless of the TLS flag.
	if strings.HasPrefix(url, "ldaps://") || s.cfg.TLS {
		ldapsURL := url
		if !strings.HasPrefix(ldapsURL, "ldaps://") {
			ldapsURL = "ldaps://" + strings.TrimPrefix(ldapsURL, "ldap://")
		}
		conn, err := ldaplib.DialURL(ldapsURL, ldaplib.DialWithTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}))
		if err != nil {
			return nil, err
		}
		return conn, nil
	}

	// Plain LDAP with optional STARTTLS.
	conn, err := ldaplib.DialURL(url)
	if err != nil {
		return nil, err
	}
	if s.cfg.TLS {
		if err := conn.StartTLS(&tls.Config{MinVersion: tls.VersionTLS12}); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("starttls: %w", err)
		}
	}
	return conn, nil
}

// extractCN parses the first CN= component from an LDAP distinguished name.
// Falls back to returning the full string when no CN is found.
func extractCN(dn string) string {
	for _, part := range strings.Split(dn, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToUpper(part), "CN=") {
			return part[3:]
		}
	}
	return dn
}
