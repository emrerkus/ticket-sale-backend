package domain

import "time"

// SeatRef: siparise/hold'a konu olan bir koltuk (event + seat ikilisi).
type SeatRef struct {
	EventID string `json:"event_id"`
	SeatID  string `json:"seat_id"`
}

type OrderItem struct {
	ID             string `json:"id"`
	EventSeatID    string `json:"event_seat_id"`
	Section        string `json:"section"`
	RowLabel       string `json:"row_label"`
	SeatNumber     string `json:"seat_number"`
	UnitPriceCents int64  `json:"unit_price_cents"`
}

type Order struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Status     string    `json:"status"`
	TotalCents int64     `json:"total_cents"`
	Currency   string    `json:"currency"`
	ExpiresAt  time.Time `json:"expires_at"`
	CreatedAt  time.Time `json:"created_at"`
}

type OrderDetail struct {
	Order
	Items   []OrderItem `json:"items"`
	Payment *Payment    `json:"payment,omitempty"`
}

type Payment struct {
	ID          string    `json:"id"`
	OrderID     string    `json:"order_id"`
	Provider    string    `json:"provider"`
	ProviderRef string    `json:"provider_ref"`
	AmountCents int64     `json:"amount_cents"`
	Currency    string    `json:"currency"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}
