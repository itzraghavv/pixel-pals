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

	"github.com/itzraghavv/pixel-pals/backend/internal/auth"
	"github.com/itzraghavv/pixel-pals/backend/internal/platform"
	"github.com/itzraghavv/pixel-pals/backend/internal/realtime"
	"github.com/itzraghavv/pixel-pals/backend/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("realtime stopped", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	c, err := platform.LoadRealtime()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(startup, c.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err = pool.Ping(startup); err != nil {
		return err
	}

	ws := realtime.NewServer(store.New(pool), auth.Tokens{Secret: []byte(c.JWTSecret)}, realtime.Options{
		Origins: c.Origins, Compat: c.Compat, AllowOverlap: c.AllowOverlap, ProximityRadius: c.ProximityRadius,
	}, log)
	mux := http.NewServeMux()
	mux.Handle("GET /ws", ws)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	server := &http.Server{
		Addr: ":" + c.Port, Handler: mux, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}
	result := make(chan error, 1)
	go func() {
		log.Info("realtime listening", "port", c.Port)
		result <- server.ListenAndServe()
	}()

	select {
	case err = <-result:
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err = server.Shutdown(shutdown)
		if err != nil {
			_ = server.Close()
		}
	}
	ws.Shutdown()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
