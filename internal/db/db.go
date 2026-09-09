// Package db PostgreSQL baglanti havuzunu (connection pool) kurar.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect verilen DATABASE_URL ile bir pgx havuzu acar ve calistigini dogrular.
//
// Havuz nedir? Onceden acilmis birkac DB baglantisini tutan ve isteklere odunc
// veren yapi. Her HTTP istegi kendi baglantisini acmak yerine havuzdan bir tane
// alir, isi bitince geri verir. Baglanti acmak pahali oldugu icin bu kritik.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL cozumlenemedi: %w", err)
	}

	// Havuz ayarlari:
	//   MaxConns        havuzun acabilecegi maksimum baglanti. Birden fazla API
	//                   instance'i varsa hepsinin toplami Postgres'in
	//                   max_connections'ini (varsayilan 100) asmamali.
	//   MinConns        bosta bile tutulacak minimum -> "soguk baslangic" gecikmesini onler.
	//   MaxConnLifetime cok uzun yasayan baglantilar (proxy arkasindaki DB'de) sorun cikarir.
	//   MaxConnIdleTime bosta bu kadar durunca baglanti kapatilir.
	cfg.MaxConns = 10
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = time.Hour / 2

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("havuz olusturulamadi: %w", err)
	}

	// NewWithConfig hemen baglanmaz (lazy). Ping ile gercekten calistigini dogrula;
	// yanlis sifre / kapali DB gibi hatalari uygulama basinda yakala ("fail fast").
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("DB ping basarisiz: %w", err)
	}

	return pool, nil
}
