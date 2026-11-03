package httpapi

import (
	"github.com/itzraghavv/pixel-pals/backend/internal/service"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type attempts struct {
	Count int
	Until time.Time
}
type Limiter struct {
	mu   sync.Mutex
	keys map[string]attempts
}

func NewLimiter() *Limiter { return &Limiter{keys: make(map[string]attempts)} }
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for k, v := range l.keys {
		if now.After(v.Until) {
			delete(l.keys, k)
		}
	}
	v := l.keys[key]
	if v.Until.IsZero() {
		v.Until = now.Add(time.Minute)
	}
	if v.Count >= 10 || len(l.keys) >= 10000 && v.Count == 0 {
		return false
	}
	v.Count++
	l.keys[key] = v
	return true
}
func (s *Server) signUp(w http.ResponseWriter, r *http.Request) {
	var in service.SignUp
	if !s.decode(w, r, &in) {
		return
	}
	id, e := s.Service.SignUp(r.Context(), in)
	if e != nil {
		s.fail(w, e)
		return
	}
	s.json(w, 200, map[string]string{"userId": id})
}
func (s *Server) signIn(w http.ResponseWriter, r *http.Request) {
	var in service.SignIn
	if !s.decode(w, r, &in) {
		return
	}
	ip, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil {
		ip = r.RemoteAddr
	}
	key := ip + "\x00" + strings.ToLower(in.Username)
	allowed := false
	if s.RateLimit != nil {
		allowed, e = s.RateLimit(r.Context(), key)
		if e != nil {
			s.fail(w, service.Fail(503, "RATE_LIMIT_UNAVAILABLE", "Sign-in temporarily unavailable"))
			return
		}
	} else {
		allowed = s.Limiter.Allow(key)
	}
	if !allowed {
		w.Header().Set("Retry-After", "60")
		s.fail(w, service.Fail(429, "RATE_LIMITED", "Too many sign-in attempts"))
		return
	}
	access, refresh, e := s.Service.SignIn(r.Context(), in)
	if e != nil {
		s.fail(w, e)
		return
	}
	s.cookie(w, refresh, 30*24*60*60)
	s.json(w, 200, map[string]string{"token": access})
}
func (s *Server) cookie(w http.ResponseWriter, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: "refresh_token", Value: value, Path: "/api/v1/auth", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: age})
}
func (s *Server) cookieOrigin(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	for _, v := range s.Config.Origins {
		if v == origin {
			return true
		}
	}
	s.fail(w, service.Fail(403, "ORIGIN_FORBIDDEN", "Origin not allowed"))
	return false
}
func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	if !s.cookieOrigin(w, r) {
		return
	}
	c, e := r.Cookie("refresh_token")
	if e != nil {
		s.fail(w, service.Fail(401, "INVALID_REFRESH", "Refresh token required"))
		return
	}
	access, refresh, e := s.Service.Refresh(r.Context(), c.Value)
	if e != nil {
		s.fail(w, e)
		return
	}
	s.cookie(w, refresh, 30*24*60*60)
	s.json(w, 200, map[string]string{"token": access})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if !s.cookieOrigin(w, r) {
		return
	}
	if c, e := r.Cookie("refresh_token"); e == nil {
		if e = s.Service.Logout(r.Context(), c.Value); e != nil {
			s.fail(w, e)
			return
		}
	}
	s.cookie(w, "", -1)
	s.json(w, 200, map[string]string{"message": "Logged out"})
}
