package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/config"
)

// NewConfigureListProfilesCmd returns the configure list-profiles command.
func NewConfigureListProfilesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-profiles",
		Short: "List all profiles",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.ConfigPath()
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}

			names := make([]string, 0, len(cfg.Profiles))
			for name := range cfg.Profiles {
				names = append(names, name)
			}
			sort.Strings(names)

			for _, name := range names {
				fmt.Fprintln(cmd.OutOrStdout(), name)
			}
			return nil
		},
	}
}
