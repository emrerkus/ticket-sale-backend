// Package repository veritabaniyla konusan TEK katmandir. Diger her yer
// (service, handler) SQL'in var oldugunu bile bilmez — sadece Go tipleri
// (domain.*) ve Go hatalariyla ugrasir.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/emrerkus/ticket-sale-backend/internal/domain"
)

// ErrNotFound: "tekil bir kayit aradim, yoktu". pgx'in kendi ErrNoRows'unu
// disariya sizdirmiyoruz — service katmani pgx'i hic import etmeden bu
// hatayi errors.Is ile tanıyabilsin diye kendi sentinel'imizi tanimliyoruz.
var ErrNotFound = errors.New("repository: kayit bulunamadi")

// EventRepository etkinlik/mekan/koltuk verisini okur.
type EventRepository struct {
	db *pgxpool.Pool
}

func NewEventRepository(db *pgxpool.Pool) *EventRepository {
	return &EventRepository{db: db}
}

// ListPublishedUpcoming: yayinda VE henuz baslamamis etkinlikler, mekan adiyla
// birlikte, baslama zamanina gore artan sirada.
func (r *EventRepository) ListPublishedUpcoming(ctx context.Context) ([]domain.EventListItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT e.id, e.venue_id, e.title, e.description,
		       e.starts_at, e.sales_start_at, e.sales_end_at, e.status,
		       v.name
		FROM events e
		JOIN venues v ON v.id = e.venue_id
		WHERE e.status = 'published' AND e.starts_at > now()
		ORDER BY e.starts_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("etkinlikler sorgulanamadi: %w", err)
	}
	defer rows.Close()

	var items []domain.EventListItem
	for rows.Next() {
		var it domain.EventListItem
		if err := rows.Scan(
			&it.ID, &it.VenueID, &it.Title, &it.Description,
			&it.StartsAt, &it.SalesStartAt, &it.SalesEndAt, &it.Status,
			&it.VenueName,
		); err != nil {
			return nil, fmt.Errorf("etkinlik satiri okunamadi: %w", err)
		}
		items = append(items, it)
	}
	// rows.Next() bir hatadan dolayi de false donebilir (baglanti koptu vs.).
	// Dongu bitince MUTLAKA rows.Err() kontrol edilir.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("etkinlikler okunurken hata: %w", err)
	}
	return items, nil
}

// GetByID: DURUMU NE OLURSA OLSUN (draft/published/cancelled) tek bir etkinlik + mekani.
// "sadece published gorunsun" kurali BILEREK burada yok — bu bir is/yetki
// kurali, veri erisimi degil. Service katmani karar verir.
func (r *EventRepository) GetByID(ctx context.Context, id string) (domain.Event, domain.Venue, error) {
	var e domain.Event
	var v domain.Venue
	err := r.db.QueryRow(ctx, `
		SELECT e.id, e.venue_id, e.title, e.description,
		       e.starts_at, e.sales_start_at, e.sales_end_at, e.status,
		       v.id, v.name, v.address
		FROM events e
		JOIN venues v ON v.id = e.venue_id
		WHERE e.id = $1`, id,
	).Scan(
		&e.ID, &e.VenueID, &e.Title, &e.Description,
		&e.StartsAt, &e.SalesStartAt, &e.SalesEndAt, &e.Status,
		&v.ID, &v.Name, &v.Address,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Event{}, domain.Venue{}, ErrNotFound
	}
	if err != nil {
		return domain.Event{}, domain.Venue{}, fmt.Errorf("etkinlik okunamadi: %w", err)
	}
	return e, v, nil
}

// ListTicketTypes: bir etkinligin fiyat kategorileri, pahalidan ucuza.
func (r *EventRepository) ListTicketTypes(ctx context.Context, eventID string) ([]domain.TicketType, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, price_cents, currency
		FROM ticket_types
		WHERE event_id = $1
		ORDER BY price_cents DESC`, eventID)
	if err != nil {
		return nil, fmt.Errorf("bilet tipleri sorgulanamadi: %w", err)
	}
	defer rows.Close()

	var out []domain.TicketType
	for rows.Next() {
		var tt domain.TicketType
		if err := rows.Scan(&tt.ID, &tt.Name, &tt.PriceCents, &tt.Currency); err != nil {
			return nil, fmt.Errorf("bilet tipi satiri okunamadi: %w", err)
		}
		out = append(out, tt)
	}
	return out, rows.Err()
}

// SeatMap: bir etkinligin TUM koltuk envanteri (available + held + sold hepsi —
// filtreleme handler/service'te degil, gerekirse burada ayri bir metotla yapilir).
func (r *EventRepository) SeatMap(ctx context.Context, eventID string) ([]domain.SeatMapEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT s.id, s.section, s.row_label, s.seat_number,
		       es.ticket_type_id, es.status
		FROM event_seats es
		JOIN seats s ON s.id = es.seat_id
		WHERE es.event_id = $1
		-- row_label/seat_number TEXT kolonlar; sayisal sirala yoksa "10" "2"den once gelir.
		ORDER BY s.section, s.row_label::int, s.seat_number::int`, eventID)
	if err != nil {
		return nil, fmt.Errorf("koltuk haritasi sorgulanamadi: %w", err)
	}
	defer rows.Close()

	var out []domain.SeatMapEntry
	for rows.Next() {
		var sm domain.SeatMapEntry
		if err := rows.Scan(
			&sm.SeatID, &sm.Section, &sm.RowLabel, &sm.SeatNumber,
			&sm.TicketTypeID, &sm.Status,
		); err != nil {
			return nil, fmt.Errorf("koltuk satiri okunamadi: %w", err)
		}
		out = append(out, sm)
	}
	return out, rows.Err()
}

