package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/output"
	"github.com/youyo/board/internal/repository"
	"github.com/youyo/board/internal/service/find"
)

// NewFindEstimateCmd returns the board find estimate command.
func NewFindEstimateCmd() *cobra.Command {
	var (
		id          int
		projectID   int
		clientName  string
		projectName string
		status      string
	)

	cmd := &cobra.Command{
		Use:   "estimate",
		Short: "Search estimates with client/project resolution",
		Long:  "Search for estimates by document ID, project ID, client name, or project name. Returns estimates with their associated client and project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 && projectID == 0 && clientName == "" && projectName == "" {
				return fmt.Errorf("at least one of --id, --project-id, --client-name, or --project-name must be specified")
			}
			if clientName != "" || projectName != "" {
				return fmt.Errorf("--client-name / --project-name are not supported in v0.7.0 service rename (N07b); will be wired in N07c")
			}
			if status != "" {
				return fmt.Errorf("--status is not supported in v0.7.0 service rename (N07b); will be wired in N07c")
			}

			svc, err := findServiceFromCmd(cmd)
			if err != nil {
				return err
			}

			opts := readOptionsFromCmd(cmd)
			q := find.FindEstimateQuery{
				FindCommonOpts: find.FindCommonOpts{
					Limit: opts.Limit,
					Opts: repository.ReadOptions{
						Refresh:      opts.Refresh,
						ForceRefresh: opts.ForceRefresh,
					},
				},
				ID:        id,
				ProjectID: projectID,
			}

			results, err := svc.FindEstimate(cmd.Context(), q)
			if err != nil {
				return err
			}

			return output.Write(os.Stdout, results, prettyFromCmd(cmd))
		},
	}

	cmd.Flags().IntVar(&id, "id", 0, "Estimate document ID (direct lookup, highest priority)")
	cmd.Flags().IntVar(&projectID, "project-id", 0, "Project ID to find its estimate")
	cmd.Flags().StringVar(&clientName, "client-name", "", "Client name to resolve estimates for")
	cmd.Flags().StringVar(&projectName, "project-name", "", "Project name to resolve estimates for")
	cmd.Flags().StringVar(&status, "status", "", "Post-filter by estimate status")

	return cmd
}
