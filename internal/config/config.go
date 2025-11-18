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

type RBACConfig struct {
	PolicyFile string   `mapstructure:"policy_file"` // Path to the RBAC policy file
	Admins     []string `mapstructure:"admins"`      // List of admin emails
}

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

	RBAC RBACConfig `mapstructure:"rbac"`

	// User authentication TTL in days.
	UserAuthTTL uint `mapstructure:"user_auth_ttl"`

	BaseURL    string `mapstructure:"base_url"` // Base URL for the application. May be relative, e.g. /entry-acces/, or absolute, e.g. https://example.com/entry-access/
	SupportURL string `mapstructure:"support_url"`

	Storage Storage `mapstructure:"storage"`

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

func getConfigPath() string {
	if runningInDocker() {
		return "/app/instance"
	}
	return "./instance"
}

// LoadConfig reads configuration from environment variables and returns a Config struct.
func LoadConfig(configFile ...string) (*Config, error) {
	var cfg Config

	v := viper.New()
	v.SetConfigName("config")
	v.AddConfigPath(getConfigPath())
	v.AddConfigPath(".")
	v.SetEnvPrefix("")

	if len(configFile) > 0 {
		for _, path := range configFile {
			v.SetConfigFile(path)
		}
	}

	for k, val := range Defaults() {
		v.SetDefault(k, val)
	}

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

	v.SetDefault("ACCESS_LIST_FOLDER", accessListFolder) // Default folder for access lists

	// Load configuration from environment variables
	v.AutomaticEnv()

	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode into struct: %v", err)
	}

	// Verify skew is sensible, at max x0.5 of the token TTL
	if cfg.TokenExpirySkew > cfg.TokenTTL/2 {
		maxSkew := cfg.TokenTTL / 2
		slog.Warn("TOKEN_EXPIRY_SKEW must be at most 0.5 * TOKEN_TTL", slog.Int("actual", int(cfg.TokenExpirySkew)), slog.Int("max", int(maxSkew)))
		cfg.TokenExpirySkew = maxSkew
	}

	// Convert relative sqlite path to absolute instance folder
	if cfg.Storage.SQLite != nil {
		if cfg.Storage.SQLite.Path == ":memory:" {
			// In-memory database, do nothing
		} else if !os.IsPathSeparator(cfg.Storage.SQLite.Path[0]) {
			cfg.Storage.SQLite.Path = fmt.Sprintf("%s/%s", getConfigPath(), cfg.Storage.SQLite.Path)
		}
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
