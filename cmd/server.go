package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	. "entry-access-control/internal"
	"entry-access-control/internal/access"
	"entry-access-control/internal/config"
	"entry-access-control/internal/storage"
	. "entry-access-control/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
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

func ServerMain(ctx context.Context, storageProvider storage.Provider) {
	// Load environment variables
	godotenv.Load()

	// Load config
	var err error
	config.Cfg, err = config.LoadConfig()
	if err != nil {
		slog.Error("Error loading config", "error", err)
		os.Exit(1)
	}

	initLogger(config.Cfg)
	slog.Debug("Configuration loaded", "config", config.Cfg)

	// Use the provider passed from cobra command (already initialized)
	if storageProvider == nil {
		slog.Error("Storage provider is nil")
		os.Exit(1)
	}
	slog.Info("Storage provider initialized")

	InitNonceStore(config.Cfg)

	if config.Cfg.SupportURL != "" {
		genSupportQr(config.Cfg.SupportURL)
	}

	// Initialize HTTP server
	server := HTTPServer()

	// Initialize access list and inject into Gin context
	accessList := access.NewAccessList("csv", config.Cfg)
	server.Use(func(c *gin.Context) {
		slog.Debug("Injecting access list into context")
		c.Set("AccessList", accessList)
		c.Next()
	}, func(c *gin.Context) {
		c.Set("Storage", storageProvider)
		c.Next()
	})

	RegisterRoutes(server)

	server.Run()
}

func init() {
	rootCmd.AddCommand(serverCmd)
}
