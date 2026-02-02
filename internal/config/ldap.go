package config

type LDAPConfig struct {
	Server       string `mapstructure:"server"`        // LDAP server address (e.g., "nauth2.cc.jyu.fi" or "ldap.example.com:389")
	BaseDN       string `mapstructure:"base_dn"`       // Base DN for searches (e.g., "ou=People,dc=cc,dc=jyu,dc=fi")
	BindDN       string `mapstructure:"bind_dn"`       // DN to bind with (optional, for authenticated searches)
	BindPassword string `mapstructure:"bind_password"` // Password for bind DN (optional)
	UseTLS       bool   `mapstructure:"use_tls"`       // Use LDAPS (TLS) instead of plain LDAP
	InsecureTLS  bool   `mapstructure:"insecure_tls"`  // Skip TLS certificate verification (not recommended for production)
}
