package main

// https://www.golinuxcloud.com/golang-jwt/

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	. "entry-access-control/internal"
	. "entry-access-control/internal/config"
	. "entry-access-control/internal/utils"

	"github.com/joho/godotenv"
	qrcode "github.com/skip2/go-qrcode"
)

const DIST_DIR = "dist"

// Initialize logger
func InitLogger(cfg *Config) {

	logLevel := strings.ToUpper(cfg.LogLevel)

	switch logLevel {
	case "DEBUG":
		slog.SetLogLoggerLevel(slog.LevelDebug)
	case "INFO":
		slog.SetLogLoggerLevel(slog.LevelInfo)
	case "WARN":
		slog.SetLogLoggerLevel(slog.LevelWarn)
	case "ERROR":
		slog.SetLogLoggerLevel(slog.LevelError)
	default:
		slog.SetLogLoggerLevel(slog.LevelInfo)
		slog.Warn("Invalid log level, defaulting to info", "log_level", cfg.LogLevel)
	}
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
	server.Run()
}
