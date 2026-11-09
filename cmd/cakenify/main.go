package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "time/tzdata" // embeds the IANA tz database in the binary — the distroless base image has none

	_ "github.com/jackc/pgx/v5/stdlib"

	"cakenify/internal/cakenify"
	"cakenify/internal/cakenify/api"
	"cakenify/internal/cakenify/core"
	"cakenify/pkg/cronjob"
	"cakenify/pkg/db/cakenifydb"
	"cakenify/pkg/ntfy"
)

func main() {
	ctx := context.Background()
	cfg := loadConfig()

	sqlDB, err := connectWithRetry(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	if err := cakenifydb.RunMigration(ctx, sqlDB); err != nil {
		slog.Error("failed to run migration", "error", err)
		os.Exit(1)
	}

	db := cakenifydb.NewClient(sqlDB)
	defer db.Close()

	notifiers := cakenify.NewNotifierRegistry(cakenify.NewNtfyNotifier(ntfy.NewClient(cfg.NtfyBaseURL)))
	svc := core.New(db, notifiers, cfg.SessionTTL)

	if cfg.AdminUsername != "" {
		if err := svc.SeedAdmin(ctx, cfg.AdminUsername, cfg.AdminPassword); err != nil {
			slog.Error("failed to seed admin user", "error", err)
			os.Exit(1)
		}
	}

	location, err := time.LoadLocation(cfg.TimeZone)
	if err != nil {
		slog.Error("invalid timezone", "timezone", cfg.TimeZone, "error", err)
		os.Exit(1)
	}

	worker := core.NewWorker(db, notifiers, location)
	job := cronjob.NewTicker(time.Minute, func() { worker.Tick(context.Background()) })
	workerCtx, stopWorker := context.WithCancel(context.Background())
	job.Start(workerCtx)

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.NewServer(svc, cfg.SecureCookies),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("cakenify listening", "port", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server error", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)

	job.Stop()
	stopWorker()
}

// connectWithRetry tolerates postgres not being ready yet right after
// `docker compose up` starts both containers together.
func connectWithRetry(ctx context.Context, dsn string) (*sql.DB, error) {
	const (
		attempts = 10
		delay    = 2 * time.Second
	)

	var lastErr error
	for i := 0; i < attempts; i++ {
		db, err := sql.Open("pgx", dsn)
		if err == nil {
			if pingErr := db.PingContext(ctx); pingErr == nil {
				return db, nil
			} else {
				lastErr = pingErr
				_ = db.Close()
			}
		} else {
			lastErr = err
		}

		slog.Warn("database not ready, retrying", "attempt", i+1, "error", lastErr)
		time.Sleep(delay)
	}

	return nil, lastErr
}
