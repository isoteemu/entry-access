// Package access provides LDAP and access control functionality.
//
// LDAP Person Search:
//
// The LDAP client provides methods to search for people in LDAP directories and retrieve their information,
// including email addresses, names, and group memberships.
//
// Example usage:
//
//	client := NewLDAPClient(
//		"nauth2.cc.jyu.fi",
//		"ou=People,dc=cc,dc=jyu,dc=fi",
//		"",
//		"",
//		false,
//		slog.Default(),
//	)
//
//	person, err := client.Search("user@example.com")
//	if err != nil {
//		// handle error
//	}
//
//	// person.Groups contains the list of group DNs
//	for _, groupDN := range person.Groups {
//		log.Printf("User is member of: %s", groupDN)
//	}
package access

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

var (
	ErrLDAPNotConfigured = errors.New("LDAP is not configured")
	ErrLDAPConnection    = errors.New("failed to connect to LDAP server")
	ErrLDAPBind          = errors.New("failed to bind to LDAP server")
	ErrLDAPSearch        = errors.New("failed to search LDAP")
	ErrPersonNotFound    = errors.New("person not found in LDAP")
)

// LDAPPerson represents a person retrieved from LDAP
type LDAPPerson struct {
	DN                 string
	Email              string
	Name               string
	GivenName          string
	Surname            string
	Phone              string
	EmployeeNumber     string
	OrganizationalUnit string
	Groups             []string // List of group names (CN extracted from memberOf when ou=Group, otherwise full DN)
}

// Client wraps LDAP client functionality
type LDAPClient struct {
	Server       string
	BaseDN       string
	BindDN       string
	BindPassword string
	UseTLS       bool
	InsecureTLS  bool
	Logger       *slog.Logger
}

// NewLDAPClient creates a new LDAP client
func NewLDAPClient(server, baseDN, bindDN, bindPassword string, useTLS bool, logger *slog.Logger) *LDAPClient {
	if logger == nil {
		logger = slog.Default()
	}
	return &LDAPClient{
		Server:       server,
		BaseDN:       baseDN,
		BindDN:       bindDN,
		BindPassword: bindPassword,
		UseTLS:       useTLS,
		Logger:       logger,
	}
}

