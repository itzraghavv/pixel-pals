package platform

import (
	"context"
	"github.com/itzraghavv/pixel-pals/backend/internal/auth"
	"github.com/redis/go-redis/v9"
)

var signInLimit = redis.NewScript(`local n=redis.call('INCR',KEYS[1]); if n==1 then redis.call('EXPIRE',KEYS[1],60) end; return n`)

// LimitSignIn shares counters across API replicas, without storing usernames or IPs.
func LimitSignIn(r *redis.Client) func(context.Context, string) (bool, error) {
	return func(ctx context.Context, key string) (bool, error) {
		n, e := signInLimit.Run(ctx, r, []string{"signin:" + auth.Digest(key)}).Int()
		return n <= 10, e
	}
}
