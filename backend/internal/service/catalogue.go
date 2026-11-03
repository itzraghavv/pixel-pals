package service

import (
	"context"
	"github.com/itzraghavv/pixel-pals/backend/internal/store/db"
)

type AvatarInput struct {
	Name     string `json:"name" validate:"required,max=100"`
	ImageURL string `json:"imageUrl" validate:"required,http_url,max=2048"`
}
type ElementInput struct {
	Width    int32  `json:"width" validate:"min=1,max=1000"`
	Height   int32  `json:"height" validate:"min=1,max=1000"`
	Static   *bool  `json:"static" validate:"required"`
	ImageURL string `json:"imageUrl" validate:"required,http_url,max=2048"`
}
type PlacementInput struct {
	ElementID string `json:"elementId" validate:"required,max=128"`
	X         *int32 `json:"x" validate:"required,gte=0,lte=1000"`
	Y         *int32 `json:"y" validate:"required,gte=0,lte=1000"`
}
type MapInput struct {
	Name            string           `json:"name" validate:"required,max=100"`
	Thumbnail       string           `json:"thumbnail" validate:"omitempty,http_url,max=2048"`
	Dimensions      string           `json:"dimensions" validate:"required"`
	DefaultElements []PlacementInput `json:"defaultElements" validate:"max=10000,dive"`
}

func (s *Service) CreateAvatar(ctx context.Context, in AvatarInput) (string, error) {
	id := ID()
	return id, s.Repo.CreateAvatar(ctx, db.CreateAvatarParams{ID: id, Name: in.Name, ImageUrl: in.ImageURL})
}
func (s *Service) CreateElement(ctx context.Context, in ElementInput) (string, error) {
	id := ID()
	return id, s.Repo.CreateElement(ctx, db.CreateElementParams{ID: id, Width: in.Width, Height: in.Height, Static: *in.Static, ImageUrl: in.ImageURL})
}
func (s *Service) UpdateElement(ctx context.Context, id, url string) error {
	n, e := s.Repo.UpdateElement(ctx, db.UpdateElementParams{ID: id, ImageUrl: url})
	if e != nil {
		return e
	}
	if n == 0 {
		return Fail(404, "ELEMENT_NOT_FOUND", "Element not found")
	}
	return nil
}
func (s *Service) CreateMap(ctx context.Context, in MapInput) (string, error) {
	w, h, e := Dimensions(in.Dimensions)
	if e != nil {
		return "", e
	}
	id := ID()
	e = s.Repo.Transaction(ctx, func(q db.Querier) error {
		if e := q.CreateMap(ctx, db.CreateMapParams{ID: id, Name: in.Name, Width: w, Height: h, ThumbnailUrl: in.Thumbnail}); e != nil {
			return e
		}
		for _, p := range in.DefaultElements {
			el, e := q.GetElement(ctx, p.ElementID)
			if Missing(e) {
				return Fail(400, "INVALID_ELEMENT", "Unknown element")
			}
			if e != nil {
				return e
			}
			if p.X == nil || p.Y == nil || !Fits(*p.X, *p.Y, w, h, el.Width, el.Height) {
				return Fail(400, "OUT_OF_BOUNDS", "Element footprint is outside map")
			}
			if e = q.CreateMapElement(ctx, db.CreateMapElementParams{ID: ID(), MapID: id, ElementID: p.ElementID, X: *p.X, Y: *p.Y}); e != nil {
				return e
			}
		}
		return nil
	})
	return id, e
}
