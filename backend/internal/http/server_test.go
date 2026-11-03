package httpapi

import (
	"bytes"
	"context"
	"errors"
	"github.com/itzraghavv/pixel-pals/backend/internal/auth"
	"github.com/itzraghavv/pixel-pals/backend/internal/platform"
	"github.com/itzraghavv/pixel-pals/backend/internal/service"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testServer() *Server {
	return &Server{Service: &service.Service{Tokens: auth.Tokens{Secret: []byte(strings.Repeat("s", 32))}}, Config: platform.Config{Compat: true, Origins: []string{"https://app.example"}}, Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}
}
func TestHTTPGuards(t *testing.T) {
	s := testServer()
	router := s.Router()
	token, _ := s.Service.Tokens.Issue("user-1", "user")
	for _, v := range []struct {
		name, method, path, body, token string
		status                          int
	}{{"missing token", "GET", "/api/v1/me", "", "", 403}, {"invalid token", "GET", "/api/v1/me", "", "bad", 401}, {"not admin", "POST", "/api/v1/admin/avatar", "{}", token, 403}, {"bad json", "POST", "/api/v1/sign-up", "{", "", 400}, {"unknown field", "POST", "/api/v1/sign-up", `{"extra":true}`, "", 400}, {"multiple bodies", "POST", "/api/v1/sign-up", "{}{}", "", 400}, {"invalid ids", "GET", "/api/v1/user/metadata/bulk?ids=oops", "", token, 400}, {"health", "GET", "/healthz", "", "", 200}, {"not found", "GET", "/no-such-route", "", "", 404}, {"wrong method", "PUT", "/api/v1/sign-up", "", "", 405}, {"invalid fields", "POST", "/api/v1/sign-up", `{"username":"a","password":"short","type":"root"}`, "", 400}, {"oversize", "POST", "/api/v1/sign-up", strings.Repeat("x", 2<<20), "", 400}} {
		t.Run(v.name, func(t *testing.T) {
			r := httptest.NewRequest(v.method, v.path, strings.NewReader(v.body))
			if v.token != "" {
				r.Header.Set("Authorization", "Bearer "+v.token)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			if w.Code != v.status {
				t.Fatalf("status %d body %s", w.Code, w.Body)
			}
			if w.Header().Get("X-Request-ID") == "" {
				t.Fatal("missing request ID")
			}
			if v.status >= 400 && (!strings.Contains(w.Body.String(), `"error"`) || !strings.Contains(w.Body.String(), `"message"`)) {
				t.Fatal(w.Body)
			}
		})
	}
}
func TestReadinessAndCookieOrigin(t *testing.T) {
	s := testServer()
	s.Ready = func(context.Context) error { return errors.New("db down") }
	router := s.Router()
	for _, v := range []struct {
		method, path, origin string
		status               int
	}{{"GET", "/readyz", "", 503}, {"POST", "/api/v1/auth/refresh", "https://evil.example", 403}, {"POST", "/api/v1/auth/refresh", "https://app.example", 401}, {"POST", "/api/v1/auth/logout", "https://app.example", 200}} {
		r := httptest.NewRequest(v.method, v.path, nil)
		r.Header.Set("Origin", v.origin)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != v.status {
			t.Fatalf("%s %d %s", v.path, w.Code, w.Body)
		}
	}
}
func TestIDs(t *testing.T) {
	for _, v := range []string{"[a,b]", `["a","b"]`, "[]"} {
		if _, e := parseIDs(v); e != nil {
			t.Fatal(e)
		}
	}
	for _, v := range []string{"a,b", "[a,]", "[a,,b]", `["a",3]`, "[[a]]", "[a b]", "[" + strings.Repeat("a,", 100) + "b]"} {
		if _, e := parseIDs(v); e == nil {
			t.Fatalf("accepted %q", v)
		}
	}
}
func TestLimiter(t *testing.T) {
	l := NewLimiter()
	for i := 0; i < 10; i++ {
		if !l.Allow("ip:user") {
			t.Fatal("early limit")
		}
	}
	if l.Allow("ip:user") {
		t.Fatal("limit failed")
	}
	if !l.Allow("other:user") {
		t.Fatal("separate IP denied")
	}
}
func TestUserIDInLogs(t *testing.T) {
	s := testServer()
	var out bytes.Buffer
	s.Logger = slog.New(slog.NewJSONHandler(&out, nil))
	token, _ := s.Service.Tokens.Issue("logged-user", "user")
	r := httptest.NewRequest(http.MethodPost, "/api/v1/admin/avatar", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	s.Router().ServeHTTP(httptest.NewRecorder(), r)
	if !strings.Contains(out.String(), `"user_id":"logged-user"`) {
		t.Fatal(out.String())
	}
}

func TestCORSAllowlist(t *testing.T) {
	for _, origins := range [][]string{nil, {"https://app.example"}} {
		s := testServer()
		s.Config.Origins = origins
		router := s.Router()
		for _, origin := range []string{"https://app.example", "https://evil.example"} {
			r := httptest.NewRequest("GET", "/healthz", nil)
			r.Header.Set("Origin", origin)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			allowed := w.Header().Get("Access-Control-Allow-Origin")
			want := ""
			if len(origins) > 0 && origin == origins[0] {
				want = origin
			}
			if allowed != want {
				t.Fatalf("origin %s allowed as %s", origin, allowed)
			}
		}
	}
}
func TestSharedLimiterErrors(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		s := testServer()
		s.RateLimit = func(context.Context, string) (bool, error) {
			if unavailable {
				return false, errors.New("redis down")
			}
			return false, nil
		}
		r := httptest.NewRequest("POST", "/api/v1/sign-in", strings.NewReader(`{"username":"user","password":"password"}`))
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		want := 429
		if unavailable {
			want = 503
		}
		if w.Code != want {
			t.Fatal(w.Code, w.Body)
		}
	}
}

type uploadStub struct{}

func (uploadStub) Sign(context.Context, string, int64) (any, error) {
	return map[string]any{"uploadUrl": "https://storage.example/upload", "publicUrl": "https://storage.example/asset.png", "headers": map[string][]string{"Content-Type": {"image/png"}}, "expiresIn": 300}, nil
}
func TestUploadContract(t *testing.T) {
	s := testServer()
	admin, _ := s.Service.Tokens.Issue("admin", "admin")
	for _, tc := range []struct {
		body    string
		enabled bool
		status  int
	}{{`{"contentType":"image/png","size":4}`, false, 503}, {`{"contentType":"image/png","size":4}`, true, 200}, {`{"contentType":"image/svg+xml","size":4}`, true, 400}, {`{"contentType":"image/png","size":2097153}`, true, 400}} {
		if tc.enabled {
			s.Upload = uploadStub{}
		} else {
			s.Upload = nil
		}
		r := httptest.NewRequest("POST", "/api/v1/admin/uploads", strings.NewReader(tc.body))
		r.Header.Set("Authorization", "Bearer "+admin)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatal(w.Code, w.Body)
		}
	}
}
