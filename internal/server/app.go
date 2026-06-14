// Package server is the LEGACY HTTP server. It still serves traffic
// today and stays here until the new gogg-api binary at
// apps/api/cmd/api takes over in production.
//
// Deprecated: as of Phase B chunk 4 (rankings + metrics + cache), the
// new stack has reached feature parity with this package across the
// /api/* surface. Do not add features here. Bug fixes for security
// or correctness only; mirror any change into apps/api/internal/* in
// the same PR.
//
// Removal lands in Phase C alongside the crawler migration (the
// rankings flow still imports internal/storage transitively). Until
// then, keep the package compiling but tag it on every PR review.
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
	httpServer *http.Server
	pool       *pgxpool.Pool
	repos      *Repos
	webDistDir string
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
		pool:       pool,
		repos:      repos,
		webDistDir: cfg.WebDistDir,
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
