package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIContactsCmd returns the board api contacts subcommand group.
func NewAPIContactsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contacts",
		Short: "Manage contacts",
	}
	cmd.AddCommand(
		newAPIContactsListCmd(),
		newAPIContactsGetCmd(),
	)
	return cmd
}

// contactListFlagsFromCmd reads the Ransack-style filter flags and returns a
// ContactListOptions. All flags are optional; any flag left at its zero value
// is omitted from the outgoing request.
func contactListFlagsFromCmd(cmd *cobra.Command) boardapi.ContactListOptions {
	clientIDEq, _ := cmd.Flags().GetInt("client-id")
	nameCont, _ := cmd.Flags().GetString("name-cont")
	emailCont, _ := cmd.Flags().GetString("email-cont")
	updatedAtGteq, _ := cmd.Flags().GetString("updated-at-gteq")
	updatedAtLteq, _ := cmd.Flags().GetString("updated-at-lteq")

	var includeArchive *bool
	if cmd.Flags().Changed("include-archive-flg") {
		v, _ := cmd.Flags().GetBool("include-archive-flg")
		includeArchive = &v
	}

	return boardapi.ContactListOptions{
		ClientIDEq:        clientIDEq,
		NameCont:          nameCont,
		EmailCont:         emailCont,
		UpdatedAtGteq:     updatedAtGteq,
		UpdatedAtLteq:     updatedAtLteq,
		IncludeArchiveFlg: includeArchive,
	}
}

func newAPIContactsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List contacts (optionally filtered by Ransack-style query params)",
		Long: `List contacts. Filters are forwarded to the BOARD API as Ransack-style
query parameters (e.g. --name-cont sends name_cont). A zero-filter request
uses the local cache; any non-zero filter bypasses the cache and calls the
API directly so server-side filter semantics take effect.

JSON output includes an _meta object (total_count, page, per_page, rate
limits, ETag, last_modified) derived from response headers. Use
--no-show-meta to omit it.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			readOpts := readOptionsFromCmd(cmd)
			filter := contactListFlagsFromCmd(cmd)
			result, err := svc.ListContacts(cmd.Context(), readOpts, filter)
			if err != nil {
				return err
			}
			showMeta, _ := cmd.Flags().GetBool("show-meta")
			if showMeta {
				return output.Write(os.Stdout, result, prettyFromCmd(cmd))
			}
			return output.Write(os.Stdout, result.Items, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().Int("client-id", 0, "Filter by client ID (Ransack client_id_eq, exact match)")
	cmd.Flags().String("name-cont", "", "Filter by contact name (Ransack name_cont, partial match)")
	cmd.Flags().String("email-cont", "", "Filter by email address (Ransack email_cont, partial match)")
	cmd.Flags().String("updated-at-gteq", "", `updated_at >= (YYYY-MM-DD HH:MM:SS)`)
	cmd.Flags().String("updated-at-lteq", "", `updated_at <= (YYYY-MM-DD HH:MM:SS)`)
	cmd.Flags().Bool("include-archive-flg", false, "Include archived contacts (send include_archive_flg=1)")
	cmd.Flags().Bool("show-meta", true, "Include _meta (pagination / rate limit / ETag) in JSON output")
	return cmd
}

func newAPIContactsGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a contact by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetContact(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Contact ID (required)")
	return cmd
}
