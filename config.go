package main

import (
	"fmt"
	"log/slog"

	"github.com/spf13/viper"
)

type Config struct {
	Secret      string `mapstructure:"secret"`
	TokenExpiry int    `mapstructure:"token_expiry"`
	NonceStore  string `mapstructure:"nonce_store"`
	LogLevel    string `mapstructure:"log_level"`
}

func LoadConfig() (*Config, error) {
	var cfg Config

	// Set defaults. Defaults needs to be defined for config fields to be populated from env.
	viper.SetDefault("SECRET", "")
	viper.SetDefault("TOKEN_EXPIRY", 30)
	viper.SetDefault("LOG_LEVEL", "info")
	viper.SetDefault("NONCE_STORE", "memory")

	// Load configuration from environment variables
	viper.AutomaticEnv()

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode into struct: %v", err)
	}

	// Warn if secret is missing - this is a critical security setting for production
	if cfg.Secret == "" {
		slog.Warn("Secret is not set")
	}

	return &cfg, nil
}
