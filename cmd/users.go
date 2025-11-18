package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"text/tabwriter"

	"entry-access-control/internal/config"

	"github.com/spf13/cobra"
)

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "Manage users and view access control information",
	Long:  `List users from the access list and display their roles and permissions.`,
}

var listUsersCmd = &cobra.Command{
	Use:   "list",
	Short: "List all users with their roles and status",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		listUsers(ctx)
	},
}

func listUsers(ctx context.Context) {
	if config.Cfg == nil {
		fmt.Fprintln(os.Stderr, "Configuration not initialized")
		os.Exit(1)
	}

	// Initialize logger with minimal output for CLI commands
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
	slog.SetDefault(logger)

	// Load RBAC and access list (reusing server initialization logic)
	rbac := LoadAccessRBAC(config.Cfg)

	// Get access list
	accessList := NewAccessListFromConfig(config.Cfg)
	if accessList == nil {
		fmt.Fprintln(os.Stderr, "Failed to initialize access list")
		os.Exit(1)
	}

	entries, err := accessList.ListAllEntries()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list entries: %v\n", err)
		os.Exit(1)
	}

	if len(entries) == 0 {
		fmt.Println("No users found in access list")
		return
	}

	// Print table header
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "USER ID\tSTATUS\tROLES")
	fmt.Fprintln(w, "-------\t------\t-----")

	// Print each user
	for _, entry := range entries {
		userID := entry.GetUserID()
		status := "Inactive"
		if entry.CanAccess("") {
			status = "Active"
		}

		roles := rbac.GetUserRoles(userID)
		rolesStr := ""
		if len(roles) > 0 {
			rolesStr = roles[0]
			for i := 1; i < len(roles); i++ {
				rolesStr += ", " + roles[i]
			}
		} else {
			rolesStr = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\n", userID, status, rolesStr)
	}

	w.Flush()
	fmt.Printf("\nTotal users: %d\n", len(entries))
}

func init() {
	rootCmd.AddCommand(usersCmd)
	usersCmd.AddCommand(listUsersCmd)
}
