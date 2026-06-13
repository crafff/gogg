package crawl

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newRunsCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "List recent crawler runs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			_, store, err := loadDeps(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			runs, err := store.ListRuns(ctx, limit)
			if err != nil {
				return err
			}
			if len(runs) == 0 {
				fmt.Println("No runs found.")
				return nil
			}

			fmt.Printf("%-6s %-12s %-14s %-8s %-28s %-28s\n",
				"ID", "STATUS", "PROFILE", "PHASE", "STARTED", "ENDED")
			fmt.Println("----------------------------------------------------------------------")
			for _, r := range runs {
				profile := "-"
				if r.Profile != nil {
					profile = *r.Profile
				}
				ended := "-"
				if r.EndedAt != nil {
					ended = r.EndedAt.Format("2006-01-02 15:04:05")
				}
				fmt.Printf("%-6d %-12s %-14s %-8d %-28s %-28s\n",
					r.ID, r.Status, profile, r.CurrentPhase,
					r.StartedAt.Format("2006-01-02 15:04:05"), ended)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "number of runs to show")
	return cmd
}
