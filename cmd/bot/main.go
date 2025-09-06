package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/lckrugel/billy-the-bot/internal/config"
	"github.com/lckrugel/billy-the-bot/internal/gateway"
	"github.com/lckrugel/billy-the-bot/internal/types"
)

func setupLogger() {
	logLevel := os.Getenv("LOG_LEVEL")

	var level slog.Level
	switch logLevel {
	case "DEBUG":
		level = slog.LevelDebug
	case "INFO":
		level = slog.LevelInfo
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo // Default to INFO
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	handler := slog.NewTextHandler(os.Stdout, opts)

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		panic("failed to load '.env' file")
	}

	setupLogger()

	botCfg, err := config.NewBotConfig()
	if err != nil {
		slog.Error("couldnt load bot config", "code", types.BAD_CONFIG, "error", err)
		panic("failed to load bot config")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	gatewayClient := gateway.NewGatewayClient(botCfg)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := gatewayClient.Run(ctx); err != nil {
			slog.Error("gatewat client error", "error", err)
		}
	}()

	sig := <-sigChan
	slog.Debug("received shutdown signal", "signal", sig)

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Debug("graceful shutdown completed")
	case <-shutdownCtx.Done():
		slog.Warn("shutdown timeout exceeded, forcing exit")
	}

	slog.Info("bot stopped")
}
