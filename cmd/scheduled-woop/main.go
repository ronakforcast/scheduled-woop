package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
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
	logger.Info("scheduled WOOP started", "pollInterval", config.PollInterval, "timezone", config.Timezone)
	runner := app.Runner{Config: config, CAST: client, Log: logger}
	if err := runner.Run(ctx); err != nil {
		logger.Error("stopped", "error", err)
		os.Exit(1)
	}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
