package realtime

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/itzraghavv/pixel-pals/backend/internal/store/db"
)

func TestProximityComponentsAndPrivateAreas(t *testing.T) {
	r := newRoom(false)
	r.proximityRadius = 2
	r.grid, _ = newGrid(db.Space{Width: 10, Height: 2}, nil)
	r.grid.areas = map[point]string{{2, 0}: "quiet", {9, 0}: "quiet"}
	a, b, c := newClient(), newClient(), newClient()
	a.UserID, b.UserID, c.UserID = "a", "b", "c"
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go r.run(ctx, time.Minute)
	call := func(fn func()) {
		t.Helper()
		if !r.call(ctx, fn) {
			t.Fatal("room unavailable")
		}
	}
	call(func() {
		r.players["a"] = participant{client: a, position: point{0, 1}}
		r.players["b"] = participant{client: b, position: point{2, 1}}
		r.players["c"] = participant{client: c, position: point{4, 1}}
		r.proximityDirty = true
	})
	expectGroup(t, a, "near:a", "b", "c")
	expectGroup(t, b, "near:a", "a", "c")
	expectGroup(t, c, "near:a", "a", "b")

	call(func() { r.proximityDirty = true })
	expectTypes(t, a)
	expectTypes(t, b)
	expectTypes(t, c)

	call(func() {
		p := r.players["b"]
		p.position = point{2, 0}
		r.players["b"] = p
		p = r.players["c"]
		p.position = point{9, 0}
		r.players["c"] = p
		r.proximityDirty = true
	})
	expectGroup(t, a, "")
	expectGroup(t, b, "area:quiet", "c")
	expectGroup(t, c, "area:quiet", "b")
}

func expectGroup(t *testing.T, c *client, group string, peers ...string) {
	t.Helper()
	select {
	case msg := <-c.out:
		payload, ok := msg.Payload.(proximity)
		if peers == nil {
			peers = []string{}
		}
		if msg.Type != "proximity" || !ok || !reflect.DeepEqual(payload, proximity{group, peers}) {
			t.Fatalf("unexpected proximity for %s: %+v", c.UserID, msg)
		}
	case <-time.After(time.Second):
		t.Fatalf("missing proximity for %s", c.UserID)
	}
}
