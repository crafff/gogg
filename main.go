package main

import (
	"context"
	"log"
	"os"

	"github.com/crafff/gogg/cmd/crawl"
	"github.com/crafff/gogg/internal/server"
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
