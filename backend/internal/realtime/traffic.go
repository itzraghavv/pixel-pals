package realtime

import "time"

// Each token is represented by its refill interval. The bucket is owned by the read pump.
type bucket struct {
	at       time.Time
	credit   time.Duration
	interval time.Duration
	capacity time.Duration
}

func newBucket(rate, burst int) bucket {
	interval := time.Second / time.Duration(rate)
	capacity := interval * time.Duration(burst)
	return bucket{at: time.Now(), credit: capacity, interval: interval, capacity: capacity}
}

func (b *bucket) allow(now time.Time) bool {
	b.credit = min(b.capacity, b.credit+now.Sub(b.at))
	b.at = now
	if b.credit < b.interval {
		return false
	}
	b.credit -= b.interval
	return true
}

func (r *room) chat(c *client, text string) {
	player, exists := r.players[c.UserID]
	if !exists || player.client != c {
		return
	}
	r.broadcast("", message{"chat", struct {
		UserID string    `json:"userId"`
		Text   string    `json:"text"`
		Time   time.Time `json:"ts"`
	}{c.UserID, text, time.Now().UTC()}})
}
