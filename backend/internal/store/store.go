package store

import (
	"context"
	"github.com/itzraghavv/pixel-pals/backend/internal/store/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository allows services to run the same generated queries on a pool or transaction.
type Repository interface {
	db.Querier
	Transaction(context.Context, func(db.Querier) error) error
}
type Postgres struct {
	*db.Queries
	Pool *pgxpool.Pool
}

func New(p *pgxpool.Pool) *Postgres { return &Postgres{db.New(p), p} }
func (p *Postgres) Transaction(ctx context.Context, fn func(db.Querier) error) error {
	return pgx.BeginFunc(ctx, p.Pool, func(tx pgx.Tx) error { return fn(db.New(tx)) })
}
