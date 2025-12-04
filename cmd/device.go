package cmd

import (
	"context"
	"entry-access-control/internal/storage"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var deviceCmd = &cobra.Command{
	Use:   "device",
	Short: "Manage device provisioning",
	Long:  `Manage device provisioning, including listing, approving, and rejecting devices.`,
}

var deviceListCmd = &cobra.Command{
	Use:   "list [status]",
	Short: "List pending devices",
	Long:  `List devices by status. Valid statuses: pending, approved, rejected. Defaults to pending.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		// Default to pending status
		status := storage.DeviceStatusPending
		if len(args) > 0 {
			switch args[0] {
			case "pending":
				status = storage.DeviceStatusPending
			case "approved":
				status = storage.DeviceStatusApproved
			case "rejected":
				status = storage.DeviceStatusRejected
			default:
				slog.Error("Invalid status", "status", args[0])
				fmt.Println("Valid statuses: pending, approved, rejected")
				os.Exit(1)
			}
		}

		devices, err := provider.ListDevices(ctx, status)
		if err != nil {
			slog.Error("Failed to list devices", "error", err)
			os.Exit(1)
		}

		if len(devices) == 0 {
			fmt.Printf("No %s devices found\n", status)
			return
		}

		// Print table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "DEVICE ID\tSTATUS\tCLIENT IP\tCREATED AT\tUPDATED AT\tAPPROVED BY")
		for _, device := range devices {
			approvedBy := ""
			if device.ApprovedBy != nil {
				approvedBy = *device.ApprovedBy
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				device.DeviceID,
				device.Status,
				device.ClientIP,
				device.CreatedAt.Format("2006-01-02 15:04:05"),
				device.UpdatedAt.Format("2006-01-02 15:04:05"),
				approvedBy,
			)
		}
		w.Flush()
	},
}

// getActiveUser returns a string identifying who is performing the action
// Format: username@hostname
func getActiveUser() string {
	username := "unknown"
	if currentUser, err := user.Current(); err == nil {
		username = currentUser.Username
	}

	hostname := "unknown"
	// Check environment variable first for SSH sessions
	if h := os.Getenv("SSH_CLIENT"); h != "" {
		ssh_client := strings.Split(h, " ")
		if len(ssh_client) > 0 {
			hostname = ssh_client[0]
		}
	} else if h, err := os.Hostname(); err == nil {
		hostname = h
	}

	return fmt.Sprintf("%s@%s", username, hostname)
}

var deviceApproveCmd = &cobra.Command{
	Use:   "approve <device_id> <entry_id>",
	Short: "Approve a pending device for a specific entry",
	Long:  `Approve a pending device and associate it with an entry point. The entry_id must be a valid entry ID.`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		deviceID := args[0]

		var entryID int64
		if _, err := fmt.Sscanf(args[1], "%d", &entryID); err != nil {
			slog.Error("Invalid entry_id", "entry_id", args[1], "error", err)
			fmt.Println("entry_id must be a valid integer")
			os.Exit(1)
		}

		// Check if device exists
		device, err := provider.GetDevice(ctx, deviceID)
		if err != nil {
			slog.Error("Device not found", "device_id", deviceID, "error", err)
			os.Exit(1)
		}

		if device.Status == storage.DeviceStatusApproved {
			fmt.Printf("Device %s is already approved\n", deviceID)
			return
		}

		// Get approver info
		approver := getActiveUser()

		// Approve device
		err = provider.UpdateDeviceStatus(ctx, deviceID, storage.DeviceStatusApproved, &approver)
		if err != nil {
			slog.Error("Failed to approve device", "device_id", deviceID, "error", err)
			os.Exit(1)
		}

		// Create approved device entry
		approvedDevice := storage.ApprovedDevice{
			DeviceID:   deviceID,
			EntryID:    entryID,
			ApprovedBy: approver,
		}

		err = provider.CreateApprovedDevice(ctx, approvedDevice)
		if err != nil {
			slog.Error("Failed to create approved device entry", "device_id", deviceID, "entry_id", entryID, "error", err)
			// Note: Device is already marked as approved in devices table, but association failed
			fmt.Printf("Warning: Device %s marked as approved but failed to associate with entry %d: %v\n", deviceID, entryID, err)
			os.Exit(1)
		}

		fmt.Printf("Device %s approved successfully for entry %d by %s\n", deviceID, entryID, approver)
	},
}

var deviceRejectCmd = &cobra.Command{
	Use:   "reject <device_id>",
	Short: "Reject a pending device",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		deviceID := args[0]

		// Check if device exists
		device, err := provider.GetDevice(ctx, deviceID)
		if err != nil {
			slog.Error("Device not found", "device_id", deviceID, "error", err)
			os.Exit(1)
		}

		if device.Status == storage.DeviceStatusRejected {
			fmt.Printf("Device %s is already rejected\n", deviceID)
			return
		}

		// Get approver info (person who rejected)
		approver := getActiveUser()

		// Reject device
		err = provider.UpdateDeviceStatus(ctx, deviceID, storage.DeviceStatusRejected, &approver)
		if err != nil {
			slog.Error("Failed to reject device", "device_id", deviceID, "error", err)
			os.Exit(1)
		}

		fmt.Printf("Device %s rejected successfully by %s\n", deviceID, approver)
	},
}

var deviceRevokeCmd = &cobra.Command{
	Use:   "revoke <device_id> <entry_id>",
	Short: "Revoke device access to a specific entry",
	Long:  `Revoke a previously approved device's access to a specific entry point.`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		deviceID := args[0]

		var entryID int64
		if _, err := fmt.Sscanf(args[1], "%d", &entryID); err != nil {
			slog.Error("Invalid entry_id", "entry_id", args[1], "error", err)
			fmt.Println("entry_id must be a valid integer")
			os.Exit(1)
		}

		// Check if approved device exists
		_, err := provider.GetApprovedDevice(ctx, deviceID, entryID)
		if err != nil {
			slog.Error("Approved device not found", "device_id", deviceID, "entry_id", entryID, "error", err)
			fmt.Printf("Device %s is not approved for entry %d or already revoked\n", deviceID, entryID)
			os.Exit(1)
		}

		// Revoke device
		err = provider.RevokeApprovedDevice(ctx, deviceID, entryID)
		if err != nil {
			slog.Error("Failed to revoke device", "device_id", deviceID, "entry_id", entryID, "error", err)
			os.Exit(1)
		}

		fmt.Printf("Device %s access to entry %d revoked successfully\n", deviceID, entryID)
	},
}

