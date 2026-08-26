package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	slog.SetDefault(logger)
	configuration, err := parseConfig(os.Args[1:], environment)
	if err != nil {
		slog.Error("配置无效", "error", err)
		os.Exit(2)
	}
	ctx := context.Background()
	if configuration.SelfCheck {
		if err := runSelfCheck(ctx, configuration.Address); err != nil {
			slog.Error("自检失败", "error", err)
			os.Exit(1)
		}
		slog.Info("自检通过")
		return
	}
	if err := runUntilSignal(ctx, configuration); err != nil {
		slog.Error("服务退出", "error", err)
		os.Exit(1)
	}
}

func runUntilSignal(ctx context.Context, configuration config) error {
	rt, err := newRuntime(ctx, configuration.Database, configuration.Address)
	if err != nil {
		return err
	}
	defer rt.close()
	listener, err := rt.listen()
	if err != nil {
		return err
	}
	serveResult := make(chan error, 1)
	go func() { serveResult <- rt.serve(listener) }()
	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serveResult:
		return err
	case <-signalCtx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := rt.server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-serveResult; err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
