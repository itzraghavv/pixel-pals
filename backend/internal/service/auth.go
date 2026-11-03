package service

import (
	"context"
	"github.com/itzraghavv/pixel-pals/backend/internal/auth"
	"github.com/itzraghavv/pixel-pals/backend/internal/store/db"
	"github.com/jackc/pgx/v5/pgtype"
	"time"
)

type SignUp struct {
	Username string `json:"username" validate:"required,min=3,max=64"`
	Password string `json:"password" validate:"required,min=8,max=128"`
	Type     string `json:"type" validate:"required,oneof=user admin"`
}
type SignIn struct {
	Username string `json:"username" validate:"required,max=64"`
	Password string `json:"password" validate:"required,max=128"`
}

func (s *Service) SignUp(ctx context.Context, in SignUp) (string, error) {
	if in.Type == "admin" && !s.AllowAdmin {
		return "", Fail(403, "ADMIN_SIGNUP_DISABLED", "Admin sign-up is disabled")
	}
	hash, e := auth.Hash(in.Password)
	if e != nil {
		return "", e
	}
	id := ID()
	e = s.Repo.CreateUser(ctx, db.CreateUserParams{ID: id, Username: in.Username, PasswordHash: hash, Role: in.Type})
	if Constraint(e) {
		return "", Fail(400, "USERNAME_TAKEN", "Username already exists")
	}
	return id, e
}
func (s *Service) SignIn(ctx context.Context, in SignIn) (access, refresh string, err error) {
	u, e := s.Repo.UserByUsername(ctx, in.Username)
	if Missing(e) {
		return "", "", Fail(403, "INVALID_CREDENTIALS", "Wrong username or password")
	}
	if e != nil {
		return "", "", e
	}
	valid, legacy := auth.Verify(in.Password, u.PasswordHash)
	if !valid {
		return "", "", Fail(403, "INVALID_CREDENTIALS", "Wrong username or password")
	}
	if legacy {
		hash, e := auth.Hash(in.Password)
		if e != nil {
			return "", "", e
		}
		if e = s.Repo.UpdatePassword(ctx, db.UpdatePasswordParams{ID: u.ID, PasswordHash: hash}); e != nil {
			return "", "", e
		}
	}
	return s.newSession(ctx, s.Repo, u)
}
func (s *Service) newSession(ctx context.Context, q db.Querier, u db.User) (string, string, error) {
	access, e := s.Tokens.Issue(u.ID, u.Role)
	if e != nil {
		return "", "", e
	}
	refresh, e := auth.Opaque()
	if e != nil {
		return "", "", e
	}
	e = q.CreateRefresh(ctx, db.CreateRefreshParams{ID: ID(), UserID: u.ID, TokenHash: auth.Digest(refresh), ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(30 * 24 * time.Hour), Valid: true}})
	return access, refresh, e
}
func (s *Service) Refresh(ctx context.Context, raw string) (access, refresh string, err error) {
	if raw == "" {
		return "", "", Fail(401, "INVALID_REFRESH", "Invalid refresh token")
	}
	err = s.Repo.Transaction(ctx, func(q db.Querier) error {
		id, e := q.ConsumeRefresh(ctx, auth.Digest(raw))
		if Missing(e) {
			return Fail(401, "INVALID_REFRESH", "Invalid refresh token")
		}
		if e != nil {
			return e
		}
		u, e := q.UserByID(ctx, id)
		if e != nil {
			return e
		}
		access, refresh, e = s.newSession(ctx, q, u)
		return e
	})
	return
}
func (s *Service) Logout(ctx context.Context, raw string) error {
	return s.Repo.RevokeRefresh(ctx, auth.Digest(raw))
}