// TryHold: event_seats uzerinde ATOMIK "available -> held" gecisi.
//
// Tek bir UPDATE ile hem okuyup hem yaziyoruz -- "once SELECT ile durumu
// kontrol et, sonra UPDATE et" diye IKI adima bolseydik, ikisi arasina
// baska bir istek girip ayni koltugu alabilirdi. WHERE status='available'
// satiri KOSULUN KENDISI -- Postgres bu satiri baskasi degistiriyorsa
// bekletir, boylece iki UPDATE asla ayni anda "basarili" olamaz.
//
// UPDATE 0 satir etkilerse SEBEBI BELIRSIZDIR: koltuk hic yok mu, yoksa var
// ama zaten held/sold mu? Bunu ayirt etmek icin (sadece basarisizlik
// durumunda, ekstra maliyeti sadece o yolda oderiz) ikinci bir SELECT atariz.
//
// WHERE kosulu iki durumu kabul eder:
//   - status = 'available'                       -> normal hold
//   - status = 'held' AND held_until < now()     -> suresi gecmis eski hold'u
//     devral (worker daha temizlememis olabilir)
func (r *EventRepository) TryHold(ctx context.Context, eventID, seatID, holderID string, heldUntil time.Time) (updated bool, exists bool, err error) {
	var id string
	updateErr := r.db.QueryRow(ctx, `
		UPDATE event_seats
		SET status = 'held', held_until = $3, held_by = $4, version = version + 1, updated_at = now()
		WHERE event_id = $1 AND seat_id = $2
		  AND (status = 'available' OR (status = 'held' AND held_until < now()))
		RETURNING id`,
		eventID, seatID, heldUntil, holderID,
	).Scan(&id)

	if updateErr == nil {
		return true, true, nil
	}
	if !errors.Is(updateErr, pgx.ErrNoRows) {
		return false, false, fmt.Errorf("hold guncellemesi basarisiz: %w", updateErr)
	}

	// UPDATE 0 satir etkiledi -- satir var mi diye ayrica bak.
	var found bool
	checkErr := r.db.QueryRow(ctx,
		`SELECT true FROM event_seats WHERE event_id = $1 AND seat_id = $2`,
		eventID, seatID,
	).Scan(&found)
	if errors.Is(checkErr, pgx.ErrNoRows) {
		return false, false, nil // hic boyle bir koltuk yok
	}
	if checkErr != nil {
		return false, false, fmt.Errorf("koltuk varligi kontrol edilemedi: %w", checkErr)
	}
	return false, true, nil // var ama musait degildi (aktif held veya sold)
}

// ReleaseHold: held -> available, SADECE holderID bu hold'un sahibiyse.
// released=false -> boyle bir hold yok VEYA baskasina ait (ayrimini bilerek
// yapmiyoruz: "senin iptal edebilecegin bir hold yok" tek mesaj yeterli).
func (r *EventRepository) ReleaseHold(ctx context.Context, eventID, seatID, holderID string) (released bool, err error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE event_seats
		SET status = 'available', held_until = NULL, held_by = NULL,
		    version = version + 1, updated_at = now()
		WHERE event_id = $1 AND seat_id = $2 AND status = 'held' AND held_by = $3`,
		eventID, seatID, holderID,
	)
	if err != nil {
		return false, fmt.Errorf("hold iptali basarisiz: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// ExpiredHold: serbest birakilan bir hold'un, loglama icin gereken bilgisi.
type ExpiredHold struct {
	EventID string
	SeatID  string
	UserID  string // held_by -- guncellemeden ONCEKI deger
}

// ExpireHolds: suresi gecmis TUM hold'lari serbest birakir. cmd/worker
// periyodik cagirir. Redis'e DOKUNMAZ -- Redis anahtarlari TTL ile
// kendiliginden silinir; bu yalnizca Postgres tarafindaki takili kalmis
// 'held' satirlarini duzeltir.
//
// WITH ... UPDATE ... FROM deseni: once "expired" CTE'siyle etkilenecek
// satirlarin ESKI halini (ozellikle held_by, UPDATE bunu NULL'a cevirmeden
// once) yakalariz, sonra ayni sorguda guncelleriz. Tek atomik ifade -- araya
// baska bir istek giremez.
func (r *EventRepository) ExpireHolds(ctx context.Context) ([]ExpiredHold, error) {
	rows, err := r.db.Query(ctx, `
		WITH expired AS (
			SELECT id, event_id, seat_id, held_by
			FROM event_seats
			WHERE status = 'held' AND held_until < now()
			FOR UPDATE
		)
		UPDATE event_seats es
		SET status = 'available', held_until = NULL, held_by = NULL,
		    version = version + 1, updated_at = now()
		FROM expired
		WHERE es.id = expired.id
		RETURNING expired.event_id, expired.seat_id, expired.held_by`)
	if err != nil {
		return nil, fmt.Errorf("suresi gecen hold'lar temizlenemedi: %w", err)
	}
	defer rows.Close()

	var out []ExpiredHold
	for rows.Next() {
		var h ExpiredHold
		if err := rows.Scan(&h.EventID, &h.SeatID, &h.UserID); err != nil {
			return nil, fmt.Errorf("hold satiri okunamadi: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
