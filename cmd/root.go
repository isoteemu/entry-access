package cmd

import (
	"entry-access-control/internal/config"
	"entry-access-control/internal/storage"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile  string
	cfg      *config.Config
	provider storage.Provider
)

var rootCmd = &cobra.Command{
	Use:   "entry-access-control",
	Short: "Entry access control management system",
	Long:  `A command-line tool for managing entry access control with entryways.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize configuration
		var err error
		if cfgFile != "" {
			// cfg, err = config.Load(cfgFile)
			panic("custom config loading not implemented yet")
		} else {
			cfg, err = config.LoadConfig()
		}
		if err != nil {
			slog.Error("Failed to load configuration", "error", err)
			os.Exit(1)
		}

		// Initialize storage provider
		provider = storage.NewProvider(&cfg.Storage)
		if provider == nil {
			slog.Error("Failed to initialize storage provider")
			os.Exit(1)
		}
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		// Cleanup
		if provider != nil {
			provider.Close()
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
}
