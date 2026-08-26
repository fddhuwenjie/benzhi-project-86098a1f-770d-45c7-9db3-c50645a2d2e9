package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"bioacoustic-corpus-release/internal/httpapi"
	"bioacoustic-corpus-release/internal/service"
	"bioacoustic-corpus-release/internal/store"
)

type runtime struct {
	store  *store.SQLiteStore
	server *http.Server
}

func newRuntime(ctx context.Context, database, address string) (*runtime, error) {
	repository, err := store.Open(ctx, database)
	if err != nil {
		return nil, err
	}
	svc := service.New(repository, time.Now)
	api := httpapi.New(svc)
	server := &http.Server{
		Addr: address, Handler: api.Handler(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second,
		MaxHeaderBytes: 32 << 10,
	}
	return &runtime{store: repository, server: server}, nil
}

func (r *runtime) close() error {
	return r.store.Close()
}

func (r *runtime) listen() (net.Listener, error) {
	listener, err := net.Listen("tcp", r.server.Addr)
	if err != nil {
		return nil, fmt.Errorf("监听 %s: %w", r.server.Addr, err)
	}
	return listener, nil
}

func (r *runtime) serve(listener net.Listener) error {
	slog.Info("HTTP 服务已启动", "addr", listener.Addr().String())
	err := r.server.Serve(listener)
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP 服务异常退出: %w", err)
	}
	return nil
}
