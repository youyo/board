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
		id             int
		clientName     string
		name           string
		status         string
		statuses       []string
		contractStatus string
	)

	cmd := &cobra.Command{
		Use:   "project",
		Short: "Search projects with client resolution",
		Long:  "Search for projects by ID, client name, project name or status. Returns projects with their associated client.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 && clientName == "" && name == "" &&
				status == "" && len(statuses) == 0 && contractStatus == "" {
				return fmt.Errorf("at least one of --id, --client-name, --name, --status, --statuses, or --contract-status must be specified")
			}

			// 3-way 排他チェック: status / statuses / contract-status は相互排他。
			setCount := 0
			if status != "" {
				setCount++
			}
			if len(statuses) > 0 {
				setCount++
			}
			if contractStatus != "" {
				setCount++
			}
			if setCount > 1 {
				return fmt.Errorf("--status, --statuses, and --contract-status are mutually exclusive")
			}

			svc, err := findServiceFromCmd(cmd)
			if err != nil {
				return err
			}

			opts := readOptionsFromCmd(cmd)
			readOpts := repository.ReadOptions{Refresh: opts.Refresh, ForceRefresh: opts.ForceRefresh}

			var clientID int
			if clientName != "" {
				clientID, err = svc.ResolveClientByName(cmd.Context(), clientName, readOpts)
				if err != nil {
					return err
				}
			}

			q := find.FindProjectQuery{
				FindCommonOpts: find.FindCommonOpts{
					Limit: opts.Limit,
					Opts:  readOpts,
				},
				ID:             id,
				ClientID:       clientID,
				Name:           name,
				Status:         status,
				Statuses:       statuses,
				ContractStatus: contractStatus,
			}

			results, err := svc.FindProject(cmd.Context(), q)
			if err != nil {
				return err
			}

			return output.Write(os.Stdout, wrapFindOutput(cmd.Context(), "project", results), prettyFromCmd(cmd))
		},
	}

	cmd.Flags().IntVar(&id, "id", 0, "Project ID (direct lookup, highest priority)")
	cmd.Flags().StringVar(&clientName, "client-name", "", "Client name to resolve projects for")
	cmd.Flags().StringVar(&name, "name", "", "Project name substring search")
	cmd.Flags().StringVar(&status, "status", "", "Filter by project status. Mutually exclusive with --statuses / --contract-status.")
	cmd.Flags().StringSliceVar(&statuses, "statuses", nil, "Filter by multiple project statuses (OR). Mutually exclusive with --status / --contract-status. Same narrowing rules apply. Max 10 items.")
	cmd.Flags().StringVar(&contractStatus, "contract-status", "", "Contract status alias (active/ended/prospect/all). Mutually exclusive with --status / --statuses. Same narrowing rules apply.")

	return cmd
}
