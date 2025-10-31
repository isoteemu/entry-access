package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	"entry-access-control/cmd"
	"entry-access-control/internal/config"
	"entry-access-control/internal/storage"

	qrcode "github.com/skip2/go-qrcode"
)

const DIST_DIR = "dist"

// Initialize logger
func InitLogger(cfg *config.Config) *slog.Logger {

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
		//AddSource: level <= slog.LevelDebug,
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, handlerOpts))
	slog.SetDefault(logger)

	slog.Debug("Logger initialized", "level", level.String())
	return logger
}

func InitStorage(cfg *config.Config) (storageProvider storage.Provider, err error) {
	storageProvider = storage.NewProvider(&cfg.Storage)
	if storageProvider == nil {
		err = fmt.Errorf("failed to initialize storage provider")
		return nil, err
	}

	slog.Info("Storage provider initialized")
	return storageProvider, nil
}

// Generate static QR code for support
func genSupportQr(url string) {
	qrCode, err := qrcode.Encode(url, qrcode.Medium, config.QR_IMAGE_SIZE)
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
	// If no arguments provided, default to running the server
	if len(os.Args) == 1 {
		os.Args = append(os.Args, "server")
	}
	cmd.Execute()
}
