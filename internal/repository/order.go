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

var (
	// ErrSeatNotHeld: kullanici bu koltugu su an tutmuyor (hold baskasinda,
	// suresi dolmus, ya da koltuk zaten baska bir aktif siparise dahil).
	ErrSeatNotHeld = errors.New("repository: koltuk bu kullanici tarafindan tutulmuyor")
	// ErrOrderNotPayable: siparis odenebilecek durumda degil (pending degil veya suresi dolmus).
	ErrOrderNotPayable = errors.New("repository: siparis odenebilir durumda degil")
	// ErrSeatLost: odeme aninda koltuk artik held/bende degil (hold suresi dolmus olabilir).
	ErrSeatLost = errors.New("repository: koltuk kaybedildi")
	// ErrMixedCurrency: sepetteki koltuklar farkli para birimlerinde.
	ErrMixedCurrency = errors.New("repository: sepette birden fazla para birimi var")
)

type OrderRepository struct {
	db *pgxpool.Pool
}

func NewOrderRepository(db *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{db: db}
}

// Checkout tuttugu koltuklari bir 'pending' siparise cevirir.
//
// Bir TRANSACTION icinde:
//  1. (idempotency) ayni anahtarla siparis varsa onu don.
//  2. Her koltuk satirini FOR UPDATE ile KILITLE ve dogrula:
//     - status = 'held' AND held_by = userID
//     - baska bir aktif (pending/paid) sipariste degil
//  3. Toplami hesapla, siparisi ve kalemleri yaz.
//  4. Koltuklarin hold suresini siparisin son odeme anina uzat.
//
// FOR UPDATE: iki es zamanli checkout ayni satiri kilitleyemez -> biri
// digerinin bitmesini bekler. Overselling'e karsi Postgres tarafindaki kilit.
func (r *OrderRepository) Checkout(
	ctx context.Context, userID string, seats []domain.SeatRef,
	ttl time.Duration, idempotencyKey string,
) (string, error) {
	if idempotencyKey != "" {
		var existingID string
		err := r.db.QueryRow(ctx,
			`SELECT id FROM orders WHERE idempotency_key = $1 AND user_id = $2`,
			idempotencyKey, userID,
		).Scan(&existingID)
		if err == nil {
			return existingID, nil // ayni istek tekrar geldi -> ayni siparis
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("idempotency kontrolu: %w", err)
		}
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("tx: %w", err)
	}
	defer tx.Rollback(ctx)

	type line struct {
		eventSeatID string
		price       int64
	}
	var lines []line
	var total int64
	currency := ""

	for _, sr := range seats {
		var l line
		var seatCurrency string
		var status, heldBy string
		err := tx.QueryRow(ctx, `
			SELECT es.id, es.status, COALESCE(es.held_by::text, ''), tt.price_cents, tt.currency
			FROM event_seats es
			JOIN ticket_types tt ON tt.id = es.ticket_type_id
			WHERE es.event_id = $1 AND es.seat_id = $2
			FOR UPDATE OF es`,
			sr.EventID, sr.SeatID,
		).Scan(&l.eventSeatID, &status, &heldBy, &l.price, &seatCurrency)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		if err != nil {
			return "", fmt.Errorf("koltuk kilitlenemedi: %w", err)
		}
		if status != "held" || heldBy != userID {
			return "", ErrSeatNotHeld
		}

		// Bu koltuk zaten aktif bir siparise dahil mi?
		var inOrder bool
		err = tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM order_items oi
				JOIN orders o ON o.id = oi.order_id
				WHERE oi.event_seat_id = $1 AND o.status IN ('pending', 'paid')
			)`, l.eventSeatID,
		).Scan(&inOrder)
		if err != nil {
			return "", fmt.Errorf("siparis kontrolu: %w", err)
		}
		if inOrder {
			return "", ErrSeatNotHeld
		}

		if currency == "" {
			currency = seatCurrency
		} else if currency != seatCurrency {
			return "", ErrMixedCurrency
		}
		total += l.price
		lines = append(lines, l)
	}

	expiresAt := time.Now().Add(ttl)
	var keyArg any
	if idempotencyKey != "" {
		keyArg = idempotencyKey
	}

	var orderID string
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (user_id, status, total_cents, currency, idempotency_key, expires_at)
		VALUES ($1, 'pending', $2, $3, $4, $5)
		RETURNING id`,
		userID, total, currency, keyArg, expiresAt,
	).Scan(&orderID)
	if err != nil {
		return "", fmt.Errorf("siparis olusturulamadi: %w", err)
	}

	for _, l := range lines {
		if _, err := tx.Exec(ctx,
			`INSERT INTO order_items (order_id, event_seat_id, unit_price_cents) VALUES ($1, $2, $3)`,
			orderID, l.eventSeatID, l.price,
		); err != nil {
			return "", fmt.Errorf("siparis kalemi eklenemedi: %w", err)
		}
		// Hold suresini siparisin son odeme anina uzat (koltuk odeme bitmeden kacmasin).
		if _, err := tx.Exec(ctx,
			`UPDATE event_seats SET held_until = $2, updated_at = now() WHERE id = $1`,
			l.eventSeatID, expiresAt,
		); err != nil {
			return "", fmt.Errorf("hold suresi uzatilamadi: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}
	return orderID, nil
}

// PurchasedSeat: basariyla satilan bir koltugun, loglama icin gereken bilgisi.
type PurchasedSeat struct {
	EventID        string
	SeatID         string
	UnitPriceCents int64
}

// Pay bir siparisi oder (mock saglayici). fail=true ise odeme reddedilir.
// Basarili odemede: koltuklar 'sold', siparis 'paid'. Hepsi tek transaction.
// Ikinci donus degeri SADECE basarili odemede doldurulur -- hangi koltuklarin
// (event_id, seat_id) satildigi, cagiran tarafin bunlari loglayabilmesi icin.
func (r *OrderRepository) Pay(ctx context.Context, orderID, userID string, fail bool) (domain.Payment, []PurchasedSeat, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Payment{}, nil, fmt.Errorf("tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var status, currency string
	var total int64
	var expiresAt time.Time
	err = tx.QueryRow(ctx, `
		SELECT status, total_cents, currency, expires_at
		FROM orders WHERE id = $1 AND user_id = $2
		FOR UPDATE`,
		orderID, userID,
	).Scan(&status, &total, &currency, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payment{}, nil, ErrNotFound
	}
	if err != nil {
		return domain.Payment{}, nil, fmt.Errorf("siparis kilitlenemedi: %w", err)
	}
	if status != "pending" || time.Now().After(expiresAt) {
		return domain.Payment{}, nil, ErrOrderNotPayable
	}

	// Odeme kaydini olustur (pending).
	var paymentID string
	err = tx.QueryRow(ctx, `
		INSERT INTO payments (order_id, provider, amount_cents, currency, status)
		VALUES ($1, 'mock', $2, $3, 'pending')
		RETURNING id`,
		orderID, total, currency,
	).Scan(&paymentID)
	if err != nil {
		return domain.Payment{}, nil, fmt.Errorf("odeme kaydi: %w", err)
	}

	providerRef := "mock_" + paymentID[:8]

	if fail {
		if _, err := tx.Exec(ctx,
			`UPDATE payments SET status = 'failed', provider_ref = $2, updated_at = now() WHERE id = $1`,
			paymentID, providerRef,
		); err != nil {
			return domain.Payment{}, nil, fmt.Errorf("odeme guncellenemedi: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.Payment{}, nil, fmt.Errorf("commit: %w", err)
		}
		p, err := r.getPayment(ctx, paymentID)
		return p, nil, err
	}

	// Basarili odeme: her koltugu 'sold'a cevir (held + benimse). event_id/seat_id'yi
	// de ceker (yalnizca event_seat_id degil) -- loglama bunlari kullanacak.
	rows, err := tx.Query(ctx, `
		SELECT es.id, es.event_id, es.seat_id, oi.unit_price_cents
		FROM order_items oi
		JOIN event_seats es ON es.id = oi.event_seat_id
		WHERE oi.order_id = $1`, orderID)
	if err != nil {
		return domain.Payment{}, nil, fmt.Errorf("kalemler okunamadi: %w", err)
	}
	type line struct {
		eventSeatID string
		seat        PurchasedSeat
	}
	var lines []line
	for rows.Next() {
		var l line
		if err := rows.Scan(&l.eventSeatID, &l.seat.EventID, &l.seat.SeatID, &l.seat.UnitPriceCents); err != nil {
			rows.Close()
			return domain.Payment{}, nil, err
		}
		lines = append(lines, l)
	}
	rows.Close()

	purchased := make([]PurchasedSeat, 0, len(lines))
	for _, l := range lines {
		tag, err := tx.Exec(ctx, `
			UPDATE event_seats
			SET status = 'sold', held_until = NULL, held_by = NULL, version = version + 1, updated_at = now()
			WHERE id = $1 AND status = 'held' AND held_by = $2`,
			l.eventSeatID, userID,
		)
		if err != nil {
			return domain.Payment{}, nil, fmt.Errorf("koltuk sold yapilamadi: %w", err)
		}
		if tag.RowsAffected() != 1 {
			// Koltuk artik bende degil (hold suresi dolmus + baskasi kapmis).
			// Tum transaction geri sarilir -> odeme de gerceklesmez.
			return domain.Payment{}, nil, ErrSeatLost
		}
		purchased = append(purchased, l.seat)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE orders SET status = 'paid', updated_at = now() WHERE id = $1`, orderID,
	); err != nil {
		return domain.Payment{}, nil, fmt.Errorf("siparis paid yapilamadi: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE payments SET status = 'succeeded', provider_ref = $2, updated_at = now() WHERE id = $1`,
		paymentID, providerRef,
	); err != nil {
		return domain.Payment{}, nil, fmt.Errorf("odeme guncellenemedi: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Payment{}, nil, fmt.Errorf("commit: %w", err)
	}
	p, err := r.getPayment(ctx, paymentID)
	return p, purchased, err
}

func (r *OrderRepository) getPayment(ctx context.Context, id string) (domain.Payment, error) {
	var p domain.Payment
	err := r.db.QueryRow(ctx, `
		SELECT id, order_id, provider, provider_ref, amount_cents, currency, status, created_at
		FROM payments WHERE id = $1`, id,
	).Scan(&p.ID, &p.OrderID, &p.Provider, &p.ProviderRef, &p.AmountCents, &p.Currency, &p.Status, &p.CreatedAt)
	if err != nil {
		return domain.Payment{}, fmt.Errorf("odeme okunamadi: %w", err)
	}
	return p, nil
}

// GetDetail bir siparisin tam detayini (kalemler + varsa odeme) doner.
func (r *OrderRepository) GetDetail(ctx context.Context, orderID, userID string) (*domain.OrderDetail, error) {
	var d domain.OrderDetail
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, status, total_cents, currency, expires_at, created_at
		FROM orders WHERE id = $1 AND user_id = $2`,
		orderID, userID,
	).Scan(&d.ID, &d.UserID, &d.Status, &d.TotalCents, &d.Currency, &d.ExpiresAt, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("siparis okunamadi: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT oi.id, oi.event_seat_id, s.section, s.row_label, s.seat_number, oi.unit_price_cents
		FROM order_items oi
		JOIN event_seats es ON es.id = oi.event_seat_id
		JOIN seats s ON s.id = es.seat_id
		WHERE oi.order_id = $1
		ORDER BY s.section, s.row_label::int, s.seat_number::int`, orderID)
	if err != nil {
		return nil, fmt.Errorf("kalemler okunamadi: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var it domain.OrderItem
		if err := rows.Scan(&it.ID, &it.EventSeatID, &it.Section, &it.RowLabel, &it.SeatNumber, &it.UnitPriceCents); err != nil {
			return nil, err
		}
		d.Items = append(d.Items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var p domain.Payment
	perr := r.db.QueryRow(ctx, `
		SELECT id, order_id, provider, provider_ref, amount_cents, currency, status, created_at
		FROM payments WHERE order_id = $1 ORDER BY created_at DESC LIMIT 1`, orderID,
	).Scan(&p.ID, &p.OrderID, &p.Provider, &p.ProviderRef, &p.AmountCents, &p.Currency, &p.Status, &p.CreatedAt)
	if perr == nil {
		d.Payment = &p
	} else if !errors.Is(perr, pgx.ErrNoRows) {
		return nil, fmt.Errorf("odeme okunamadi: %w", perr)
	}

	return &d, nil
}

// ListByUser bir kullanicinin tum siparisleri, yeniden eskiye.
func (r *OrderRepository) ListByUser(ctx context.Context, userID string) ([]domain.Order, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, status, total_cents, currency, expires_at, created_at
		FROM orders WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("siparisler okunamadi: %w", err)
	}
	defer rows.Close()
	var out []domain.Order
	for rows.Next() {
		var o domain.Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.Status, &o.TotalCents, &o.Currency, &o.ExpiresAt, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// ExpiredOrder: suresi dolup iptal edilen bir siparisin, loglama icin gereken bilgisi.
type ExpiredOrder struct {
	OrderID string
	UserID  string
	Seats   []ExpiredHold // bu siparisle birakilan koltuklar (ExpiredHold, event.go'da tanimli)
}

// ExpireOrders suresi gecmis 'pending' siparisleri 'expired' yapar ve
// tuttuklari koltuklari geri birakir. cmd/worker periyodik cagirir.
func (r *OrderRepository) ExpireOrders(ctx context.Context) ([]ExpiredOrder, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("tx: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT id, user_id FROM orders
		WHERE status = 'pending' AND expires_at < now()
		FOR UPDATE SKIP LOCKED`)
	if err != nil {
		return nil, fmt.Errorf("suresi gecen siparisler: %w", err)
	}
	byOrder := map[string]*ExpiredOrder{}
	var ids []string
	for rows.Next() {
		var id, userID string
		if err := rows.Scan(&id, &userID); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
		byOrder[id] = &ExpiredOrder{OrderID: id, UserID: userID}
	}
	rows.Close()
	if len(ids) == 0 {
		return nil, nil
	}

	// Loglama icin: bu siparislerin hangi koltuklari tuttugunu (event_id, seat_id)
	// koltuklari birakmadan ONCE oku.
	seatRows, err := tx.Query(ctx, `
		SELECT oi.order_id, es.event_id, es.seat_id
		FROM order_items oi
		JOIN event_seats es ON es.id = oi.event_seat_id
		WHERE oi.order_id = ANY($1) AND es.status = 'held'`, ids)
	if err != nil {
		return nil, fmt.Errorf("siparis koltuklari okunamadi: %w", err)
	}
	for seatRows.Next() {
		var orderID string
		var h ExpiredHold
		if err := seatRows.Scan(&orderID, &h.EventID, &h.SeatID); err != nil {
			seatRows.Close()
			return nil, err
		}
		if o, ok := byOrder[orderID]; ok {
			h.UserID = o.UserID
			o.Seats = append(o.Seats, h)
		}
	}
	seatRows.Close()
	if err := seatRows.Err(); err != nil {
		return nil, err
	}

	// Koltuklari geri birak (sadece hala held olanlar).
	if _, err := tx.Exec(ctx, `
		UPDATE event_seats
		SET status = 'available', held_until = NULL, held_by = NULL, version = version + 1, updated_at = now()
		WHERE status = 'held' AND id IN (
			SELECT event_seat_id FROM order_items WHERE order_id = ANY($1)
		)`, ids,
	); err != nil {
		return nil, fmt.Errorf("koltuklar birakilamadi: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE orders SET status = 'expired', updated_at = now() WHERE id = ANY($1)`, ids,
	); err != nil {
		return nil, fmt.Errorf("siparisler expired yapilamadi: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	out := make([]ExpiredOrder, 0, len(byOrder))
	for _, o := range byOrder {
		out = append(out, *o)
	}
	return out, nil
}
