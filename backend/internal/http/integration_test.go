//go:build integration

package httpapi

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/itzraghavv/pixel-pals/backend/internal/auth"
	"github.com/itzraghavv/pixel-pals/backend/internal/service"
	"github.com/itzraghavv/pixel-pals/backend/internal/store"
	"github.com/itzraghavv/pixel-pals/backend/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/crypto/scrypt"
)

func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		c, e := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "postgres:16-alpine", Env: map[string]string{"POSTGRES_USER": "pals", "POSTGRES_PASSWORD": "pals", "POSTGRES_DB": "pals"}, ExposedPorts: []string{"5432/tcp"}, WaitingFor: wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(time.Minute)}, Started: true})
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { _ = c.Terminate(ctx) })
		host, e := c.Host(ctx)
		if e != nil {
			t.Fatal(e)
		}
		port, e := c.MappedPort(ctx, "5432/tcp")
		if e != nil {
			t.Fatal(e)
		}
		dsn = fmt.Sprintf("postgres://pals:pals@%s:%s/pals?sslmode=disable", host, port.Port())
	}
	// Use an isolated schema even with an explicitly supplied development database.
	admin, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	schema := "test_" + strings.ReplaceAll(service.ID(), "-", "")
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); admin.Close() })
	u, e := url.Parse(dsn)
	if e != nil {
		t.Fatal(e)
	}
	query := u.Query()
	query.Set("search_path", schema+",public")
	u.RawQuery = query.Encode()
	conn, e := sql.Open("pgx", u.String())
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	goose.SetBaseFS(migrations.Files)
	if e = goose.SetDialect("postgres"); e != nil {
		t.Fatal(e)
	}
	if e = goose.UpContext(ctx, conn, "."); e != nil {
		t.Fatal(e)
	}
	pool, e := pgxpool.New(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(pool.Close)
	return pool
}
func TestAPIContract(t *testing.T) {
	pool := integrationPool(t)
	s := testServer()
	s.Service.Repo = store.New(pool)
	s.Service.AllowAdmin = true
	s.Service.Compat = true
	s.Metrics = NewMetrics(pool)
	handler := s.Router()
	request := func(method, path, body, token string, want int) (map[string]any, *httptest.ResponseRecorder) {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s want %d got %d: %s", method, path, want, w.Code, w.Body)
		}
		var out map[string]any
		if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
			t.Fatal(e)
		}
		return out, w
	}
	signup := func(name, role string) string {
		out, _ := request("POST", "/api/v1/sign-up", fmt.Sprintf(`{"username":%q,"password":"password123","type":%q}`, name, role), "", 200)
		return out["userId"].(string)
	}
	signin := func(name string) (string, *http.Cookie) {
		out, w := request("POST", "/api/v1/sign-in", fmt.Sprintf(`{"username":%q,"password":"password123"}`, name), "", 200)
		cookies := w.Result().Cookies()
		if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteLaxMode {
			t.Fatal("insecure refresh cookie")
		}
		return out["token"].(string), cookies[0]
	}
	adminID := signup("admin", "admin")
	ownerID := signup("owner", "user")
	signup("stranger", "user")
	admin, _ := signin("admin")
	owner, cookie := signin("owner")
	stranger, _ := signin("stranger")
	request("POST", "/api/v1/sign-up", `{"username":"OWNER","password":"password123","type":"user"}`, "", 400)
	request("POST", "/api/v1/sign-in", `{"username":"owner","password":"wrong"}`, "", 403)
	s.Service.AllowAdmin = false
	request("POST", "/api/v1/sign-up", `{"username":"another-admin","password":"password123","type":"admin"}`, "", 403)
	a, _ := request("POST", "/api/v1/admin/avatar", `{"name":"Explorer","imageUrl":"https://example.com/avatar.png"}`, admin, 200)
	avatarID := a["avatarId"].(string)
	request("POST", "/api/v1/user/metadata", `{"avatarId":"missing"}`, owner, 400)
	request("POST", "/api/v1/user/metadata", fmt.Sprintf(`{"avatarId":%q}`, avatarID), owner, 200)
	me, _ := request("GET", "/api/v1/me", "", owner, 200)
	if me["id"] != ownerID || me["avatar"].(map[string]any)["id"] != avatarID {
		t.Fatal(me)
	}
	request("GET", "/api/v1/avatars", "", "", 200)
	bulk, _ := request("GET", "/api/v1/user/metadata/bulk?ids="+url.QueryEscape("["+ownerID+","+adminID+"]"), "", owner, 200)
	if len(bulk["avatars"].([]any)) != 2 {
		t.Fatal(bulk)
	}
	el, _ := request("POST", "/api/v1/admin/element", `{"width":2,"height":2,"static":true,"imageUrl":"https://example.com/wall.png"}`, admin, 200)
	elementID := el["id"].(string)
	request("PUT", "/api/v1/admin/element/missing", `{"imageUrl":"https://example.com/new.png"}`, admin, 404)
	request("PUT", "/api/v1/admin/element/"+elementID, `{"imageUrl":"https://example.com/new.png"}`, admin, 200)
	request("GET", "/api/v1/element", "", "", 200)
	request("GET", "/api/v1/elements", "", "", 200)
	mapBody := fmt.Sprintf(`{"name":"Lounge","thumbnail":"https://example.com/map.png","dimensions":"10x10","defaultElements":[{"elementId":%q,"x":1,"y":1}]}`, elementID)
	m, _ := request("POST", "/api/v1/admin/map", mapBody, admin, 200)
	mapID := m["id"].(string)
	request("GET", "/api/v1/maps", "", "", 200)
	request("POST", "/api/v1/admin/map", strings.Replace(mapBody, `"x":1`, `"x":9`, 1), admin, 400)
	request("POST", "/api/v1/admin/map", strings.Replace(mapBody, elementID, "missing", 1), admin, 400)
	var count int
	if e := pool.QueryRow(context.Background(), "SELECT count(*) FROM maps").Scan(&count); e != nil || count != 1 {
		t.Fatalf("map transaction rollback: %d %v", count, e)
	}
	request("POST", "/api/v1/space", `{"name":"Bad","mapId":"missing"}`, owner, 403)
	sp, _ := request("POST", "/api/v1/space", fmt.Sprintf(`{"name":"Home","dimensions":"1x1","mapId":%q}`, mapID), owner, 200)
	spaceID := sp["spaceId"].(string)
	path := "/api/v1/space/" + spaceID
	space, _ := request("GET", path, "", owner, 200)
	if space["dimensions"] != "10x10" || len(space["elements"].([]any)) != 1 {
		t.Fatal(space)
	}
	request("GET", path, "", stranger, 403)
	request("DELETE", path, "", stranger, 403)
	request("GET", "/api/v1/space/missing", "", owner, 400)
	s.Service.Compat = false
	request("GET", "/api/v1/space/missing", "", owner, 404)
	s.Service.Compat = true
	request("POST", "/api/v1/space", `{"name":"Blank","dimensions":"5x5"}`, owner, 200)
	list, _ := request("GET", "/api/v1/space/all", "", owner, 200)
	if len(list["spaces"].([]any)) != 2 {
		t.Fatal(list)
	}
	placementBody := fmt.Sprintf(`{"spaceId":%q,"elementId":%q,"x":8,"y":8}`, spaceID, elementID)
	request("POST", "/api/v1/space/element", strings.Replace(placementBody, `"x":8`, `"x":10`, 1), owner, 400)
	request("POST", "/api/v1/space/element", strings.Replace(placementBody, `"x":8`, `"x":9`, 1), owner, 400)
	request("POST", "/api/v1/space/element", placementBody, stranger, 403)
	p, _ := request("POST", "/api/v1/space/element", placementBody, owner, 200)
	placementID := p["id"].(string)
	request("DELETE", "/api/v1/space/element", fmt.Sprintf(`{"id":%q}`, placementID), stranger, 403)
	request("DELETE", "/api/v1/space/element", fmt.Sprintf(`{"id":%q}`, placementID), owner, 200)
	request("POST", path+"/invites", "", stranger, 403)
	invite, _ := request("POST", path+"/invites", "", owner, 200)
	accept := "/api/v1/invites/" + invite["token"].(string) + "/accept"
	request("POST", accept, "", stranger, 200)
	request("POST", accept, "", stranger, 200)
	request("GET", path, "", stranger, 200)
	request("DELETE", path, "", stranger, 403)
	request("POST", "/api/v1/invites/invalid/accept", "", stranger, 404)
	request("DELETE", path, "", owner, 200)
	if e := pool.QueryRow(context.Background(), "SELECT count(*) FROM space_elements WHERE space_id=$1", spaceID).Scan(&count); e != nil || count != 0 {
		t.Fatal("cascade failed", count, e)
	}
	if e := pool.QueryRow(context.Background(), "SELECT count(*) FROM space_events").Scan(&count); e != nil || count != 3 {
		t.Fatal("outbox missing", count, e)
	}
	// Exactly one concurrent refresh succeeds; the consumed credential cannot be replayed.
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
			r.AddCookie(cookie)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			codes <- w.Code
		}()
	}
	wg.Wait()
	close(codes)
	success := 0
	for c := range codes {
		if c == 200 {
			success++
		} else if c != 401 {
			t.Fatal(c)
		}
	}
	if success != 1 {
		t.Fatal("refresh rotation is not atomic")
	}
	_, current := signin("owner")
	r := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	r.AddCookie(current)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	r = httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
	r.AddCookie(current)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("logout did not revoke session")
	}
	salt := "0123456789abcdef0123456789abcdef"
	key, _ := scrypt.Key([]byte("password123"), []byte(salt), 16384, 8, 1, 64)
	_, e := pool.Exec(context.Background(), "UPDATE users SET password_hash=$1 WHERE id=$2", salt+"."+hex.EncodeToString(key), ownerID)
	if e != nil {
		t.Fatal(e)
	}
	signin("owner")
	var hash string
	if e = pool.QueryRow(context.Background(), "SELECT password_hash FROM users WHERE id=$1", ownerID).Scan(&hash); e != nil || !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatal("legacy hash not migrated", e)
	}
	metrics := httptest.NewRecorder()
	handler.ServeHTTP(metrics, httptest.NewRequest("GET", "/metrics", nil))
	if !strings.Contains(metrics.Body.String(), "http_requests_total") || !strings.Contains(metrics.Body.String(), "db_pool_total_connections") {
		t.Fatal(metrics.Body)
	}
	// The raw refresh credential must never appear in the database.
	var stored int
	if e = pool.QueryRow(context.Background(), "SELECT count(*) FROM refresh_tokens WHERE token_hash=$1", auth.Digest(current.Value)).Scan(&stored); e != nil || stored != 1 {
		t.Fatal("refresh not hashed", e)
	}
}
