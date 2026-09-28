package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/container"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		slog.Error("worker stopped", "error", err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	app, err := container.NewWorker(ctx)
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = app.Close(shutdownCtx)
	}()

	listener, err := net.Listen("tcp", app.Settings.WorkerHealthAddr)
	if err != nil {
		return err
	}
	healthServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/health" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}),
		ReadHeaderTimeout: 2 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = healthServer.Shutdown(shutdownCtx)
	}()
	go func() {
		if err := healthServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("worker health server", "error", err.Error())
		}
	}()
	return app.Runner.Run(ctx)
}
