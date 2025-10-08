package config

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/viper"

	"entry-access-control/internal/email"
)

const DEFAULT_SUPPORT_URL = "https://github.com/isoteemu/entry-access"
const QR_IMAGE_SIZE = 512

type Config struct {
	// Secret key for signing tokens. Must be set in production.
	Secret string `mapstructure:"secret"`
	// TTL for tokens in seconds
	TokenTTL uint `mapstructure:"token_ttl"`
	// QR code expiry skew in seconds. QR code TTL is calculated `TokenExpiry + TokenExpirySkew`. NOT IMPLEMENTED YET
	TokenExpirySkew uint   `mapstructure:"token_expiry_skew"`
	NonceStore      string `mapstructure:"nonce_store"`
	LogLevel        string `mapstructure:"log_level"`

	// Comma separated list of allowed CIDR networks. Empty means allow all.
	AllowedNetworks  string `mapstructure:"allowed_networks"`
	AccessListFolder string `mapstructure:"access_list_folder"` // Folder for access list CSVs

	Admins []string `mapstructure:"admins"` // List of admin emails

	// User authentication TTL in days.
	UserAuthTTL uint `mapstructure:"user_auth_ttl"`

	SupportURL string `mapstructure:"support_url"`

	// Email login configuration
	Email email.SMTPConfig `mapstructure:",squash"`
}

var Cfg *Config

// Check if running in Docker container by checking for the presence of /.dockerenv file
func runningInDocker() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return false
}

// LoadConfig reads configuration from environment variables and returns a Config struct.
func LoadConfig() (*Config, error) {
	var cfg Config

	// Set defaults. Defaults needs to be defined for config fields to be populated from env.
	viper.SetDefault("SECRET", "")
	viper.SetDefault("TOKEN_TTL", 60)
	viper.SetDefault("TOKEN_EXPIRY_SKEW", 5)
	viper.SetDefault("LOG_LEVEL", "info")
	viper.SetDefault("NONCE_STORE", "memory")
	viper.SetDefault("ALLOWED_NETWORKS", "")

	// TODO: Testing, remove default in production
	viper.SetDefault("ADMINS", []string{})

	viper.SetDefault("USER_AUTH_TTL", 8) // 8 days

	viper.SetDefault("SUPPORT_URL", DEFAULT_SUPPORT_URL)

	var accessListFolder string
	// If running in Docker, use /app/instance, otherwise use ./instance relative to cwd
	if runningInDocker() {
		accessListFolder = "/app/instance/"
	} else {
		// Default folder for access lists
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("unable to get current working directory: %v", err)
		}
		accessListFolder = fmt.Sprintf("%s/instance/", cwd)
	}

	viper.SetDefault("ACCESS_LIST_FOLDER", accessListFolder) // Default folder for access lists

	// Email defaults
	viper.SetDefault("EMAIL_HOST", "host.docker.internal")
	viper.SetDefault("EMAIL_PORT", "25")
	viper.SetDefault("EMAIL_USERNAME", "")
	viper.SetDefault("EMAIL_PASSWORD", "")
	viper.SetDefault("EMAIL_FROM", "noreply@example.com")

	// Load configuration from environment variables
	viper.AutomaticEnv()

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode into struct: %v", err)
	}

	// Verify skew is sensible, at max x0.5 of the token TTL
	if cfg.TokenExpirySkew > cfg.TokenTTL/2 {
		maxSkew := cfg.TokenTTL / 2
		slog.Warn("TOKEN_EXPIRY_SKEW must be at most 0.5 * TOKEN_TTL", slog.Int("actual", int(cfg.TokenExpirySkew)), slog.Int("max", int(maxSkew)))
		cfg.TokenExpirySkew = maxSkew
	}

	// Warn if secret is missing - this is a critical security setting for production
	if cfg.Secret == "" {
		if os.Getenv("GIN_MODE") == "release" {
			panic("SECRET configuration variable is required in production")
		} else {
			slog.Warn("Secret is not set. Do not use in production.")
		}
	}

	return &cfg, nil
}
