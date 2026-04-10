package main

import (
	"context"
	"log"

	"github.com/crafff/gogg/internal/server"
)

func main() {
	app, err := server.NewApp(context.Background(), server.LoadConfigFromEnv())
	if err != nil {
		log.Fatalf("failed to initialize app: %v", err)
	}

	if err := app.Run(context.Background()); err != nil {
		log.Fatalf("server stopped with error: %v", err)
	}
}
