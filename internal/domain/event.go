// Package domain uygulamanin is nesnelerini (varliklarini) tutar.
//
// Bu tipler HICBIR katmana ozel degil: repository bunlari doldurur, service
// bunlar uzerinde is kurali uygular, handler bunlari JSON'a cevirir. Boylece
// "bir Event nedir" sorusunun cevabi tek bir yerde durur.
package domain

import "time"

// Venue: bir etkinligin yapildigi fiziksel mekan.
type Venue struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

// Event: veritabanindaki events satirinin bire bir karsiligi.
type Event struct {
	ID           string    `json:"id"`
	VenueID      string    `json:"venue_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	StartsAt     time.Time `json:"starts_at"`
	SalesStartAt time.Time `json:"sales_start_at"`
	SalesEndAt   time.Time `json:"sales_end_at"`
	Status       string    `json:"status"`
}

// EventListItem: GET /events icin. Event'in tum alanlarini "gomer" (embed) —
// yani JSON'a cevrilirken Event'in alanlari (id, title, ...) DIREKT ust
// seviyede gorunur, ayrica "venue_name" eklenir. Liste ekraninda koltuk
// haritasi gibi agir veriye gerek yok, sadece mekan adi yeter.
type EventListItem struct {
	Event
	VenueName string `json:"venue_name"`
}

// TicketType: bir etkinligin fiyat kategorisi.
type TicketType struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PriceCents int64  `json:"price_cents"`
	Currency   string `json:"currency"`
}

// SeatMapEntry: koltuk haritasindaki TEK bir koltugun anlik durumu.
type SeatMapEntry struct {
	SeatID       string `json:"seat_id"`
	Section      string `json:"section"`
	RowLabel     string `json:"row_label"`
	SeatNumber   string `json:"seat_number"`
	TicketTypeID string `json:"ticket_type_id"`
	Status       string `json:"status"`
}

// EventDetail: GET /events/{id} icin. Event + mekan + fiyatlar + TUM koltuk haritasi.
type EventDetail struct {
	Event
	Venue       Venue          `json:"venue"`
	TicketTypes []TicketType   `json:"ticket_types"`
	SeatMap     []SeatMapEntry `json:"seat_map"`
}

// SeatHold: bir koltugun basarili "hold" isleminin sonucu.
type SeatHold struct {
	EventID   string    `json:"event_id"`
	SeatID    string    `json:"seat_id"`
	HolderID  string    `json:"holder_id"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
}
