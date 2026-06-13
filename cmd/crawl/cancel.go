package crawl

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func newCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <run_id>",
		Short: "Mark a running run as failed/cancelled",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid run ID: %s", args[0])
			}
			ctx := context.Background()
			_, store, err := loadDeps(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			if err := store.CancelRun(ctx, id); err != nil {
				return err
			}
			fmt.Printf("Run #%d marked as failed.\n", id)
			return nil
		},
	}
}
