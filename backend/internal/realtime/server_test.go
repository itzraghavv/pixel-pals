package realtime

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/itzraghavv/pixel-pals/backend/internal/auth"
	"github.com/itzraghavv/pixel-pals/backend/internal/store/db"
	"github.com/jackc/pgx/v5"
)

type fixtureRepo struct{}

func (fixtureRepo) GetSpace(_ context.Context, id string) (db.Space, error) {
	if id != "room" {
		return db.Space{}, pgx.ErrNoRows
	}
	return db.Space{ID: id, CreatorID: "a", Width: 2, Height: 1}, nil
}

func (fixtureRepo) GetSpaceElements(context.Context, string) ([]db.GetSpaceElementsRow, error) {
	return nil, nil
}

func (fixtureRepo) GetSpaceAreaTiles(context.Context, string) ([]db.GetSpaceAreaTilesRow, error) {
	return nil, nil
}

func (fixtureRepo) IsMember(_ context.Context, p db.IsMemberParams) (bool, error) {
	return p.UserID == "b", nil
}

func (fixtureRepo) UserByID(_ context.Context, id string) (db.User, error) {
	return db.User{ID: id, Username: "player-" + id}, nil
}

func (fixtureRepo) BulkMetadata(context.Context, []string) ([]db.BulkMetadataRow, error) {
	return nil, nil
}

func startFixture(t *testing.T) (*Server, string, auth.Tokens) {
	t.Helper()
	tokens := auth.Tokens{Secret: []byte("test-realtime-secret-with-32-bytes")}
	s := NewServer(fixtureRepo{}, tokens, Options{Origins: []string{"http://localhost:3000"}, Compat: true, AllowOverlap: true}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	httpServer := httptest.NewServer(s)
	t.Cleanup(httpServer.Close)
	t.Cleanup(s.Shutdown)
	return s, "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws", tokens
}

func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	return c
}

func send(t *testing.T, c *websocket.Conn, kind string, payload any) {
	t.Helper()
	if err := c.WriteJSON(message{kind, payload}); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, c *websocket.Conn, kind string) json.RawMessage {
	t.Helper()
	var msg inbound
	for {
		if err := c.ReadJSON(&msg); err != nil {
			t.Fatal(err)
		}
		if msg.Type != "proximity" || kind == "proximity" {
			break
		}
	}
	if msg.Type != kind {
		t.Fatalf("got %q, want %q", msg.Type, kind)
	}
	return msg.Payload
}

func issue(t *testing.T, tokens auth.Tokens, id string) string {
	t.Helper()
	token, err := tokens.Issue(id, "user")
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func joinSocket(t *testing.T, url, token string) (*websocket.Conn, point) {
	t.Helper()
	c := dial(t, url+"?spaceId=room")
	send(t, c, "join", map[string]string{"spaceId": "room", "token": token})
	var joined struct {
		Self  presence
		Spawn *point
		Users []presence
	}
	if err := json.Unmarshal(read(t, c, "space-joined"), &joined); err != nil ||
		joined.Self.UserID == "" || joined.Spawn == nil || *joined.Spawn != joined.Self.point || joined.Users == nil {
		t.Fatalf("invalid join response: %+v (%v)", joined, err)
	}
	return c, *joined.Spawn
}

func TestWebSocketMovement(t *testing.T) {
	_, url, tokens := startFixture(t)
	a, spawn := joinSocket(t, url, issue(t, tokens, "a"))
	b, _ := joinSocket(t, url, issue(t, tokens, "b"))
	read(t, a, "user-joined")
	if err := a.WriteMessage(websocket.TextMessage, []byte("{")); err != nil {
		t.Fatal(err)
	}
	send(t, a, "unknown", map[string]string{})
	send(t, a, "ping", map[string]int{"t": 123})
	if got := string(read(t, a, "pong")); got != "{\"t\":123}" {
		t.Fatalf("pong payload %s", got)
	}
	send(t, a, "move", point{10, 10})
	if got := read(t, a, "movement-rejected"); !strings.Contains(string(got), "\"x\":") {
		t.Fatalf("rejection %s", got)
	}
	send(t, a, "move", point{1 - spawn.X, 0})
	var moved struct {
		UserID string
		point
		Direction string `json:"dir"`
	}
	if err := json.Unmarshal(read(t, b, "movement"), &moved); err != nil ||
		moved.UserID != "a" || moved.X != 1-spawn.X || moved.Direction == "" {
		t.Fatalf("invalid movement: %+v (%v)", moved, err)
	}
	_ = a.Close()
	read(t, b, "user-left")
	send(t, b, "ping", map[string]int{"t": 0})
	read(t, b, "pong") // no second user-left
}

func TestWebSocketJoinFailures(t *testing.T) {
	_, url, tokens := startFixture(t)
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, auth.Claims{
		Role: "user", RegisteredClaims: jwt.RegisteredClaims{
			Subject: "a", IssuedAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		},
	}).SignedString(tokens.Secret)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, query, space, token, kind string
		code                            int
	}{
		{"first-message", "", "room", "", "move", closeJoin},
		{"expired", "", "room", expired, "join", closeAccess},
		{"non-member", "", "room", issue(t, tokens, "c"), "join", closeAccess},
		{"missing-space", "", "missing", issue(t, tokens, "a"), "join", closeMissing},
		{"wrong-route", "?spaceId=other", "room", issue(t, tokens, "a"), "join", closeAccess},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := dial(t, url+tc.query)
			send(t, c, tc.kind, map[string]string{"spaceId": tc.space, "token": tc.token})
			read(t, c, "error")
			_, _, err := c.ReadMessage()
			if !websocket.IsCloseError(err, tc.code) {
				t.Fatalf("close = %v, want %d", err, tc.code)
			}
		})
	}
	conn, response, err := websocket.DefaultDialer.Dial(url, http.Header{"Origin": {"https://invalid.example"}})
	if conn != nil {
		_ = conn.Close()
	}
	if err == nil || response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin handshake: %v, %v", response, err)
	}
}
