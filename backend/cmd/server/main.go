// Command server runs the Subscription Radar API (and, optionally, serves the
// built frontend so the whole app runs from one process).
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"subscription-radar/internal/api"
)

func main() {
	addr := flag.String("addr", envOr("ADDR", ":8080"), "listen address")
	static := flag.String("static", "", "directory with the built frontend (optional)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := &http.Server{
		Addr:              *addr,
		Handler:           api.NewHandler(api.Options{StaticDir: *static, Logger: logger}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("listening", "addr", *addr, "static", *static)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
