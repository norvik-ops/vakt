package ldap

// Config holds LDAP connection and search configuration.
type Config struct {
	URL         string // e.g. ldap://dc.company.com:389 or ldaps://dc.company.com:636
	BindDN      string // e.g. CN=svc-account,DC=company,DC=com
	BindPass    string
	BaseDN      string // e.g. DC=company,DC=com
	UserFilter  string // default: (objectClass=person)
	GroupFilter string // default: (objectClass=group)
	TLS         bool
}

// Enabled reports whether the LDAP integration is fully configured.
// A minimum viable config requires URL, BindDN, and BaseDN.
func (c Config) Enabled() bool {
	return c.URL != "" && c.BindDN != "" && c.BaseDN != ""
}