var devicePruneCmd = &cobra.Command{
	Use:   "prune [--days N] [--status STATUS]",
	Short: "Remove old devices",
	Long: `Remove devices older than a specified number of days.
By default, removes pending and rejected devices older than 7 days.
Use --status to filter by device status (pending, approved, rejected).
Use --days to specify the age threshold (default: 7).`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		// Get flags
		days, _ := cmd.Flags().GetInt("days")
		statusStr, _ := cmd.Flags().GetString("status")

		// Default to pending status if not specified
		status := storage.DeviceStatusPending
		if statusStr != "" {
			switch strings.ToLower(statusStr) {
			case "pending":
				status = storage.DeviceStatusPending
			case "approved":
				status = storage.DeviceStatusApproved
			case "rejected":
				status = storage.DeviceStatusRejected
			default:
				slog.Error("Invalid status", "status", statusStr)
				fmt.Println("Valid statuses: pending, approved, rejected")
				os.Exit(1)
			}
		}

		// Calculate cutoff time
		olderThan := time.Now().AddDate(0, 0, -days)

		fmt.Printf("Pruning %s devices older than %d days (created before %s)...\n",
			status, days, olderThan.Format("2006-01-02 15:04:05"))

		// Prune devices
		count, err := provider.PruneDevices(ctx, olderThan, status)
		if err != nil {
			slog.Error("Failed to prune devices", "error", err)
			os.Exit(1)
		}

		if count == 0 {
			fmt.Println("No devices to prune")
		} else {
			fmt.Printf("Successfully pruned %d device(s)\n", count)
		}
	},
}

func init() {
	// Add flags to prune command
	devicePruneCmd.Flags().IntP("days", "d", 7, "Remove devices older than this many days")
	devicePruneCmd.Flags().StringP("status", "s", "pending", "Filter by device status (pending, approved, rejected)")

	deviceCmd.AddCommand(deviceListCmd)
	deviceCmd.AddCommand(deviceApproveCmd)
	deviceCmd.AddCommand(deviceRejectCmd)
	deviceCmd.AddCommand(deviceRevokeCmd)
	deviceCmd.AddCommand(devicePruneCmd)
	rootCmd.AddCommand(deviceCmd)
}
