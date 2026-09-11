// Package eventlog, biletle ilgili IS OLAYLARINI (hold/satin alma/birakma)
// yapisal loglar olarak kaydeder. Bu loglar hem stdout'a (docker logs / terminal)
// hem de Grafana Loki'ye gider (bkz. internal/logging) -- boylece "hangi koltuk,
// hangi kullanici tarafindan tutuldu/satin alindi/birakildi" Grafana'da
// aranabilir, filtrelenebilir, grafiklenebilir olur.
//
// Neden ayri bir paket (dogrudan handler'larda log.Info(...) yazmak yerine)?
//   - TEK bir yerde tutarli alan adlari (seat_id, user_id, event_id, ...) garanti
//     edilir -- bir yerde "user_id", baska yerde "userId" yazma hatasi olmaz.
//   - "event" alani (log turu) burada sabitlenir; Loki etiketi bunun uzerine kurulu.
//   - Cagiran kod (service katmani) "ne oldugunu" soyler, "nasil loglanacagini" bilmez.
package eventlog

import (
	"context"
	"log/slog"
)

// Bu sabitler Loki'de "event" ETIKETININ alacagi degerlerdir. Az sayida ve
// sabit oldugu icin etiket olmaya uygunlar (bkz. internal/logging/loki.go).
const (
	EventSeatHeld      = "seat_held"
	EventSeatReleased  = "seat_released"
	EventSeatPurchased = "seat_purchased"
	EventOrderExpired  = "order_expired"
	EventPaymentFailed = "payment_failed"
)

// ReleaseReason: bir koltuk neden serbest kaldi.
type ReleaseReason string

const (
	ReasonUser         ReleaseReason = "user_cancelled" // kullanici kendi iptal etti
	ReasonHoldExpired  ReleaseReason = "hold_expired"   // 10dk hold suresi doldu, worker topladi
	ReasonOrderExpired ReleaseReason = "order_expired"  // siparis odenmeden 15dk gecti
)

// Logger, is olaylarini loglayan kucuk bir sarmalayici.
type Logger struct {
	log *slog.Logger
}

func New(base *slog.Logger) *Logger {
	return &Logger{log: base}
}

// SeatHeld: bir koltuk bir kullanici tarafindan gecici olarak tutuldu.
func (l *Logger) SeatHeld(ctx context.Context, eventID, seatID, userID string, expiresAt any) {
	l.log.LogAttrs(ctx, slog.LevelInfo, "koltuk tutuldu",
		slog.String("event", EventSeatHeld),
		slog.String("event_id", eventID),
		slog.String("seat_id", seatID),
		slog.String("user_id", userID),
		slog.Any("expires_at", expiresAt),
	)
}

// SeatReleased: bir koltuk serbest birakildi (kullanici iptal etti VEYA suresi doldu).
func (l *Logger) SeatReleased(ctx context.Context, eventID, seatID, userID string, reason ReleaseReason) {
	l.log.LogAttrs(ctx, slog.LevelInfo, "koltuk birakildi",
		slog.String("event", EventSeatReleased),
		slog.String("event_id", eventID),
		slog.String("seat_id", seatID),
		slog.String("user_id", userID),
		slog.String("reason", string(reason)),
	)
}

// SeatPurchased: bir koltuk basariyla satin alindi (odeme onaylandi).
func (l *Logger) SeatPurchased(ctx context.Context, eventID, seatID, userID, orderID string, amountCents int64, currency string) {
	l.log.LogAttrs(ctx, slog.LevelInfo, "koltuk satin alindi",
		slog.String("event", EventSeatPurchased),
		slog.String("event_id", eventID),
		slog.String("seat_id", seatID),
		slog.String("user_id", userID),
		slog.String("order_id", orderID),
		slog.Int64("amount_cents", amountCents),
		slog.String("currency", currency),
	)
}

// PaymentFailed: mock odeme sağlayıcısı bir denemeyi reddetti.
func (l *Logger) PaymentFailed(ctx context.Context, orderID, userID string, amountCents int64, currency string) {
	l.log.LogAttrs(ctx, slog.LevelInfo, "odeme reddedildi",
		slog.String("event", EventPaymentFailed),
		slog.String("order_id", orderID),
		slog.String("user_id", userID),
		slog.Int64("amount_cents", amountCents),
		slog.String("currency", currency),
	)
}

// OrderExpired: bir siparis odenmeden suresi doldu, iptal edildi.
func (l *Logger) OrderExpired(ctx context.Context, orderID, userID string, seatCount int) {
	l.log.LogAttrs(ctx, slog.LevelInfo, "siparis suresi doldu",
		slog.String("event", EventOrderExpired),
		slog.String("order_id", orderID),
		slog.String("user_id", userID),
		slog.Int("seat_count", seatCount),
	)
}
