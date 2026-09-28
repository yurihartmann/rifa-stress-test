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

	_ "github.com/yurihartmann/rifa-stress-test/apps/api/api"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/container"
)

//	@title						Raffle Stress Test API
//	@version					1.0
//	@description				HTTP API for the raffle stress-test laboratory.
//	@BasePath					/v1
//	@securityDefinitions.apikey	AdminBearer
//	@in							header
//	@name						Authorization
//	@description				Admin token formatted as "Bearer <token>".
//	@securityDefinitions.apikey	WebhookSecret
//	@in							header
//	@name						X-Webhook-Secret
//	@description				Shared payment webhook secret.

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		slog.Error("api stopped", "error", err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	app, err := container.NewAPI(ctx)
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = app.Close(shutdownCtx)
	}()

	server := &http.Server{
		Addr:              app.Settings.HTTPAddr,
		Handler:           app.Router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
