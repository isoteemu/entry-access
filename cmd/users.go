package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"text/tabwriter"

	"entry-access-control/internal/access"
	"entry-access-control/internal/config"

	"github.com/spf13/cobra"
)

// initMinimalLogger creates a logger with minimal output for CLI commands
func initMinimalLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
}

// initLDAPClient creates an LDAP client if configured, returns nil otherwise
func initLDAPClient(logger *slog.Logger) *access.LDAPClient {
	if config.Cfg.LDAP.Server == "" || config.Cfg.LDAP.BaseDN == "" {
		return nil
	}
	return access.NewLDAPClient(
		config.Cfg.LDAP.Server,
		config.Cfg.LDAP.BaseDN,
		config.Cfg.LDAP.BindDN,
		config.Cfg.LDAP.BindPassword,
		config.Cfg.LDAP.UseTLS,
		logger,
	)
}

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

	logger := initMinimalLogger()
	slog.SetDefault(logger)

	rbac := LoadAccessRBAC(config.Cfg)

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

	ldapClient := initLDAPClient(logger)

	// Print table header
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "USER ID\tSTATUS\tROLES\tLDAP UID")
	fmt.Fprintln(w, "-------\t------\t-----\t--------")

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

		ldapUID := "-"
		if ldapClient != nil {
			person, err := ldapClient.Search(userID)
			if err == nil && person != nil {
				ldapUID = access.ExtractUID(person.DN)
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", userID, status, rolesStr, ldapUID)
	}

	w.Flush()
	fmt.Printf("\nTotal users: %d\n", len(entries))
}

var searchCmd = &cobra.Command{
	Use:   "search <email>",
	Short: "Search for a user in LDAP by email",
	Long:  `Search for a user in LDAP directory by email address and display their information including groups.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		email := args[0]
		ctx := context.Background()
		searchUserByEmail(ctx, email)
	},
}

func searchUserByEmail(ctx context.Context, email string) {
	if config.Cfg == nil {
		fmt.Fprintln(os.Stderr, "Configuration not initialized")
		os.Exit(1)
	}

	logger := initMinimalLogger()
	ldapClient := initLDAPClient(logger)

	if ldapClient == nil {
		fmt.Fprintln(os.Stderr, "LDAP is not configured. Set LDAP_SERVER and LDAP_BASE_DN environment variables.")
		os.Exit(1)
	}

	// Search for user
	person, err := ldapClient.Search(email)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to search for user: %v\n", err)
		os.Exit(1)
	}

	if person == nil {
		fmt.Fprintf(os.Stderr, "Error: User not found\n")
		os.Exit(1)
	}

	// Display user information
	fmt.Println("\n=== User Information ===")
	fmt.Printf("Name: %s\n", person.Name)
	fmt.Printf("Email: %s\n", person.Email)
	fmt.Printf("Given Name: %s\n", person.GivenName)
	fmt.Printf("Surname: %s\n", person.Surname)
	fmt.Printf("Phone: %s\n", person.Phone)
	fmt.Printf("Employee Number: %s\n", person.EmployeeNumber)
	fmt.Printf("Organizational Unit: %s\n", person.OrganizationalUnit)
	fmt.Printf("DN: %s\n", person.DN)

	// Display groups
	fmt.Println("\n=== Groups ===")
	if len(person.Groups) == 0 {
		fmt.Println("No groups found")
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "INDEX\tGROUP DN")
		fmt.Fprintln(w, "-----\t--------")
		for i, groupDN := range person.Groups {
			fmt.Fprintf(w, "%d\t%s\n", i+1, groupDN)
		}
		w.Flush()
	}
	fmt.Printf("\nTotal groups: %d\n", len(person.Groups))
}

func init() {
	rootCmd.AddCommand(usersCmd)
	usersCmd.AddCommand(listUsersCmd)
	usersCmd.AddCommand(searchCmd)
}