// connect establishes a connection to the LDAP server
func (c *LDAPClient) connect() (*ldap.Conn, error) {
	var conn *ldap.Conn
	var err error

	if c.UseTLS {
		tlsConfig := &tls.Config{
			ServerName:         c.Server,
			InsecureSkipVerify: c.InsecureTLS,
		}
		conn, err = ldap.DialURL("ldaps://"+c.Server, ldap.DialWithTLSConfig(tlsConfig))
	} else {
		conn, err = ldap.Dial("tcp", c.Server)
	}

	if err != nil {
		c.Logger.Error("Failed to connect to LDAP server",
			slog.String("server", c.Server),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("%w: %v", ErrLDAPConnection, err)
	}

	return conn, nil
}

// bind authenticates with the LDAP server
func (c *LDAPClient) bind(conn *ldap.Conn) error {
	if c.BindDN == "" || c.BindPassword == "" {
		// Anonymous bind
		return nil
	}

	err := conn.Bind(c.BindDN, c.BindPassword)
	if err != nil {
		c.Logger.Error("Failed to bind to LDAP server",
			slog.String("bindDN", c.BindDN),
			slog.String("error", err.Error()))
		return fmt.Errorf("%w: %v", ErrLDAPBind, err)
	}

	return nil
}

// parseGroupDN extracts a readable group name from a DN.
// If the DN contains ou=Group, it returns just the CN value.
// Otherwise, it returns the full DN.
func parseGroupDN(dn string) string {
	// Parse the DN to check if it contains ou=Group
	parsedDN, err := ldap.ParseDN(dn)
	if err != nil {
		return dn // Return full DN if parsing fails
	}

	// Check if this is a group (contains ou=Group)
	isGroup := false
	for _, rdn := range parsedDN.RDNs {
		for _, attr := range rdn.Attributes {
			if attr.Type == "ou" && attr.Value == "Group" {
				isGroup = true
				break
			}
		}
		if isGroup {
			break
		}
	}

	// If it's a group, extract just the CN
	if isGroup && len(parsedDN.RDNs) > 0 {
		for _, attr := range parsedDN.RDNs[0].Attributes {
			if attr.Type == "cn" {
				return attr.Value
			}
		}
	}

	// Return full DN for non-group entries or if CN not found
	return dn
}

// parseGroupDNs processes a list of group DNs and returns readable group names
func parseGroupDNs(groupDNs []string) []string {
	result := make([]string, len(groupDNs))
	for i, dn := range groupDNs {
		result[i] = parseGroupDN(dn)
	}
	return result
}

// ExtractUID extracts the UID (CN or UID value) from a DN
func ExtractUID(dn string) string {
	parsedDN, err := ldap.ParseDN(dn)
	if err != nil {
		return dn
	}

	if len(parsedDN.RDNs) > 0 {
		for _, attr := range parsedDN.RDNs[0].Attributes {
			if attr.Type == "cn" || attr.Type == "uid" {
				return attr.Value
			}
		}
	}

	return dn
}

// Search searches for a person in LDAP by email or uid
func (c *LDAPClient) Search(email string) (*LDAPPerson, error) {
	if c.Server == "" || c.BaseDN == "" {
		return nil, ErrLDAPNotConfigured
	}

	conn, err := c.connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := c.bind(conn); err != nil {
		return nil, err
	}

	filter := ""
	escapedEmail := ldap.EscapeFilter(email)
	if strings.Contains(email, "@") == true {
		filter = fmt.Sprintf("(mail=%s)", escapedEmail)

	} else {
		filter = fmt.Sprintf("(uid=%s)", escapedEmail)
	}

	c.Logger.Debug("Searching LDAP",
		slog.String("baseDN", c.BaseDN),
		slog.String("filter", filter))

	sr := ldap.NewSearchRequest(
		c.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,     // size limit
		0,     // time limit
		false, // types only
		filter,
		[]string{"mail", "cn", "givenName", "sn", "telephoneNumber", "employeeNumber", "ou", "dn", "memberOf"},
		nil,
	)

	res, err := conn.Search(sr)
	if err != nil {
		c.Logger.Error("LDAP search failed",
			slog.String("filter", filter),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("%w: %v", ErrLDAPSearch, err)
	}

	if len(res.Entries) == 0 {
		c.Logger.Debug("Person not found in LDAP", slog.String("email", email))
		return nil, ErrPersonNotFound
	}

	if len(res.Entries) > 1 {
		c.Logger.Warn("Multiple entries found for email",
			slog.String("email", email),
			slog.Int("count", len(res.Entries)))
	}

	entry := res.Entries[0]
	person := &LDAPPerson{
		DN:                 entry.DN,
		Email:              entry.GetAttributeValue("mail"),
		Name:               entry.GetAttributeValue("cn"),
		GivenName:          entry.GetAttributeValue("givenName"),
		Surname:            entry.GetAttributeValue("sn"),
		Phone:              entry.GetAttributeValue("telephoneNumber"),
		EmployeeNumber:     entry.GetAttributeValue("employeeNumber"),
		OrganizationalUnit: entry.GetAttributeValue("ou"),
		Groups:             parseGroupDNs(entry.GetAttributeValues("memberOf")),
	}

	c.Logger.Debug("Found person in LDAP",
		slog.String("email", email),
		slog.String("dn", person.DN),
		slog.String("name", person.Name),
		slog.Int("groupCount", len(person.Groups)))

	return person, nil
}

// ValidateCredentials validates a user's credentials against LDAP
func (c *LDAPClient) ValidateCredentials(dn, password string) error {
	if c.Server == "" {
		return ErrLDAPNotConfigured
	}

	conn, err := c.connect()
	if err != nil {
		return err
	}
	defer conn.Close()

	err = conn.Bind(dn, password)
	if err != nil {
		c.Logger.Debug("Failed to authenticate user",
			slog.String("dn", dn),
			slog.String("error", err.Error()))
		return fmt.Errorf("authentication failed: %w", err)
	}

	c.Logger.Debug("User authenticated successfully", slog.String("dn", dn))
	return nil
}

// Verify checks if LDAP server is configured and reachable
func (c *LDAPClient) Verify() error {
	if c.Server == "" || c.BaseDN == "" {
		return ErrLDAPNotConfigured
	}

	conn, err := c.connect()
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := c.bind(conn); err != nil {
		return err
	}

	c.Logger.Info("LDAP connection verified successfully", slog.String("server", c.Server))
	return nil
}
