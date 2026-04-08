package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/output"
	"github.com/youyo/board/internal/repository"
	"github.com/youyo/board/internal/service/find"
)

// NewFindUserCmd returns the board find user command.
func NewFindUserCmd() *cobra.Command {
	var (
		id   int
		name string
		text string
	)

	cmd := &cobra.Command{
		Use:   "user",
		Short: "Search users",
		Long:  "Search for users by ID, name, or free text. Returns matching users.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 && name == "" && text == "" {
				return fmt.Errorf("at least one of --id, --name, or --text must be specified")
			}

			svc, err := findServiceFromCmd(cmd)
			if err != nil {
				return err
			}

			opts := readOptionsFromCmd(cmd)
			q := find.FindUserQuery{
				ID:   id,
				Name: name,
				Text: text,
				Limit: opts.Limit,
				Opts: repository.ReadOptions{
					Refresh:      opts.Refresh,
					ForceRefresh: opts.ForceRefresh,
				},
			}

			results, err := svc.FindUser(cmd.Context(), q)
			if err != nil {
				return err
			}

			return output.Write(os.Stdout, results, prettyFromCmd(cmd))
		},
	}

	cmd.Flags().IntVar(&id, "id", 0, "User ID (direct lookup, highest priority)")
	cmd.Flags().StringVar(&name, "name", "", "User name substring search")
	cmd.Flags().StringVar(&text, "text", "", "Free-text search across name, email")

	return cmd
}
