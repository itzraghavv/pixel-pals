package realtime

import (
	"context"
	"testing"
	"time"

	"github.com/itzraghavv/pixel-pals/backend/internal/store/db"
)

func TestRoomMovementAndLeave(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r := newRoom(true)
	r.grid, _ = newGrid(db.Space{Width: 3, Height: 1}, []db.GetSpaceElementsRow{
		{X: 2, Width: 1, Height: 1, Static: true},
	})
	go r.run(ctx, time.Minute)
	a, b := newClient(), newClient()
	a.UserID, b.UserID = "a", "b"
	call := func(fn func()) {
		t.Helper()
		if !r.call(ctx, fn) {
			t.Fatal("room unavailable")
		}
	}
	call(func() {
		if err := r.join(a); err != nil {
			t.Error(err)
		}
		if err := r.join(b); err != nil {
			t.Error(err)
		}
		p := r.players[a.UserID]
		r.move(a, point{2, 0})
		r.move(a, point{1 - p.position.X, 0})
		r.leave(a)
		r.leave(a)
	})
	expectTypes(t, a, "space-joined", "user-joined", "movement-rejected")
	expectTypes(t, b, "space-joined", "movement", "user-left")
}

func expectTypes(t *testing.T, c *client, types ...string) {
	t.Helper()
	for _, want := range types {
		select {
		case got := <-c.out:
			if got.Type != want {
				t.Fatalf("got %q, want %q", got.Type, want)
			}
		default:
			t.Fatalf("missing %q", want)
		}
	}
	if len(c.out) != 0 {
		t.Fatalf("unexpected message: %v", <-c.out)
	}
}

func TestRoomEvictsSlowConsumer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r := newRoom(false)
	r.grid, _ = newGrid(db.Space{Width: 3, Height: 1}, nil)
	a, b, slow := newClient(), newClient(), newClient()
	a.UserID, b.UserID, slow.UserID = "a", "b", "slow"
	r.players[a.UserID] = participant{client: a, position: point{0, 0}}
	r.players[b.UserID] = participant{client: b, position: point{2, 0}}
	r.players[slow.UserID] = participant{client: slow, position: point{2, 0}}
	for range cap(slow.out) {
		slow.out <- delivery{}
	}
	go r.run(ctx, time.Minute)
	if !r.call(ctx, func() { r.move(a, point{1, 0}); r.leave(slow) }) {
		t.Fatal("room unavailable")
	}
	select {
	case <-slow.stopped:
		if slow.reason.code != closeSlow {
			t.Fatalf("close code = %d", slow.reason.code)
		}
	default:
		t.Fatal("slow consumer remained connected")
	}
	expectTypes(t, b, "movement", "user-left")
}
