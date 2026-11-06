package realtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/itzraghavv/pixel-pals/backend/internal/platform"
	"github.com/itzraghavv/pixel-pals/backend/internal/service"
	"github.com/redis/go-redis/v9"
)

// WatchSpaceUpdates consumes committed layout changes. Redis sends a new subscription
// confirmation after reconnect, so refresh local rooms to cover missed notifications.
func (s *Server) WatchSpaceUpdates(sub *redis.PubSub) {
	events := sub.ChannelWithSubscriptions()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	pending := make(map[string]struct{})
	refresh := func(id string) {
		if err := s.refreshSpace(id); err != nil {
			pending[id] = struct{}{}
			if s.ctx.Err() == nil {
				s.log.Error("reload realtime space", "space_id", id, "error", err)
			}
		} else {
			delete(pending, id)
		}
	}
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			for id := range pending {
				refresh(id)
			}
		case event, ok := <-events:
			if !ok {
				return
			}
			switch event := event.(type) {
			case *redis.Subscription:
				if event.Kind != "subscribe" || event.Channel != platform.SpaceUpdatesChannel {
					continue
				}
				s.mu.Lock()
				ids := make([]string, 0, len(s.rooms))
				for id := range s.rooms {
					ids = append(ids, id)
				}
				s.mu.Unlock()
				for _, id := range ids {
					refresh(id)
				}
			case *redis.Message:
				var update platform.SpaceUpdated
				if json.Unmarshal([]byte(event.Payload), &update) == nil &&
					update.Type == platform.SpaceUpdatesChannel && update.SpaceID != "" {
					refresh(update.SpaceID)
				}
			}
		}
	}
}

func (s *Server) refreshSpace(id string) error {
	s.mu.Lock()
	r := s.rooms[id]
	s.mu.Unlock()
	if r == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
	defer cancel()
	var err error
	if !r.call(ctx, func() {
		space, loadErr := s.repo.GetSpace(ctx, id)
		if service.Missing(loadErr) {
			r.grid = grid{}
			for _, p := range r.players {
				p.client.stop(closeReason{closeMissing, "Space deleted"})
			}
			clear(r.players)
			return
		}
		if loadErr != nil {
			err = loadErr
			return
		}
		err = s.loadGrid(ctx, r, space)
		if err == nil {
			r.layoutChanged()
		}
	}) {
		return ctx.Err()
	}
	return err
}

func (r *room) layoutChanged() {
	r.broadcast("", message{"space-updated", struct{}{}})
	for id, player := range r.players {
		if r.grid.free(player.position) {
			continue
		}
		target, ok := r.spawn(id)
		if !ok {
			player.client.stop(closeReason{1013, "Space has no free tile"})
			r.leave(player.client)
			continue
		}
		if !r.send(player.client, message{"movement-rejected", target}) {
			r.leave(player.client)
			continue
		}
		player.position = target
		r.players[id] = player
		r.broadcastMovement(player, id)
	}
	r.proximityDirty = true
}
