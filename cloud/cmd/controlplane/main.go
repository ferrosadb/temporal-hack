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

	"github.com/example/temporal-hack/cloud/internal/api"
	"github.com/example/temporal-hack/cloud/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	addr := envOr("LISTEN_ADDR", ":8081")
	tsdbDSN := envOr("TSDB_DSN", "postgres://temporal:temporal@localhost:5432/telemetry?sslmode=disable")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	tsdb, err := store.OpenTimescale(ctx, tsdbDSN)
	if err != nil {
		logger.Error("tsdb open", "err", err)
		os.Exit(1)
	}
	defer tsdb.Close()

	srv := &http.Server{
		Addr:              addr,
		Handler:           api.NewRouter(api.Deps{TSDB: tsdb, Logger: logger}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdown, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = srv.Shutdown(shutdown)
	}()

	logger.Info("control plane listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("listen", "err", err)
		os.Exit(1)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
