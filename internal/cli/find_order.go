package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/output"
	"github.com/youyo/board/internal/repository"
	"github.com/youyo/board/internal/service/find"
)

// NewFindOrderCmd returns the board find order command.
func NewFindOrderCmd() *cobra.Command {
	var (
		id          int
		clientName  string
		projectName string
		text        string
		status      string
	)

	cmd := &cobra.Command{
		Use:   "order",
		Short: "Search orders with client/project resolution",
		Long:  "Search for orders by ID, client name, project name, free text, or status. Returns orders with their associated client and project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 && clientName == "" && projectName == "" && text == "" && status == "" {
				return fmt.Errorf("at least one of --id, --client-name, --project-name, --text, or --status must be specified")
			}

			svc, err := findServiceFromCmd(cmd)
			if err != nil {
				return err
			}

			opts := readOptionsFromCmd(cmd)
			q := find.FindOrderQuery{
				ID:          id,
				ClientName:  clientName,
				ProjectName: projectName,
				Text:        text,
				Status:      status,
				Limit:       opts.Limit,
				Opts: repository.ReadOptions{
					Refresh:      opts.Refresh,
					ForceRefresh: opts.ForceRefresh,
				},
			}

			results, err := svc.FindOrder(cmd.Context(), q)
			if err != nil {
				return err
			}

			return output.Write(os.Stdout, results, prettyFromCmd(cmd))
		},
	}

	cmd.Flags().IntVar(&id, "id", 0, "Order ID (direct lookup, highest priority)")
	cmd.Flags().StringVar(&clientName, "client-name", "", "Client name to resolve orders for")
	cmd.Flags().StringVar(&projectName, "project-name", "", "Project name to resolve orders for")
	cmd.Flags().StringVar(&text, "text", "", "Free-text search across title, memo")
	cmd.Flags().StringVar(&status, "status", "", "Filter by order status")

	return cmd
}
