package platform

import (
	"context"
	"encoding/json"
	"github.com/itzraghavv/pixel-pals/backend/internal/store/db"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"time"
)

const SpaceUpdatesChannel = "space.updated"

type SpaceUpdated struct {
	Type    string `json:"type"`
	SpaceID string `json:"spaceId"`
	EventID int64  `json:"eventId"`
}

// DeliverEvents provides at-least-once delivery. Consumers reload idempotently.
// API replicas may publish duplicate invalidations; only acknowledged rows are removed.
func DeliverEvents(ctx context.Context, q db.Querier, r *redis.Client, log *slog.Logger) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			work, cancel := context.WithTimeout(ctx, 5*time.Second)
			rows, e := q.PendingEvents(work)
			if e == nil {
				for _, event := range rows {
					body, _ := json.Marshal(SpaceUpdated{Type: SpaceUpdatesChannel, SpaceID: event.SpaceID, EventID: event.ID})
					if e = r.Publish(work, SpaceUpdatesChannel, body).Err(); e != nil {
						break
					}
					if e = q.DeleteEvent(work, event.ID); e != nil {
						break
					}
				}
			}
			cancel()
			if e != nil && ctx.Err() == nil {
				log.Error("outbox delivery failed", "error", e)
			}
		}
	}
}
