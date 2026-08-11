package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/castai/scheduled-woop/internal/app"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	configPath := env("CONFIG_PATH", "/etc/scheduled-woop/config.yaml")
	apiKeyPath := env("CAST_API_KEY_FILE", "/etc/scheduled-woop-secret/api-key")
	config, err := app.LoadConfig(configPath)
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	key, err := os.ReadFile(apiKeyPath)
	if err != nil {
		logger.Error("read API key", "error", err)
		os.Exit(1)
	}
	client, err := app.NewCASTClient(env("CAST_API_URL", "https://api.cast.ai"), strings.TrimSpace(string(key)))
	if err != nil {
		logger.Error("create CAST client", "error", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	metrics := app.NewMetrics()
	server := &http.Server{
		Addr:              env("LISTEN_ADDRESS", ":8080"),
		Handler:           metrics.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		logger.Error("bind observability server", "address", server.Addr, "error", err)
		os.Exit(1)
	}
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("observability server started", "address", listener.Addr().String())
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
			cancel()
		}
	}()
	metrics.SetReady(true)
	defer func() {
		metrics.SetReady(false)
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownContext)
	}()
	logger.Info("scheduled WOOP started", "pollInterval", config.PollInterval, "timezone", config.Timezone)
	runner := app.Runner{Config: config, CAST: client, Log: logger, Metrics: metrics}
	if err := runner.Run(ctx); err != nil {
		logger.Error("stopped", "error", err)
		os.Exit(1)
	}
	select {
	case err := <-serverErrors:
		logger.Error("observability server failed", "error", err)
		os.Exit(1)
	default:
	}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
