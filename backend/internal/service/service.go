package service

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/itzraghavv/pixel-pals/backend/internal/auth"
	"github.com/itzraghavv/pixel-pals/backend/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"strconv"
	"strings"
)

type Error struct {
	Status        int
	Code, Message string
}

func (e *Error) Error() string                    { return e.Message }
func Fail(status int, code, message string) error { return &Error{status, code, message} }
func Missing(err error) bool                      { return errors.Is(err, pgx.ErrNoRows) }
func Constraint(err error) bool {
	var p *pgconn.PgError
	return errors.As(err, &p) && strings.HasPrefix(p.Code, "23")
}

type Service struct {
	Repo               store.Repository
	Tokens             auth.Tokens
	AllowAdmin, Compat bool
}

func ID() string { return uuid.Must(uuid.NewV7()).String() }
func Dimensions(s string) (int32, int32, error) {
	parts := strings.Split(s, "x")
	if len(parts) != 2 {
		return 0, 0, Fail(400, "INVALID_DIMENSIONS", "Dimensions must be WxH")
	}
	w, e := strconv.Atoi(parts[0])
	h, f := strconv.Atoi(parts[1])
	if e != nil || f != nil || w < 1 || h < 1 || w > 1000 || h > 1000 {
		return 0, 0, Fail(400, "INVALID_DIMENSIONS", "Dimensions must be between 1 and 1000 tiles")
	}
	return int32(w), int32(h), nil
}
func Dim(w, h int32) string { return fmt.Sprintf("%dx%d", w, h) }
func Fits(x, y, w, h, ew, eh int32) bool {
	return x >= 0 && y >= 0 && ew > 0 && eh > 0 && x <= w-ew && y <= h-eh
}
