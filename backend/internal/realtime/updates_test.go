package realtime

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/itzraghavv/pixel-pals/backend/internal/store/db"
	"github.com/jackc/pgx/v5"
)

type updatingRepo struct {
	fixtureRepo
	deleted atomic.Bool
	wall    atomic.Int32
}

func (r *updatingRepo) GetSpace(ctx context.Context, id string) (db.Space, error) {
	if r.deleted.Load() {
		return db.Space{}, pgx.ErrNoRows
	}
	return r.fixtureRepo.GetSpace(ctx, id)
}

func (r *updatingRepo) GetSpaceElements(context.Context, string) ([]db.GetSpaceElementsRow, error) {
	if x := r.wall.Load(); x > 0 {
		return []db.GetSpaceElementsRow{{X: x - 1, Width: 1, Height: 1, Static: true}}, nil
	}
	return nil, nil
}

func TestSpaceRefreshMovesBlockedPlayersAndClosesDeletedSpace(t *testing.T) {
	repo := &updatingRepo{}
	s, url, tokens := startWithRepo(t, repo)
	a, spawn := joinSocket(t, url, issue(t, tokens, "a"))

	repo.wall.Store(int32(spawn.X) + 1)
	if err := s.refreshSpace("room"); err != nil {
		t.Fatal(err)
	}
	read(t, a, "space-updated")
	var p point
	if err := json.Unmarshal(read(t, a, "movement-rejected"), &p); err != nil || p != (point{1 - spawn.X, 0}) {
		t.Fatalf("relocation = %v (%v)", p, err)
	}
	send(t, a, "move", spawn)
	read(t, a, "movement-rejected")

	repo.deleted.Store(true)
	if err := s.refreshSpace("room"); err != nil {
		t.Fatal(err)
	}
	read(t, a, "error")
	if _, _, err := a.ReadMessage(); !websocket.IsCloseError(err, closeMissing) {
		t.Fatalf("deletion close = %v", err)
	}
}

func TestShutdownClosesUpgradedSessions(t *testing.T) {
	s, url, tokens := startFixture(t)
	a, _ := joinSocket(t, url, issue(t, tokens, "a"))
	pending := dial(t, url)
	s.Shutdown()
	for _, conn := range []*websocket.Conn{a, pending} {
		var payload struct{ Code string }
		if err := json.Unmarshal(read(t, conn, "error"), &payload); err != nil || payload.Code != "server-restart" {
			t.Fatalf("shutdown payload: %+v (%v)", payload, err)
		}
		if _, _, err := conn.ReadMessage(); !websocket.IsCloseError(err, 1012) {
			t.Fatalf("restart close = %v", err)
		}
	}
}
