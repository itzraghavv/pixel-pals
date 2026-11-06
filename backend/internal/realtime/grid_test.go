package realtime

import (
	"testing"

	"github.com/itzraghavv/pixel-pals/backend/internal/store/db"
)

func TestGridSpawnAndFootprints(t *testing.T) {
	g, err := newGrid(db.Space{Width: 3, Height: 2}, []db.GetSpaceElementsRow{
		{Width: 2, Height: 2, Static: true}, {X: 2, Width: 1, Height: 1, Static: true},
		{X: 2, Y: 1, Width: 1, Height: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	onlyFree := point{2, 1}
	for range 100 {
		if p, ok := g.spawn(func(point) bool { return false }); !ok || p != onlyFree {
			t.Fatalf("spawn = %v, %v", p, ok)
		}
	}
	if _, ok := g.spawn(func(p point) bool { return p == onlyFree }); ok {
		t.Fatal("occupied tile was used")
	}
}
