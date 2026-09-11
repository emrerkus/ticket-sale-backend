// cmd/worker — arka plan gorevleri calistiran ayri bir program.
//
// Su anki tek isi: SURESI GECMIS koltuk hold'larini serbest birakmak.
//
// Neden ayri bir process? API istek/yanit dunyasinda yasar -- kimse istek
// atmasa hicbir sey yapmaz. Ama "10 dakika once tutulan koltugu simdi
// bosalt" gibi isler, kimse bir sey istemese bile ZAMANLA olmali. Bu tur
// isler icin ayri, surekli calisan bir process gerekir.
//
// Redis ile iliskisi: Redis'teki hold anahtarlari zaten TTL ile KENDILIGINDEN
// silinir. Bu worker SADECE Postgres tarafini duzeltir -- Redis anahtari
// silinmis ama event_seats.status hala 'held' kalmis satirlari 'available'a
// cevirir. Iki mekanizma birbirini tamamlar.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/emrerkus/ticket-sale-backend/internal/config"
	"github.com/emrerkus/ticket-sale-backend/internal/db"
	"github.com/emrerkus/ticket-sale-backend/internal/eventlog"
	"github.com/emrerkus/ticket-sale-backend/internal/logging"
	"github.com/emrerkus/ticket-sale-backend/internal/repository"
)

const serviceName = "ticketsale-worker"

// Ne siklikla tarayalim?
//
//	Cok sik  -> gereksiz DB yuku.
//	Cok seyrek -> koltuk bosaldiktan sonra "available" gorunene kadar gecen
//	              sure uzar (kullanici "bos koltuk yok" saniyor).
//
// 15sn makul bir orta nokta.
const sweepInterval = 15 * time.Second

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Error("config yuklenemedi", "err", err)
		os.Exit(1)
	}

	// API'deki ile ayni graceful shutdown kalibi: SIGINT/SIGTERM -> ctx iptal.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// API'deki ile ayni: stdout + Grafana Loki'ye yazan logger.
	log = slog.New(logging.NewMultiHandler(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		logging.NewLokiHandler(ctx, cfg.LokiURL, serviceName),
	))
	events := eventlog.New(log)

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("DB baglanamadi", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	eventRepo := repository.NewEventRepository(pool)
	orderRepo := repository.NewOrderRepository(pool)
	log.Info("worker basladi", "sweep_interval", sweepInterval)

	// Baslar baslamaz bir kez calistir, sonra periyodik. (Uygulama uzun sure
	// kapali kaldiysa birikmis hold'lari/siparisleri beklemeden temizle.)
	sweep(ctx, log, events, eventRepo, orderRepo)

	// time.Ticker: her sweepInterval'da bir kanala ("ticker.C") deger yollar.
	// cmd/api'deki server.Run'daki select kalibinin aynisi.
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("worker kapaniyor")
			return
		case <-ticker.C:
			sweep(ctx, log, events, eventRepo, orderRepo)
		}
	}
}

// sweep bir tarama turu: (1) suresi gecmis siparisleri iptal et ve koltuklarini
// birak, (2) siparise bagli olmayan suresi gecmis hold'lari birak.
// Sira onemli: once siparisler (koltuklari o birakir), sonra kalan hold'lar.
// Her serbest kalan koltuk icin ayri bir "seat_released" olayi loglanir --
// Grafana'da "kim, hangi koltugu, neden birakti" sorusuna cevap verir.
func sweep(ctx context.Context, log *slog.Logger, events *eventlog.Logger, eventRepo *repository.EventRepository, orderRepo *repository.OrderRepository) {
	if expired, err := orderRepo.ExpireOrders(ctx); err != nil {
		if ctx.Err() == nil {
			log.Error("suresi gecen siparisler temizlenemedi", "err", err)
		}
	} else if len(expired) > 0 {
		log.Info("suresi gecen siparisler iptal edildi", "adet", len(expired))
		for _, o := range expired {
			events.OrderExpired(ctx, o.OrderID, o.UserID, len(o.Seats))
			for _, seat := range o.Seats {
				events.SeatReleased(ctx, seat.EventID, seat.SeatID, seat.UserID, eventlog.ReasonOrderExpired)
			}
		}
	}

	if expired, err := eventRepo.ExpireHolds(ctx); err != nil {
		if ctx.Err() == nil {
			log.Error("suresi gecen hold'lar temizlenemedi", "err", err)
		}
	} else if len(expired) > 0 {
		log.Info("suresi gecen hold'lar serbest birakildi", "adet", len(expired))
		for _, h := range expired {
			events.SeatReleased(ctx, h.EventID, h.SeatID, h.UserID, eventlog.ReasonHoldExpired)
		}
	}
}
