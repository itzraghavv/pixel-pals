//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/itzraghavv/pixel-pals/backend/internal/auth"
	"github.com/itzraghavv/pixel-pals/backend/internal/platform"
	"github.com/itzraghavv/pixel-pals/backend/internal/realtime"
	"github.com/itzraghavv/pixel-pals/backend/internal/service"
	"github.com/itzraghavv/pixel-pals/backend/internal/store"
	"github.com/itzraghavv/pixel-pals/backend/internal/store/db"
	"github.com/redis/go-redis/v9"
)

func TestRealtimeOutboxContract(t *testing.T) {
	raw := os.Getenv("TEST_REDIS_URL")
	if raw == "" {
		t.Skip("TEST_REDIS_URL is required")
	}
	options, err := redis.ParseURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	repo := store.New(integrationPool(t))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	redisClient := redis.NewClient(options)
	defer redisClient.Close()
	sub := redisClient.Subscribe(ctx, platform.SpaceUpdatesChannel)
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatal(err)
	}

	const owner = "realtime-owner"
	if err := repo.CreateUser(ctx, db.CreateUserParams{ID: owner, Username: owner, PasswordHash: "fixture", Role: "user"}); err != nil {
		t.Fatal(err)
	}
	tokens := auth.Tokens{Secret: []byte("integration-realtime-secret-at-least-32-bytes")}
	svc := service.Service{Repo: repo, Tokens: tokens}
	id, err := svc.CreateSpace(ctx, owner, service.SpaceInput{Name: "Realtime", Dimensions: "2x1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool.Exec(ctx, "INSERT INTO space_area_tiles (space_id,x,y,area_id) VALUES ($1,0,0,'quiet'), ($1,1,0,'quiet')", id); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateElement(ctx, db.CreateElementParams{ID: "wall", Width: 1, Height: 1, Static: true}); err != nil {
		t.Fatal(err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ws := realtime.NewServer(repo, tokens, realtime.Options{AllowOverlap: true, ProximityRadius: 3}, log)
	httpServer := httptest.NewServer(ws)
	defer httpServer.Close()
	watchDone := make(chan struct{})
	go func() { defer close(watchDone); ws.WatchSpaceUpdates(sub) }()
	defer func() { ws.Shutdown(); <-watchDone }()

	token, err := tokens.Issue(owner, "user")
	if err != nil {
		t.Fatal(err)
	}
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(httpServer.URL, "http")+"/ws?spaceId="+id, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(8 * time.Second))
	if err := conn.WriteJSON(map[string]any{"type": "join", "payload": map[string]string{"spaceId": id, "token": token}}); err != nil {
		t.Fatal(err)
	}
	read := func(kind string, payload any) {
		t.Helper()
		var msg struct {
			Type    string
			Payload json.RawMessage
		}
		if err := conn.ReadJSON(&msg); err != nil || msg.Type != kind {
			t.Fatalf("want %s, got %s (%v)", kind, msg.Type, err)
		}
		if err := json.Unmarshal(msg.Payload, payload); err != nil {
			t.Fatal(err)
		}
	}
	var joined struct{ Self struct{ X, Y int32 } }
	read("space-joined", &joined)
	var group struct{ GroupID string }
	read("proximity", &group)
	if group.GroupID != "area:quiet" {
		t.Fatalf("group = %q", group.GroupID)
	}

	deliverCtx, stop := context.WithCancel(ctx)
	deliverDone := make(chan struct{})
	go func() { defer close(deliverDone); platform.DeliverEvents(deliverCtx, repo, redisClient, log) }()
	defer func() { stop(); <-deliverDone }()
	if _, err := svc.AddPlacement(ctx, owner, service.AddPlacementInput{SpaceID: id, ElementID: "wall", X: &joined.Self.X, Y: &joined.Self.Y}); err != nil {
		t.Fatal(err)
	}
	var update map[string]any
	read("space-updated", &update)
	var corrected struct{ X, Y int32 }
	read("movement-rejected", &corrected)
	if corrected.X == joined.Self.X || corrected.Y != 0 {
		t.Fatalf("relocation = %+v from %+v", corrected, joined.Self)
	}
}
