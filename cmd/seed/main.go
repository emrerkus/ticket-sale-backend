// cmd/seed — gelistirme icin ornek veri uretir.
//
// NEDEN migration DEGIL?
//
//	Migration'lar SEMA icindir (tablolar, kolonlar) ve her ortamda calisir.
//	Ornek veri (test etkinligi, sahte koltuklar) sadece lokal/gelistirme icindir;
//	prod'a asla gitmemeli. O yuzden ayri bir arac.
//
// Kullanim:
//
//	go run ./cmd/seed
//
// Idempotent: her calistirmada katalog tablolarini TEMIZLEYIP bastan yazar.
// Yani iki kez calistirmak guvenli, cift kayit olusmaz.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/emrerkus/ticket-sale-backend/internal/config"
	"github.com/emrerkus/ticket-sale-backend/internal/db"
)

// Koltuk izgarasinin boyutu. 2 blok * 5 sira * 10 koltuk = 100 koltuk.
const (
	sectionCount = 2
	rowsPerBlock = 5
	seatsPerRow  = 10
)

// seat: uretecegimiz tek bir fiziksel koltuk.
type seat struct {
	id         string
	section    string // "A", "B"
	rowLabel   string // "1".."5"
	seatNumber string // "1".."10"
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Error("config yuklenemedi", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("DB baglanamadi", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := seed(ctx, pool); err != nil {
		log.Error("seed basarisiz", "err", err)
		os.Exit(1)
	}
	log.Info("seed tamamlandi")
}

func seed(ctx context.Context, pool *pgxpool.Pool) error {
	// --- Tek bir transaction: ya hepsi yazilir ya hicbiri ---
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("tx baslatilamadi: %w", err)
	}
	// Commit basarili olursa Rollback bir sey yapmaz (no-op). Hata donersek
	// veya panik olursa burasi her seyi geri alir.
	defer tx.Rollback(ctx)

	// 1) Katalogu + siparisleri temizle. CASCADE bagli tablolari da temizler
	//    (events -> ticket_types, event_seats; orders -> order_items, payments).
	//    users KASITLI olarak korunur (test kullanicilari kalsin).
	_, err = tx.Exec(ctx, `
		TRUNCATE venues, events, seats, ticket_types, event_seats,
		         orders, order_items, payments
		RESTART IDENTITY CASCADE`)
	if err != nil {
		return fmt.Errorf("truncate: %w", err)
	}

	// 2) Mekan
	venueID := uuid.NewString()
	_, err = tx.Exec(ctx,
		`INSERT INTO venues (id, name, address) VALUES ($1, $2, $3)`,
		venueID, "Kadikoy Sahne", "Caferaga Mah., Kadikoy/Istanbul")
	if err != nil {
		return fmt.Errorf("venue insert: %w", err)
	}

	// 3) Etkinlik — YAYINDA, satis su an acik, gelecekte basliyor.
	eventID := uuid.NewString()
	now := time.Now()
	_, err = tx.Exec(ctx, `
		INSERT INTO events
			(id, venue_id, title, description, starts_at, sales_start_at, sales_end_at, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'published')`,
		eventID, venueID,
		"Cok Yasa Turkce - Stand Up",
		"Bir aksamlik dogaclama komedi gecesi.",
		now.Add(14*24*time.Hour), // starts_at: 2 hafta sonra
		now.Add(-1*time.Hour),    // sales_start_at: 1 saat once acildi
		now.Add(13*24*time.Hour), // sales_end_at: etkinlikten 1 gun once
	)
	if err != nil {
		return fmt.Errorf("event insert: %w", err)
	}

	// 4) Iki bilet tipi. price_cents = KURUS (150.00 TL -> 15000).
	ttFull := uuid.NewString()
	ttStudent := uuid.NewString()
	_, err = tx.Exec(ctx, `
		INSERT INTO ticket_types (id, event_id, name, price_cents, currency) VALUES
			($1, $3, 'Tam',     15000, 'TRY'),
			($2, $3, 'Ogrenci',  9000, 'TRY')`,
		ttFull, ttStudent, eventID)
	if err != nil {
		return fmt.Errorf("ticket_types insert: %w", err)
	}

	// 5) Koltuklari uret (bellekte).
	seats := buildSeats()

	// 6) seats tablosuna toplu yaz. CopyFrom, tek tek INSERT'ten cok daha hizli:
	//    tum satirlari tek bir COPY protokol akisinda gonderir.
	_, err = tx.CopyFrom(ctx,
		pgx.Identifier{"seats"},
		[]string{"id", "venue_id", "section", "row_label", "seat_number"},
		pgx.CopyFromSlice(len(seats), func(i int) ([]any, error) {
			s := seats[i]
			return []any{s.id, venueID, s.section, s.rowLabel, s.seatNumber}, nil
		}),
	)
	if err != nil {
		return fmt.Errorf("seats copy: %w", err)
	}

	// 7) Her koltuk icin bir event_seats (envanter) satiri uret.
	rows := buildEventSeats(eventID, seats, ttFull, ttStudent)

	_, err = tx.CopyFrom(ctx,
		pgx.Identifier{"event_seats"},
		[]string{"id", "event_id", "seat_id", "ticket_type_id", "status"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("event_seats copy: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	fmt.Println("--------------------------------------------------")
	fmt.Println("venue_id  :", venueID)
	fmt.Println("event_id  :", eventID)
	fmt.Println("ticket_type Tam     :", ttFull)
	fmt.Println("ticket_type Ogrenci :", ttStudent)
	fmt.Printf("koltuk    : %d  (event_seats: %d)\n", len(seats), len(rows))
	fmt.Println("--------------------------------------------------")
	return nil
}

// buildSeats sectionCount blok * rowsPerBlock sira * seatsPerRow koltukluk bir
// izgara uretir. section "A", "B", ... ; row_label ve seat_number "1"den baslar.
func buildSeats() []seat {
	totalSeats := sectionCount * rowsPerBlock * seatsPerRow
	seats := make([]seat, 0, totalSeats)

	for b := 0; b < sectionCount; b++ {
		sectionName := string(rune('A' + b)) // 0 -> "A", 1 -> "B"
		for row := 1; row <= rowsPerBlock; row++ {
			rowStr := strconv.Itoa(row)
			for n := 1; n <= seatsPerRow; n++ {
				seatStr := strconv.Itoa(n)

				seats = append(seats, seat{
					id:         uuid.NewString(),
					section:    sectionName,
					rowLabel:   rowStr,
					seatNumber: seatStr,
				})
			}
		}
	}

	return seats
}

// buildEventSeats her koltuk icin bir envanter satiri uretir.
// Kural: "A" blogu Tam bilet, diger bloklar Ogrenci bileti.
// Donen deger pgx.CopyFromRows'un bekledigi bicimde: [][]any,
// kolon sirasi {id, event_id, seat_id, ticket_type_id, status}.
func buildEventSeats(eventID string, seats []seat, ttFull, ttStudent string) [][]any {
	rows := make([][]any, 0, len(seats))
	for _, s := range seats {
		ttID := ttStudent
		if s.section == "A" {
			ttID = ttFull
		}
		rows = append(rows, []any{uuid.NewString(), eventID, s.id, ttID, "available"})
	}
	return rows
}
