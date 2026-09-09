package lock_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/emrerkus/ticket-sale-backend/internal/lock"
)

// testClient: lokal Redis'e baglanir. Redis yoksa (CI'de vb.) testi ATLA.
func testClient(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skip("redis'e ulasilamiyor, test atlaniyor:", err)
	}
	return rdb
}

// Iddia: ayni anahtar icin 50 goroutine YARISIRSA, TAM OLARAK biri kazanir.
// "Ilk gelen kazanir" garantisinin kaniti (overselling savunmasinin Redis tarafi).
func TestAcquire_TamOlarakBirKazananVarConcurrency(t *testing.T) {
	rdb := testClient(t)
	ctx := context.Background()
	key := "test:lock:" + uuid.NewString()
	defer rdb.Del(ctx, key)

	l := lock.New(rdb)

	const n = 50
	var wins int64
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, ok, err := l.Acquire(ctx, key, 5*time.Second)
			if err == nil && ok {
				atomic.AddInt64(&wins, 1)
			}
		}()
	}
	wg.Wait()

	if wins != 1 {
		t.Fatalf("tam olarak 1 kazanan bekleniyordu, %d cikti", wins)
	}
}

// Iddia: yanlis token ile Release BASARISIZ olur (kilit yerinde kalir);
// dogru token ile Release BASARILI olur.
func TestRelease_SadeceDogruTokenSerbestBirakir(t *testing.T) {
	rdb := testClient(t)
	ctx := context.Background()
	key := "test:lock:" + uuid.NewString()
	defer rdb.Del(ctx, key)

	l := lock.New(rdb)

	token, ok, err := l.Acquire(ctx, key, 5*time.Second)
	if err != nil || !ok {
		t.Fatalf("acquire basarisiz olmamaliydi: ok=%v err=%v", ok, err)
	}

	// Yanlis token: kilit yerinde kalmali.
	if err := l.Release(ctx, key, "yanlis-token"); err != lock.ErrNotHeld {
		t.Fatalf("ErrNotHeld bekleniyordu, %v cikti", err)
	}
	if exists, _ := rdb.Exists(ctx, key).Result(); exists != 1 {
		t.Fatal("kilit hala var olmali (yanlis token silmemeli)")
	}

	// Dogru token: kilit silinmeli.
	if err := l.Release(ctx, key, token); err != nil {
		t.Fatalf("release basarili olmaliydi: %v", err)
	}
	if exists, _ := rdb.Exists(ctx, key).Result(); exists != 0 {
		t.Fatal("kilit artik olmamali")
	}
}
