package dedup

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis compartilha a janela entre réplicas da bridge: a assinatura compartilhada
// do EMQX pode entregar a redelivery a outra instância.
type Redis struct {
	client redis.Cmdable
	ttl    time.Duration
	prefix string
}

func NewRedis(client redis.Cmdable, ttl time.Duration) *Redis {
	return &Redis{client: client, ttl: ttl, prefix: "bridge:dedup:"}
}

func (r *Redis) Claim(ctx context.Context, id string) (bool, error) {
	return r.client.SetNX(ctx, r.prefix+id, 1, r.ttl).Result()
}

func (r *Redis) Release(ctx context.Context, id string) error {
	return r.client.Del(ctx, r.prefix+id).Err()
}
