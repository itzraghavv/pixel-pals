//go:build integration

package realtime

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/itzraghavv/pixel-pals/backend/internal/platform"
	"github.com/redis/go-redis/v9"
)

func TestRedisSpaceUpdates(t *testing.T) {
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL is required")
	}
	options, err := redis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(options)
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sub := client.Subscribe(ctx, platform.SpaceUpdatesChannel)
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatal(err)
	}

	repo := &updatingRepo{}
	server, wsURL, tokens := startWithRepo(t, repo)
	done := make(chan struct{})
	go func() {
		defer close(done)
		server.WatchSpaceUpdates(sub)
	}()
	defer func() {
		server.Shutdown()
		<-done
	}()

	conn, spawn := joinSocket(t, wsURL, issue(t, tokens, "a"))
	repo.wall.Store(int32(spawn.X) + 1)
	payload, err := json.Marshal(platform.SpaceUpdated{Type: platform.SpaceUpdatesChannel, SpaceID: "room", EventID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Publish(ctx, platform.SpaceUpdatesChannel, payload).Err(); err != nil {
		t.Fatal(err)
	}
	read(t, conn, "space-updated")
	read(t, conn, "movement-rejected")
}
