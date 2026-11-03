package main

import (
	"context"
	"errors"
	"github.com/itzraghavv/pixel-pals/backend/internal/auth"
	httpapi "github.com/itzraghavv/pixel-pals/backend/internal/http"
	"github.com/itzraghavv/pixel-pals/backend/internal/platform"
	"github.com/itzraghavv/pixel-pals/backend/internal/service"
	"github.com/itzraghavv/pixel-pals/backend/internal/storage"
	"github.com/itzraghavv/pixel-pals/backend/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if e := run(log); e != nil {
		log.Error("api stopped", "error", e)
		os.Exit(1)
	}
}
func run(log *slog.Logger) error {
	c, e := platform.Load()
	if e != nil {
		return e
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pool, e := pgxpool.New(startup, c.DatabaseURL)
	if e != nil {
		return e
	}
	defer pool.Close()
	if e = pool.Ping(startup); e != nil {
		return e
	}
	ro, e := redis.ParseURL(c.RedisURL)
	if e != nil {
		return e
	}
	ro.DialTimeout = 2 * time.Second
	ro.ReadTimeout = 2 * time.Second
	ro.WriteTimeout = 2 * time.Second
	redisClient := redis.NewClient(ro)
	defer redisClient.Close()
	if e = redisClient.Ping(startup).Err(); e != nil {
		return e
	}
	repo := store.New(pool)
	svc := &service.Service{Repo: repo, Tokens: auth.Tokens{Secret: []byte(c.JWTSecret)}, AllowAdmin: c.AllowAdmin, Compat: c.Compat}
	api := &httpapi.Server{Service: svc, Config: c, Logger: log, Metrics: httpapi.NewMetrics(pool), Ready: func(ctx context.Context) error {
		if e := pool.Ping(ctx); e != nil {
			return e
		}
		return redisClient.Ping(ctx).Err()
	}}
	api.RateLimit = platform.LimitSignIn(redisClient)
	uploads, e := storage.New(c)
	if e != nil {
		return e
	}
	if uploads != nil {
		api.Upload = uploads
	}
	// Keep outbox running while HTTP drains; then stop it before closing the pool.
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); platform.DeliverEvents(workerCtx, repo, redisClient, log) }()
	server := &http.Server{Addr: ":" + c.Port, Handler: api.Router(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	result := make(chan error, 1)
	go func() { log.Info("api listening", "port", c.Port); result <- server.ListenAndServe() }()
	select {
	case e = <-result:
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		e = server.Shutdown(shutdown)
		cancel()
		if e != nil {
			_ = server.Close()
		}
	}
	workerCancel()
	<-workerDone
	if errors.Is(e, http.ErrServerClosed) {
		return nil
	}
	return e
}
