package main

import (
	"context"
	"log"
	"os"

	"github.com/crafff/gogg/cmd/crawl"
	// The legacy server package is marked Deprecated as of Phase B
	// chunk 4. This legacy entry point is allowed to keep importing
	// it until apps/api/cmd/api takes over in production; new code
	// should use that path instead.
	server "github.com/crafff/gogg/internal/server" //nolint:staticcheck // legacy entry point
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:          "gogg",
		Short:        "League of Legends stats server and data crawler",
		SilenceUsage: true,
	}

	// 'gogg serve' – HTTP API server
	root.AddCommand(&cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP API server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := server.NewApp(context.Background(), server.LoadConfigFromEnv())
			if err != nil {
				return err
			}
			return app.Run(context.Background())
		},
	})

	// 'gogg crawl ...' – data crawler
	root.AddCommand(crawl.NewRootCmd())

	if err := root.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
