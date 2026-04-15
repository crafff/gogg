package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repos struct {
	rankingStore *RankingStore
	versionStore *VersionStore
}

// App contains the HTTP server runtime.
type App struct {
	httpServer   *http.Server
	pool         *pgxpool.Pool
	repos        *Repos
	webDistDir   string
}

func NewApp(ctx context.Context, cfg Config) (*App, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseDSN)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	repos := &Repos{
		rankingStore: NewRankingStore(pool),
		versionStore: NewVersionStore(pool),
	}

	app := &App{
		pool:         pool,
		repos:        repos,
		webDistDir:   cfg.WebDistDir,
	}

	// Addr 
	app.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.Port),
		Handler:           NewRouter(app),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return app, nil
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		log.Printf("server listening on %s", a.httpServer.Addr)
		err := a.httpServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)

	select {
	case <-ctx.Done():
		return a.shutdown()
	case <-stop:
		return a.shutdown()
	case err := <-errCh:
		return err
	}
}

func (a *App) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("shutting down server")
	if err := a.httpServer.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
		if closeErr := a.httpServer.Close(); closeErr != nil {
			log.Printf("force close failed: %v", closeErr)
		}
		if a.pool != nil {
			a.pool.Close()
		}
		return err
	}

	if a.pool != nil {
		a.pool.Close()
	}

	return nil
}
