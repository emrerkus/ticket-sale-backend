// Package cache Redis baglantisini kurar.
// Redis su an koltuk hold kilitleri (internal/lock, SET NX PX) icin kullaniliyor.
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Connect bir Redis client'i kurar ve PING ile calistigini dogrular.
//
// redis.Client kendi ic havuzunu (connection pool) yonetir — tek bir *Client'i
// tum uygulamada paylasabilirsin, goroutine-safe'dir.
func Connect(ctx context.Context, addr string) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
		// Uretimde: sifre, DB numarasi, TLS buraya. Lokal'de gerek yok.
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("Redis ping basarisiz (%s): %w", addr, err)
	}

	return client, nil
}
