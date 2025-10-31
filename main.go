package main

import (
	"fmt"
	"log/slog"
	"os"

	"entry-access-control/cmd"
	"entry-access-control/internal/config"
	"entry-access-control/internal/storage"
)

const DIST_DIR = "dist"

func InitStorage(cfg *config.Config) (storageProvider storage.Provider, err error) {
	storageProvider = storage.NewProvider(&cfg.Storage)
	if storageProvider == nil {
		err = fmt.Errorf("failed to initialize storage provider")
		return nil, err
	}

	slog.Info("Storage provider initialized")
	return storageProvider, nil
}

func main() {
	// If no arguments provided, default to running the server
	if len(os.Args) == 1 {
		os.Args = append(os.Args, "server")
	}
	cmd.Execute()
}
