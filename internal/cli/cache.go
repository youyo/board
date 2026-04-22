package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/app"
	"github.com/youyo/board/internal/output"
)

// NewCacheCmd returns the board cache subcommand group.
func NewCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage the local cache",
		Long:  "Inspect, expire, clear, and display the path of the SQLite cache.",
	}
	cmd.AddCommand(
		newCacheStatusCmd(),
		newCacheExpireCmd(),
		newCacheClearCmd(),
		newCachePathCmd(),
	)
	return cmd
}

// newCacheStatusCmd returns the cache status subcommand.
// It outputs a JSON summary of the sync_state for all resources.
func newCacheStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show cache status for all resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, ok := app.AppFromContext(cmd.Context())
			if !ok {
				return errNoApp
			}

			states, err := a.SyncStore.ListAll(cmd.Context(), a.ProfileName)
			if err != nil {
				return fmt.Errorf("cache status: %w", err)
			}

			// Convert to JSON output format.
			type stateRow struct {
				Resource          string `json:"resource"`
				LastSyncedAt      string `json:"last_synced_at,omitempty"`
				LastFullSyncedAt  string `json:"last_full_synced_at,omitempty"`
				LastSyncStatus    string `json:"last_sync_status,omitempty"`
				ExpiredAt         string `json:"expired_at,omitempty"`
				MustFullResync    bool   `json:"must_full_resync"`
				ConsecutiveErrors int64  `json:"consecutive_errors"`
			}

			rows := make([]stateRow, 0, len(states))
			for _, s := range states {
				row := stateRow{
					Resource:          s.ResourceName,
					MustFullResync:    s.MustFullResync,
					ConsecutiveErrors: s.ConsecutiveFailures,
				}
				if s.LastSyncedAt.Valid {
					row.LastSyncedAt = s.LastSyncedAt.String
				}
				if s.LastFullSyncedAt.Valid {
					row.LastFullSyncedAt = s.LastFullSyncedAt.String
				}
				if s.LastSyncStatus.Valid {
					row.LastSyncStatus = s.LastSyncStatus.String
				}
				if s.ExpiredAt.Valid {
					row.ExpiredAt = s.ExpiredAt.String
				}
				rows = append(rows, row)
			}

			pretty := prettyFromCmd(cmd)
			return output.Write(os.Stdout, rows, pretty)
		},
	}
}

// newCacheExpireCmd returns the cache expire subcommand.
// Use --resource for a specific resource; omit to expire all resources.
func newCacheExpireCmd() *cobra.Command {
	var resource string

	cmd := &cobra.Command{
		Use:   "expire",
		Short: "Expire the cache (refreshed on next access)",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, ok := app.AppFromContext(cmd.Context())
			if !ok {
				return errNoApp
			}

			now := time.Now().UTC().Format(time.RFC3339)
			if resource != "" {
				if err := a.SyncStore.Expire(cmd.Context(), a.ProfileName, resource, now); err != nil {
					return fmt.Errorf("cache expire: %w", err)
				}
				_, _ = fmt.Fprintf(os.Stdout, "{\"expired\":%q}\n", resource)
			} else {
				if err := a.SyncStore.ExpireAll(cmd.Context(), a.ProfileName, now); err != nil {
					return fmt.Errorf("cache expire: %w", err)
				}
				_, _ = fmt.Fprintf(os.Stdout, "{\"expired\":\"all\"}\n")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&resource, "resource", "", "Target resource name (omit for all resources)")
	return cmd
}

// newCacheClearCmd returns the cache clear subcommand.
// Use --resource for a specific resource; omit to clear all resources.
func newCacheClearCmd() *cobra.Command {
	var resource string

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Delete the cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, ok := app.AppFromContext(cmd.Context())
			if !ok {
				return errNoApp
			}

			if resource != "" {
				// Delete resource_cache and sync_state entries for the specified resource.
				if err := a.ResourceCache.DeleteAll(cmd.Context(), a.ProfileName, resource); err != nil {
					return fmt.Errorf("cache clear: resource_cache: %w", err)
				}
				if err := a.SyncStore.Delete(cmd.Context(), a.ProfileName, resource); err != nil {
					return fmt.Errorf("cache clear: sync_state: %w", err)
				}
				_, _ = fmt.Fprintf(os.Stdout, "{\"cleared\":%q}\n", resource)
			} else {
				// Delete all resources: remove all rows from resource_cache and sync_state.
				// resource_cache has no DeleteAllProfiles equivalent,
				// so iterate over all 22 resources and delete them per profile.
				resources := allResourceNames()
				for _, r := range resources {
					if err := a.ResourceCache.DeleteAll(cmd.Context(), a.ProfileName, r); err != nil {
						return fmt.Errorf("cache clear: resource_cache[%s]: %w", r, err)
					}
				}
				if err := a.SyncStore.DeleteAll(cmd.Context(), a.ProfileName); err != nil {
					return fmt.Errorf("cache clear: sync_state: %w", err)
				}
				_, _ = fmt.Fprintf(os.Stdout, "{\"cleared\":\"all\"}\n")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&resource, "resource", "", "Target resource name (omit for all resources)")
	return cmd
}

// newCachePathCmd returns the cache path subcommand.
// It displays the file path of the SQLite DB.
func newCachePathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show the path to the cache DB",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, ok := app.AppFromContext(cmd.Context())
			if !ok {
				return errNoApp
			}
			// Retrieve the DB path via app (DB.DSN is unexported, so it is passed from app).
			_, _ = fmt.Fprintln(os.Stdout, a.DBPath)
			return nil
		},
	}
}

// allResourceNames returns a slice of all 22 resource names.
func allResourceNames() []string {
	return []string{
		"clients", "client_branches", "contacts",
		"projects", "project_costs",
		"estimates", "invoices", "orders", "deliveries", "receipts",
		"vendors", "vendor_branches", "vendor_contacts",
		"purchase_orders", "payments",
		"users", "groups", "payment_terms",
		"project_types", "purchase_types", "accounting_types", "document_send_channels",
	}
}
