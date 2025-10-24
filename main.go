package main

// https://www.golinuxcloud.com/golang-jwt/

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	. "entry-access-control/internal"
	"entry-access-control/internal/access"
	. "entry-access-control/internal/config"
	. "entry-access-control/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	qrcode "github.com/skip2/go-qrcode"
)

const DIST_DIR = "dist"

// Initialize logger
func InitLogger(cfg *Config) *slog.Logger {

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
		Level:     level,
		AddSource: level <= slog.LevelDebug,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, handlerOpts))
	slog.SetDefault(logger)

	slog.Debug("Logger initialized", "level", level.String())
	return logger
}

// Generate static QR code for support
func genSupportQr(url string) {
	qrCode, err := qrcode.Encode(url, qrcode.Medium, QR_IMAGE_SIZE)
	if err != nil {
		log.Fatalf("Error generating support QR code: %v", err)
	}

	filePath := fmt.Sprintf("%s/assets/support_qr.png", DIST_DIR)

	// Save the QR code to a file
	if err := os.WriteFile(filePath, qrCode, 0644); err != nil {
		slog.Error("Error saving support QR code", "error", err)
	} else {
		slog.Debug("Support QR code saved successfully", "file_path", filePath, "support_url", url)
	}
}

func main() {
	// Load config
	var err error

	godotenv.Load()

	Cfg, err = LoadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	InitLogger(Cfg)
	InitNonceStore(Cfg)

	if Cfg.SupportURL != "" {
		genSupportQr(Cfg.SupportURL)
	}

	// Initialize HTTP server
	server := HTTPServer()

	// Initialize access list and inject into Gin context
	accessList := access.NewAccessList("csv", Cfg)
	server.Use(func(c *gin.Context) {
		c.Set("AccessList", accessList)
		c.Next()
	})

	server.Run()
}
