package service

import (
	"context"
	"github.com/itzraghavv/pixel-pals/backend/internal/store/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Service) Avatars(ctx context.Context) ([]db.Avatar, error) { return s.Repo.ListAvatars(ctx) }
func (s *Service) Elements(ctx context.Context) ([]db.Element, error) {
	return s.Repo.ListElements(ctx)
}
func (s *Service) Maps(ctx context.Context) ([]db.Map, error) { return s.Repo.ListMaps(ctx) }
func (s *Service) Spaces(ctx context.Context, uid string) ([]db.Space, error) {
	return s.Repo.ListSpaces(ctx, uid)
}
func (s *Service) Metadata(ctx context.Context, ids []string) ([]db.BulkMetadataRow, error) {
	return s.Repo.BulkMetadata(ctx, ids)
}
func (s *Service) SetAvatar(ctx context.Context, uid, id string) error {
	if _, e := s.Repo.GetAvatar(ctx, id); e != nil {
		if Missing(e) {
			return Fail(400, "AVATAR_NOT_FOUND", "Unknown avatar")
		}
		return e
	}
	return s.Repo.SetAvatar(ctx, db.SetAvatarParams{ID: uid, AvatarID: pgtype.Text{String: id, Valid: true}})
}
func (s *Service) Me(ctx context.Context, uid string) (db.User, *db.Avatar, error) {
	u, e := s.Repo.UserByID(ctx, uid)
	if Missing(e) {
		return u, nil, Fail(401, "INVALID_TOKEN", "User no longer exists")
	}
	if e != nil {
		return u, nil, e
	}
	if !u.AvatarID.Valid {
		return u, nil, nil
	}
	a, e := s.Repo.GetAvatar(ctx, u.AvatarID.String)
	return u, &a, e
}
