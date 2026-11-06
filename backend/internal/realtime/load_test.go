//go:build load

package realtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/itzraghavv/pixel-pals/backend/internal/auth"
	"github.com/itzraghavv/pixel-pals/backend/internal/store/db"
)

const (
	botsPerRoom = 50
	movesPerBot = 20
	moveEvery   = 250 * time.Millisecond
)

type loadRepo struct{ fixtureRepo }

func (loadRepo) GetSpace(_ context.Context, id string) (db.Space, error) {
	return db.Space{ID: id, Width: 2, Height: 1}, nil
}
func (loadRepo) IsMember(context.Context, db.IsMemberParams) (bool, error) { return true, nil }

type loadBot struct {
	id   string
	conn *websocket.Conn
	at   point
	sent chan time.Time
}

func TestRealtimeLoad(t *testing.T) {
	t.Run("single", func(t *testing.T) { runLoad(t, 1, 1) })
	t.Run("two-nodes", func(t *testing.T) { runLoad(t, 2, 20) })
}

func runLoad(t *testing.T, nodes, rooms int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	tokens := auth.Tokens{Secret: []byte("realtime-load-only-signing-secret")}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	upstreams := make([]*httptest.Server, nodes)
	for i := range upstreams {
		server := NewServer(loadRepo{}, tokens, Options{AllowOverlap: true, ProximityRadius: 3}, log)
		upstream := httptest.NewServer(server)
		defer upstream.Close()
		defer server.Shutdown()
		upstreams[i] = upstream
	}
	entry := upstreams[0].URL
	if nodes > 1 {
		proxies := make([]*httputil.ReverseProxy, nodes)
		for i, upstream := range upstreams {
			target, err := url.Parse(upstream.URL)
			if err != nil {
				t.Fatal(err)
			}
			proxies[i] = httputil.NewSingleHostReverseProxy(target)
		}
		proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			room, err := strconv.Atoi(strings.TrimPrefix(r.URL.Query().Get("spaceId"), "load-"))
			if err != nil || room < 0 || room >= rooms {
				http.Error(w, "Invalid routing space", http.StatusBadRequest)
				return
			}
			proxies[room%nodes].ServeHTTP(w, r)
		}))
		defer proxy.Close()
		entry = proxy.URL
	}

	wsURL := "ws" + strings.TrimPrefix(entry, "http") + "/ws?spaceId="
	all := make([]*loadBot, 0, rooms*botsPerRoom)
	var workers sync.WaitGroup
	defer func() {
		cancel()
		for _, bot := range all {
			_ = bot.conn.Close()
		}
		workers.Wait()
	}()
	failures := make(chan error, 1)
	fail := func(err error) {
		if ctx.Err() == nil {
			select {
			case failures <- err:
			default:
			}
		}
	}
	done := make(chan struct{}, rooms*botsPerRoom)
	samples := make(chan time.Duration, rooms*(botsPerRoom-1)*movesPerBot)
	for room := range rooms {
		space := fmt.Sprintf("load-%d", room)
		bots := make(map[string]*loadBot, botsPerRoom)
		for i := range botsPerRoom {
			id := fmt.Sprintf("bot-%02d", i)
			bots[id] = &loadBot{id: id, sent: make(chan time.Time, movesPerBot)}
		}
		for i := range botsPerRoom {
			id := fmt.Sprintf("bot-%02d", i)
			bot := bots[id]
			conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL+space, nil)
			if err != nil {
				t.Fatal(err)
			}
			bot.conn = conn
			all = append(all, bot)
			deadline, _ := ctx.Deadline()
			_ = conn.SetReadDeadline(deadline)
			send(t, conn, "join", map[string]string{"spaceId": space, "token": issue(t, tokens, id)})
			var joined struct {
				Self  point
				Users []presence
			}
			if err := json.Unmarshal(read(t, conn, "space-joined"), &joined); err != nil || len(joined.Users) != i {
				t.Fatalf("%s %s joined with %d peers, want %d (%v)", space, id, len(joined.Users), i, err)
			}
			bot.at = joined.Self
			workers.Go(func() {
				if err := bot.readLoad(bots, i == 0, done, samples); err != nil {
					fail(fmt.Errorf("%s %s: %w", space, id, err))
				}
			})
		}
	}

	start := time.Now().Add(time.Second)
	for i, bot := range all {
		// Stagger moves across the test to sustain four moves/s without burst bias.
		slot := (i%botsPerRoom)*rooms + i/botsPerRoom
		offset := time.Duration(slot) * moveEvery / time.Duration(len(all))
		workers.Go(func() {
			if err := bot.moveLoad(ctx, start.Add(offset), i%botsPerRoom == 0); err != nil {
				fail(fmt.Errorf("%s: %w", bot.id, err))
			}
		})
	}

	for range all {
		select {
		case <-done:
		case err := <-failures:
			t.Fatal(err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	latencies := make([]time.Duration, 0, cap(samples))
	for range cap(samples) {
		select {
		case sample := <-samples:
			latencies = append(latencies, sample)
		default:
			t.Fatalf("got %d latency samples, want %d", len(latencies), cap(samples))
		}
	}
	slices.Sort(latencies)
	p95 := latencies[(len(latencies)*95+99)/100-1]
	t.Logf("%d bots, %d rooms, %d realtime instances, %d movements, %d broadcasts delivered, p95 %s",
		len(all), rooms, nodes, len(all)*movesPerBot, len(all)*movesPerBot*(botsPerRoom-1), p95)
	if nodes == 1 && p95 >= 50*time.Millisecond {
		t.Fatalf("p95 %s exceeds the 50ms target", p95)
	}
}

func (b *loadBot) moveLoad(ctx context.Context, start time.Time, observer bool) error {
	next := b.at
	for step := range movesPerBot {
		timer := time.NewTimer(time.Until(start.Add(time.Duration(step) * moveEvery)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		next.X = 1 - next.X
		_ = b.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if !observer {
			b.sent <- time.Now()
		}
		if err := b.conn.WriteJSON(message{"move", next}); err != nil {
			return err
		}
	}
	return nil
}

func (b *loadBot) readLoad(bots map[string]*loadBot, observer bool, done chan<- struct{}, samples chan<- time.Duration) error {
	received := 0
	for {
		_, body, err := b.conn.ReadMessage()
		if err != nil {
			return err
		}
		if bytes.HasPrefix(body, []byte("{\"type\":\"movement-rejected\"")) {
			return fmt.Errorf("unexpected rejection: %s", body)
		}
		if !bytes.HasPrefix(body, []byte("{\"type\":\"movement\"")) {
			continue
		}
		if observer {
			var movement struct{ Payload struct{ UserID string } }
			if err := json.Unmarshal(body, &movement); err != nil {
				return err
			}
			sender := bots[movement.Payload.UserID]
			if sender == nil {
				return fmt.Errorf("unknown sender %s", movement.Payload.UserID)
			}
			select {
			case sent := <-sender.sent:
				samples <- time.Since(sent)
			default:
				return fmt.Errorf("movement without send timestamp: %s", movement.Payload.UserID)
			}
		}
		received++
		if received == (botsPerRoom-1)*movesPerBot {
			done <- struct{}{}
		}
	}
}
