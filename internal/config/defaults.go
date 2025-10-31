package config

var defaults = map[string]any{
	"secret":            "",
	"token_ttl":         60,
	"token_expiry_skew": 5,
	"log_level":         "info",

	"nonce_store": "memory",

	"allowed_networks": "",

	"admins":        []string{},
	"user_auth_ttl": 8, // 8 days
	"support_url":   DEFAULT_SUPPORT_URL,
	"base_url":      "/",

	"email.host":     "host.docker.internal",
	"email.port":     "25",
	"email.username": "",
	"email.password": "",
	"email.from":     "noreply@example.com",

	"storage.type":        "sqlite",
	"storage.sqlite.path": "./data/storage.db",
}

func Defaults() map[string]any {
	values := make(map[string]any)
	for k, v := range defaults {
		values[k] = v
	}
	return values
}
