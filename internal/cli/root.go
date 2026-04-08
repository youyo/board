package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/app"
)

// globalFlags はルートコマンドで受け取る共通フラグの値を保持する。
type globalFlags struct {
	profile      string
	refresh      bool
	forceRefresh bool
	pretty       bool
	limit        int
}

// NewRootCmd は board の root Cobra command を返す。
// version には埋め込みバージョン文字列を渡す。
func NewRootCmd(version string) *cobra.Command {
	var gf globalFlags

	rootCmd := &cobra.Command{
		Use:   "board",
		Short: "BOARD CLI - BOARD API client and MCP server",
		Long:  "board は BOARD API のクライアント CLI および ローカル HTTP MCP サーバー。",
		// PersistentPreRunE は configure コマンド群をスキップし、それ以外で App を初期化する。
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// configure コマンド群は App 初期化不要（設定ファイルを前提としない）
			if strings.HasPrefix(cmd.CommandPath(), "board configure") {
				return nil
			}

			a, err := app.New(gf.profile)
			if err != nil {
				return err
			}

			// Context に App を格納（各サブコマンドで AppFromContext して参照）
			ctx := app.WithApp(cmd.Context(), a)
			cmd.SetContext(ctx)
			return nil
		},
		// PersistentPostRunE は DB 接続を閉じる。RunE がエラーを返した場合も実行される。
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			// configure コマンド群は App を初期化していないのでスキップ
			if strings.HasPrefix(cmd.CommandPath(), "board configure") {
				return nil
			}

			a, ok := app.AppFromContext(cmd.Context())
			if !ok {
				return nil
			}
			return a.Close()
		},
	}

	rootCmd.Version = version
	rootCmd.SetVersionTemplate("board version {{.Version}}\n")

	// 共通フラグ（全サブコマンドに継承）
	pf := rootCmd.PersistentFlags()
	pf.StringVarP(&gf.profile, "profile", "p", "", "使用するプロファイル名（デフォルト: current_profile）")
	pf.BoolVar(&gf.refresh, "refresh", false, "差分リフレッシュを強制実行")
	pf.BoolVar(&gf.forceRefresh, "force-refresh", false, "全件リフレッシュを強制実行")
	pf.BoolVar(&gf.pretty, "pretty", false, "JSON を整形表示")
	pf.IntVar(&gf.limit, "limit", 50, "返却件数上限（0 = 無制限）")

	// サブコマンドを登録
	rootCmd.AddCommand(NewConfigureCmd())
	rootCmd.AddCommand(NewAPICmd())

	return rootCmd
}
