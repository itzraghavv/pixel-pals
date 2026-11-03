package service

import (
	"context"
	"github.com/itzraghavv/pixel-pals/backend/internal/auth"
	"github.com/itzraghavv/pixel-pals/backend/internal/store/db"
	"github.com/jackc/pgx/v5/pgtype"
	"time"
)

type SpaceInput struct {
	Name       string `json:"name" validate:"required,max=100"`
	Dimensions string `json:"dimensions"`
	MapID      string `json:"mapId" validate:"max=128"`
}
type AddPlacementInput struct {
	SpaceID   string `json:"spaceId" validate:"required,max=128"`
	ElementID string `json:"elementId" validate:"required,max=128"`
	X         *int32 `json:"x" validate:"required,gte=0,lte=1000"`
	Y         *int32 `json:"y" validate:"required,gte=0,lte=1000"`
}

func (s *Service) CreateSpace(ctx context.Context, uid string, in SpaceInput) (string, error) {
	id := ID()
	e := s.Repo.Transaction(ctx, func(q db.Querier) error {
		var w, h int32
		var thumbnail string
		var placements []db.MapElement
		if in.MapID != "" {
			m, e := q.GetMap(ctx, in.MapID)
			if Missing(e) {
				status := 400
				if s.Compat {
					status = 403
				}
				return Fail(status, "MAP_NOT_FOUND", "Unknown map")
			}
			if e != nil {
				return e
			}
			w, h, thumbnail = m.Width, m.Height, m.ThumbnailUrl
			placements, e = q.GetMapElements(ctx, m.ID)
			if e != nil {
				return e
			}
		} else {
			var e error
			w, h, e = Dimensions(in.Dimensions)
			if e != nil {
				return e
			}
		}
		if e := q.CreateSpace(ctx, db.CreateSpaceParams{ID: id, Name: in.Name, Width: w, Height: h, ThumbnailUrl: thumbnail, CreatorID: uid}); e != nil {
			return e
		}
		if e := q.AddMember(ctx, db.AddMemberParams{SpaceID: id, UserID: uid, Role: "owner"}); e != nil {
			return e
		}
		for _, p := range placements {
			if e := q.CreatePlacement(ctx, db.CreatePlacementParams{ID: ID(), SpaceID: id, ElementID: p.ElementID, X: p.X, Y: p.Y}); e != nil {
				return e
			}
		}
		return nil
	})
	return id, e
}
func (s *Service) spaceMissing() error {
	status := 404
	if s.Compat {
		status = 400
	}
	return Fail(status, "SPACE_NOT_FOUND", "Space not found")
}
func (s *Service) owned(ctx context.Context, q db.Querier, id, uid string) (db.Space, error) {
	space, e := q.LockSpace(ctx, id)
	if Missing(e) {
		return space, s.spaceMissing()
	}
	if e != nil {
		return space, e
	}
	if space.CreatorID != uid {
		return space, Fail(403, "NOT_OWNER", "Only the owner can modify this space")
	}
	return space, nil
}
func (s *Service) GetSpace(ctx context.Context, id, uid string) (db.Space, []db.GetSpaceElementsRow, error) {
	space, e := s.Repo.GetSpace(ctx, id)
	if Missing(e) {
		return space, nil, s.spaceMissing()
	}
	if e != nil {
		return space, nil, e
	}
	if space.CreatorID != uid {
		member, e := s.Repo.IsMember(ctx, db.IsMemberParams{SpaceID: id, UserID: uid})
		if e != nil {
			return space, nil, e
		}
		if !member {
			return space, nil, Fail(403, "NOT_MEMBER", "Space membership required")
		}
	}
	els, e := s.Repo.GetSpaceElements(ctx, id)
	return space, els, e
}
func (s *Service) DeleteSpace(ctx context.Context, id, uid string) error {
	return s.Repo.Transaction(ctx, func(q db.Querier) error {
		if _, e := s.owned(ctx, q, id, uid); e != nil {
			return e
		}
		if e := q.DeleteSpace(ctx, id); e != nil {
			return e
		}
		return q.EnqueueSpaceEvent(ctx, id)
	})
}
func (s *Service) AddPlacement(ctx context.Context, uid string, in AddPlacementInput) (string, error) {
	id := ID()
	e := s.Repo.Transaction(ctx, func(q db.Querier) error {
		space, e := s.owned(ctx, q, in.SpaceID, uid)
		if e != nil {
			return e
		}
		el, e := q.GetElement(ctx, in.ElementID)
		if Missing(e) {
			return Fail(400, "ELEMENT_NOT_FOUND", "Unknown element")
		}
		if e != nil {
			return e
		}
		if in.X == nil || in.Y == nil || !Fits(*in.X, *in.Y, space.Width, space.Height, el.Width, el.Height) {
			return Fail(400, "OUT_OF_BOUNDS", "Element footprint is outside space")
		}
		if e = q.CreatePlacement(ctx, db.CreatePlacementParams{ID: id, SpaceID: space.ID, ElementID: el.ID, X: *in.X, Y: *in.Y}); e != nil {
			return e
		}
		return q.EnqueueSpaceEvent(ctx, space.ID)
	})
	return id, e
}
func (s *Service) RemovePlacement(ctx context.Context, id, uid string) error {
	return s.Repo.Transaction(ctx, func(q db.Querier) error {
		p, e := q.GetPlacement(ctx, id)
		if Missing(e) {
			return Fail(400, "PLACEMENT_NOT_FOUND", "Placed element not found")
		}
		if e != nil {
			return e
		}
		if _, e = s.owned(ctx, q, p.SpaceID, uid); e != nil {
			return e
		}
		if e = q.DeletePlacement(ctx, id); e != nil {
			return e
		}
		return q.EnqueueSpaceEvent(ctx, p.SpaceID)
	})
}
func (s *Service) Invite(ctx context.Context, id, uid string) (string, error) {
	raw, e := auth.Opaque()
	if e != nil {
		return "", e
	}
	e = s.Repo.Transaction(ctx, func(q db.Querier) error {
		if _, e := s.owned(ctx, q, id, uid); e != nil {
			return e
		}
		return q.CreateInvite(ctx, db.CreateInviteParams{TokenHash: auth.Digest(raw), SpaceID: id, ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(7 * 24 * time.Hour), Valid: true}})
	})
	return raw, e
}
func (s *Service) AcceptInvite(ctx context.Context, raw, uid string) (string, error) {
	var id string
	e := s.Repo.Transaction(ctx, func(q db.Querier) error {
		invite, e := q.GetInvite(ctx, auth.Digest(raw))
		if Missing(e) {
			return Fail(404, "INVITE_NOT_FOUND", "Invite is invalid or expired")
		}
		if e != nil {
			return e
		}
		id = invite.SpaceID
		return q.AddMember(ctx, db.AddMemberParams{SpaceID: id, UserID: uid, Role: "member"})
	})
	return id, e
}
