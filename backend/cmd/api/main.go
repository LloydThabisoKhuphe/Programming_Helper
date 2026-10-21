package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/auth"
	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/config"
	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/db"
	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/httpapi"
	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/projects"
	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/submissions"
)

func main() {
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	handler := httpapi.New(
		cfg,
		pool,
		auth.NewStore(pool, cfg.SessionTTL),
		projects.NewStore(pool),
		submissions.NewStore(pool),
		logger,
	)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Finish in-flight requests when you press Ctrl+C.
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	logger.Info("api listening", "addr", srv.Addr, "env", cfg.Env)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
	logger.Info("api stopped")
}
