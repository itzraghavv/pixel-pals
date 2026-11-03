package httpapi

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/itzraghavv/pixel-pals/backend/internal/service"
	"github.com/itzraghavv/pixel-pals/backend/internal/store/db"

	"net/http"
	"strings"
)

func avatarDTO(a db.Avatar) any {
	return map[string]any{"id": a.ID, "name": a.Name, "imageUrl": a.ImageUrl}
}
func elementDTO(e db.Element) any {
	return map[string]any{"id": e.ID, "width": e.Width, "height": e.Height, "static": e.Static, "imageUrl": e.ImageUrl}
}
func (s *Server) avatars(w http.ResponseWriter, r *http.Request) {
	rows, e := s.Service.Avatars(r.Context())
	if e != nil {
		s.fail(w, e)
		return
	}
	out := make([]any, 0, len(rows))
	for _, v := range rows {
		out = append(out, avatarDTO(v))
	}
	s.json(w, 200, map[string]any{"avatars": out})
}
func (s *Server) elements(w http.ResponseWriter, r *http.Request) {
	rows, e := s.Service.Elements(r.Context())
	if e != nil {
		s.fail(w, e)
		return
	}
	out := make([]any, 0, len(rows))
	for _, v := range rows {
		out = append(out, elementDTO(v))
	}
	s.json(w, 200, map[string]any{"elements": out})
}
func (s *Server) maps(w http.ResponseWriter, r *http.Request) {
	rows, e := s.Service.Maps(r.Context())
	if e != nil {
		s.fail(w, e)
		return
	}
	out := make([]any, 0, len(rows))
	for _, v := range rows {
		out = append(out, map[string]any{"id": v.ID, "name": v.Name, "thumbnail": v.ThumbnailUrl, "dimensions": service.Dim(v.Width, v.Height)})
	}
	s.json(w, 200, map[string]any{"maps": out})
}
func (s *Server) createAvatar(w http.ResponseWriter, r *http.Request) {
	var in service.AvatarInput
	if !s.decode(w, r, &in) {
		return
	}
	id, e := s.Service.CreateAvatar(r.Context(), in)
	if e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, map[string]string{"avatarId": id})
}
func (s *Server) createElement(w http.ResponseWriter, r *http.Request) {
	var in service.ElementInput
	if !s.decode(w, r, &in) {
		return
	}
	id, e := s.Service.CreateElement(r.Context(), in)
	if e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, map[string]string{"id": id})
}
func (s *Server) updateElement(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ImageURL string `json:"imageUrl" validate:"required,http_url,max=2048"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	if e := s.Service.UpdateElement(r.Context(), chi.URLParam(r, "id"), in.ImageURL); e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, map[string]string{"message": "Element updated"})
}
func (s *Server) createMap(w http.ResponseWriter, r *http.Request) {
	var in service.MapInput
	if !s.decode(w, r, &in) {
		return
	}
	id, e := s.Service.CreateMap(r.Context(), in)
	if e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, map[string]string{"id": id})
}
func (s *Server) setAvatar(w http.ResponseWriter, r *http.Request) {
	var in struct {
		AvatarID string `json:"avatarId" validate:"required,max=128"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	e := s.Service.SetAvatar(r.Context(), user(r).Subject, in.AvatarID)
	if e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, map[string]string{"message": "Avatar updated"})
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u, a, e := s.Service.Me(r.Context(), user(r).Subject)
	if e != nil {
		s.fail(w, e)
		return
	}
	var avatar any
	if a != nil {
		avatar = avatarDTO(*a)
	}
	s.json(w, 200, map[string]any{"id": u.ID, "username": u.Username, "role": u.Role, "avatar": avatar})
}
func parseIDs(raw string) ([]string, error) {
	bad := service.Fail(400, "INVALID_IDS", "ids must be a bracketed list of at most 100 IDs")
	if len(raw) > 13000 || len(raw) < 2 || raw[0] != '[' || raw[len(raw)-1] != ']' {
		return nil, bad
	}
	var ids []string
	if strings.Contains(raw, "\"") {
		if e := json.Unmarshal([]byte(raw), &ids); e != nil {
			return nil, bad
		}
	} else {
		body := strings.TrimSpace(raw[1 : len(raw)-1])
		if body != "" {
			ids = strings.Split(body, ",")
		}
	}
	if len(ids) > 100 {
		return nil, bad
	}
	for i, v := range ids {
		v = strings.TrimSpace(v)
		if len(v) == 0 || len(v) > 128 {
			return nil, bad
		}
		for _, c := range v {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
				return nil, bad
			}
		}
		ids[i] = v
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, nil
}
func (s *Server) bulkMetadata(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()["ids"]
	if len(values) != 1 {
		s.fail(w, service.Fail(400, "INVALID_IDS", "Expected one ids query parameter"))
		return
	}
	ids, e := parseIDs(values[0])
	if e != nil {
		s.fail(w, e)
		return
	}
	rows, e := s.Service.Metadata(r.Context(), ids)
	if e != nil {
		s.fail(w, e)
		return
	}
	out := make([]any, 0, len(rows))
	for _, v := range rows {
		out = append(out, map[string]string{"userId": v.UserID, "imageUrl": v.ImageUrl})
	}
	s.json(w, 200, map[string]any{"avatars": out})
}
func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ContentType string `json:"contentType" validate:"required,oneof=image/png image/webp"`
		Size        int64  `json:"size" validate:"min=1,max=2097152"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	if s.Upload == nil {
		s.fail(w, service.Fail(503, "UPLOADS_DISABLED", "Uploads are not configured"))
		return
	}
	result, e := s.Upload.Sign(r.Context(), in.ContentType, in.Size)
	if e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, result)
}
