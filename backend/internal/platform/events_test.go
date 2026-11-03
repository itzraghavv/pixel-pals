//go:build integration

package platform

import (
	"context"
	"encoding/json"
	"github.com/itzraghavv/pixel-pals/backend/internal/service"
	"github.com/itzraghavv/pixel-pals/backend/internal/store/db"
	"github.com/redis/go-redis/v9"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

type eventRepo struct {
	db.Querier
	mu     sync.Mutex
	events []db.SpaceEvent
}

func (e *eventRepo) PendingEvents(context.Context) ([]db.SpaceEvent, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]db.SpaceEvent{}, e.events...), nil
}
func (e *eventRepo) DeleteEvent(_ context.Context, id int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, v := range e.events {
		if v.ID == id {
			e.events = append(e.events[:i], e.events[i+1:]...)
			break
		}
	}
	return nil
}
func TestRedisDeliveryAndRateLimit(t *testing.T) {
	raw := os.Getenv("TEST_REDIS_URL")
	if raw == "" {
		t.Skip("set TEST_REDIS_URL for Redis integration tests")
	}
	o, e := redis.ParseURL(raw)
	if e != nil {
		t.Fatal(e)
	}
	client := redis.NewClient(o)
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	sub := client.Subscribe(ctx, "space.updated")
	defer sub.Close()
	if _, e = sub.Receive(ctx); e != nil {
		t.Fatal(e)
	}
	repo := &eventRepo{events: []db.SpaceEvent{{ID: 1, SpaceID: "delivery-test"}}}
	done := make(chan struct{})
	go func() {
		defer close(done)
		DeliverEvents(ctx, repo, client, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	}()
	message, e := sub.ReceiveMessage(ctx)
	if e != nil {
		t.Fatal(e)
	}
	var event map[string]any
	if e = json.Unmarshal([]byte(message.Payload), &event); e != nil || event["spaceId"] != "delivery-test" || event["type"] != "space.updated" {
		t.Fatal(message, e)
	}
	cancel()
	<-done
	rows, _ := repo.PendingEvents(context.Background())
	if len(rows) != 0 {
		t.Fatal("delivered event not acknowledged")
	}
	limiter := LimitSignIn(client)
	key := service.ID()
	for i := 0; i < 10; i++ {
		allowed, e := limiter(context.Background(), key)
		if e != nil || !allowed {
			t.Fatalf("attempt %d: %v", i, e)
		}
	}
	allowed, e := limiter(context.Background(), key)
	if e != nil || allowed {
		t.Fatal("shared rate limit failed", e)
	}
}
