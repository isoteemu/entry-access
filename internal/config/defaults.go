package config

var defaults = map[string]any{
	"secret":            "",
	"token_ttl":         60,
	"token_expiry_skew": 5,
	"log_level":         "info",

	"nonce_store": "memory",

	"allowed_networks": "",

	"user_auth_ttl": 8, // 8 days
	"support_url":   DEFAULT_SUPPORT_URL,
	"base_url":      "/",

	"RBAC": map[string]any{
		"policy_file": "./rbac.yaml",
		"admins":      []string{},
	},

	"Storage": map[string]any{
		"SQLite": map[string]any{
			"Path": "./storage.db",
		},
	},

	"Email": map[string]any{
		"Host":     "host.docker.internal",
		"Port":     25,
		"Username": "",
		"Password": "",
		"From":     "noreply@example.com",
	},
}

func Defaults() map[string]any {
	values := make(map[string]any)
	for k, v := range defaults {
		values[k] = v
	}
	return values
}
