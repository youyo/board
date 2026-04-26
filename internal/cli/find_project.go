package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/output"
	"github.com/youyo/board/internal/repository"
	"github.com/youyo/board/internal/service/find"
)

// NewFindProjectCmd returns the board find project command.
func NewFindProjectCmd() *cobra.Command {
	var (
		id         int
		clientName string
		name       string
		text       string
		status     string
	)

	cmd := &cobra.Command{
		Use:   "project",
		Short: "Search projects with client resolution",
		Long:  "Search for projects by ID, client name, project name, free text, or status. Returns projects with their associated client.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 && clientName == "" && name == "" && text == "" && status == "" {
				return fmt.Errorf("at least one of --id, --client-name, --name, --text, or --status must be specified")
			}
			if clientName != "" {
				return fmt.Errorf("--client-name is not supported in v0.7.0 service rename (N07b); will be wired in N07c")
			}

			svc, err := findServiceFromCmd(cmd)
			if err != nil {
				return err
			}

			opts := readOptionsFromCmd(cmd)
			q := find.FindProjectQuery{
				FindCommonOpts: find.FindCommonOpts{
					Limit: opts.Limit,
					Opts: repository.ReadOptions{
						Refresh:      opts.Refresh,
						ForceRefresh: opts.ForceRefresh,
					},
				},
				ID:     id,
				Name:   name,
				Text:   text,
				Status: status,
			}

			results, err := svc.FindProject(cmd.Context(), q)
			if err != nil {
				return err
			}

			return output.Write(os.Stdout, results, prettyFromCmd(cmd))
		},
	}

	cmd.Flags().IntVar(&id, "id", 0, "Project ID (direct lookup, highest priority)")
	cmd.Flags().StringVar(&clientName, "client-name", "", "Client name to resolve projects for")
	cmd.Flags().StringVar(&name, "name", "", "Project name substring search")
	cmd.Flags().StringVar(&text, "text", "", "Free-text search across name, code, memo")
	cmd.Flags().StringVar(&status, "status", "", "Filter by project status")

	return cmd
}
