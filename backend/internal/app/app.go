package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/inox/inox/backend/internal/api"
	"github.com/inox/inox/backend/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	Config *config.Config
	Logger *slog.Logger
	DB     *pgxpool.Pool
}

func New(cfg *config.Config, log *slog.Logger, db *pgxpool.Pool) *App {
	return &App{
		Config: cfg,
		Logger: log,
		DB:     db,
	}
}

func (a *App) Run() error {
	router := api.NewRouter()

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", a.Config.HTTPPort),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		a.Logger.Info("starting HTTP server", "addr", srv.Addr, "env", a.Config.Environment)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.Logger.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	stop()
	a.Logger.Info("shutting down HTTP server gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		a.Logger.Error("server shutdown error", "error", err)
	}

	// Cleanly close database connections so PostgreSQL terminates sockets gracefully.
	if a.DB != nil {
		a.Logger.Info("closing PostgreSQL connection pool...")
		a.DB.Close()
	}

	a.Logger.Info("server exited cleanly")
	return nil
}
