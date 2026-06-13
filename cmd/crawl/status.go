package crawl

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show status of the current or most recent run",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			_, store, err := loadDeps(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			runs, err := store.ListRuns(ctx, 1)
			if err != nil {
				return err
			}
			if len(runs) == 0 {
				fmt.Println("No runs found.")
				return nil
			}
			r := runs[0]

			profile := "-"
			if r.Profile != nil {
				profile = *r.Profile
			}
			tier := "-"
			if r.CurrentTier != nil {
				tier = *r.CurrentTier
			}
			ended := "-"
			if r.EndedAt != nil {
				ended = r.EndedAt.Format("2006-01-02 15:04:05")
			}

			fmt.Printf("Run #%d\n", r.ID)
			fmt.Printf("  Status:        %s\n", r.Status)
			fmt.Printf("  Profile:       %s\n", profile)
			fmt.Printf("  Mode:          %s\n", r.Mode)
			fmt.Printf("  Target tiers:  %v\n", r.TargetTiers)
			fmt.Printf("  Phase:         %d\n", r.CurrentPhase)
			fmt.Printf("  Current tier:  %s\n", tier)
			fmt.Printf("  Started:       %s\n", r.StartedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("  Ended:         %s\n", ended)
			return nil
		},
	}
}
