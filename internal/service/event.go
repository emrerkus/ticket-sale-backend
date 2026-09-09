// Package service is kurallarinin yasadigi yer. Handler HTTP bilir, repository
// SQL bilir; service ikisini de bilmeden "bir etkinligi getirmek ne demek"
// sorusunu cevaplar (gecerli id mi? public'e gorunur mu? parcalari nasil birlestirilir?).
package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/emrerkus/ticket-sale-backend/internal/domain"
	"github.com/emrerkus/ticket-sale-backend/internal/repository"
)

// Handler'in errors.Is ile tanıyacagi iki sentinel hata. repository.ErrNotFound'u
// DOGRUDAN disariya sizdirmiyoruz -- handler service'in ic detaylarini (hangi
// katman neyi nasil sakliyor) bilmemeli, sadece "gecersiz istek mi, yok mu, sunucu hatasi mi" bilmeli.
var (
	ErrInvalidID = errors.New("service: gecersiz id")
	ErrNotFound  = errors.New("service: kayit bulunamadi")
)

type EventService struct {
	repo *repository.EventRepository
}

func NewEventService(repo *repository.EventRepository) *EventService {
	return &EventService{repo: repo}
}

// List: public'e acik, yayindaki ve yaklasan etkinlikler.
func (s *EventService) List(ctx context.Context) ([]domain.EventListItem, error) {
	return s.repo.ListPublishedUpcoming(ctx)
}

// Get: TEK bir etkinligin tum detayini (mekan + fiyatlar + koltuk haritasi)
// birlestirip doner. Sadece "published" durumundaki etkinlikler public'e gorunur;
// draft/cancelled icin de ErrNotFound donuyoruz -- 403 degil 404, cunku "boyle
// bir kaynak yok" demek, "var ama yetkin yok" demekten daha az bilgi sizdirir.
//
// id once burada UUID olarak dogrulanir: bozuk bir metni Postgres'e gonderirsek
// "invalid input syntax for type uuid" hatasi aliriz ve bunu 500 sanabiliriz.
func (s *EventService) Get(ctx context.Context, id string) (*domain.EventDetail, error) {
	// 1. UUID Doğrulaması (Fail-Fast)
	// Hatalı veya rastgele bir metni DB'ye gönderip 500 hatasına sebep olmamak için.
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidID
	}

	// 2. Veritabanından temel etkinliği ve mekanı çekme
	event, venue, err := s.repo.GetByID(ctx, id)
	if err != nil {
		// Repository katmanının hatasını, servis katmanının hatasına (ErrNotFound) çevirerek sızmayı önlüyoruz.
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		// Beklenmeyen bir DB hatasıysa (bağlantı kopması vb.) yukarı taşı.
		return nil, err
	}

	// 3. İş Kuralı (Business Logic) İzolasyonu
	// Etkinlik var olsa bile yayında değilse 404 (Bulunamadı) dön.
	if event.Status != "published" {
		return nil, ErrNotFound
	}

	// 4. Bilet tiplerini getir
	ticketTypes, err := s.repo.ListTicketTypes(ctx, id)
	if err != nil {
		return nil, err
	}

	// 5. Koltuk haritasını ve envanter (event_seats) durumunu getir
	seatMap, err := s.repo.SeatMap(ctx, id)
	if err != nil {
		return nil, err
	}

	// 6. Aggregate Root (Veri Toplama): Tüm parçaları tek bir tutarlı yapıda birleştirme
	detail := domain.EventDetail{
		Event:       event,
		Venue:       venue,
		TicketTypes: ticketTypes,
		SeatMap:     seatMap,
	}

	return &detail, nil
}
