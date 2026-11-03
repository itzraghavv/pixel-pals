package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-playground/validator/v10"
	"github.com/itzraghavv/pixel-pals/backend/internal/auth"
	"github.com/itzraghavv/pixel-pals/backend/internal/platform"
	"github.com/itzraghavv/pixel-pals/backend/internal/service"
)

type identityKey struct{}
type logKey struct{}
type requestLog struct{ UserID string }
type Server struct {
	Service   *service.Service
	Config    platform.Config
	Logger    *slog.Logger
	Ready     func(context.Context) error
	Validate  *validator.Validate
	Limiter   *Limiter
	Upload    UploadSigner
	Metrics   *Metrics
	RateLimit func(context.Context, string) (bool, error)
}
type UploadSigner interface {
	Sign(context.Context, string, int64) (any, error)
}

func (s *Server) Router() http.Handler {
	if s.Validate == nil {
		s.Validate = validator.New()
	}
	if s.Logger == nil {
		s.Logger = slog.Default()
	}
	if s.Limiter == nil {
		s.Limiter = NewLimiter()
	}
	r := chi.NewRouter()
	r.Use(middleware.RequestID, s.observe, s.recover)
	if s.Metrics != nil {
		r.Use(s.Metrics.Middleware)
	}
	r.Use(cors.Handler(cors.Options{AllowOriginFunc: func(r *http.Request, origin string) bool {
		for _, allowed := range s.Config.Origins {
			if origin == allowed {
				return true
			}
		}
		return false
	}, AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}, AllowedHeaders: []string{"Authorization", "Content-Type"}, AllowCredentials: true, MaxAge: 300}))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			next.ServeHTTP(w, r)
		})
	})
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { s.json(w, 200, map[string]string{"status": "ok"}) })
	if s.Metrics != nil {
		r.Handle("/metrics", s.Metrics.Handler())
	}
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if s.Ready != nil {
			if e := s.Ready(ctx); e != nil {
				s.fail(w, service.Fail(503, "NOT_READY", "Dependencies unavailable"))
				return
			}
		}
		s.json(w, 200, map[string]string{"status": "ready"})
	})
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/sign-up", s.signUp)
		r.Post("/sign-in", s.signIn)
		r.Post("/auth/refresh", s.refresh)
		r.Post("/auth/logout", s.logout)
		r.Get("/avatars", s.avatars)
		r.Get("/element", s.elements)
		r.Get("/elements", s.elements)
		r.Get("/maps", s.maps)
		r.Group(func(r chi.Router) {
			r.Use(s.requireUser)
			r.Get("/me", s.me)
			r.Post("/user/metadata", s.setAvatar)
			r.Get("/user/metadata/bulk", s.bulkMetadata)
			r.Post("/space", s.createSpace)
			r.Get("/space/all", s.listSpaces)
			r.Post("/space/element", s.addPlacement)
			r.Delete("/space/element", s.removePlacement)
			r.Get("/space/{id}", s.getSpace)
			r.Delete("/space/{id}", s.deleteSpace)
			r.Post("/space/{id}/invites", s.invite)
			r.Post("/invites/{token}/accept", s.acceptInvite)
			r.Route("/admin", func(r chi.Router) {
				r.Use(s.requireAdmin)
				r.Post("/avatar", s.createAvatar)
				r.Post("/element", s.createElement)
				r.Put("/element/{id}", s.updateElement)
				r.Post("/map", s.createMap)
				r.Post("/uploads", s.upload)
			})
		})
	})
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		s.fail(w, service.Fail(404, "NOT_FOUND", "Route not found"))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		s.fail(w, service.Fail(405, "METHOD_NOT_ALLOWED", "Method not allowed"))
	})
	return r
}
func (s *Server) json(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (s *Server) fail(w http.ResponseWriter, e error) {
	var api *service.Error
	if !errors.As(e, &api) {
		s.Logger.Error("request failed", "error", e)
		api = &service.Error{Status: 500, Code: "INTERNAL_ERROR", Message: "Internal server error"}
	}
	v := map[string]any{"error": map[string]string{"code": api.Code, "message": api.Message}}
	if s.Config.Compat {
		v["message"] = api.Message
	}
	s.json(w, api.Status, v)
}
func (s *Server) decode(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		s.fail(w, service.Fail(400, "INVALID_REQUEST", "Invalid JSON body"))
		return false
	}
	if e := d.Decode(new(any)); e != io.EOF {
		s.fail(w, service.Fail(400, "INVALID_REQUEST", "Expected one JSON object"))
		return false
	}
	if e := s.Validate.Struct(v); e != nil {
		s.fail(w, service.Fail(400, "VALIDATION_ERROR", "Invalid request fields"))
		return false
	}
	return true
}
func user(r *http.Request) *auth.Claims { return r.Context().Value(identityKey{}).(*auth.Claims) }
func (s *Server) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" {
			s.fail(w, service.Fail(403, "AUTH_REQUIRED", "Authentication required"))
			return
		}
		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			s.fail(w, service.Fail(401, "INVALID_TOKEN", "Invalid access token"))
			return
		}
		claims, e := s.Service.Tokens.Verify(parts[1])
		if e != nil {
			s.fail(w, service.Fail(401, "INVALID_TOKEN", "Invalid access token"))
			return
		}
		if log, ok := r.Context().Value(logKey{}).(*requestLog); ok {
			log.UserID = claims.Subject
		}
		r = r.WithContext(context.WithValue(r.Context(), identityKey{}, claims))
		next.ServeHTTP(w, r)
	})
}
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user(r).Role != "admin" {
			s.fail(w, service.Fail(403, "ADMIN_REQUIRED", "Admin access required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if e := recover(); e != nil {
				s.Logger.Error("panic", "error", e)
				s.fail(w, service.Fail(500, "INTERNAL_ERROR", "Internal server error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
func (s *Server) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log := new(requestLog)
		r = r.WithContext(context.WithValue(r.Context(), logKey{}, log))
		w.Header().Set("X-Request-ID", middleware.GetReqID(r.Context()))
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		s.Logger.Info("http request", "request_id", middleware.GetReqID(r.Context()), "user_id", log.UserID, "method", r.Method, "route", chi.RouteContext(r.Context()).RoutePattern(), "status", ww.Status(), "latency_ms", time.Since(start).Milliseconds())
	})
}
