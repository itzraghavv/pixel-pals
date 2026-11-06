package realtime

import (
	"slices"
	"sort"
)

type proximity struct {
	GroupID string   `json:"groupId"`
	Peers   []string `json:"peers"`
}

func (r *room) updateProximity() {
	ids := make([]string, 0, len(r.players))
	for id := range r.players {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	parents := make([]int, len(ids))
	positions := make([]point, len(ids))
	areas := make([]string, len(ids))
	for i, id := range ids {
		parents[i] = i
		positions[i] = r.players[id].position
		areas[i] = r.grid.areas[positions[i]]
	}
	root := func(i int) int {
		for parents[i] != i {
			parents[i] = parents[parents[i]]
			i = parents[i]
		}
		return i
	}
	for i, pos := range positions {
		for j := range i {
			if areas[i] != areas[j] {
				continue
			}
			other := positions[j]
			if areas[i] == "" && (abs(pos.X-other.X) > r.proximityRadius || abs(pos.Y-other.Y) > r.proximityRadius) {
				continue
			}
			a, b := root(i), root(j)
			parents[max(a, b)] = min(a, b)
		}
	}
	groups := make(map[int][]string)
	for i, id := range ids {
		parent := root(i)
		groups[parent] = append(groups[parent], id)
	}
	for i, id := range ids {
		parent := root(i)
		members := groups[parent]
		current := proximity{}
		if areas[i] != "" {
			current.GroupID = "area:" + areas[i]
		} else if len(members) > 1 {
			current.GroupID = "near:" + members[0]
		}
		player, exists := r.players[id]
		if !exists {
			continue
		}
		self, _ := slices.BinarySearch(members, id)
		previous := player.proximity
		if current.GroupID == previous.GroupID && len(previous.Peers) == len(members)-1 &&
			slices.Equal(previous.Peers[:self], members[:self]) &&
			slices.Equal(previous.Peers[self:], members[self+1:]) {
			continue
		}
		if len(members) > 1 {
			current.Peers = make([]string, 0, len(members)-1)
			for _, peer := range members {
				if peer != id {
					current.Peers = append(current.Peers, peer)
				}
			}
		}
		player.proximity = current
		r.players[id] = player
		if current.Peers == nil {
			current.Peers = []string{}
		}
		if !r.send(player.client, message{"proximity", current}) {
			r.leave(player.client)
		}
	}
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
