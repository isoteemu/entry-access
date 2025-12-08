package cmd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	. "entry-access-control/internal"
	"entry-access-control/internal/access"
	"entry-access-control/internal/config"
	"entry-access-control/internal/nonce"
	"entry-access-control/internal/routes"
	"entry-access-control/internal/storage"

	"github.com/gin-gonic/gin"
	qrcode "github.com/skip2/go-qrcode"
	"github.com/spf13/cobra"
)

const DIST_DIR = "dist"

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the entry access control server",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		fmt.Println("Starting entry access control server...")
		ServerMain(ctx, provider)
	},
}

// Initialize logger
func initLogger(cfg *config.Config) *slog.Logger {
	// Determine level from config and set it on the handler options.
	var level slog.Level
	switch strings.ToUpper(cfg.LogLevel) {
	case "DEBUG":
		level = slog.LevelDebug
	case "INFO":
		level = slog.LevelInfo
	case "WARN", "WARNING":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
		println("Invalid log level in config, defaulting to INFO")
	}
	handlerOpts := &slog.HandlerOptions{
		Level: level,
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, handlerOpts))
	slog.SetDefault(logger)

	slog.Debug("Logger initialized", "level", level.String())
	return logger
}

// Generate static QR code for support
func genSupportQr(url string) {
	qrCode, err := qrcode.Encode(url, qrcode.Medium, config.QR_IMAGE_SIZE)
	if err != nil {
		slog.Error("Error generating support QR code", "error", err)
		return
	}

	filePath := fmt.Sprintf("%s/assets/support_qr.png", DIST_DIR)

	// Save the QR code to a file
	if err := os.WriteFile(filePath, qrCode, 0644); err != nil {
		slog.Error("Error saving support QR code", "error", err)
	} else {
		slog.Debug("Support QR code saved successfully", "file_path", filePath, "support_url", url)
	}
}

// generateSecretKey generates a cryptographically secure random secret key
func generateSecretKey() (string, error) {
	// Generate 32 bytes (256 bits) of random data
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("failed to generate random key: %w", err)
	}
	// Encode as base64 for storage
	return base64.StdEncoding.EncodeToString(key), nil
}

// ensureSecretKey ensures a secret key exists, either from config, file, or generates one
func ensureSecretKey(cfg *config.Config) error {
	// If secret is already set in config, nothing to do
	if cfg.Secret != "" {
		return nil
	}

	secretFilePath := filepath.Join(cfg.InstancePath, ".secret.key")

	// Try to read existing secret from file
	if data, err := os.ReadFile(secretFilePath); err == nil {
		secret := strings.TrimSpace(string(data))
		if secret != "" {
			cfg.Secret = secret
			slog.Info("Loaded secret key from file", "path", secretFilePath)
			return nil
		}
	}

	// Generate new secret key
	secret, err := generateSecretKey()
	if err != nil {
		return fmt.Errorf("failed to generate secret key: %w", err)
	}

	// Try to save to file
	if err := os.WriteFile(secretFilePath, []byte(secret), 0600); err != nil {
		return fmt.Errorf("failed to save secret key to file: %w", err)
	}

	cfg.Secret = secret
	slog.Info("Generated and saved new secret key", "path", secretFilePath)
	return nil
}

func NewAccessListFromConfig(cfg *config.Config) access.AccessList {
	// Initialize access list
	// TODO: Load type from config
	accessList := access.NewAccessList("csv", cfg)
	if accessList == nil {
		slog.Error("Failed to initialize access list")
		return nil
	}
	return accessList
}

func LoadAccessRBAC(cfg *config.Config) *access.RBAC {
	// Initialize access list
	accessList := NewAccessListFromConfig(cfg)
	if accessList == nil {
		slog.Error("Failed to initialize access list")
		os.Exit(1)
	}

	// Initialize RBAC
	rbac := access.GetRBAC()
	if err := rbac.LoadPolicy(config.Cfg.RBAC.PolicyFile); err != nil {
		slog.Error("Failed to load RBAC policy", "error", err, "file", config.Cfg.RBAC.PolicyFile)
		os.Exit(1)
	}
	// Inject students from access list as "student" role
	accessListEntries, err := accessList.ListAllEntries()
	if err != nil {
		slog.Error("Failed to list access list entries", "error", err)
		os.Exit(1)
	}
	for _, entry := range accessListEntries {
		rbac.AssignRole(entry.GetUserID(), entry.GetUserRoles()...)
	}
	return rbac
}

func ServerMain(ctx context.Context, storageProvider storage.Provider) {

	if config.Cfg == nil {
		panic("Config not initialized.")
	}

	// Ensure secret key exists (load from file or generate new one)
	if err := ensureSecretKey(config.Cfg); err != nil {
		slog.Error("Failed to ensure secret key", "error", err)
		os.Exit(1)
	}

	// Use the provider passed from cobra command (already initialized)
	if storageProvider == nil {
		slog.Error("Storage provider is nil")
		os.Exit(1)
	}

	nonce.InitNonceStore(config.Cfg, storageProvider)

	if config.Cfg.SupportURL != "" {
		genSupportQr(config.Cfg.SupportURL)
	}

	// Initialize HTTP server
	server := HTTPServer()

	// Initialize RBAC and access list
	rbac := LoadAccessRBAC(config.Cfg)

	// Middleware to inject storage provider into context
	server.Use(func(c *gin.Context) {
		c.Set("Storage", storageProvider)
		c.Next()
	}, func(c *gin.Context) {
		c.Set("RBAC", rbac)
		c.Next()
	}, routes.ErrorHandler())

	RegisterRoutes(server)

	server.Run()
}

func init() {
	rootCmd.AddCommand(serverCmd)
}
