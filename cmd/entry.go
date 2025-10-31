package cmd

import (
	"context"
	"entry-access-control/internal/storage"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var entryCmd = &cobra.Command{
	Use:   "entry",
	Short: "Manage entryways",
	Long:  `Create, list, and delete entryways in the access control system.`,
}

var entryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all entryways",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		entries, err := provider.ListEntries(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing entries: %v\n", err)
			os.Exit(1)
		}

		if len(entries) == 0 {
			fmt.Println("No entryways found.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tCREATED AT")
		for _, entry := range entries {
			fmt.Fprintf(w, "%d\t%s\t%s\n", entry.ID, entry.Name, entry.CreatedAt.Format(time.RFC3339))
		}
		w.Flush()
	},
}

var entryCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new entryway",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		entry := storage.Entry{
			Name:      args[0],
			CreatedAt: time.Now(),
		}

		if err := provider.CreateEntry(ctx, entry); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating entry: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Entryway '%s' created successfully.\n", entry.Name)
	},
}

var entryDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete an entryway by ID",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		var id int64
		if _, err := fmt.Sscanf(args[0], "%d", &id); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid ID: %v\n", err)
			os.Exit(1)
		}

		entry := storage.Entry{ID: id}
		if err := provider.DeleteEntry(ctx, entry); err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting entry: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Entryway ID %d deleted successfully.\n", id)
	},
}

func init() {
	rootCmd.AddCommand(entryCmd)
	entryCmd.AddCommand(entryListCmd)
	entryCmd.AddCommand(entryCreateCmd)
	entryCmd.AddCommand(entryDeleteCmd)
}
