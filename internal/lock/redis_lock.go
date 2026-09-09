// Package lock, Redis uzerinde DAGITIK KILIT (distributed lock) saglar.
//
// Neden gerekli? Ayni anda 50 kullanici ayni koltuga tiklarsa, "musait mi"
// kontrolu ile "tut" islemi arasinda bir yaris durumu (race condition)
// olusabilir. Redis'in SET ... NX komutu ATOMIKTIR: "sadece yoksa yaz" tek
// bir islemde olur, araya baska hicbir istemci giremez. Bu da "ilk gelen
// kazanir" garantisini verir.
//
// Bu paket GENEL AMACLIDIR -- "koltuk" kelimesini bilmez, sadece "anahtar/key"
// bilir. Koltuklarla ilgili anlam (event_id, seat_id) service katmaninda
// eklenecek (internal/service/hold.go).
package lock

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ErrNotHeld: Release cagrisi yapildi ama kilit ya hic yok ya da BASKASINA ait.
var ErrNotHeld = errors.New("lock: kilit bu token'a ait degil veya yok")

// Locker Redis uzerinden kilit alip birakan kucuk bir yardimci.
type Locker struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Locker {
	return &Locker{rdb: rdb}
}

// Acquire bir kilidi almaya calisir. Basarili olursa, SADECE bu cagrinin
// sahip oldugu rastgele bir "token" doner. Kilidi sonradan SADECE bu token'i
// bilen serbest birakabilir -- bu, "baskasinin kilidini yanlislikla silme"
// klasik hatasini onler (ornek: A'nin kilidi suresi dolar, B yeni bir kilit
// alir, A'nin gecikmis "release" cagrisi B'nin TAZE kilidini silmemeli).
func (l *Locker) Acquire(ctx context.Context, key string, ttl time.Duration) (token string, acquired bool, err error) {
	token = uuid.NewString()

	// SET key token NX PX <ttl-ms>
	//   NX = "sadece key YOKSA yaz"           (Not eXists)
	//   PX = "bu kadar ms sonra KENDILIGINDEN sil" (native TTL -- ayri bir
	//        temizlik isine gerek yok, Redis kendi siliyor)
	// Bu TEK komut atomiktir. "Once var mi bak, yoksa yaz" iki ayri adim
	// olsaydi, ikisi arasina baska bir istemci girip ayni anda basarili
	// olabilirdi -- iki kullanici ayni koltugu "tuttum" sanirdi.
	ok, err := l.rdb.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return "", false, err
	}
	return token, ok, nil
}

// releaseScript: Redis icinde ATOMIK "kontrol et VE sil" (compare-and-delete).
//
// Neden Lua script gerekiyor? "Once GET ile token'i oku, esitse DEL ile sil"
// diye IKI ayri komut yazsaydik, tam bu ikisinin arasinda kilidin suresi
// dolup baskasi YENI bir kilit alabilirdi -- sen de elindeki (artik eski)
// token'a gore, aslinda baskasina ait TAZE kilidi silmis olurdun. Redis,
// bir Lua script'i TEK ve BOLUNEMEZ bir adim olarak calistirir; script
// calisirken araya baska hicbir komut giremez.
var releaseScript = redis.NewScript(`
	if redis.call("GET", KEYS[1]) == ARGV[1] then
		return redis.call("DEL", KEYS[1])
	end
	return 0
`)

// Release bir kilidi SADECE onu Acquire eden token biliyorsa serbest birakir.
// Token uyusmuyorsa (kilit baskasina ait ya da zaten silinmis) ErrNotHeld doner.
func (l *Locker) Release(ctx context.Context, key, token string) error {
	res, err := releaseScript.Run(ctx, l.rdb, []string{key}, token).Int()
	if err != nil {
		return err
	}
	if res == 0 {
		return ErrNotHeld
	}
	return nil
}

// ForceRelease bir kilidi TOKEN KONTROLU YAPMADAN siler.
//
// SADECE cagiran taraf sahipligi BASKA BIR YOLDAN kesin dogruladiysa
// kullanilmalidir -- ornegin Postgres'te "held_by = benimId" eslesmesi zaten
// kanitlandiktan sonra. O noktada elimizde Redis token'i olmaz ama koltugun
// sahibi oldugumuzu biliriz; kilit ya bizimdir ya da suresi coktan dolmustur,
// iki durumda da silmek guvenlidir. Genel durumda Release kullan.
func (l *Locker) ForceRelease(ctx context.Context, key string) error {
	return l.rdb.Del(ctx, key).Err()
}
