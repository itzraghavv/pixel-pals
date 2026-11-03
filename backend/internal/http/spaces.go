package httpapi

import (
	"github.com/go-chi/chi/v5"
	"github.com/itzraghavv/pixel-pals/backend/internal/service"
	"net/http"
)

func (s *Server) createSpace(w http.ResponseWriter, r *http.Request) {
	var in service.SpaceInput
	if !s.decode(w, r, &in) {
		return
	}
	id, e := s.Service.CreateSpace(r.Context(), user(r).Subject, in)
	if e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, map[string]string{"spaceId": id})
}
func (s *Server) listSpaces(w http.ResponseWriter, r *http.Request) {
	rows, e := s.Service.Spaces(r.Context(), user(r).Subject)
	if e != nil {
		s.fail(w, e)
		return
	}
	out := make([]any, 0, len(rows))
	for _, v := range rows {
		out = append(out, map[string]any{"id": v.ID, "name": v.Name, "thumbnail": v.ThumbnailUrl, "dimensions": service.Dim(v.Width, v.Height)})
	}
	s.json(w, 200, map[string]any{"spaces": out})
}
func (s *Server) getSpace(w http.ResponseWriter, r *http.Request) {
	space, placements, e := s.Service.GetSpace(r.Context(), chi.URLParam(r, "id"), user(r).Subject)
	if e != nil {
		s.fail(w, e)
		return
	}
	out := make([]any, 0, len(placements))
	for _, p := range placements {
		out = append(out, map[string]any{"id": p.ID, "x": p.X, "y": p.Y, "element": map[string]any{"id": p.ElementID, "width": p.Width, "height": p.Height, "static": p.Static, "imageUrl": p.ImageUrl}})
	}
	s.json(w, 200, map[string]any{"id": space.ID, "name": space.Name, "dimensions": service.Dim(space.Width, space.Height), "elements": out})
}
func (s *Server) deleteSpace(w http.ResponseWriter, r *http.Request) {
	if e := s.Service.DeleteSpace(r.Context(), chi.URLParam(r, "id"), user(r).Subject); e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, map[string]string{"message": "Space deleted"})
}
func (s *Server) addPlacement(w http.ResponseWriter, r *http.Request) {
	var in service.AddPlacementInput
	if !s.decode(w, r, &in) {
		return
	}
	id, e := s.Service.AddPlacement(r.Context(), user(r).Subject, in)
	if e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, map[string]string{"id": id, "message": "Element added"})
}
func (s *Server) removePlacement(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID string `json:"id" validate:"required,max=128"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	if e := s.Service.RemovePlacement(r.Context(), in.ID, user(r).Subject); e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, map[string]string{"message": "Element removed"})
}
func (s *Server) invite(w http.ResponseWriter, r *http.Request) {
	token, e := s.Service.Invite(r.Context(), chi.URLParam(r, "id"), user(r).Subject)
	if e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, map[string]string{"token": token, "acceptPath": "/api/v1/invites/" + token + "/accept"})
}
func (s *Server) acceptInvite(w http.ResponseWriter, r *http.Request) {
	id, e := s.Service.AcceptInvite(r.Context(), chi.URLParam(r, "token"), user(r).Subject)
	if e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, map[string]string{"spaceId": id})
}
