package realtime

import (
	"fmt"
	"math/rand/v2"

	"github.com/itzraghavv/pixel-pals/backend/internal/store/db"
)

type point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type grid struct {
	width, height int
	blocked       []bool
}

func newGrid(space db.Space, placements []db.GetSpaceElementsRow) (grid, error) {
	if space.Width < 1 || space.Height < 1 || space.Width > 1000 || space.Height > 1000 {
		return grid{}, fmt.Errorf("invalid space dimensions: %dx%d", space.Width, space.Height)
	}
	g := grid{int(space.Width), int(space.Height), make([]bool, int(space.Width)*int(space.Height))}
	for _, p := range placements {
		if !p.Static {
			continue
		}
		for y := max(0, int(p.Y)); y < min(g.height, int(p.Y)+int(p.Height)); y++ {
			for x := max(0, int(p.X)); x < min(g.width, int(p.X)+int(p.Width)); x++ {
				g.blocked[y*g.width+x] = true
			}
		}
	}
	return g, nil
}

func (g grid) free(p point) bool {
	return p.X >= 0 && p.X < g.width && p.Y >= 0 && p.Y < g.height && !g.blocked[p.Y*g.width+p.X]
}

func (g grid) spawn(occupied func(point) bool) (point, bool) {
	at := func(i int) point { return point{i % g.width, i / g.width} }
	for range 32 {
		i := rand.IntN(len(g.blocked))
		if !g.blocked[i] && !occupied(at(i)) {
			return at(i), true
		}
	}
	// Dense layouts need a bounded fallback. Reservoir sampling stays uniform.
	var chosen point
	count := 0
	for i, blocked := range g.blocked {
		if blocked || occupied(at(i)) {
			continue
		}
		count++
		if rand.IntN(count) == 0 {
			chosen = at(i)
		}
	}
	return chosen, count > 0
}

func direction(from, to point) string {
	switch {
	case to.Y == from.Y && to.X == from.X+1:
		return "right"
	case to.Y == from.Y && to.X == from.X-1:
		return "left"
	case to.X == from.X && to.Y == from.Y+1:
		return "down"
	case to.X == from.X && to.Y == from.Y-1:
		return "up"
	default:
		return ""
	}
}
