package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/output"
	"github.com/youyo/board/internal/repository"
	"github.com/youyo/board/internal/service/find"
)

// NewFindDeliveryCmd returns the board find delivery command.
func NewFindDeliveryCmd() *cobra.Command {
	var (
		id          int
		projectID   int
		clientName  string
		projectName string
		status      string
	)

	cmd := &cobra.Command{
		Use:   "delivery",
		Short: "Search deliveries with client/project resolution",
		Long:  "Search for deliveries by document ID, project ID, client name, or project name. Returns deliveries with their associated client and project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 && projectID == 0 && clientName == "" && projectName == "" {
				return fmt.Errorf("at least one of --id, --project-id, --client-name, or --project-name must be specified")
			}
			if status != "" {
				return fmt.Errorf("--status filtering is not supported for documents (no Status field on entity)")
			}

			svc, err := findServiceFromCmd(cmd)
			if err != nil {
				return err
			}

			opts := readOptionsFromCmd(cmd)
			q := find.FindDeliveryQuery{
				FindCommonOpts: find.FindCommonOpts{
					Limit: opts.Limit,
					Opts: repository.ReadOptions{
						Refresh:      opts.Refresh,
						ForceRefresh: opts.ForceRefresh,
					},
				},
				ID:          id,
				ProjectID:   projectID,
				ClientName:  clientName,
				ProjectName: projectName,
			}

			results, err := svc.FindDelivery(cmd.Context(), q)
			if err != nil {
				return err
			}

			return output.Write(os.Stdout, wrapFindOutput(cmd.Context(), "delivery", results), prettyFromCmd(cmd))
		},
	}

	cmd.Flags().IntVar(&id, "id", 0, "Delivery document ID (direct lookup, highest priority)")
	cmd.Flags().IntVar(&projectID, "project-id", 0, "Project ID to find its delivery")
	cmd.Flags().StringVar(&clientName, "client-name", "", "Client name to resolve deliveries for")
	cmd.Flags().StringVar(&projectName, "project-name", "", "Project name to resolve deliveries for")
	cmd.Flags().StringVar(&status, "status", "", "Post-filter by delivery status")

	return cmd
}
