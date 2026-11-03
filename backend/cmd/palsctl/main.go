package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/itzraghavv/pixel-pals/backend/internal/auth"
	"github.com/itzraghavv/pixel-pals/backend/internal/service"
	"github.com/itzraghavv/pixel-pals/backend/internal/store"
	"github.com/itzraghavv/pixel-pals/backend/internal/store/db"
	"github.com/itzraghavv/pixel-pals/backend/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"os"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: palsctl migrate|seed|promote-admin <username>")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if os.Args[1] == "migrate" {
		conn, e := sql.Open("pgx", dsn)
		if e != nil {
			return e
		}
		defer conn.Close()
		goose.SetBaseFS(migrations.Files)
		if e = goose.SetDialect("postgres"); e != nil {
			return e
		}
		return goose.UpContext(ctx, conn, ".")
	}
	pool, e := pgxpool.New(ctx, dsn)
	if e != nil {
		return e
	}
	defer pool.Close()
	switch os.Args[1] {
	case "promote-admin":
		if len(os.Args) != 3 {
			return fmt.Errorf("usage: palsctl promote-admin <username>")
		}
		tag, e := pool.Exec(ctx, "UPDATE users SET role='admin' WHERE username=$1", os.Args[2])
		if e == nil && tag.RowsAffected() == 0 {
			return fmt.Errorf("user not found")
		}
		return e
	case "seed":
		return seed(ctx, store.New(pool))
	default:
		return fmt.Errorf("unknown command %q", os.Args[1])
	}
}
func seed(ctx context.Context, repo *store.Postgres) error {
	password := os.Getenv("SEED_PASSWORD")
	if len(password) < 8 {
		return fmt.Errorf("SEED_PASSWORD (at least 8 characters) is required")
	}
	hash, e := auth.Hash(password)
	if e != nil {
		return e
	}
	return repo.Transaction(ctx, func(q db.Querier) error {
		// A transaction and stable IDs make repeated local seeding safe.
		if _, e := q.GetAvatar(ctx, "seed-avatar"); service.Missing(e) {
			if e = q.CreateAvatar(ctx, db.CreateAvatarParams{ID: "seed-avatar", Name: "Explorer", ImageUrl: "https://placehold.co/32x32/png"}); e != nil {
				return e
			}
		} else if e != nil {
			return e
		}
		if _, e := q.GetElement(ctx, "seed-wall"); service.Missing(e) {
			if e = q.CreateElement(ctx, db.CreateElementParams{ID: "seed-wall", Width: 1, Height: 1, Static: true, ImageUrl: "https://placehold.co/32x32/png"}); e != nil {
				return e
			}
		} else if e != nil {
			return e
		}
		for _, u := range []struct{ id, name, role string }{{"seed-admin", "demo-admin", "admin"}, {"seed-user", "demo-user", "user"}} {
			if _, e := q.UserByUsername(ctx, u.name); service.Missing(e) {
				if e = q.CreateUser(ctx, db.CreateUserParams{ID: u.id, Username: u.name, PasswordHash: hash, Role: u.role}); e != nil {
					return e
				}
			} else if e != nil {
				return e
			}
		}
		for _, m := range []struct {
			id, name string
			w, h     int32
		}{{"seed-lounge", "Lounge", 20, 20}, {"seed-garden", "Garden", 30, 20}} {
			if _, e := q.GetMap(ctx, m.id); service.Missing(e) {
				if e = q.CreateMap(ctx, db.CreateMapParams{ID: m.id, Name: m.name, Width: m.w, Height: m.h}); e != nil {
					return e
				}
				if e = q.CreateMapElement(ctx, db.CreateMapElementParams{ID: m.id + "-wall", MapID: m.id, ElementID: "seed-wall", X: 0, Y: 0}); e != nil {
					return e
				}
			} else if e != nil {
				return e
			}
		}
		return nil
	})
}
