// cmd/api uygulamanin giris noktasi (entrypoint).
//
// Go proje duzeni:
//
//	cmd/<isim>/main.go  -> calistirilabilir program(lar). Ince olur: sadece "kur ve baslat".
//	internal/...        -> asil kod. "internal" ozeldir: baska repo'lar import edemez.
//
// main.go'nun tek isi: config yukle, logger kur, bagimliliklari ac, sunucuyu
// calistir, kapanista her seyi temiz kapat.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/emrerkus/ticket-sale-backend/internal/cache"
	"github.com/emrerkus/ticket-sale-backend/internal/config"
	"github.com/emrerkus/ticket-sale-backend/internal/db"
	"github.com/emrerkus/ticket-sale-backend/internal/eventlog"
	"github.com/emrerkus/ticket-sale-backend/internal/logging"
	"github.com/emrerkus/ticket-sale-backend/internal/server"
	"github.com/joho/godotenv"
)

// serviceName: Grafana Loki'de bu servisin loglarini "service" etiketiyle ayirt eder.
const serviceName = "ticketsale-api"

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := godotenv.Load(); err != nil {
		log.Info(".env yuklenmedi (opsiyonel)", "err", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Error("config yuklenemedi", "err", err)
		os.Exit(1)
	}

	// signal.NotifyContext: SIGINT (Ctrl+C) / SIGTERM gelince ctx iptal olur.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- Loglama: hem stdout'a (terminal/docker logs) HEM Grafana Loki'ye ---
	// MultiHandler ayni kaydi ikisine de yollar. Loki'ye ulasilamasa bile
	// (henuz ayaga kalkmadi, kapali vs.) uygulama normal calismaya devam eder --
	// LokiHandler asenkron ve non-blocking'tir (bkz. internal/logging/loki.go).
	log = slog.New(logging.NewMultiHandler(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		logging.NewLokiHandler(ctx, cfg.LokiURL, serviceName),
	))
	events := eventlog.New(log)

	// --- Bagimliliklar: Postgres havuzu + Redis client ---
	// Baglanamadigimiz an burada duruyoruz ("fail fast"): yarim calisan bir
	// API'den, hic baslamayan bir API iyidir.
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("Postgres'e baglanilamadi", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb, err := cache.Connect(ctx, cfg.RedisAddr)
	if err != nil {
		log.Error("Redis'e baglanilamadi", "err", err)
		os.Exit(1)
	}
	defer func() { _ = rdb.Close() }()

	log.Info("bagimliliklar hazir", "postgres", "ok", "redis", "ok")

	srv := server.New(cfg, log, pool, rdb, events)
	if err := srv.Run(ctx); err != nil {
		log.Error("sunucu hatayla durdu", "err", err)
		os.Exit(1)
	}
}
