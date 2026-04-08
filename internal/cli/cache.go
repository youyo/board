package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/app"
	"github.com/youyo/board/internal/output"
)

// NewCacheCmd は board cache サブコマンドグループを返す。
func NewCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "キャッシュ管理",
		Long:  "SQLite キャッシュの状態確認・期限切れ設定・削除・パス表示を行う。",
	}
	cmd.AddCommand(
		newCacheStatusCmd(),
		newCacheExpireCmd(),
		newCacheClearCmd(),
		newCachePathCmd(),
	)
	return cmd
}

// newCacheStatusCmd は cache status サブコマンドを返す。
// 全リソースの sync_state サマリーを JSON 出力する。
func newCacheStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "全リソースのキャッシュ状態を表示",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, ok := app.AppFromContext(cmd.Context())
			if !ok {
				return errNoApp
			}

			states, err := a.SyncStore.ListAll(cmd.Context(), a.ProfileName)
			if err != nil {
				return fmt.Errorf("cache status: %w", err)
			}

			// JSON 出力用に変換
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

// newCacheExpireCmd は cache expire サブコマンドを返す。
// --resource で個別リソース、省略時は全リソースを期限切れにする。
func newCacheExpireCmd() *cobra.Command {
	var resource string

	cmd := &cobra.Command{
		Use:   "expire",
		Short: "キャッシュを期限切れにする（次回アクセス時にリフレッシュ）",
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
				fmt.Fprintf(os.Stdout, "{\"expired\":%q}\n", resource)
			} else {
				if err := a.SyncStore.ExpireAll(cmd.Context(), a.ProfileName, now); err != nil {
					return fmt.Errorf("cache expire: %w", err)
				}
				fmt.Fprintf(os.Stdout, "{\"expired\":\"all\"}\n")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&resource, "resource", "", "対象リソース名（省略時は全リソース）")
	return cmd
}

// newCacheClearCmd は cache clear サブコマンドを返す。
// --resource で個別リソース、省略時は全リソースのキャッシュを削除する。
func newCacheClearCmd() *cobra.Command {
	var resource string

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "キャッシュを削除する",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, ok := app.AppFromContext(cmd.Context())
			if !ok {
				return errNoApp
			}

			if resource != "" {
				// resource_cache + sync_state を個別削除
				if err := a.ResourceCache.DeleteAll(cmd.Context(), a.ProfileName, resource); err != nil {
					return fmt.Errorf("cache clear: resource_cache: %w", err)
				}
				if err := a.SyncStore.Delete(cmd.Context(), a.ProfileName, resource); err != nil {
					return fmt.Errorf("cache clear: sync_state: %w", err)
				}
				fmt.Fprintf(os.Stdout, "{\"cleared\":%q}\n", resource)
			} else {
				// 全リソース削除: resource_cache は全行削除、sync_state も全行削除
				// resource_cache には DeleteAllProfiles に相当するメソッドがないため
				// プロファイル単位で全22リソースをループ削除する
				resources := allResourceNames()
				for _, r := range resources {
					if err := a.ResourceCache.DeleteAll(cmd.Context(), a.ProfileName, r); err != nil {
						return fmt.Errorf("cache clear: resource_cache[%s]: %w", r, err)
					}
				}
				if err := a.SyncStore.DeleteAll(cmd.Context(), a.ProfileName); err != nil {
					return fmt.Errorf("cache clear: sync_state: %w", err)
				}
				fmt.Fprintf(os.Stdout, "{\"cleared\":\"all\"}\n")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&resource, "resource", "", "対象リソース名（省略時は全リソース）")
	return cmd
}

// newCachePathCmd は cache path サブコマンドを返す。
// SQLite DB のファイルパスを表示する。
func newCachePathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "キャッシュ DB のパスを表示",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, ok := app.AppFromContext(cmd.Context())
			if !ok {
				return errNoApp
			}
			// DB のパスは app 経由で取得（DB.DSN は非公開のため、app から渡す）
			fmt.Fprintln(os.Stdout, a.DBPath)
			return nil
		},
	}
}

// allResourceNames は全22リソース名のスライスを返す。
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
