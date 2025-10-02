package config

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/viper"
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
	AllowedNetworks string `mapstructure:"allowed_networks"`

	SupportURL string `mapstructure:"support_url"`
}

var Cfg *Config

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
	viper.SetDefault("SUPPORT_URL", DEFAULT_SUPPORT_URL)

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
